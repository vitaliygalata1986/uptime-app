package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCookie(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, secure := range []bool{false, true} {
		a := NewHandler(&Service{}, secure, nil)
		a.now = func() time.Time { return now }
		w := httptest.NewRecorder()
		a.setCookie(w, "token", now.Add(2*time.Hour))
		c := w.Result().Cookies()[0]
		if c.Name != cookieName || !c.HttpOnly || c.Secure != secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/api/auth" || c.Domain != "" || c.MaxAge != 7200 || !c.Expires.Equal(now.Add(2*time.Hour)) {
			t.Fatalf("cookie: %+v", c)
		}
		w = httptest.NewRecorder()
		a.setCookie(w, "", time.Time{})
		d := w.Result().Cookies()[0]
		if d.MaxAge != -1 || d.Value != "" || d.Path != c.Path || d.Domain != c.Domain || d.Secure != c.Secure || !d.Expires.Before(now) {
			t.Fatalf("delete: %+v", d)
		}
		w = httptest.NewRecorder()
		a.setCookie(w, "token", now.Add(time.Millisecond))
		if w.Result().Cookies()[0].MaxAge != -1 {
			t.Fatal("subsecond expiry became session cookie")
		}
	}
}
