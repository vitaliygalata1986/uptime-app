package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"uptime-app/backend/internal/store"
)

var ErrInvalidInput = errors.New("valid email and password of 8–128 characters required")
var ErrInvalidName = errors.New("name must contain 1–100 characters without control characters")

type User struct {
	ID        uint64    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Result struct {
	User        User      `json:"user"`
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	Refresh     string    `json:"-"`
	ExpiresAt   time.Time `json:"-"`
}

type Service struct {
	Store     *store.Store
	Tokens    *Tokens
	Now       func() time.Time
	dummyHash string
}

// New prepares authentication, including a dummy password hash for unknown-account logins.
func New(repository *store.Store, tokens *Tokens) (*Service, error) {
	dummy, err := RandomToken()
	if err != nil {
		return nil, err
	}
	hash, err := HashPassword(dummy)
	if err != nil {
		return nil, err
	}
	return &Service{Store: repository, Tokens: tokens, Now: time.Now, dummyHash: hash}, nil
}

// Validate normalizes the email and checks UTF-8 credentials, counting password Unicode code points.
// The password itself is never trimmed or otherwise normalized.
func Validate(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	length := utf8.RuneCountInString(password)
	if !utf8.ValidString(email) || !utf8.ValidString(password) || len(email) > 254 || err != nil || address.Address != email ||
		length < 8 || length > 128 {
		return "", ErrInvalidInput
	}
	return email, nil
}

// Register creates an account and fixed-lifetime session, then issues its access JWT.
func (s *Service) Register(ctx context.Context, email, password string) (Result, error) {
	email, err := Validate(email, password)
	if err != nil {
		return Result{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Result{}, err
	}
	// PostgreSQL хранит TIMESTAMPTZ с точностью до микросекунд.
	// Ответ регистрации должен совпадать с последующим чтением пользователя.
	now := s.Now().UTC().Truncate(time.Microsecond)
	user := store.User{Email: email, PasswordHash: hash, CreatedAt: now}
	session, token, raw, err := newSession(now, 0)
	if err != nil {
		return Result{}, err
	}
	if err := s.Store.CreateAccount(ctx, &user, &session, &token); err != nil {
		return Result{}, err
	}
	return s.result(user, raw, session.ExpiresAt)
}

// Login verifies credentials and creates an independent session for this sign-in.
func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	email, err := Validate(email, password)
	if err != nil {
		return Result{}, err
	}
	user, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrUnauthorized) {
		// Выполняем дорогое хеширование и для неизвестного email, чтобы уменьшить разницу во времени ответа.
		VerifyPassword(password, s.dummyHash)
		return Result{}, store.ErrUnauthorized
	}
	if err != nil {
		return Result{}, err
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return Result{}, store.ErrUnauthorized
	}
	session, token, raw, err := newSession(s.Now().UTC(), user.ID)
	if err != nil {
		return Result{}, err
	}
	if err := s.Store.CreateSession(ctx, &session, &token); err != nil {
		return Result{}, err
	}
	return s.result(user, raw, session.ExpiresAt)
}

// Refresh rotates a refresh token without extending the session's original 24-hour lifetime.
func (s *Service) Refresh(ctx context.Context, raw string) (Result, error) {
	if !ValidRefresh(raw) {
		return Result{}, store.ErrUnauthorized
	}
	next, err := RandomToken()
	if err != nil {
		return Result{}, err
	}
	user, session, err := s.Store.Rotate(ctx, RefreshHash(raw), RefreshHash(next), s.Now)
	if err != nil {
		return Result{}, err
	}
	return s.result(user, next, session.ExpiresAt)
}

// Logout revokes the refresh session; issued access JWTs remain valid until their expiry.
func (s *Service) Logout(ctx context.Context, raw string) error {
	if !ValidRefresh(raw) {
		return nil
	}
	return s.Store.Revoke(ctx, RefreshHash(raw), s.Now())
}

// Me validates an access JWT and loads its current public account, without checking refresh state.
func (s *Service) Me(ctx context.Context, raw string) (User, error) {
	id, err := s.Tokens.Parse(raw)
	if err != nil {
		return User{}, store.ErrUnauthorized
	}
	user, err := s.Store.UserByID(ctx, id)
	return publicUser(user), err
}

// ValidateName trims surrounding whitespace and accepts 1–100 Unicode code points without controls.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	length := utf8.RuneCountInString(name)
	if !utf8.ValidString(name) || length < 1 || length > 100 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", ErrInvalidName
	}
	return name, nil
}

// UpdateProfile derives ownership from the access JWT and returns the saved, validated name.
func (s *Service) UpdateProfile(ctx context.Context, raw, name string) (User, error) {
	id, err := s.Tokens.Parse(raw)
	if err != nil {
		return User{}, store.ErrUnauthorized
	}
	name, err = ValidateName(name)
	if err != nil {
		return User{}, err
	}
	user, err := s.Store.UpdateUserName(ctx, id, name)
	return publicUser(user), err
}

func (s *Service) result(user store.User, refresh string, expires time.Time) (Result, error) {
	access, err := s.Tokens.Issue(user.ID)
	return Result{User: publicUser(user), AccessToken: access, TokenType: "Bearer",
		ExpiresIn: int(AccessTTL.Seconds()), Refresh: refresh, ExpiresAt: expires}, err
}

func publicUser(user store.User) User {
	return User{ID: user.ID, Email: user.Email, Name: user.Name, AvatarURL: user.AvatarURL, CreatedAt: user.CreatedAt}
}

func newSession(now time.Time, userID uint64) (store.AuthSession, store.RefreshToken, string, error) {
	id, err := RandomToken()
	if err != nil {
		return store.AuthSession{}, store.RefreshToken{}, "", err
	}
	raw, err := RandomToken()
	return store.AuthSession{ID: id, UserID: userID, CreatedAt: now, ExpiresAt: now.Add(SessionTTL)},
		store.RefreshToken{TokenHash: RefreshHash(raw), CreatedAt: now}, raw, err
}
