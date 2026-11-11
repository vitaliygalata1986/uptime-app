package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
)

func avatarImage(t *testing.T, format string, width int) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, 2))
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&b, img, nil)
	} else {
		err = png.Encode(&b, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func (h *harness) upload(t *testing.T, token string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("avatar", "../../untrusted.svg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/auth/avatar", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", "http://localhost:3000")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	return w
}

func (h *harness) avatarFile(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func TestIntegrationAvatar(t *testing.T) {
	h := integration(t)
	account, cookie := result(t, h.do("POST", "register", credentials("avatar@example.com", testPassword), "", ""), 201)
	other, _ := result(t, h.do("POST", "register", credentials("other@example.com", testPassword), "", ""), 201)
	img := avatarImage(t, "png", 2)
	expect(t, h.upload(t, "", img, nil), 401)
	expect(t, h.upload(t, "invalid", img, nil), 401)
	w := h.upload(t, account.AccessToken, img, map[string]string{"name": "  Аватар 🙂  "})
	expect(t, w, 200)
	var user auth.User
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	if user.AvatarURL == "" || !strings.HasSuffix(user.AvatarURL, ".png") || user.Name != "Аватар 🙂" || user.ID != account.User.ID {
		t.Fatalf("unexpected user: %+v", user)
	}
	stored, err := h.db.UserByID(context.Background(), user.ID)
	if err != nil || stored.AvatarURL != user.AvatarURL || stored.Name != user.Name {
		t.Fatalf("not persisted: %+v %v", stored, err)
	}
	untouched, err := h.db.UserByID(context.Background(), other.User.ID)
	if err != nil || untouched.AvatarURL != "" {
		t.Fatalf("other account changed: %+v %v", untouched, err)
	}
	served := h.avatarFile(user.AvatarURL)
	expect(t, served, 200)
	if served.Header().Get("Content-Type") != "image/png" || served.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(served.Header())
	}
	if _, err := png.Decode(served.Body); err != nil {
		t.Fatal(err)
	}
	var me auth.User
	w = h.do("GET", "me", "", "", account.AccessToken)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || me != user {
		t.Fatalf("me: %s %v", w.Body, err)
	}
	refreshed, _ := result(t, h.do("POST", "refresh", `{}`, cookie.Value, ""), 200)
	logged, _ := result(t, h.do("POST", "login", credentials(account.User.Email, testPassword), "", ""), 200)
	if refreshed.User != user || logged.User != user {
		t.Fatal("auth response lost avatar")
	}
	w = h.upload(t, account.AccessToken, avatarImage(t, "jpeg", 3), nil)
	expect(t, w, 200)
	var replaced auth.User
	if err := json.Unmarshal(w.Body.Bytes(), &replaced); err != nil {
		t.Fatal(err)
	}
	if replaced.Name != user.Name || replaced.AvatarURL == user.AvatarURL || !strings.HasSuffix(replaced.AvatarURL, ".jpg") {
		t.Fatalf("replacement: %+v", replaced)
	}
	expect(t, h.avatarFile(user.AvatarURL), 404)
	served = h.avatarFile(replaced.AvatarURL)
	expect(t, served, 200)
	if _, err := jpeg.Decode(served.Body); err != nil {
		t.Fatal(err)
	}
	expect(t, h.do("PATCH", "profile", `{"name":"Name only"}`, "", account.AccessToken), 200)
	stored, err = h.db.UserByID(context.Background(), user.ID)
	if err != nil || stored.AvatarURL != replaced.AvatarURL {
		t.Fatal("name change lost avatar", err)
	}
	h.clock.Add(int64(16 * time.Minute))
	expect(t, h.upload(t, account.AccessToken, img, nil), 401)
}

func TestIntegrationAvatarValidation(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("avatar-validation@example.com", testPassword), "", ""), 201)
	img := avatarImage(t, "png", 2)
	tests := []struct {
		name   string
		data   []byte
		fields map[string]string
		status int
	}{
		{"svg", []byte(`<svg onload="alert(1)"/>`), nil, 400},
		{"truncated", img[:len(img)-8], nil, 400},
		{"dimensions", avatarImage(t, "png", 4097), nil, 400},
		{"file limit", bytes.Repeat([]byte("a"), auth.MaxAvatarBytes+1), nil, 413},
		{"body limit", bytes.Repeat([]byte("a"), auth.MaxAvatarBytes+(128<<10)), nil, 413},
		{"invalid name", img, map[string]string{"name": " "}, 400},
		{"foreign id", img, map[string]string{"id": "2"}, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expect(t, h.upload(t, account.AccessToken, tc.data, tc.fields), tc.status)
		})
	}
	stored, err := h.db.UserByID(context.Background(), account.User.ID)
	if err != nil || stored.AvatarURL != "" || stored.Name != "" {
		t.Fatalf("invalid save changed profile: %+v %v", stored, err)
	}
	expect(t, h.avatarFile("/uploads/avatars/"), 404)
	expect(t, h.avatarFile("/uploads/avatars/not-an-avatar.svg"), 404)
	// Multipart upload has its own preflight; Origin and content-type guards remain active.
	r := httptest.NewRequest("OPTIONS", "/api/auth/avatar", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	expect(t, w, 204)
	expect(t, h.do("POST", "avatar", `{}`, "", account.AccessToken), 400)
}

func TestIntegrationAvatarMigrationPreservesUsers(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("legacy-avatar@example.com", testPassword), "", ""), 201)
	expect(t, h.do("PATCH", "profile", `{"name":"Existing user"}`, "", account.AccessToken), 200)
	before, err := h.db.UserByID(context.Background(), account.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.SQL.Exec("DROP TABLE monitors; ALTER TABLE users DROP COLUMN avatar_url"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.SQL.Exec("DELETE FROM goose_db_version WHERE version_id >= 3"); err != nil {
		t.Fatal(err)
	}
	if err := h.db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := h.db.UserByID(context.Background(), account.User.ID)
	if err != nil || before != after {
		t.Fatalf("migration changed user: %+v %v", after, err)
	}
}

func TestIntegrationAvatarDatabaseFailureCleansNewFile(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("avatar-rollback@example.com", testPassword), "", ""), 201)
	if _, err := h.db.SQL.Exec("ALTER TABLE users ADD CONSTRAINT reject_avatar CHECK (avatar_url = '')"); err != nil {
		t.Fatal(err)
	}
	expect(t, h.upload(t, account.AccessToken, avatarImage(t, "png", 2), map[string]string{"name": "Must roll back"}), 500)
	files, err := os.ReadDir(h.avatarDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("failed save left files: %v %v", files, err)
	}
	user, err := h.db.UserByID(context.Background(), account.User.ID)
	if err != nil || user.Name != "" || user.AvatarURL != "" {
		t.Fatalf("failed save changed user: %+v %v", user, err)
	}
}
