package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"uptime-app/backend/internal/httpx"
	"uptime-app/backend/internal/store"
)

// JSON may use six bytes for each URL character, with extra room for the enclosing object.
const maxMonitorJSONBytes = 6*maxMonitorURLLength + 1024

// CreateRequest supplies the target URL and polling interval to store; polling is not implemented.
type CreateRequest struct {
	URL             string `json:"url" binding:"required" maxLength:"4096" example:"https://example.com"`
	IntervalSeconds int    `json:"interval_seconds" binding:"required" minimum:"1" maximum:"2147483647" example:"60"`
}

// UpdateRequest supplies one or both monitor fields. A supplied field must have a value.
type UpdateRequest struct {
	URL             json.RawMessage `json:"url,omitempty" swaggertype:"string" maxLength:"4096" example:"https://example.com"`
	IntervalSeconds json.RawMessage `json:"interval_seconds,omitempty" swaggertype:"integer" minimum:"1" maximum:"2147483647" example:"60"`
}

type Handler struct {
	service *Service
}

// NewHandler binds monitor HTTP requests to the service; route assembly supplies auth guards.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ServeHTTP dispatches monitor requests using only the verified owner ID from the request context.
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
	case http.MethodPatch, http.MethodDelete:
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 63)
		if err != nil || id == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_monitor_id", "Use a positive monitor ID")
			return
		}
		if r.Method == http.MethodPatch {
			h.update(w, r, id)
			return
		}
		h.delete(w, r, id)
	default:
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
// @Failure 403 {object} httpx.ErrorResponse "Origin is forbidden"
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
// @Param monitor body CreateRequest true "HTTP(S) URL up to 4096 bytes after trimming, without credentials or fragment, and a positive integer interval in seconds"
// @Success 201 {object} Monitor
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 409 {object} httpx.ErrorResponse "Account monitor limit reached"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/monitors [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID := httpx.UserID(r.Context())
	var body CreateRequest
	if err := httpx.DecodeWithLimit(w, r, &body, maxMonitorJSONBytes); err != nil {
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

// @Summary Update a monitor
// @Tags monitors
// @Description Updates one or both fields of a monitor owned by the current account. Omitted fields retain their values. Limited to 10 requests per user per minute.
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param id path int64 true "Positive monitor ID" minimum(1)
// @Param monitor body UpdateRequest true "At least one non-null field: an HTTP(S) URL up to 4096 bytes after trimming, without credentials or fragment, or a positive integer interval in seconds"
// @Success 200 {object} Monitor
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 404 {object} httpx.ErrorResponse "Monitor not found for this account"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/monitors/{id} [patch]
func (h *Handler) update(w http.ResponseWriter, r *http.Request, id uint64) {
	var body UpdateRequest
	if err := httpx.DecodeWithLimit(w, r, &body, maxMonitorJSONBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected URL or interval_seconds")
		return
	}
	value, err := optionalField[string](body.URL)
	if err != nil {
		h.fail(w, ErrInvalidURL)
		return
	}
	interval, err := optionalField[int](body.IntervalSeconds)
	if err != nil {
		h.fail(w, ErrInvalidInterval)
		return
	}
	changes := store.MonitorChanges{URL: value, IntervalSeconds: interval}
	updated, err := h.service.Update(r.Context(), httpx.UserID(r.Context()), id, changes)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

// @Summary Delete a monitor
// @Tags monitors
// @Description Deletes a monitor owned by the current account. Limited to 10 requests per user per minute.
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param id path int64 true "Positive monitor ID" minimum(1)
// @Success 204 "No Content"
// @Failure 400 {object} httpx.ErrorResponse "Invalid monitor ID"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 404 {object} httpx.ErrorResponse "Monitor not found for this account"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/monitors/{id} [delete]
func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id uint64) {
	if err := h.service.Delete(r.Context(), httpx.UserID(r.Context()), id); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func optionalField[T string | int](raw json.RawMessage) (*T, error) {
	if raw == nil {
		return nil, nil
	}
	var value *T
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, errors.New("field must not be null")
	}
	return value, nil
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidURL):
		message := fmt.Sprintf(
			"Use an HTTP or HTTPS URL up to %d bytes, without credentials or fragment",
			maxMonitorURLLength,
		)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_url", message)
	case errors.Is(err, ErrInvalidInterval):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_interval", "Use a whole number of seconds from 1 to 2147483647")
	case errors.Is(err, ErrInvalidCursor):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_cursor", "Use the next_cursor returned by the previous page")
	case errors.Is(err, errEmptyUpdate):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Provide at least one of url or interval_seconds")
	case errors.Is(err, store.ErrMonitorNotFound):
		httpx.WriteError(w, http.StatusNotFound, "monitor_not_found", "Monitor not found")
	case errors.Is(err, store.ErrMonitorLimit):
		httpx.WriteError(w, http.StatusConflict, "monitor_limit", "An account can have at most 100 monitors")
	case errors.Is(err, store.ErrUnauthorized):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
	default:
		slog.Error("Monitor request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Could not process monitor")
	}
}
