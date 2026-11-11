package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/health"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/store"
	"uptime-app/backend/internal/upload"
)

const testPassword = "  correct password with spaces  "

type harness struct {
	api       *API
	db        *store.Store
	clock     *atomic.Int64
	avatarDir string
}

func integration(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is unset: PostgreSQL integration tests skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	schema := "auth_test_" + hex.EncodeToString(b)
	if _, err := admin.SQL.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.SQL.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.SQL.Close()
		if _, err := admin.SQL.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.SQL.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	clock := new(atomic.Int64)
	clock.Store(time.Now().UTC().Truncate(time.Second).UnixNano())
	now := func() time.Time { return time.Unix(0, clock.Load()).UTC() }
	tokens := &auth.Tokens{Secret: []byte(strings.Repeat("k", 32)), Issuer: "test-api", Audience: "test-web", Now: now}
	service, err := auth.New(db, tokens)
	if err != nil {
		t.Fatal(err)
	}
	service.Now = now
	avatarDir := t.TempDir()
	avatars, err := auth.NewAvatarStorage(avatarDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := upload.NewStorage(t.TempDir(), "/uploads/files/")
	if err != nil {
		t.Fatal(err)
	}
	api := New(auth.NewHandler(service, false, avatars), health.NewHandler(db.SQL), upload.NewHandler(files), monitor.NewHandler(&monitor.Service{Store: db}), config.Config{LocalHTTP: true, AllowedOrigins: []string{"http://localhost:3000"}})
	api.now = now
	return &harness{api: api, db: db, clock: clock, avatarDir: avatarDir}
}

func (h *harness) do(method, route, body, refresh, access string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/auth/"+route, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	if refresh != "" {
		r.AddCookie(&http.Cookie{Name: "refresh_token", Value: refresh})
	}
	if access != "" {
		r.Header.Set("Authorization", "Bearer "+access)
	}
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	return w
}

func credentials(email, password string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(b)
}

func result(t *testing.T, w *httptest.ResponseRecorder, status int) (auth.Result, *http.Cookie) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want %d got %d: %s", status, w.Code, w.Body)
	}
	var r auth.Result
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &shape); err != nil {
		t.Fatal(err)
	}
	if len(shape) != 4 || r.TokenType != "Bearer" || r.ExpiresIn != 900 || r.AccessToken == "" || r.User.ID == 0 {
		t.Fatalf("response contract: %s", w.Body)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "refresh_token" {
			if !auth.ValidRefresh(cookie.Value) || !cookie.HttpOnly || cookie.MaxAge < 1 {
				t.Fatalf("refresh cookie: %+v", cookie)
			}
			if strings.Contains(w.Body.String(), cookie.Value) || strings.Contains(w.Body.String(), "hash") {
				t.Fatal("secret leaked in response")
			}
			return r, cookie
		}
	}
	t.Fatal("refresh cookie missing")
	return r, nil
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want %d got %d: %s", status, w.Code, w.Body)
	}
}

func TestIntegrationHealthReadiness(t *testing.T) {
	h := integration(t)
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	expect(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("readiness response: %s", w.Body)
	}
	if err := h.db.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	expect(t, w, http.StatusServiceUnavailable)
	if !strings.Contains(w.Body.String(), `"code":"unavailable"`) {
		t.Fatalf("database failure response: %s", w.Body)
	}
}

func TestIntegrationRegistrationAndMigrations(t *testing.T) {
	h := integration(t)
	r, cookie := result(t, h.do("POST", "register", credentials("  USER@Example.COM  ", testPassword), "", ""), 201)
	if r.User.Email != "user@example.com" {
		t.Fatal("email not normalized")
	}
	expect(t, h.do("POST", "register", credentials("user@example.com", testPassword), "", ""), 409)
	var hash, tokenHash string
	var created, expires time.Time
	if err := h.db.SQL.QueryRow("SELECT password_hash FROM users WHERE id=$1", r.User.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(testPassword, hash) || auth.VerifyPassword(strings.TrimSpace(testPassword), hash) {
		t.Fatal("password changed")
	}
	if err := h.db.SQL.QueryRow("SELECT token_hash, auth_sessions.created_at, expires_at FROM refresh_tokens JOIN auth_sessions ON auth_sessions.id=refresh_tokens.session_id").Scan(&tokenHash, &created, &expires); err != nil {
		t.Fatal(err)
	}
	if tokenHash != auth.RefreshHash(cookie.Value) || expires.Sub(created) != 24*time.Hour {
		t.Fatal("hash storage or session TTL")
	}
	var count int
	if err := h.db.SQL.QueryRow("SELECT count(*) FROM goose_db_version WHERE version_id=1 AND is_applied").Scan(&count); err != nil || count != 1 {
		t.Fatalf("migration count: %d %v", count, err)
	}
	if err := h.db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	expect(t, h.do("GET", "me", "", "", r.AccessToken), 200)
}

func TestIntegrationConcurrentRegistration(t *testing.T) {
	h := integration(t)
	responses := make(chan *httptest.ResponseRecorder, 4)
	start := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			<-start
			responses <- h.do("POST", "register", credentials("same@example.com", testPassword), "", "")
		}()
	}
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 4; i++ {
		w := <-responses
		switch w.Code {
		case 201:
			success++
		case 409:
			conflict++
		default:
			t.Fatalf("concurrent register: %d %s", w.Code, w.Body)
		}
	}
	var users, sessions, tokens int
	if err := h.db.SQL.QueryRow("SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM auth_sessions), (SELECT count(*) FROM refresh_tokens)").Scan(&users, &sessions, &tokens); err != nil {
		t.Fatal(err)
	}
	if success != 1 || conflict != 3 || users != 1 || sessions != 1 || tokens != 1 {
		t.Fatalf("counts: %d %d %d %d %d", success, conflict, users, sessions, tokens)
	}
}

func TestIntegrationPublicUserTimePrecision(t *testing.T) {
	h := integration(t)
	h.clock.Add(123456789)
	registered, _ := result(t, h.do("POST", "register", credentials("precision@example.com", testPassword), "", ""), 201)
	w := h.do("GET", "me", "", "", registered.AccessToken)
	expect(t, w, 200)
	var me auth.User
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.ID != registered.User.ID || me.Email != registered.User.Email || !me.CreatedAt.Equal(registered.User.CreatedAt) {
		t.Fatal("registration and me return different public user data")
	}
}

func TestIntegrationLoginAndMe(t *testing.T) {
	h := integration(t)
	registered, _ := result(t, h.do("POST", "register", credentials("me@example.com", testPassword), "", ""), 201)
	wrong := h.do("POST", "login", credentials("me@example.com", "incorrect password here"), "", "")
	missing := h.do("POST", "login", credentials("missing@example.com", "incorrect password here"), "", "")
	expect(t, wrong, 401)
	expect(t, missing, 401)
	if wrong.Body.String() != missing.Body.String() {
		t.Fatal("login reveals email existence")
	}
	expect(t, h.do("POST", "login", credentials("me@example.com", strings.TrimSpace(testPassword)), "", ""), 401)
	logged, cookie := result(t, h.do("POST", "login", credentials(" ME@Example.COM ", testPassword), "", ""), 200)
	if registered.User.ID != logged.User.ID || cookie.Value == "" {
		t.Fatal("login identity")
	}
	w := h.do("GET", "me", "", "", logged.AccessToken)
	expect(t, w, 200)
	var user map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil || len(user) != 5 || user["email"] != "me@example.com" || user["name"] != "" || user["avatar_url"] != "" {
		t.Fatalf("public user: %s", w.Body)
	}
	expect(t, h.do("GET", "me", "", "", ""), 401)
	expect(t, h.do("GET", "me", "", "", "broken.jwt.token"), 401)
	h.clock.Add(int64(auth.AccessTTL))
	expect(t, h.do("GET", "me", "", "", logged.AccessToken), 401)
}

func TestIntegrationRefreshReuseAndIndependentDevices(t *testing.T) {
	h := integration(t)
	_, a := result(t, h.do("POST", "register", credentials("devices@example.com", testPassword), "", ""), 201)
	_, b := result(t, h.do("POST", "login", credentials("devices@example.com", testPassword), "", ""), 200)
	h.clock.Add(int64(time.Hour))
	_, rotated := result(t, h.do("POST", "refresh", `{}`, a.Value, ""), 200)
	if rotated.Value == a.Value || !rotated.Expires.Equal(a.Expires) || rotated.MaxAge != 23*60*60 {
		t.Fatal("refresh extended session or reused token")
	}
	expect(t, h.do("POST", "refresh", `{}`, a.Value, ""), 401)
	expect(t, h.do("POST", "refresh", `{}`, rotated.Value, ""), 401)
	var revoked int
	if err := h.db.SQL.QueryRow("SELECT count(*) FROM auth_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked); err != nil || revoked != 1 {
		t.Fatalf("revocation not committed: %d %v", revoked, err)
	}
	_, second := result(t, h.do("POST", "refresh", `{}`, b.Value, ""), 200)
	expect(t, h.do("POST", "logout", `{}`, rotated.Value, ""), 204)
	result(t, h.do("POST", "refresh", `{}`, second.Value, ""), 200)
}

func TestIntegrationConcurrentRefresh(t *testing.T) {
	h := integration(t)
	_, cookie := result(t, h.do("POST", "register", credentials("race@example.com", testPassword), "", ""), 201)
	responses := make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-start; responses <- h.do("POST", "refresh", `{}`, cookie.Value, "") }()
	}
	close(start)
	success, denied := 0, 0
	var next *http.Cookie
	for i := 0; i < 2; i++ {
		w := <-responses
		switch w.Code {
		case 200:
			success++
			_, next = result(t, w, 200)
		case 401:
			denied++
		default:
			t.Fatalf("refresh race: %d %s", w.Code, w.Body)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatalf("refresh race: %d success, %d denied", success, denied)
	}
	expect(t, h.do("POST", "refresh", `{}`, next.Value, ""), 401)
	var revoked bool
	if err := h.db.SQL.QueryRow("SELECT revoked_at IS NOT NULL FROM auth_sessions").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revocation rolled back: %v %v", revoked, err)
	}
	var tokens, used int
	if err := h.db.SQL.QueryRow("SELECT count(*), count(used_at) FROM refresh_tokens").Scan(&tokens, &used); err != nil || tokens != 2 || used != 1 {
		t.Fatalf("token history: %d %d %v", tokens, used, err)
	}
}

func TestIntegrationFixedSessionExpiry(t *testing.T) {
	h := integration(t)
	_, cookie := result(t, h.do("POST", "register", credentials("expiry@example.com", testPassword), "", ""), 201)
	h.clock.Add(int64(23 * time.Hour))
	_, rotated := result(t, h.do("POST", "refresh", `{}`, cookie.Value, ""), 200)
	if rotated.MaxAge != 3600 || !rotated.Expires.Equal(cookie.Expires) {
		t.Fatal("session expiry moved")
	}
	h.clock.Add(int64(time.Hour))
	expect(t, h.do("POST", "refresh", `{}`, rotated.Value, ""), 401)
}

func TestIntegrationLogout(t *testing.T) {
	h := integration(t)
	r, cookie := result(t, h.do("POST", "register", credentials("logout@example.com", testPassword), "", ""), 201)
	w := h.do("POST", "logout", `{}`, cookie.Value, "")
	expect(t, w, 204)
	deleted := w.Result().Cookies()[0]
	if deleted.MaxAge != -1 || deleted.Value != "" || deleted.Path != cookie.Path || deleted.Domain != cookie.Domain {
		t.Fatal("logout did not delete cookie")
	}
	for _, raw := range []string{cookie.Value, "", "invalid"} {
		expect(t, h.do("POST", "logout", `{}`, raw, ""), 204)
	}
	expect(t, h.do("POST", "refresh", `{}`, cookie.Value, ""), 401)
	expect(t, h.do("GET", "me", "", "", r.AccessToken), 200)
	h.clock.Add(int64(auth.AccessTTL))
	expect(t, h.do("GET", "me", "", "", r.AccessToken), 401)
}

func TestIntegrationRegistrationRollback(t *testing.T) {
	h := integration(t)
	// Ошибка создания токена должна откатить и пользователя, и сессию.
	user := store.User{Email: "rollback@example.com", PasswordHash: "test", CreatedAt: time.Now()}
	session := store.AuthSession{ID: "rollback", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	token := store.RefreshToken{TokenHash: "violates_hash_length", CreatedAt: time.Now()}
	if err := h.db.CreateAccount(context.Background(), &user, &session, &token); err == nil {
		t.Fatal("constraint failure expected")
	}
	var count int
	if err := h.db.SQL.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 0 {
		t.Fatal(fmt.Sprintf("registration was not atomic: %d %v", count, err))
	}
}
