package monitor

import (
	"errors"
	"log/slog"
	"net/http"

	"uptime-app/backend/internal/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := httpx.UserID(r.Context())
	if userID == 0 {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		monitors, err := h.service.List(r.Context(), userID)
		if err != nil {
			h.fail(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"monitors": monitors})
	case http.MethodPost:
		var body struct {
			URL             string `json:"url"`
			IntervalSeconds int    `json:"interval_seconds"`
		}
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
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidURL):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_url", "Use an HTTP or HTTPS URL up to 2048 bytes, without credentials or fragment")
	case errors.Is(err, ErrInvalidInterval):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_interval", "Use a whole number of seconds from 1 to 2147483647")
	default:
		slog.Error("Monitor request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Could not process monitor")
	}
}
