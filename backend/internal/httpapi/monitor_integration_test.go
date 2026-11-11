package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/monitor"
)

func (h *harness) monitors(method, body, access string) *httptest.ResponseRecorder {
	return h.monitorRequest(method, "/api/monitors", body, access)
}

func (h *harness) monitorRequest(method, target, body, access string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
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
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[],"next_cursor":""}` {
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
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[],"next_cursor":""}` {
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
			h.api.limiter = newLimiter()
			expect(t, h.monitors("POST", body, account.AccessToken), http.StatusBadRequest)
		})
	}
	w := h.monitors("GET", "", account.AccessToken)
	expect(t, w, 200)
	if strings.TrimSpace(w.Body.String()) != `{"monitors":[],"next_cursor":""}` {
		t.Fatalf("invalid requests created points: %s", w.Body)
	}
}

func seedMonitors(t *testing.T, h *harness, owner uint64, count int, created time.Time) {
	t.Helper()
	_, err := h.db.SQL.Exec(`INSERT INTO monitors (user_id, url, interval_seconds, created_at)
		SELECT $1, 'https://seed.example/' || n, 60, $2 FROM generate_series(1, $3) AS n`, owner, created, count)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationMonitorQuotaConcurrent(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("quota@example.com", testPassword), "", ""), 201)
	other, _ := result(t, h.do("POST", "register", credentials("quota-other@example.com", testPassword), "", ""), 201)
	seedMonitors(t, h, account.User.ID, monitor.MaxMonitorsPerUser-1, time.Now().UTC())
	responses := make(chan *httptest.ResponseRecorder, 8)
	start := make(chan struct{})
	for range 8 {
		go func() {
			<-start
			responses <- h.monitors("POST", `{"url":"https://last.example","interval_seconds":60}`, account.AccessToken)
		}()
	}
	close(start)
	success, denied := 0, 0
	for range 8 {
		w := <-responses
		switch w.Code {
		case http.StatusCreated:
			success++
		case http.StatusConflict:
			if !strings.Contains(w.Body.String(), `"code":"monitor_limit"`) {
				t.Fatalf("quota response: %s", w.Body)
			}
			denied++
		default:
			t.Fatalf("concurrent quota: %d %s", w.Code, w.Body)
		}
	}
	var count int
	if err := h.db.SQL.QueryRow("SELECT count(*) FROM monitors WHERE user_id=$1", account.User.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if success != 1 || denied != 7 || count != monitor.MaxMonitorsPerUser {
		t.Fatalf("quota: %d successes, %d denied, %d rows", success, denied, count)
	}
	expect(t, h.monitors("POST", `{"url":"https://other.example","interval_seconds":60}`, other.AccessToken), 201)
}

func TestIntegrationMonitorRateLimits(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("rate@example.com", testPassword), "", ""), 201)
	other, _ := result(t, h.do("POST", "register", credentials("rate-other@example.com", testPassword), "", ""), 201)
	for range 12 {
		expect(t, h.monitors("POST", `{}`, "invalid"), 401)
	}
	for range 10 {
		expect(t, h.monitors("POST", `{"url":"https://rate.example","interval_seconds":60}`, account.AccessToken), 201)
	}
	// A different token and a different connection still share the account's limit.
	logged, _ := result(t, h.do("POST", "login", credentials("rate@example.com", testPassword), "", ""), 200)
	r := httptest.NewRequest("POST", "/api/monitors", strings.NewReader(`{"url":"https://rate.example","interval_seconds":60}`))
	r.RemoteAddr = "198.51.100.42:4321"
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+logged.AccessToken)
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	expect(t, w, 429)
	if w.Header().Get("Retry-After") != "60" {
		t.Fatalf("retry-after: %v", w.Header())
	}
	expect(t, h.monitors("POST", `{"url":"https://rate.example","interval_seconds":60}`, other.AccessToken), 201)
	for range 60 {
		expect(t, h.monitors("GET", "", account.AccessToken), 200)
	}
	expect(t, h.monitors("GET", "", account.AccessToken), 429)
	expect(t, h.monitors("GET", "", other.AccessToken), 200)
	h.clock.Add(int64(time.Minute))
	expect(t, h.monitors("POST", `{"url":"https://rate.example","interval_seconds":60}`, account.AccessToken), 201)
	expect(t, h.monitors("GET", "", account.AccessToken), 200)
}

func TestIntegrationMonitorPagination(t *testing.T) {
	h := integration(t)
	account, _ := result(t, h.do("POST", "register", credentials("pages@example.com", testPassword), "", ""), 201)
	other, _ := result(t, h.do("POST", "register", credentials("pages-other@example.com", testPassword), "", ""), 201)
	created := time.Now().UTC().Truncate(time.Microsecond)
	seedMonitors(t, h, other.User.ID, 2, created)
	seedMonitors(t, h, account.User.ID, 45, created)
	seen := make(map[uint64]bool)
	cursor, firstCursor := "", ""
	var previous uint64 = ^uint64(0)
	for pageNumber := 0; ; pageNumber++ {
		w := h.monitorRequest("GET", "/api/monitors?cursor="+url.QueryEscape(cursor), "", account.AccessToken)
		expect(t, w, 200)
		var page monitor.Page
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		want := monitor.PageSize
		if pageNumber == 2 {
			want = 5
		}
		if pageNumber > 2 || len(page.Monitors) != want {
			t.Fatalf("page %d has %d items", pageNumber, len(page.Monitors))
		}
		for _, item := range page.Monitors {
			if seen[item.ID] || item.ID >= previous {
				t.Fatalf("duplicate or out-of-order item: %d", item.ID)
			}
			seen[item.ID] = true
			previous = item.ID
		}
		if pageNumber == 0 {
			firstCursor = page.NextCursor
			// New records must not shift subsequent pages or cause duplicates.
			expect(t, h.monitors("POST", `{"url":"https://new.example","interval_seconds":60}`, account.AccessToken), 201)
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 45 || firstCursor == "" {
		t.Fatalf("pagination missed records: %d", len(seen))
	}
	w := h.monitorRequest("GET", "/api/monitors?cursor="+url.QueryEscape(firstCursor), "", other.AccessToken)
	expect(t, w, 200)
	var otherPage monitor.Page
	if err := json.Unmarshal(w.Body.Bytes(), &otherPage); err != nil || len(otherPage.Monitors) != 2 {
		t.Fatalf("cross-account cursor: %s, %v", w.Body, err)
	}
	for _, item := range otherPage.Monitors {
		if seen[item.ID] {
			t.Fatal("cursor exposed another account's monitor")
		}
	}
	for _, query := range []string{"cursor=invalid", "cursor=" + strings.Repeat("a", 257), "cursor=e30", "cursor=one&cursor=two", "limit=1000", "cursor=%zz"} {
		t.Run(query[:min(len(query), 40)], func(t *testing.T) {
			w := h.monitorRequest("GET", "/api/monitors?"+query, "", account.AccessToken)
			expect(t, w, 400)
			if !strings.Contains(w.Body.String(), `"code":"invalid_cursor"`) {
				t.Fatalf("cursor error: %s", w.Body)
			}
		})
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
