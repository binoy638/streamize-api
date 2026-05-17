package httpserver

import (
	"database/sql"
	"net/http"

	"github.com/binoy638/streamize-api/apps/api/internal/config"
)

type HealthHandler struct {
	Config config.Config
	DB     *sql.DB
}

type healthResponse struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Database    string `json:"database"`
}

func (h HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response := healthResponse{
		Status:      "ok",
		Service:     "streamize-api",
		Environment: h.Config.Environment,
		Database:    "ok",
	}

	if err := h.DB.PingContext(r.Context()); err != nil {
		response.Status = "degraded"
		response.Database = "error"
		writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}

	writeJSON(w, http.StatusOK, response)
}
