package xlog

import (
	"fmt"
	"log/slog"
	"testing"
)

// NewTestLogger returns a logger that writes through testing.TB.Log,
// so output appears only when a test fails or when running with -v.
func NewTestLogger(t testing.TB) *TestLogger {
	return &TestLogger{
		t:      t,
		Logger: slog.Logger{},
	}
}

type TestLogger struct {
	t testing.TB

	// This makes sure TestLogger implements slog.Logger.
	slog.Logger
}

func (l *TestLogger) Info(msg string, attrs ...any) {
	l.t.Helper()
	l.t.Log(format("INFO", msg, attrs))
}

func (l *TestLogger) Warn(msg string, attrs ...any) {
	l.t.Helper()
	l.t.Log(format("WARN", msg, attrs))
}

func (l *TestLogger) Error(msg string, attrs ...any) {
	l.t.Helper()
	l.t.Log(format("ERROR", msg, attrs))
}

func format(level, msg string, attrs []any) string {
	s := fmt.Sprintf("[%s] %s", level, msg)
	for i := 0; i+1 < len(attrs); i += 2 {
		s += fmt.Sprintf(" %v=%v", attrs[i], attrs[i+1])
	}
	return s
}
