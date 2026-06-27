package xhttpsrv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xos"
	ddhttp "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

type HTTPServerParams struct {
	registerRoutes func(*http.ServeMux) error
	middlewares    []func(http.Handler) http.Handler
	enableTracing  bool
	serviceName    string
	logger         Logger
	swagger        *Swagger
}

type Swagger struct {
	Title   string
	OpenAPI string
}

type Config struct {
	Port int `validate:"required,min=1,max=65535"`
}

type Opt func(*HTTPServerParams)

func DefaultParams() HTTPServerParams {
	return HTTPServerParams{
		registerRoutes: func(_ *http.ServeMux) error { return nil },
		enableTracing:  true,
		serviceName:    "default-http-service-name-to-replace",
		logger:         &nopLogger{},
	}
}

type nopLogger struct{}

func (n *nopLogger) Info(string, ...any)  {}
func (n *nopLogger) Warn(string, ...any)  {}
func (n *nopLogger) Error(string, ...any) {}

func WithRegisterFunc(registerFunc func(*http.ServeMux) error) Opt {
	return func(params *HTTPServerParams) {
		params.registerRoutes = registerFunc
	}
}

func WithServiceName(serviceName string) Opt {
	return func(params *HTTPServerParams) {
		params.serviceName = serviceName
	}
}

func WithLogger(logger Logger) Opt {
	return func(params *HTTPServerParams) {
		params.logger = logger
	}
}

func WithTracingEnabled(enabled bool) Opt {
	return func(params *HTTPServerParams) {
		params.enableTracing = enabled
	}
}

func WithSwagger(swagger *Swagger) Opt {
	return func(params *HTTPServerParams) {
		params.swagger = swagger
	}
}

// WithMiddleware adds a middleware to the chain. Middlewares are applied in
// registration order (first registered = outermost). Useful for cross-cutting
// concerns such as CORS or auth that must run before route dispatch.
func WithMiddleware(mw func(http.Handler) http.Handler) Opt {
	return func(params *HTTPServerParams) {
		params.middlewares = append(params.middlewares, mw)
	}
}

func StartHTTPServer(config Config, opts ...Opt) error {
	params := DefaultParams()
	for _, opt := range opts {
		opt(&params)
	}

	ctx, stop := xos.WithSignal(context.Background())
	defer stop()

	mux := http.NewServeMux()
	if params.registerRoutes != nil {
		if err := params.registerRoutes(mux); err != nil {
			return fmt.Errorf("failed to register routes: %w", err)
		}
	}

	if err := registerSwaggerHandlersIfNeeded(mux, params.swagger, params.logger); err != nil {
		return err
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		SetRequestError(r.Context(), errors.New("page not found"))
		w.WriteHeader(http.StatusNotFound)
	})

	handler := chainMiddlewares(mux, params.middlewares...)
	finalHandler := WithDefaultMiddlewares(handler, params.logger)
	if params.enableTracing {
		finalHandler = ddhttp.WrapHandler(finalHandler, params.serviceName, "http.server")
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.Port),
		Handler:           finalHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		params.logger.Info("starting http server", "port", config.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			params.logger.Info("failed serving", "error", err)
		}
	}()

	<-ctx.Done()
	params.logger.Info("shutting down http server, press ctrl+c again to force")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	params.logger.Info("http server stopped gracefully")

	return nil
}
