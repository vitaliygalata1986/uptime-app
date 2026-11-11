package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/monitor"
)

func (h *harness) monitors(method, body, access string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/monitors", strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	if access != "" {
		r.Header.Set("Authorization", "Bearer "+access)
	}
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	return w
}

func TestIntegrationMonitors(t *testing.T) {
	h := integration(t)
	first, _ := result(t, h.do("POST", "register", credentials("monitor@example.com", testPassword), "", ""), 201)
	second, _ := result(t, h.do("POST", "register", credentials("other-monitor@example.com", testPassword), "", ""), 201)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		expect(t, h.monitors(method, `{}`, ""), 401)
		expect(t, h.monitors(method, `{}`, "invalid"), 401)
	}
	w := h.monitors("GET", "", first.AccessToken)
	expect(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[]}` {
		t.Fatalf("empty list contract: %s", w.Body)
	}

	var created []monitor.Monitor
	for _, interval := range []int{1, 5, 7, 60, 600, 3600, 7200, monitor.MaxIntervalSeconds} {
		value := fmt.Sprintf("https://example.com/%d?health=1", interval)
		body, _ := json.Marshal(map[string]any{"url": "  " + value + "  ", "interval_seconds": interval})
		w := h.monitors("POST", string(body), first.AccessToken)
		expect(t, w, http.StatusCreated)
		var item monitor.Monitor
		if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		if item.ID == 0 || item.URL != value || item.IntervalSeconds != interval || item.CreatedAt.IsZero() {
			t.Fatalf("created monitor: %+v", item)
		}
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &shape); err != nil || len(shape) != 4 {
			t.Fatalf("public monitor fields: %s, %v", w.Body, err)
		}
		var storedURL string
		var storedInterval int
		var owner uint64
		if err := h.db.SQL.QueryRow("SELECT url, interval_seconds, user_id FROM monitors WHERE id = $1", item.ID).
			Scan(&storedURL, &storedInterval, &owner); err != nil || storedURL != item.URL || storedInterval != interval || owner != first.User.ID {
			t.Fatalf("persisted monitor: %q %d owner=%d, %v", storedURL, storedInterval, owner, err)
		}
		created = append(created, item)
	}
	// Re-running migrations preserves created points and existing accounts.
	if err := h.db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	w = h.monitors("GET", "", first.AccessToken)
	expect(t, w, 200)
	var list struct {
		Monitors []monitor.Monitor `json:"monitors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Monitors) != len(created) {
		t.Fatalf("list: %s, %v", w.Body, err)
	}
	for i, got := range list.Monitors {
		want := created[len(created)-1-i]
		if got.ID != want.ID || got.URL != want.URL || got.IntervalSeconds != want.IntervalSeconds || !got.CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("list item %d: %+v, want %+v", i, got, want)
		}
	}
	w = h.monitors("GET", "", second.AccessToken)
	expect(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[]}` {
		t.Fatalf("another account can see points: %s", w.Body)
	}
	expect(t, h.monitors("POST", `{"url":"https://other.example","interval_seconds":420}`, second.AccessToken), 201)
	w = h.monitors("GET", "", first.AccessToken)
	expect(t, w, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Monitors) != len(created) {
		t.Fatalf("another account changed first list: %s, %v", w.Body, err)
	}
	h.clock.Add(int64(16 * time.Minute))
	expect(t, h.monitors("GET", "", first.AccessToken), 401)
	expect(t, h.monitors("POST", `{"url":"https://example.com","interval_seconds":7}`, first.AccessToken), 401)
}

func TestIntegrationMonitorValidation(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("invalid-monitor@example.com", testPassword), "", ""), 201)
	for _, body := range []string{
		`{}`, `null`, `[]`, `{`, `{} {}`, strings.Repeat(" ", 8193),
		`{"url":null,"interval_seconds":7}`, `{"url":"example.com","interval_seconds":7}`,
		`{"url":"ftp://example.com","interval_seconds":7}`,
		`{"url":"https://example.com","interval_seconds":0}`,
		`{"url":"https://example.com","interval_seconds":-1}`,
		`{"url":"https://example.com","interval_seconds":1.5}`,
		`{"url":"https://example.com","interval_seconds":2147483648}`,
		`{"url":"https://example.com","interval_seconds":null}`,
		`{"url":"https://example.com","interval_seconds":"7"}`,
		`{"url":"https://example.com","interval_seconds":7,"user_id":999}`,
		`{"url":"https://example.com","interval_seconds":7,"status":"up"}`,
	} {
		t.Run(body[:min(len(body), 90)], func(t *testing.T) {
			expect(t, h.monitors("POST", body, account.AccessToken), http.StatusBadRequest)
		})
	}
	w := h.monitors("GET", "", account.AccessToken)
	expect(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[]}` {
		t.Fatalf("invalid requests created points: %s", w.Body)
	}
}

func TestMonitorRouteGuards(t *testing.T) {
	for _, tc := range []struct {
		method, origin, contentType, access, requested string
		status                                         int
	}{
		{"GET", "", "", "", "", 401},
		{"POST", "", "application/json", "", "", 403},
		{"POST", "https://evil.example", "application/json", "", "", 403},
		{"POST", "http://localhost:3000", "text/plain", "", "", 400},
		{"POST", "http://localhost:3000", "application/json", "", "", 401},
		{"POST", "http://localhost:3000", "application/json", "Bearer", "", 401},
		{"OPTIONS", "http://localhost:3000", "", "", "GET", 204},
		{"OPTIONS", "http://localhost:3000", "", "", "POST", 204},
		{"OPTIONS", "http://localhost:3000", "", "", "DELETE", 403},
		{"OPTIONS", "https://evil.example", "", "", "POST", 403},
		{"DELETE", "", "", "", "", 405},
	} {
		r := httptest.NewRequest(tc.method, "/api/monitors", strings.NewReader(`{}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", tc.contentType)
		r.Header.Set("Authorization", tc.access)
		r.Header.Set("Access-Control-Request-Method", tc.requested)
		r.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, r)
		expect(t, w, tc.status)
		if tc.status == 204 && w.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
			t.Fatalf("monitor preflight: %v", w.Header())
		}
		if tc.status == 405 && w.Header().Get("Allow") != "GET, POST, OPTIONS" {
			t.Fatalf("monitor methods: %v", w.Header())
		}
	}
}
