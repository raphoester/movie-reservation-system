package lintasyncapi_test

import (
	"testing"
	"testing/fstest"

	"github.com/raphoester/movie-reservation-system/internal/tooling/cmd/lint_asyncapi/internal/lintasyncapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinter_Lint(t *testing.T) {
	spec := func(address string) []byte {
		return []byte(`asyncapi: 3.0.0
channels:
  example:
    address: ` + address)
	}
	buildLinter := func(files fstest.MapFS) lintasyncapi.Linter {
		return lintasyncapi.NewLinter(files)
	}

	t.Run("should pass when all public addresses start with public.", func(t *testing.T) {
		fsys := fstest.MapFS{
			"public/reservations/asyncapi.yaml": {Data: spec("public.reservations.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		assert.Empty(t, violations)
	})

	t.Run("should pass when all private addresses start with private.", func(t *testing.T) {
		fsys := fstest.MapFS{
			"private/reservations/asyncapi.yaml": {Data: spec("private.reservations.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		assert.Empty(t, violations)
	})

	t.Run("should report a violation when a public address has a wrong prefix", func(t *testing.T) {
		fsys := fstest.MapFS{
			"public/reservations/asyncapi.yaml": {Data: spec("movie-reservation-system.reservations.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		require.Len(t, violations, 1)
		assert.Equal(t, "public/reservations/asyncapi.yaml", violations[0].File)
		assert.Equal(t, "movie-reservation-system.reservations.example-event", violations[0].Address)
		assert.Equal(t, "public", violations[0].RequiredPrefix)
	})

	t.Run("should report a violation when a private address has a wrong prefix", func(t *testing.T) {
		fsys := fstest.MapFS{
			"private/reservations/asyncapi.yaml": {Data: spec("movie-reservation-system.internal.reservations.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		require.Len(t, violations, 1)
		assert.Equal(t, "private/reservations/asyncapi.yaml", violations[0].File)
		assert.Equal(t, "private", violations[0].RequiredPrefix)
	})

	t.Run("should report all violations across multiple files", func(t *testing.T) {
		fsys := fstest.MapFS{
			"public/reservations/asyncapi.yaml":  {Data: spec("wrong.reservations.example-event")},
			"private/reservations/asyncapi.yaml": {Data: spec("wrong.reservations.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		assert.Len(t, violations, 2)
	})

	t.Run("should resolve prefix correctly for deeply nested context paths", func(t *testing.T) {
		fsys := fstest.MapFS{
			"public/reservations/inventory/asyncapi.yaml": {Data: spec("public.reservations.inventory.example-event")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		assert.Empty(t, violations)
	})

	t.Run("should ignore asyncapi.yaml files outside public/ and private/", func(t *testing.T) {
		fsys := fstest.MapFS{
			"asyncapi.yaml": {Data: spec("anything.goes.here")},
		}
		violations, err := buildLinter(fsys).Lint()
		require.NoError(t, err)
		assert.Empty(t, violations)
	})

	t.Run("should return an error for malformed YAML", func(t *testing.T) {
		fsys := fstest.MapFS{
			"public/reservations/asyncapi.yaml": {Data: []byte(`channels: [invalid: yaml: here`)},
		}
		_, err := buildLinter(fsys).Lint()
		assert.Error(t, err)
	})
}
