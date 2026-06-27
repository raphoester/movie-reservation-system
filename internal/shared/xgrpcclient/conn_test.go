package xgrpcclient_test

import (
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xgrpcclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
)

func TestNewConn_RejectsHTTPScheme(t *testing.T) {
	_, err := xgrpcclient.NewConn(
		xgrpcclient.Config{BaseURL: "http://reservations-grpc-api:50051"},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not include a scheme")
}

func TestNewConn_RejectsHTTPSScheme(t *testing.T) {
	_, err := xgrpcclient.NewConn(
		xgrpcclient.Config{BaseURL: "https://reservations-grpc-api:50051"},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not include a scheme")
}

func TestNewConn_AcceptsHostPort(t *testing.T) {
	conn, err := xgrpcclient.NewConn(
		xgrpcclient.Config{BaseURL: "reservations-grpc-api:50051"},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestNewConn_AcceptsLocalhost(t *testing.T) {
	conn, err := xgrpcclient.NewConn(
		xgrpcclient.Config{BaseURL: "localhost:50051"},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}
