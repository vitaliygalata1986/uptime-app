package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
)

func TestIntegrationProfile(t *testing.T) {
	h := integration(t)
	first, cookie := result(t, h.do("POST", "register", credentials("profile@example.com", testPassword), "", ""), 201)
	second, _ := result(t, h.do("POST", "register", credentials("other@example.com", testPassword), "", ""), 201)
	if first.User.Name != "" {
		t.Fatal("new users should have an empty name")
	}

	expect(t, h.do("PATCH", "profile", `{"name":"Alice"}`, "", ""), 401)
	expect(t, h.do("PATCH", "profile", `{"name":"Alice"}`, "", "invalid"), 401)
	for _, body := range []string{`{}`, `{"name":null}`, `{"name":" "}`, `{"name":"a\nb"}`, `{"name":"Alice","id":2}`, `{"name":"Alice","email":"other@example.com"}`, `{"name":"Alice","created_at":"2020-01-01"}`, `{"name":"Alice"} {}`} {
		expect(t, h.do("PATCH", "profile", body, "", first.AccessToken), 400)
	}
	long, _ := json.Marshal(map[string]string{"name": strings.Repeat("я", 101)})
	expect(t, h.do("PATCH", "profile", string(long), "", first.AccessToken), 400)

	w := h.do("PATCH", "profile", `{"name":"  Анна Марія 🙂  "}`, "", first.AccessToken)
	expect(t, w, 200)
	var user auth.User
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	if user.Name != "Анна Марія 🙂" || user.ID != first.User.ID || user.Email != first.User.Email || !user.CreatedAt.Equal(first.User.CreatedAt) {
		t.Fatalf("unexpected profile: %+v", user)
	}
	if strings.Contains(w.Body.String(), "hash") {
		t.Fatal("private fields exposed")
	}
	stored, err := h.db.UserByID(context.Background(), first.User.ID)
	if err != nil || stored.Name != user.Name {
		t.Fatalf("name was not persisted: %+v %v", stored, err)
	}
	other, err := h.db.UserByID(context.Background(), second.User.ID)
	if err != nil || other.Name != "" {
		t.Fatalf("other user changed: %+v %v", other, err)
	}

	w = h.do("GET", "me", "", "", first.AccessToken)
	expect(t, w, http.StatusOK)
	var me auth.User
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || me != user {
		t.Fatalf("profile read: %s, %v", w.Body, err)
	}
	expect(t, h.do("PATCH", "profile", `{"name":"Анна Марія 🙂"}`, "", first.AccessToken), 200)
	refreshed, _ := result(t, h.do("POST", "refresh", `{}`, cookie.Value, ""), 200)
	if refreshed.User.Name != user.Name {
		t.Fatal("refresh lost name")
	}
	loggedIn, _ := result(t, h.do("POST", "login", credentials(first.User.Email, testPassword), "", ""), 200)
	if loggedIn.User.Name != user.Name {
		t.Fatal("login lost name")
	}
	h.clock.Add(int64(16 * time.Minute))
	expect(t, h.do("PATCH", "profile", `{"name":"Expired"}`, "", first.AccessToken), 401)
}

func TestIntegrationProfileNameSpaces(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("name-spaces@example.com", testPassword), "", ""), 201)
	for _, name := range []string{"Виталий", "Виталий тест", "Виталий 123", "Виталий  любой текст"} {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"name": name})
			w := h.do("PATCH", "profile", string(body), "", account.AccessToken)
			expect(t, w, http.StatusOK)
			var updated auth.User
			if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil || updated.Name != name {
				t.Fatalf("update response: %s, %v", w.Body, err)
			}
			stored, err := h.db.UserByID(context.Background(), account.User.ID)
			if err != nil || stored.Name != name {
				t.Fatalf("name was not persisted: got %q, %v", stored.Name, err)
			}
			w = h.do("GET", "me", "", "", account.AccessToken)
			expect(t, w, http.StatusOK)
			var me auth.User
			if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || me.Name != name {
				t.Fatalf("profile read: %s, %v", w.Body, err)
			}
		})
	}
}

func TestIntegrationProfileMigrationPreservesUsers(t *testing.T) {
	h := integration(t)
	created, _ := result(t, h.do("POST", "register", credentials("legacy@example.com", testPassword), "", ""), 201)
	before, err := h.db.UserByID(context.Background(), created.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Restore the original schema inside this test's isolated PostgreSQL schema.
	if _, err := h.db.SQL.Exec("ALTER TABLE users DROP COLUMN name"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.SQL.Exec("DELETE FROM goose_db_version WHERE version_id = 2"); err != nil {
		t.Fatal(err)
	}
	if err := h.db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := h.db.UserByID(context.Background(), created.User.ID)
	if err != nil || after != before {
		t.Fatalf("migration changed existing user: %+v %v", after, err)
	}
}
