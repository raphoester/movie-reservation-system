package xbootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhttpsrv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xos"
	"github.com/raphoester/movie-reservation-system/internal/shared/xruntime"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type WorkerConfig interface {
	xconfigs.AppConfig

	GetHTTPServerConfig() xhttpsrv.Config
	GetLoggerConfig() xlog.Config
}

type WorkerDiSequenceProps[C WorkerConfig] struct {
	Config   C
	Logger   *slog.Logger
	Closable FnRegistrar
	Health   *xhealth.HealthRegistry
}

func Worker[C WorkerConfig](
	config C,
	diSequence func(ctx context.Context, props WorkerDiSequenceProps[C]) error,
) error {
	if err := xconfigs.LoadConfig(config); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := getLogger(config)

	closable := NewClosableRegistry()
	defer func() {
		if err := closable.Close(); err != nil {
			logger.Error(
				"failed to close worker dependencies",
				"error", err,
			)
		}
	}()

	ctx := context.Background()
	shouldTrace := xruntime.Detect(ctx).IsCloudDeployment
	if shouldTrace {
		tracer.Start(
			tracer.WithServiceName(config.GetName()),
			tracer.WithEnv(config.Environment()),
		)

		closable.Register(tracer.Stop)
	}

	healthRegistry := xhealth.NewHealthRegistry()

	if err := diSequence(ctx, WorkerDiSequenceProps[C]{
		Config:   config,
		Logger:   logger,
		Closable: closable,
		Health:   healthRegistry,
	}); err != nil {
		return fmt.Errorf("worker dependency injection: %w", err)
	}

	mux := http.NewServeMux()
	monitor := xhttpsrv.RegisterHealth(mux, healthRegistry, logger)
	go monitor.Start(context.Background())

	serverCfg := config.GetHTTPServerConfig()
	healthServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverCfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting health server", "port", serverCfg.Port)
		if err := healthServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("health server error", "error", err)
		}
	}()

	ctx, stop := xos.WithSignal(context.Background())
	defer stop()

	<-ctx.Done()
	logger.Info("shutting down worker, press ctrl+c again to force")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown health server: %w", err)
	}

	return nil
}
