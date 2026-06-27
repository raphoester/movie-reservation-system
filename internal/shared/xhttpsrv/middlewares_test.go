package xhttpsrv_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xhttpsrv"
	"github.com/stretchr/testify/assert"
)

type capturedLog struct {
	msg   string
	attrs map[string]any
}

type spyLogger struct {
	warns  []capturedLog
	errors []capturedLog
}

func (s *spyLogger) Info(string, ...any) {}
func (s *spyLogger) Warn(msg string, attrs ...any) {
	s.warns = append(s.warns, capturedLog{msg: msg, attrs: attrsToMap(attrs)})
}

func (s *spyLogger) Error(msg string, attrs ...any) {
	s.errors = append(s.errors, capturedLog{msg: msg, attrs: attrsToMap(attrs)})
}

func attrsToMap(attrs []any) map[string]any {
	m := make(map[string]any, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		if k, ok := attrs[i].(string); ok {
			m[k] = attrs[i+1]
		}
	}
	return m
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("unmatched route logs page not found", func(t *testing.T) {
		spy := &spyLogger{}
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			xhttpsrv.SetRequestError(r.Context(), errors.New("page not found"))
			w.WriteHeader(http.StatusNotFound)
		})
		handler := xhttpsrv.WithDefaultMiddlewares(mux, spy)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/nonexistent/route", nil)
		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Len(t, spy.warns, 1)
		assert.Equal(t, "page not found", spy.warns[0].attrs["error"])
	})

	t.Run("4xx with SetRequestError logs the explicit error", func(t *testing.T) {
		spy := &spyLogger{}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /reservation/{id}", func(w http.ResponseWriter, r *http.Request) {
			xhttpsrv.SetRequestError(r.Context(), errors.New("reservation not found"))
			w.WriteHeader(http.StatusNotFound)
		})
		handler := xhttpsrv.WithDefaultMiddlewares(mux, spy)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/reservation/ON1", nil)
		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Len(t, spy.warns, 1)
		assert.Equal(t, "reservation not found", spy.warns[0].attrs["error"])
	})

	t.Run("2xx does not log error field", func(t *testing.T) {
		spy := &spyLogger{}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /reservation/{id}", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := xhttpsrv.WithDefaultMiddlewares(mux, spy)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/reservation/ON1", nil)
		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, spy.warns)
		assert.Empty(t, spy.errors)
	})
}
