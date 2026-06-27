package xhttpsrv_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhttpsrv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterHealth(t *testing.T) {
	t.Run("registers /health route on the mux", func(t *testing.T) {
		mux := http.NewServeMux()
		registry := xhealth.NewHealthRegistry()
		logger := slog.New(slog.DiscardHandler)

		xhttpsrv.RegisterHealth(mux, registry, logger)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		mux.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp xhttpsrv.HTTPResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Equal(t, "healthy", resp.Status)
	})
}

func TestNewHealthHandlerFromMonitor(t *testing.T) {
	newLogger := func() *slog.Logger {
		return slog.New(slog.DiscardHandler)
	}

	decodeResponse := func(t *testing.T, w *httptest.ResponseRecorder) xhttpsrv.HTTPResponse {
		t.Helper()
		var resp xhttpsrv.HTTPResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		return resp
	}

	// Seed monitor status by running each check once via a pre-cancelled context.
	// Start blocks on wg.Wait(), so all checks complete before it returns.
	seedMonitorStatus := func(t *testing.T, monitor *xhealth.Monitor) {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		monitor.Start(ctx)
	}

	t.Run("no checks - returns 200 healthy", func(t *testing.T) {
		registry := xhealth.NewHealthRegistry()
		monitor := xhealth.NewMonitor(registry, newLogger(), xhealth.MonitorConfig{})
		handler := xhttpsrv.NewHealthHandlerFromMonitor(monitor)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "healthy", resp.Status)
		assert.Empty(t, resp.Checks)
	})

	t.Run("all checks passing - returns 200 healthy", func(t *testing.T) {
		registry := xhealth.NewHealthRegistry()
		registry.Register(xhealth.NewFuncCheck("db", func(_ context.Context) error { return nil }))

		monitor := xhealth.NewMonitor(registry, newLogger(), xhealth.MonitorConfig{})
		seedMonitorStatus(t, monitor)

		handler := xhttpsrv.NewHealthHandlerFromMonitor(monitor)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		handler(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "healthy", resp.Status)
		assert.Equal(t, xhttpsrv.CheckResult{Status: "pass"}, resp.Checks["db"])
	})

	t.Run("a check fails - returns 503 unhealthy", func(t *testing.T) {
		registry := xhealth.NewHealthRegistry()
		registry.Register(xhealth.NewFuncCheck("db", func(_ context.Context) error {
			return errors.New("connection refused")
		}))

		monitor := xhealth.NewMonitor(registry, newLogger(), xhealth.MonitorConfig{})
		seedMonitorStatus(t, monitor)

		handler := xhttpsrv.NewHealthHandlerFromMonitor(monitor)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/health", nil)
		handler(w, r)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		resp := decodeResponse(t, w)
		assert.Equal(t, "unhealthy", resp.Status)
		assert.Equal(t, xhttpsrv.CheckResult{Status: "fail"}, resp.Checks["db"])
	})
}
