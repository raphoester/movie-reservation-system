package xbootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xgrpcsrv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xruntime"
	"google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type GRPCServerConfig interface {
	xconfigs.AppConfig

	GetGRPCServerConfig() xgrpcsrv.Config
	GetLoggerConfig() xlog.Config
}

type GrpcDiSequenceProps[C GRPCServerConfig] struct {
	Config   C
	Logger   *slog.Logger
	Server   *grpc.Server
	Closable FnRegistrar
	Health   *xhealth.HealthRegistry
}

func GrpcServer[C GRPCServerConfig](
	config C,
	diSequence func(ctx context.Context, props GrpcDiSequenceProps[C]) error,
) error {
	ctx := context.Background()
	if err := xconfigs.LoadConfig(config); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := getLogger(config)

	closable := NewClosableRegistry()
	defer func() {
		if err := closable.Close(); err != nil {
			logger.Error(
				"failed to close dependencies to grpc server",
				"error", err,
			)
		}
	}()

	shouldTrace := xruntime.Detect(ctx).IsCloudDeployment
	if shouldTrace {
		tracer.Start(
			tracer.WithServiceName(config.GetName()),
			tracer.WithEnv(config.Environment()),
		)

		closable.Register(tracer.Stop)
	}

	healthRegistry := xhealth.NewHealthRegistry()

	grpcConfig := config.GetGRPCServerConfig()

	if err := xgrpcsrv.StartGRPCServer(
		grpcConfig,
		xgrpcsrv.WithLogger(logger),
		xgrpcsrv.WithTracingEnabled(shouldTrace),
		xgrpcsrv.WithServiceName(config.GetName()),
		xgrpcsrv.WithReflection(grpcConfig.Reflection),
		xgrpcsrv.WithRegisterFunc(func(server *grpc.Server) error {
			err := diSequence(ctx, GrpcDiSequenceProps[C]{
				Config:   config,
				Logger:   logger,
				Server:   server,
				Closable: closable,
				Health:   healthRegistry,
			})
			if err != nil {
				return err
			}

			monitor := xgrpcsrv.RegisterHealth(server, healthRegistry, logger)
			go monitor.Start(ctx)

			return nil
		}),
	); err != nil {
		if err := closable.Close(); err != nil {
			logger.Error("failed to clean up resources after server start failure", "error", err)
		}
		return fmt.Errorf("failed to create server: %w", err)
	}

	return nil
}
