package xgrpcsrv

import (
	"context"
	"fmt"
	"net"
	"time"

	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/raphoester/movie-reservation-system/internal/shared/xos"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	ddgrpc "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
)

type Config struct {
	Port       int `validate:"required,min=1,max=65535"`
	Reflection bool
}

type Params struct {
	registerFunc  func(server *grpc.Server) error
	logger        Logger
	enableTracing bool
	serviceName   string
	reflection    bool
}

func DefaultParams() Params {
	return Params{
		logger:        &nopLogger{},
		serviceName:   "default-grpc-service-name-to-replace",
		enableTracing: true,
		registerFunc:  func(_ *grpc.Server) error { return nil },
	}
}

type Opt func(*Params)

func WithLogger(logger Logger) Opt {
	return func(params *Params) { params.logger = logger }
}

func WithTracingEnabled(enabled bool) Opt {
	return func(params *Params) {
		params.enableTracing = enabled
	}
}

func WithServiceName(serviceName string) Opt {
	return func(params *Params) { params.serviceName = serviceName }
}

func WithRegisterFunc(registerFunc func(server *grpc.Server) error) Opt {
	return func(params *Params) { params.registerFunc = registerFunc }
}

func WithReflection(enabled bool) Opt {
	return func(params *Params) { params.reflection = enabled }
}

func newServerFromParams(params Params) (*grpc.Server, error) {
	// first in the list is the outermost interceptor
	// last in the list is the innermost interceptor
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		logUnaryInterceptor(params.logger),
		grpc_recovery.UnaryServerInterceptor(withRecoveryHandler()),
	}
	if params.enableTracing {
		unaryInterceptors = append(unaryInterceptors, ddgrpc.UnaryServerInterceptor(ddgrpc.WithServiceName(params.serviceName)))
	}
	unaryInterceptors = append(
		unaryInterceptors,
		handleCanceledRequestsInterceptor(),
		validateInterceptor(),
	)

	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
	}

	server := grpc.NewServer(serverOpts...)

	if params.registerFunc != nil {
		if err := params.registerFunc(server); err != nil {
			return nil, fmt.Errorf("failed to register gRPC services: %w", err)
		}
	}

	if params.reflection {
		reflection.Register(server)
	}

	return server, nil
}

func StartGRPCServer(
	config Config,
	opts ...Opt,
) error {
	params := DefaultParams()
	for _, opt := range opts {
		opt(&params)
	}

	server, err := newServerFromParams(params)
	if err != nil {
		return err
	}

	ctx, stop := xos.WithSignal(context.Background())
	defer stop()

	lis, err := (&net.ListenConfig{
		KeepAlive: 3 * time.Minute,
	}).Listen(ctx, "tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		params.logger.Info("starting gRPC server", "port", config.Port)
		if err := server.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("gRPC server failed unexpectedly: %w", err)
	case <-ctx.Done():
		params.logger.Info("shutting down gRPC server gracefully...")
	}

	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		params.logger.Info("gRPC server stopped gracefully")
	case <-time.After(10 * time.Second):
		params.logger.Warn("graceful shutdown timed out, forcing stop")
		server.Stop()
	}

	return nil
}
