package xos

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithSignal spawns a ctx context that cancels on SIGINT and SIGTERM, perfect
// for blocking the main function while processing.
// SIGINT usually spawns from terminal, SIGTERM from kubernetes.
func WithSignal(ctx context.Context) (nc context.Context, cancel func()) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}
