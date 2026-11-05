package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTTL  = 15 * time.Minute
	SessionTTL = 24 * time.Hour
)

type Tokens struct {
	Secret   []byte
	Issuer   string
	Audience string
	Now      func() time.Time
}

func (t *Tokens) Issue(userID uint64) (string, error) {
	now := t.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject: strconv.FormatUint(userID, 10), Issuer: t.Issuer,
		Audience: jwt.ClaimStrings{t.Audience}, IssuedAt: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.Secret)
}

func (t *Tokens) Parse(raw string) (uint64, error) {
	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		return t.Secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(),
		jwt.WithIssuer(t.Issuer), jwt.WithAudience(t.Audience), jwt.WithIssuedAt(), jwt.WithTimeFunc(t.Now))
	if err != nil || !token.Valid {
		return 0, errors.New("invalid access token")
	}
	id, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid token subject")
	}
	return id, nil
}

func RandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ValidRefresh(raw string) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	return err == nil && len(b) == 32
}

func RefreshHash(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
