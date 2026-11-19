package monitor

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"uptime-app/backend/internal/httpx"
	"uptime-app/backend/internal/store"
)

// CreateRequest supplies the target URL and polling interval to store; polling is not implemented.
type CreateRequest struct {
	URL             string `json:"url" binding:"required" maxLength:"2048" example:"https://example.com"`
	IntervalSeconds int    `json:"interval_seconds" binding:"required" minimum:"1" maximum:"2147483647" example:"60"`
}

type Handler struct {
	service *Service
}

// NewHandler binds monitor HTTP requests to the service; route assembly supplies auth guards.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ServeHTTP lists or creates monitors using only the verified owner ID from the request context.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := httpx.UserID(r.Context())
	if userID == 0 {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.create(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// @Summary List the current account monitors
// @Tags monitors
// @Description Returns up to 20 monitors, newest first. An empty next_cursor marks the final page. Limited to 60 requests per user per minute.
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Opaque next_cursor from the previous page; no other query parameters" maxlength(256)
// @Success 200 {object} Page
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/monitors [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID := httpx.UserID(r.Context())
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query) > 1 || (len(query) == 1 && len(query["cursor"]) != 1) {
		h.fail(w, ErrInvalidCursor)
		return
	}
	page, err := h.service.List(r.Context(), userID, query.Get("cursor"))
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

// @Summary Create a monitor
// @Tags monitors
// @Description Stores a monitor without polling the target. At most 100 monitors per account; limited to 10 requests per user per minute.
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param monitor body CreateRequest true "HTTP(S) URL without credentials or fragment and a positive integer interval in seconds"
// @Success 201 {object} Monitor
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 409 {object} httpx.ErrorResponse "Account or monitor quota conflict"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/monitors [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID := httpx.UserID(r.Context())
	var body CreateRequest
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected URL and interval_seconds")
		return
	}
	monitor, err := h.service.Create(r.Context(), userID, body.URL, body.IntervalSeconds)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, monitor)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidURL):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_url", "Use an HTTP or HTTPS URL up to 2048 bytes, without credentials or fragment")
	case errors.Is(err, ErrInvalidInterval):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_interval", "Use a whole number of seconds from 1 to 2147483647")
	case errors.Is(err, ErrInvalidCursor):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_cursor", "Use the next_cursor returned by the previous page")
	case errors.Is(err, store.ErrMonitorLimit):
		httpx.WriteError(w, http.StatusConflict, "monitor_limit", "An account can have at most 100 monitors")
	case errors.Is(err, store.ErrUnauthorized):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
	default:
		slog.Error("Monitor request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Could not process monitor")
	}
}
