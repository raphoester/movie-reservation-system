//go:build integration

package xconfigs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertSecretKey(t *testing.T) {
	awsCfg := awsSession(t)

	client := secretsmanager.NewFromConfig(awsCfg)

	secretName := func() string {
		return fmt.Sprintf("movie-reservation-system/integration-test/%d", time.Now().UnixNano())
	}

	createSecret := func(ctx context.Context, t *testing.T, name string, vals map[string]string) {
		t.Helper()
		b, err := json.Marshal(vals)
		require.NoError(t, err)
		_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(name),
			SecretString: aws.String(string(b)),
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.DeleteSecret(t.Context(), &secretsmanager.DeleteSecretInput{
				SecretId:                   aws.String(name),
				ForceDeleteWithoutRecovery: aws.Bool(true),
			})
		})
	}

	createEmptySecret := func(ctx context.Context, t *testing.T, name string) {
		t.Helper()
		_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{Name: aws.String(name)})
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = client.DeleteSecret(t.Context(), &secretsmanager.DeleteSecretInput{
				SecretId:                   aws.String(name),
				ForceDeleteWithoutRecovery: aws.Bool(true),
			})
		})
	}

	t.Run("caseA: secret does not exist — returns actionable error", func(t *testing.T) {
		ctx := t.Context()
		identifier := fmt.Sprintf("awssm://%s:password", secretName())

		err := xconfigs.UpsertSecretKey(ctx, &awsCfg, identifier, "hunter2")

		assert.ErrorContains(t, err, "does not exist")
		assert.ErrorContains(t, err, "Terraform")
	})

	t.Run("caseA2: secret exists but has no versions yet — succeeds", func(t *testing.T) {
		ctx := t.Context()
		name := secretName()
		createEmptySecret(ctx, t, name)

		err := xconfigs.UpsertSecretKey(ctx, &awsCfg, fmt.Sprintf("awssm://%s:password", name), "hunter2")
		require.NoError(t, err)

		out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
		require.NoError(t, err)
		var stored map[string]string
		require.NoError(t, json.Unmarshal([]byte(*out.SecretString), &stored))
		assert.Equal(t, "hunter2", stored["password"])
	})

	t.Run("caseB: key absent — writes key, preserves other keys", func(t *testing.T) {
		ctx := t.Context()
		name := secretName()
		createSecret(ctx, t, name, map[string]string{"username": "alice"})

		err := xconfigs.UpsertSecretKey(ctx, &awsCfg, fmt.Sprintf("awssm://%s:password", name), "hunter2")
		require.NoError(t, err)

		out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
		require.NoError(t, err)
		var stored map[string]string
		require.NoError(t, json.Unmarshal([]byte(*out.SecretString), &stored))
		assert.Equal(t, "hunter2", stored["password"])
		assert.Equal(t, "alice", stored["username"])
	})

	t.Run("caseC: key set to replace-me — overwrites value", func(t *testing.T) {
		ctx := t.Context()
		name := secretName()
		createSecret(ctx, t, name, map[string]string{"password": "replace-me"})

		err := xconfigs.UpsertSecretKey(ctx, &awsCfg, fmt.Sprintf("awssm://%s:password", name), "hunter2")
		require.NoError(t, err)

		out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
		require.NoError(t, err)
		var stored map[string]string
		require.NoError(t, json.Unmarshal([]byte(*out.SecretString), &stored))
		assert.Equal(t, "hunter2", stored["password"])
	})
}
