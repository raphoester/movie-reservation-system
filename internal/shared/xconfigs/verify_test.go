package xconfigs_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type accessDeniedSecretManager struct{}

func (m *accessDeniedSecretManager) AccessSecretIfEligible(_ context.Context, identifier string) (string, error) {
	return "", fmt.Errorf("%w: %s", xconfigs.ErrAWSAccessDenied, identifier)
}

func (m *accessDeniedSecretManager) AccessSecretCallbackIfEligible(identifier string) (xconfigs.StringCallback, error) {
	return nil, fmt.Errorf("%w: %s", xconfigs.ErrAWSAccessDenied, identifier)
}

func TestVerifyConfigSecrets(t *testing.T) {
	sharedFS := fstest.MapFS{
		"base.yaml": &fstest.MapFile{Data: []byte("")},
		"dev.yaml":  &fstest.MapFile{Data: []byte("")},
	}

	writeServiceConfig := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		configDir := filepath.Join(dir, "configs")
		require.NoError(t, os.MkdirAll(configDir, 0o755))
		path := filepath.Join(configDir, "dev.yaml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	t.Run("should return an error when access to a secret is denied", func(t *testing.T) {
		ctx := t.Context()
		configPath := writeServiceConfig(t, "dbPassword: awssm://my-secret:key")

		_, err := xconfigs.VerifyConfigSecrets(ctx, sharedFS, "dev", configPath, &accessDeniedSecretManager{})

		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrAWSAccessDenied)
	})
}
