package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"uptime-app/backend/internal/httpx"
	"uptime-app/backend/internal/store"
)

const cookieName = "refresh_token"

// Handler owns auth HTTP input, responses, cookies and service error mapping.
type Handler struct {
	service *Service
	secure  bool
	now     func() time.Time
}

func NewHandler(service *Service, secureCookies bool) *Handler {
	now := time.Now
	if service.Now != nil {
		now = service.Now
	}
	return &Handler{service: service, secure: secureCookies, now: now}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.credentials(w, r, h.service.Register, http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.credentials(w, r, h.service.Login, http.StatusOK)
}

func (h *Handler) credentials(w http.ResponseWriter, r *http.Request, action func(context.Context, string, string) (Result, error), status int) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected a JSON object with email and password")
		return
	}
	result, err := action(r.Context(), input.Email, input.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, result.Refresh, result.ExpiresAt)
	httpx.WriteJSON(w, status, result)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	raw, ok := h.sessionToken(w, r)
	if !ok {
		return
	}
	result, err := h.service.Refresh(r.Context(), raw)
	if err != nil {
		if errors.Is(err, store.ErrUnauthorized) {
			h.setCookie(w, "", time.Time{})
		}
		h.fail(w, err)
		return
	}
	h.setCookie(w, result.Refresh, result.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	raw, ok := h.sessionToken(w, r)
	if !ok {
		return
	}
	if err := h.service.Logout(r.Context(), raw); err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sessionToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if err := httpx.Decode(w, r, &struct{}{}); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected an empty JSON object")
		return "", false
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value, true
	}
	return "", true
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		h.fail(w, store.ErrUnauthorized)
		return
	}
	user, err := h.service.Me(r.Context(), parts[1])
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		h.fail(w, store.ErrUnauthorized)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(w, r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected a JSON object with name only")
		return
	}
	user, err := h.service.UpdateProfile(r.Context(), parts[1], input.Name)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) setCookie(w http.ResponseWriter, raw string, expires time.Time) {
	c := &http.Cookie{Name: cookieName, Value: raw, Path: "/api/auth", HttpOnly: true,
		Secure: h.secure, SameSite: http.SameSiteLaxMode, Expires: expires.UTC(),
		MaxAge: int(expires.Sub(h.now()).Seconds())}
	if raw == "" || c.MaxAge < 1 {
		c.Value, c.MaxAge, c.Expires = "", -1, time.Unix(1, 0).UTC()
	}
	http.SetCookie(w, c)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidName):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_name", err.Error())
	case errors.Is(err, ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, store.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials or token")
	case errors.Is(err, store.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "Email already registered")
	default:
		slog.Error("auth request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
