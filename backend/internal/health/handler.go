package health

import (
	"database/sql"
	"net/http"

	"uptime-app/backend/internal/httpx"
)

type Handler struct {
	db *sql.DB
}

// Response reports database readiness.
type Response struct {
	Status string `json:"status" example:"ok"`
}

// NewHandler binds readiness checks to the application's database pool.
func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// ServeHTTP reports readiness only when PostgreSQL responds within the request context.
//
// @Summary Check database readiness
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 503 {object} httpx.ErrorResponse "Storage or database unavailable"
// @Router /healthz [get]
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.db.PingContext(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable", "Database unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, Response{Status: "ok"})
}
