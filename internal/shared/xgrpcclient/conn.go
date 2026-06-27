package xgrpcclient

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"
)

// NewConn dials a gRPC server from the given Config.
// Returns an error if BaseURL contains a scheme (e.g. "http://") — gRPC
// targets here must be plain host:port; including a scheme changes target
// parsing/resolver selection and will typically cause dialing to fail.
func NewConn(cfg Config, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if strings.Contains(cfg.BaseURL, "://") {
		return nil, fmt.Errorf("grpc base url must not include a scheme (got %q): use host:port format", cfg.BaseURL)
	}
	client, err := grpc.NewClient(cfg.BaseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server at %q: %w", cfg.BaseURL, err)
	}

	return client, nil
}
