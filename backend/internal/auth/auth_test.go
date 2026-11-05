package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidation(t *testing.T) {
	for _, n := range []int{8, 128} {
		if email, err := Validate("  User@Example.COM  ", strings.Repeat("ж", n)); err != nil || email != "user@example.com" {
			t.Fatalf("boundary %d: %s, %v", n, email, err)
		}
	}
	for _, password := range []string{strings.Repeat("a", 7), strings.Repeat("a", 129), "bad\xffencoding"} {
		if _, err := Validate("a@example.com", password); err == nil {
			t.Fatal("bad password accepted")
		}
	}
	for _, email := range []string{"", "invalid", "Name <a@example.com>", "a@example.com\r\nInjected: true"} {
		if _, err := Validate(email, strings.Repeat("a", 15)); err == nil {
			t.Fatal("bad email accepted")
		}
	}
}

func TestPassword(t *testing.T) {
	password := "  Длинный пароль без обрезки  "
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if hash == second || !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatal("salt or parameters")
	}
	if !VerifyPassword(password, hash) || VerifyPassword(strings.TrimSpace(password), hash) || VerifyPassword("wrong password", hash) {
		t.Fatal("password changed or verification failed")
	}
	for _, malformed := range []string{"", "$argon2id$v=19$m=999999,t=2,p=1$a$b", strings.Replace(hash, "v=19", "v=18", 1)} {
		if VerifyPassword(password, malformed) {
			t.Fatal("malformed hash accepted")
		}
	}
}

func TestJWT(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	tokens := &Tokens{Secret: []byte(strings.Repeat("k", 32)), Issuer: "api", Audience: "web", Now: func() time.Time { return now }}
	raw, err := tokens.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := tokens.Parse(raw); err != nil || id != 42 {
		t.Fatalf("valid token: %d %v", id, err)
	}
	claims := jwt.RegisteredClaims{Subject: "42", Issuer: "api", Audience: jwt.ClaimStrings{"web"},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL))}
	for _, tc := range []struct {
		name   string
		change func(*jwt.RegisteredClaims)
		method jwt.SigningMethod
		key    []byte
	}{
		{"algorithm", func(*jwt.RegisteredClaims) {}, jwt.SigningMethodHS384, tokens.Secret},
		{"signature", func(*jwt.RegisteredClaims) {}, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32))},
		{"issuer", func(c *jwt.RegisteredClaims) { c.Issuer = "other" }, jwt.SigningMethodHS256, tokens.Secret},
		{"audience", func(c *jwt.RegisteredClaims) { c.Audience = jwt.ClaimStrings{"other"} }, jwt.SigningMethodHS256, tokens.Secret},
		{"expired", func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(now) }, jwt.SigningMethodHS256, tokens.Secret},
		{"missing expiry", func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, tokens.Secret},
		{"missing subject", func(c *jwt.RegisteredClaims) { c.Subject = "" }, jwt.SigningMethodHS256, tokens.Secret},
		{"future issuance", func(c *jwt.RegisteredClaims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Minute)) }, jwt.SigningMethodHS256, tokens.Secret},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := claims
			tc.change(&copy)
			bad, err := jwt.NewWithClaims(tc.method, copy).SignedString(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Parse(bad); err == nil {
				t.Fatal("invalid JWT accepted")
			}
		})
	}
	now = now.Add(AccessTTL)
	if _, err := tokens.Parse(raw); err == nil {
		t.Fatal("JWT still valid at 15 minutes")
	}
}

func TestRefreshTokens(t *testing.T) {
	a, err := RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := RandomToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) != 43 || !ValidRefresh(a) || ValidRefresh("invalid") || len(RefreshHash(a)) != 64 || RefreshHash(a) == RefreshHash(b) {
		t.Fatal("refresh randomness, format or hashing")
	}
}
