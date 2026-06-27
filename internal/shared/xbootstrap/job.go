package xbootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xos"
	"github.com/raphoester/movie-reservation-system/internal/shared/xruntime"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// JobConfig is the config interface required by Job.
type JobConfig interface {
	xconfigs.AppConfig

	GetLoggerConfig() xlog.Config
}

// JobProps are the dependencies provided to the job function.
type JobProps[C JobConfig] struct {
	Config   C
	Logger   *slog.Logger
	Closable FnRegistrar
}

// Job bootstraps a job that runs a task and exits when done.
// The job receives a signal-aware context so it can be cancelled on SIGTERM.
func Job[C JobConfig](
	config C,
	job func(ctx context.Context, props JobProps[C]) error,
) error {
	if err := xconfigs.LoadConfig(config); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := getLogger(config)

	closable := NewClosableRegistry()
	defer func() {
		if err := closable.Close(); err != nil {
			logger.Error("failed to close job dependencies", "error", err)
		}
	}()

	ctx, stop := xos.WithSignal(context.Background())

	if xruntime.Detect(ctx).IsCloudDeployment {
		tracer.Start(
			tracer.WithServiceName(config.GetName()),
			tracer.WithEnv(config.Environment()),
		)

		closable.Register(tracer.Stop)
	}
	defer stop()

	if err := job(ctx, JobProps[C]{
		Config:   config,
		Logger:   logger,
		Closable: closable,
	}); err != nil {
		return fmt.Errorf("job: %w", err)
	}

	return nil
}
