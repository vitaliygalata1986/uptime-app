package health

import (
	"database/sql"
	"net/http"

	"uptime-app/backend/internal/httpx"
)

type Handler struct {
	db *sql.DB
}

// NewHandler binds readiness checks to the application's database pool.
func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// ServeHTTP reports readiness only when PostgreSQL responds within the request context.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.db.PingContext(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable", "Database unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
