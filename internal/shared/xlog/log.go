package xlog

import (
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/raphoester/movie-reservation-system/internal/shared/xenv"
)

type Config struct {
	Level string `validate:"required,oneof=debug info warn error"`
}

type Level slog.Level

var levelsMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func New(config Config) *slog.Logger {
	level := levelsMap[config.Level]
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: DatadogReplaceAttr,
	})

	return slog.New(handler)
}

// DatadogReplaceAttr rewrites the built-in slog time attribute to RFC3339 (no
// sub-second precision) so Datadog's strptime log parser can parse it.
func DatadogReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(time.Time); ok {
			a.Value = slog.StringValue(t.UTC().Format(time.RFC3339))
		}
	}
	return a
}

func QuickNew() *slog.Logger {
	if xenv.IsLocal() {
		return NewForLocalConsole(Config{Level: "debug"})
	}

	return New(Config{Level: "info"})
}

// NewForLocalConsole creates a logger with human-friendly colored output for local development.
// When a serviceName is provided, each log line is prefixed with a stable per-service color
// so that multiple services running together in the same terminal stream are visually distinct.
func NewForLocalConsole(config Config, serviceNames ...string) *slog.Logger {
	level := levelsMap[config.Level]

	var w io.Writer = os.Stdout
	opts := &tint.Options{
		Level:      level,
		TimeFormat: time.TimeOnly,
	}

	if len(serviceNames) > 0 && serviceNames[0] != "" {
		name := serviceNames[0]
		color := stableColor(name)
		prefix := fmt.Sprintf("\x1b[%dm[%s]\x1b[0m ", color, name)
		w = &prefixWriter{prefix: []byte(prefix), out: os.Stdout}
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Suppress the "app" attribute since the service name is shown as the line prefix.
			if len(groups) == 0 && a.Key == "app" {
				return slog.Attr{
					Key:   "",
					Value: slog.Value{},
				}
			}
			return a
		}
	}

	return slog.New(tint.NewHandler(w, opts))
}

// stableColor maps a service name to a consistent ANSI foreground color code.
func stableColor(name string) int {
	colors := []int{32, 33, 34, 35, 36, 91, 92, 93, 94, 95, 96}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return colors[h.Sum32()%uint32(len(colors))]
}

type prefixWriter struct {
	prefix []byte
	out    io.Writer
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	line := make([]byte, 0, len(w.prefix)+len(p))
	line = append(line, w.prefix...)
	line = append(line, p...)
	if _, err := w.out.Write(line); err != nil {
		return 0, fmt.Errorf("failed to write log entry: %w", err)
	}
	return len(p), nil
}
