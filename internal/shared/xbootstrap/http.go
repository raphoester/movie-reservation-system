package xbootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhttpsrv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xruntime"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type HTTPServerConfig interface {
	xconfigs.AppConfig

	GetHTTPServerConfig() xhttpsrv.Config
	GetLoggerConfig() xlog.Config
}

// MiddlewareRegistry allows a DI sequence to register HTTP middlewares that are
// applied to the mux. Middlewares are applied in registration order (first = outermost).
type MiddlewareRegistry struct {
	fns []func(http.Handler) http.Handler
}

func (r *MiddlewareRegistry) Add(fn func(http.Handler) http.Handler) {
	r.fns = append(r.fns, fn)
}

// OAPIHTTPDiSequenceProps is the dependency injection props for OAPIHTTPServer.
// Unlike HTTPDiSequenceProps, it does not expose the mux — route registration
// is handled by bootstrap via the registerRoutes callback.
type OAPIHTTPDiSequenceProps[C HTTPServerConfig] struct {
	Config      C
	Logger      *slog.Logger
	Closable    FnRegistrar
	Health      *xhealth.HealthRegistry
	Middlewares *MiddlewareRegistry
}

// OAPIHTTPServer bootstraps an HTTP server whose routes are entirely defined by
// an OpenAPI spec. diSequence builds and returns the StrictServerInterface
// implementation; registerRoutes wires it onto the mux — pass the generated
// RegisterRoutes function from the spec package.
func OAPIHTTPServer[C HTTPServerConfig, SI any](
	config C,
	diSequence func(ctx context.Context, props OAPIHTTPDiSequenceProps[C]) (SI, error),
	registerRoutes func(SI, *http.ServeMux),
	opts ...xhttpsrv.Opt,
) error {
	ctx := context.Background()
	if err := xconfigs.LoadConfig(config); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := getLogger(config)

	closable := NewClosableRegistry()
	defer func() {
		if err := closable.Close(); err != nil {
			logger.Error("failed to close dependencies to http server", "error", err)
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
	middlewareReg := &MiddlewareRegistry{fns: nil}

	serverOpts := append([]xhttpsrv.Opt{
		xhttpsrv.WithServiceName(config.GetName()),
		xhttpsrv.WithLogger(logger),
		xhttpsrv.WithTracingEnabled(shouldTrace),
		xhttpsrv.WithRegisterFunc(func(mux *http.ServeMux) error {
			si, err := diSequence(ctx, OAPIHTTPDiSequenceProps[C]{
				Config:      config,
				Logger:      logger,
				Closable:    closable,
				Health:      healthRegistry,
				Middlewares: middlewareReg,
			})
			if err != nil {
				return fmt.Errorf("failed to build server: %w", err)
			}

			registerRoutes(si, mux)

			monitor := xhttpsrv.RegisterHealth(mux, healthRegistry, logger)
			go monitor.Start(ctx)

			return nil
		}),
		// DI-registered middlewares are applied inner (before caller opts like CORS).
		// The closure captures middlewareReg by reference; by the time this opt is
		// evaluated inside StartHTTPServer (after WithRegisterFunc has run), the DI
		// sequence has already populated it.
		xhttpsrv.WithMiddleware(func(next http.Handler) http.Handler {
			h := next
			for i := len(middlewareReg.fns) - 1; i >= 0; i-- {
				h = middlewareReg.fns[i](h)
			}
			return h
		}),
	}, opts...)

	if err := xhttpsrv.StartHTTPServer(config.GetHTTPServerConfig(), serverOpts...); err != nil {
		return fmt.Errorf("bootstrap HTTP server: %w", err)
	}

	return nil
}
