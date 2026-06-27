package xgrpcsrv_test

import (
	"net"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xgrpcsrv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/mocktracer"
)

func TestDDTracingUnaryInterceptor(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	const serviceName = "test-grpc-service"

	srv, err := xgrpcsrv.NewServer(
		// tracing is enabled by default
		// xgrpcsrv.WithTracingEnabled(true),
		xgrpcsrv.WithServiceName(serviceName),
		xgrpcsrv.WithRegisterFunc(func(s *grpc.Server) error {
			grpc_health_v1.RegisterHealthServer(s, health.NewServer())
			return nil
		}),
	)
	require.NoError(t, err)

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { srv.Stop() })
	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = grpc_health_v1.NewHealthClient(conn).Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	spans := mt.FinishedSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "grpc.server", span.OperationName())
	assert.Equal(t, serviceName, span.Tag(ext.ServiceName))
	assert.Equal(t, "/grpc.health.v1.Health/Check", span.Tag(ext.ResourceName))
}

func TestDDTracingUnaryInterceptor_Disabled(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	srv, err := xgrpcsrv.NewServer(
		xgrpcsrv.WithTracingEnabled(false),
		xgrpcsrv.WithRegisterFunc(func(s *grpc.Server) error {
			grpc_health_v1.RegisterHealthServer(s, health.NewServer())
			return nil
		}),
	)
	require.NoError(t, err)

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { srv.Stop() })
	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = grpc_health_v1.NewHealthClient(conn).Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	assert.Empty(t, mt.FinishedSpans())
}
