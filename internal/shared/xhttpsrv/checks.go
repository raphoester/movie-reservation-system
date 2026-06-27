package xhttpsrv

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
)

type HTTPResponse struct {
	Status string                 `json:"status"` // "healthy" or "unhealthy"
	Checks map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
	Status string `json:"status"` // "pass" or "fail"
}

func RegisterHealth(mux *http.ServeMux, registry *xhealth.HealthRegistry, logger *slog.Logger) *xhealth.Monitor {
	monitor := xhealth.NewMonitor(registry, logger, xhealth.MonitorConfig{})
	mux.HandleFunc("/health", NewHealthHandlerFromMonitor(monitor))
	return monitor
}

func NewHealthHandlerFromMonitor(monitor *xhealth.Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status := monitor.Status()

		response := HTTPResponse{
			Checks: make(map[string]CheckResult),
			Status: "unhealthy", // default to unhealthy, will be updated if all checks pass
		}

		allHealthy := true
		for name, healthy := range status {
			if healthy {
				response.Checks[name] = CheckResult{
					Status: "pass",
				}
				continue
			}

			allHealthy = false
			response.Checks[name] = CheckResult{
				Status: "fail",
			}
		}

		statusCode := http.StatusServiceUnavailable
		if allHealthy {
			response.Status = "healthy"
			statusCode = http.StatusOK
		}

		w.WriteHeader(statusCode)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}
