package xgrpcsrv

import "google.golang.org/grpc"

// NewServer builds a gRPC server with the standard interceptor chain (DD tracing, recovery,
// logging, cancellation handling, and validation). Callers are responsible for serving
// and stopping the returned server.
func NewServer(opts ...Opt) (*grpc.Server, error) {
	params := DefaultParams()
	for _, opt := range opts {
		opt(&params)
	}
	return newServerFromParams(params)
}
