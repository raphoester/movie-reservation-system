package xlog_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_timeFormatIsRFC3339(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: xlog.DatadogReplaceAttr,
	})
	logger := slog.New(handler)
	logger.Info("test message")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	raw, ok := entry["time"].(string)
	require.True(t, ok, "time field must be a string")

	_, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err, "time field must parse as RFC3339")
	assert.NotContains(t, raw, ".", "time field must not contain sub-second precision")
}
