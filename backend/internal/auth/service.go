package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"uptime-app/backend/internal/store"
)

var ErrInvalidInput = errors.New("valid email and password of 8–128 characters required")

type User struct {
	ID        uint64    `json:"id"`
	Email     string    `json:"email"`
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

func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	email, err := Validate(email, password)
	if err != nil {
		return Result{}, err
	}
	user, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrUnauthorized) {
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

func (s *Service) Logout(ctx context.Context, raw string) error {
	if !ValidRefresh(raw) {
		return nil
	}
	return s.Store.Revoke(ctx, RefreshHash(raw), s.Now())
}

func (s *Service) Me(ctx context.Context, raw string) (User, error) {
	id, err := s.Tokens.Parse(raw)
	if err != nil {
		return User{}, store.ErrUnauthorized
	}
	user, err := s.Store.UserByID(ctx, id)
	return publicUser(user), err
}

func (s *Service) result(user store.User, refresh string, expires time.Time) (Result, error) {
	access, err := s.Tokens.Issue(user.ID)
	return Result{User: publicUser(user), AccessToken: access, TokenType: "Bearer",
		ExpiresIn: int(AccessTTL.Seconds()), Refresh: refresh, ExpiresAt: expires}, err
}

func publicUser(user store.User) User {
	return User{ID: user.ID, Email: user.Email, CreatedAt: user.CreatedAt}
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
