package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var (
	ErrUnauthorized = errors.New("invalid credentials or session")
	ErrEmailTaken   = errors.New("email already registered")
)

// Модели хранилища никогда не используются как HTTP-ответы.
type User struct {
	ID           uint64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type AuthSession struct {
	ID        string
	UserID    uint64
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshToken struct {
	ID        uint64
	SessionID string
	TokenHash string
	CreatedAt time.Time
	UsedAt    *time.Time
}

type Store struct {
	db  *gorm.DB
	SQL *sql.DB
}

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true, Logger: logger.Default.LogMode(logger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(time.Hour)
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{db: db, SQL: pool}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, s.SQL, files)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	return user, unauthorizedIfMissing(err)
}

func (s *Store) UserByID(ctx context.Context, id uint64) (User, error) {
	var user User
	err := s.db.WithContext(ctx).First(&user, id).Error
	return user, unauthorizedIfMissing(err)
}

func (s *Store) CreateAccount(ctx context.Context, user *User, session *AuthSession, token *RefreshToken) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrEmailTaken
			}
			return err
		}
		session.UserID = user.ID
		return createSession(tx, session, token)
	})
}

func (s *Store) CreateSession(ctx context.Context, session *AuthSession, token *RefreshToken) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createSession(tx, session, token)
	})
}

func createSession(tx *gorm.DB, session *AuthSession, token *RefreshToken) error {
	if err := tx.Create(session).Error; err != nil {
		return err
	}
	token.SessionID = session.ID
	return tx.Create(token).Error
}

func (s *Store) Rotate(ctx context.Context, hash, nextHash string, now func() time.Time) (User, AuthSession, error) {
	var user User
	var session AuthSession
	denied := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token RefreshToken
		if err := tx.Where("token_hash = ?", hash).First(&token).Error; err != nil {
			return unauthorizedIfMissing(err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", token.SessionID).Error; err != nil {
			return unauthorizedIfMissing(err)
		}
		// После ожидания блокировки заново читаем токен и текущее время.
		if err := tx.First(&token, token.ID).Error; err != nil {
			return err
		}
		t := now().UTC()
		if session.RevokedAt != nil || !t.Before(session.ExpiresAt) {
			return ErrUnauthorized
		}
		if token.UsedAt != nil {
			denied = true
			// nil фиксирует отзыв. ErrUnauthorized возвращается только после коммита.
			return tx.Model(&session).Update("revoked_at", t).Error
		}
		if err := tx.Model(&token).Update("used_at", t).Error; err != nil {
			return err
		}
		if err := tx.Create(&RefreshToken{SessionID: session.ID, TokenHash: nextHash, CreatedAt: t}).Error; err != nil {
			return err
		}
		return tx.First(&user, session.UserID).Error
	})
	if err == nil && denied {
		err = ErrUnauthorized
	}
	return user, session, err
}

func (s *Store) Revoke(ctx context.Context, hash string, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token RefreshToken
		if err := tx.Where("token_hash = ?", hash).First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var session AuthSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", token.SessionID).Error; err != nil {
			return err
		}
		if session.RevokedAt != nil {
			return nil
		}
		return tx.Model(&session).Update("revoked_at", now.UTC()).Error
	})
}

func unauthorizedIfMissing(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUnauthorized
	}
	return err
}
