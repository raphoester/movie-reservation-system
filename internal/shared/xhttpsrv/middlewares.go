package xhttpsrv

import (
	"context"
	"net/http"
	"time"
)

func chainMiddlewares(
	h http.Handler,
	mws ...func(http.Handler) http.Handler,
) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type errContextKey struct{}

type errorHolder struct {
	err error
}

// SetRequestError records err for the current request's log entry.
// It only has effect when called within an xhttpsrv-managed handler.
func SetRequestError(ctx context.Context, err error) {
	if h, ok := ctx.Value(errContextKey{}).(*errorHolder); ok {
		h.err = err
	}
}

func loggingMiddleware(logger Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			begin := time.Now()

			logger.Info(
				"Incoming request",
				"method", r.Method,
				"url", r.URL.String(),
			)

			holder := &errorHolder{
				err: nil,
			}
			r = r.WithContext(context.WithValue(r.Context(), errContextKey{}, holder))

			sr := &statusRecorder{
				ResponseWriter: w,
				status:         0,
			}

			next.ServeHTTP(sr, r)

			attrs := []any{
				"method", r.Method,
				"url", r.URL.String(),
				"duration", time.Since(begin),
				"status", sr.status,
			}

			if holder.err != nil {
				attrs = append(attrs, "error", holder.err.Error())
			}

			switch {
			case sr.status >= 500:
				logger.Error("Request failed", attrs...)
			case sr.status >= 400:
				logger.Warn("Request failed", attrs...)
			default:
				logger.Info("Request completed", attrs...)
			}
		})
	}
}

type Logger interface {
	Info(msg string, attrs ...any)
	Warn(msg string, attrs ...any)
	Error(msg string, attrs ...any)
}

// WithDefaultMiddlewares wraps the given handler with the standard middleware
// chain used by the HTTP server (logging, status recording). Tracing is not
// included — it is added separately by the bootstrap layer when enabled.
func WithDefaultMiddlewares(handler http.Handler, logger Logger) http.Handler {
	return chainMiddlewares(handler, loggingMiddleware(logger))
}
