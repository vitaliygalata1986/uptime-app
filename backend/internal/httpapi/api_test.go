package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/upload"
)

func localAPI() *API {
	return New(auth.NewHandler(&auth.Service{}, false, nil), http.NotFoundHandler(), upload.NewHandler(nil), monitor.NewHandler(nil), config.Config{LocalHTTP: true, AllowedOrigins: []string{"http://localhost:3000"}})
}

func TestRequestGuards(t *testing.T) {
	for _, tc := range []struct {
		name, path, body, origin, contentType string
		status                                int
	}{
		{"origin missing", "/register", `{}`, "", "application/json", 403},
		{"origin forbidden", "/register", `{}`, "https://evil.example", "application/json", 403},
		{"wrong media", "/register", `{}`, "http://localhost:3000", "text/plain", 400},
		{"bad JSON", "/register", `{`, "http://localhost:3000", "application/json", 400},
		{"null", "/refresh", `null`, "http://localhost:3000", "application/json", 400},
		{"array", "/refresh", `[]`, "http://localhost:3000", "application/json", 400},
		{"unknown", "/refresh", `{"extra":true}`, "http://localhost:3000", "application/json", 400},
		{"trailing", "/refresh", `{} {}`, "http://localhost:3000", "application/json", 400},
		{"too large", "/refresh", strings.Repeat(" ", 8193), "http://localhost:3000", "application/json", 400},
		{"bad utf8", "/register", "{\"email\":\"a\xff@example.com\"}", "http://localhost:3000", "application/json", 400},
		{"missing refresh", "/refresh", `{}`, "http://localhost:3000", "application/json; charset=utf-8", 401},
		{"empty logout", "/logout", `{}`, "http://localhost:3000", "application/json", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/auth"+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			localAPI().ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cacheable auth response")
			}
			if tc.status >= 400 && !strings.Contains(w.Body.String(), `"error":{"code":`) {
				t.Fatal("wrong error shape")
			}
			if tc.origin == "https://evil.example" && w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("forbidden CORS")
			}
		})
	}
}

func TestPreflight(t *testing.T) {
	for _, tc := range []struct {
		origin, method, headers string
		status                  int
	}{
		{"http://localhost:3000", "POST", "content-type, Authorization", 204},
		{"https://evil.example", "POST", "content-type", 403},
		{"", "POST", "content-type", 403},
		{"http://localhost:3000", "DELETE", "content-type", 403},
		{"http://localhost:3000", "POST", "X-Unsupported", 403},
	} {
		r := httptest.NewRequest("OPTIONS", "/api/auth/login", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.method)
		r.Header.Set("Access-Control-Request-Headers", tc.headers)
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
		if w.Code == 204 && (w.Header().Get("Access-Control-Allow-Credentials") != "true" || w.Header().Get("Access-Control-Allow-Origin") != tc.origin) {
			t.Fatal("CORS credentials")
		}
	}
}

func TestRouteMethods(t *testing.T) {
	for path, method := range map[string]string{
		"/api/auth/register":        http.MethodPost,
		"/api/auth/login":           http.MethodPost,
		"/api/auth/refresh":         http.MethodPost,
		"/api/auth/logout":          http.MethodPost,
		"/api/auth/me":              http.MethodGet,
		"/api/auth/profile":         http.MethodPatch,
		"/api/auth/avatar":          http.MethodPost,
		"/api/uploads":              http.MethodPost,
		"/uploads/files/not-a-file": http.MethodGet,
		"/healthz":                  http.MethodGet,
	} {
		t.Run(path, func(t *testing.T) {
			a := localAPI()
			r := httptest.NewRequest(http.MethodOptions, path, nil)
			r.Header.Set("Origin", "http://localhost:3000")
			r.Header.Set("Access-Control-Request-Method", method)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Methods") != method {
				t.Fatalf("preflight: %d %s", w.Code, w.Body)
			}
			r = httptest.NewRequest(http.MethodDelete, path, nil)
			w = httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != method+", OPTIONS" {
				t.Fatalf("method guard: %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestProfileRequestGuards(t *testing.T) {
	for _, tc := range []struct {
		origin, contentType, access, body string
		status                            int
	}{
		{"", "application/json", "", `{}`, 403},
		{"https://evil.example", "application/json", "", `{}`, 403},
		{"http://localhost:3000", "text/plain", "", `{}`, 400},
		{"http://localhost:3000", "application/json", "", `{"name":"Alice"}`, 401},
		{"http://localhost:3000", "application/json", "Bearer invalid", `{"name":"Alice","email":"other@example.com"}`, 400},
		{"http://localhost:3000", "application/json", "Bearer invalid", `null`, 400},
	} {
		r := httptest.NewRequest(http.MethodPatch, "/api/auth/profile", strings.NewReader(tc.body))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", tc.contentType)
		r.Header.Set("Authorization", tc.access)
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
}

func TestUnknownRoutes(t *testing.T) {
	for _, path := range []string{"/missing", "/api/auth/login/", "/api/auth/unknown", "/healthz/"} {
		w := httptest.NewRecorder()
		localAPI().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"not_found"`) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

func TestLimiterConcurrencyAndCleanup(t *testing.T) {
	l := newLimiter()
	now := time.Now()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.allow("login:ip", 10, now); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed %d", allowed.Load())
	}
	if ok, retry := l.allow("login:ip", 10, now); ok || retry != time.Minute {
		t.Fatal("missing retry duration")
	}
	l.allow("other:ip", 5, now.Add(time.Minute))
	if _, exists := l.entries["login:ip"]; exists {
		t.Fatal("stale entries retained")
	}
}

func TestRateLimitsIgnoreProxyHeaders(t *testing.T) {
	for path, limit := range map[string]int{"register": 5, "login": 10, "refresh": 30} {
		a := localAPI()
		for i := 0; i <= limit; i++ {
			r := httptest.NewRequest("POST", "/api/auth/"+path, strings.NewReader(`{}`))
			r.RemoteAddr = "192.0.2.10:1234"
			r.Header.Set("Origin", "http://localhost:3000")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Forwarded-For", time.Now().String())
			r.Header.Set("X-Real-IP", time.Now().String())
			w := httptest.NewRecorder()
			a.ServeHTTP(w, r)
			if i < limit && w.Code == 429 {
				t.Fatal("early limit")
			}
			if i == limit && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
				t.Fatalf("%s missing limit: %d", path, w.Code)
			}
		}
	}
}
