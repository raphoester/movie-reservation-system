package xgrpcsrv

import (
	"log/slog"

	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func RegisterHealth(
	server *grpc.Server,
	registry *xhealth.HealthRegistry,
	logger *slog.Logger,
) *xhealth.Monitor {
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	monitor := xhealth.NewMonitor(registry, logger, xhealth.MonitorConfig{
		OnStatusChange: func(name string, healthy bool, err error) {
			var status grpc_health_v1.HealthCheckResponse_ServingStatus

			if healthy {
				logger.Info("health check recovered", "component", name)
				status = grpc_health_v1.HealthCheckResponse_SERVING
			} else {
				logger.Error("health check failed", "component", name, "error", err)
				status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
			}

			healthServer.SetServingStatus("", status)
		},
	})

	return monitor
}
