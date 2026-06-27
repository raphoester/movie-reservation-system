// Package xtunnel defines the shared interfaces for TCP port-forwarding tunnels.
// It is intentionally dependency-free so both xpg and xaws can import it without cycles.
package xtunnel

import (
	"context"
	"fmt"
	"log/slog"
)

// Tunnel represents an active port-forwarding tunnel.
type Tunnel interface {
	LocalPort() int
	Close() error
}

// Opener opens a TCP tunnel to a remote host and returns the active tunnel.
// A nil Tunnel with a nil error means no tunnel was needed; the caller should
// use the original host and port unchanged.
type Opener interface {
	OpenTunnel(ctx context.Context, remoteHost string, remotePort int) (Tunnel, error)
}

// Nopener is an Opener that never opens a tunnel. Use it for non-cloud environments
// where port-forwarding is not needed.
type Nopener struct{}

func (n *Nopener) OpenTunnel(_ context.Context, _ string, _ int) (Tunnel, error) {
	return &nopTunnel{}, nil
}

var _ Opener = (*Nopener)(nil)

type nopTunnel struct{}

func (t *nopTunnel) LocalPort() int { return -1 }
func (t *nopTunnel) Close() error   { return nil }

var _ Tunnel = (*nopTunnel)(nil)

func WithLoggingDecorator(opener Opener, logger *slog.Logger) Opener {
	return &loggingOpener{opener: opener, logger: logger}
}

type loggingOpener struct {
	opener Opener
	logger *slog.Logger
}

func (o *loggingOpener) OpenTunnel(ctx context.Context, remoteHost string, remotePort int) (Tunnel, error) {
	o.logger.Info("opening tunnel", "remote_host", remoteHost, "remote_port", remotePort)
	tunnel, err := o.opener.OpenTunnel(ctx, remoteHost, remotePort)
	if err != nil {
		o.logger.Error("failed to open tunnel", "error", err)
		return nil, fmt.Errorf("open tunnel: %w", err)
	}
	o.logger.Info("tunnel opened", "local_port", tunnel.LocalPort())
	return tunnel, nil
}
