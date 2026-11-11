package httpapi

import (
	"math"
	"mime"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/httpx"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/upload"
)

type route struct {
	method  string
	handler http.HandlerFunc
	limit   int
}

type API struct {
	routes  map[string]route
	origins map[string]bool
	limiter *limiter
	now     func() time.Time
	avatars http.HandlerFunc
	uploads http.HandlerFunc
}

func New(authHandler *auth.Handler, healthHandler http.Handler, uploadHandler *upload.Handler, monitorHandler *monitor.Handler, cfg config.Config) *API {
	a := &API{
		routes: map[string]route{
			"/api/auth/register": {http.MethodPost, authHandler.Register, 5},
			"/api/auth/login":    {http.MethodPost, authHandler.Login, 10},
			"/api/auth/refresh":  {http.MethodPost, authHandler.Refresh, 30},
			"/api/auth/logout":   {http.MethodPost, authHandler.Logout, 0},
			"/api/auth/me":       {http.MethodGet, authHandler.Me, 0},
			"/api/auth/profile":  {http.MethodPatch, authHandler.UpdateProfile, 0},
			"/api/auth/avatar":   {http.MethodPost, authHandler.UploadAvatar, 10},
			"/api/uploads":       {http.MethodPost, authHandler.RequireUser(uploadHandler.Upload), 10},
			"/api/monitors":      {"GET, POST", authHandler.RequireUser(monitorHandler.ServeHTTP), 0},
			"/healthz":           {http.MethodGet, healthHandler.ServeHTTP, 0},
		},
		avatars: authHandler.AvatarFile,
		uploads: uploadHandler.File,
		origins: make(map[string]bool), limiter: newLimiter(), now: time.Now,
	}
	for _, origin := range cfg.AllowedOrigins {
		a.origins[origin] = true
	}
	return a
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !a.origins[origin] {
			httpx.WriteError(w, http.StatusForbidden, "forbidden_origin", "Origin is not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	matched, ok := a.routes[r.URL.Path]
	if strings.HasPrefix(r.URL.Path, "/uploads/avatars/") {
		matched, ok = route{http.MethodGet, a.avatars, 0}, true
	}
	if strings.HasPrefix(r.URL.Path, "/uploads/files/") {
		matched, ok = route{http.MethodGet, a.uploads, 0}, true
	}
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Route not found")
		return
	}
	method := matched.method
	if r.Method == http.MethodOptions {
		a.preflight(w, r, method)
		return
	}
	if !slices.Contains(strings.Split(method, ", "), r.Method) {
		w.Header().Set("Allow", method+", OPTIONS")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPatch {
		if origin == "" {
			httpx.WriteError(w, http.StatusForbidden, "forbidden_origin", "Origin is required")
			return
		}
		contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		expectedType := "application/json"
		if r.URL.Path == "/api/auth/avatar" || r.URL.Path == "/api/uploads" {
			expectedType = "multipart/form-data"
		}
		if err != nil || contentType != expectedType {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Content-Type must be "+expectedType)
			return
		}
		if limit := matched.limit; limit != 0 {
			if ok, retry := a.limiter.allow(r.URL.Path+":"+remoteIP(r.RemoteAddr), limit, a.now()); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests")
				return
			}
		}
	}
	matched.handler(w, r)
}

func (a *API) preflight(w http.ResponseWriter, r *http.Request, method string) {
	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	if r.Header.Get("Origin") == "" || !slices.Contains(strings.Split(method, ", "), r.Header.Get("Access-Control-Request-Method")) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden_origin", "Preflight is not allowed")
		return
	}
	for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		h := strings.TrimSpace(strings.ToLower(header))
		if h != "" && h != "authorization" && h != "content-type" {
			httpx.WriteError(w, http.StatusForbidden, "forbidden_origin", "Request header is not allowed")
			return
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", method)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusNoContent)
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return "unknown"
}
