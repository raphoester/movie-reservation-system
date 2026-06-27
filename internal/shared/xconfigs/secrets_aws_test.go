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

func TestAWSSecretManager(t *testing.T) {
	buildNewClient := func(t *testing.T) *secretsmanager.Client {
		t.Helper()
		return secretsmanager.NewFromConfig(awsSession(t))
	}

	// buildSM creates a manager backed directly by the given client, without a cache layer.
	// Use this for all tests except those explicitly testing caching behaviour.
	buildSM := func(client *secretsmanager.Client) *xconfigs.AWSSecretManager {
		return xconfigs.NewAWSSecretManager(xconfigs.NewAWSClientAdapter(client))
	}

	getTestSecretName := func() string {
		return fmt.Sprintf(
			"movie-reservation-system/integration-test/%d",
			// pay attention to this slight source of flakiness
			time.Now().UnixNano(),
		)
	}

	createSecretOnAws := func(
		ctx context.Context,
		t *testing.T,
		client *secretsmanager.Client,
		secretName string,
		vals map[string]string,
	) {
		t.Helper()
		secretJSON, err := json.Marshal(vals)
		require.NoError(t, err)

		_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secretName),
			SecretString: aws.String(string(secretJSON)),
		})
		require.NoError(t, err)
	}

	deleteSecretOnAws := func(ctx context.Context, t *testing.T, client *secretsmanager.Client, secretName string) {
		t.Helper()
		_, err := client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
		require.NoError(t, err)
	}

	t.Run("should return ErrNotEligible for identifiers that do not match the expected scheme", func(t *testing.T) {
		sm := buildSM(secretsmanager.NewFromConfig(aws.Config{}))

		_, err := sm.AccessSecretIfEligible(t.Context(), "plain-value-without-scheme")
		assert.ErrorIs(t, err, xconfigs.ErrNotEligible)
	})

	t.Run("should return ErrAwsMalformedKey for identifiers with the correct scheme but missing keys", func(t *testing.T) {
		sm := buildSM(secretsmanager.NewFromConfig(aws.Config{}))

		_, err := sm.AccessSecretIfEligible(t.Context(), "awssm://no-key")
		assert.ErrorIs(t, err, xconfigs.ErrAwsMalformedKey)
	})

	t.Run("should return ErrAwsMalformedKey when secretName or secretKey is empty", func(t *testing.T) {
		// Validation fires before any AWS call, so a zero-config client is sufficient.
		sm := buildSM(secretsmanager.NewFromConfig(aws.Config{}))

		cases := []struct {
			name       string
			identifier string
		}{
			{name: "empty secret key", identifier: "awssm://my-secret:"},
			{name: "empty secret name", identifier: "awssm://:my-key"},
			{name: "both empty", identifier: "awssm://:"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := sm.AccessSecretIfEligible(t.Context(), tc.identifier)
				assert.ErrorIs(t, err, xconfigs.ErrAwsMalformedKey)
			})
		}
	})

	t.Run("should return the value for a matching key in the secret", func(t *testing.T) {
		ctx := t.Context()
		client := buildNewClient(t)
		secretName := getTestSecretName()
		key := "db-password"
		insertedValue := "secret-db-pass"

		createSecretOnAws(ctx, t, client, secretName, map[string]string{key: insertedValue})
		t.Cleanup(func() { deleteSecretOnAws(t.Context(), t, client, secretName) })

		sm := buildSM(client)

		retrievedValue, err := sm.AccessSecretIfEligible(ctx, sm.SecretIdentifier(secretName, key))
		require.NoError(t, err)
		assert.Equal(t, insertedValue, retrievedValue)
	})

	t.Run("should return an error when the secret does not exist", func(t *testing.T) {
		ctx := t.Context()
		client := buildNewClient(t)
		secretName := getTestSecretName()
		key := "db-password"

		sm := buildSM(client)

		_, err := sm.AccessSecretIfEligible(ctx, sm.SecretIdentifier(secretName, key))
		assert.Error(t, err)
	})

	t.Run("should return an error when the secret does not have a matching key in the secret", func(t *testing.T) {
		ctx := t.Context()
		client := buildNewClient(t)
		secretName := getTestSecretName()
		nonExistentKey := "api-key"

		createSecretOnAws(ctx, t, client, secretName, nil)
		t.Cleanup(func() { deleteSecretOnAws(t.Context(), t, client, secretName) })

		sm := buildSM(client)

		_, err := sm.AccessSecretIfEligible(ctx, sm.SecretIdentifier(secretName, nonExistentKey))
		assert.ErrorIs(t, err, xconfigs.ErrAWSKeyNotFound)
	})

	t.Run("should serve subsequent keys from the same secret without an additional AWS call", func(t *testing.T) {
		ctx := t.Context()
		client := buildNewClient(t)
		secretName := getTestSecretName()
		key1 := "key-one"
		key2 := "key-two"
		value1 := "value-one"
		value2 := "value-two"

		createSecretOnAws(ctx, t, client, secretName, map[string]string{key1: value1, key2: value2})

		// Explicitly construct with cache — this is the behaviour under test.
		sm := xconfigs.NewAWSSecretManager(xconfigs.NewAwsCache(xconfigs.NewAWSClientAdapter(client)))

		// First access fetches from AWS and populates the cache.
		result1, err := sm.AccessSecretIfEligible(ctx, sm.SecretIdentifier(secretName, key1))
		require.NoError(t, err)
		assert.Equal(t, value1, result1)

		// Delete the secret so any subsequent AWS call would fail.
		deleteSecretOnAws(ctx, t, client, secretName)

		// Second access for a different key in the same secret must be served from cache.
		result2, err := sm.AccessSecretIfEligible(ctx, sm.SecretIdentifier(secretName, key2))
		require.NoError(t, err)
		assert.Equal(t, value2, result2)
	})

	t.Run("AccessSecretCallbackIfEligible", func(t *testing.T) {
		t.Run("should return ErrNotEligible for identifiers that do not match the expected scheme", func(t *testing.T) {
			sm := buildSM(secretsmanager.NewFromConfig(aws.Config{}))

			_, err := sm.AccessSecretCallbackIfEligible("plain-value-without-scheme")
			assert.ErrorIs(t, err, xconfigs.ErrNotEligible)
		})

		t.Run("should return ErrAwsMalformedKey for identifiers with the correct scheme but missing key separator", func(t *testing.T) {
			sm := buildSM(secretsmanager.NewFromConfig(aws.Config{}))

			_, err := sm.AccessSecretCallbackIfEligible("awssm://no-key")
			assert.ErrorIs(t, err, xconfigs.ErrAwsMalformedKey)
		})

		t.Run("should not call AWS at callback creation time", func(t *testing.T) {
			ctx := t.Context()
			client := buildNewClient(t)
			secretName := getTestSecretName() // intentionally never created in AWS
			key := "db-password"
			sm := buildSM(client)

			// Creating the callback for a non-existent secret must succeed — no AWS call is made here.
			cb, err := sm.AccessSecretCallbackIfEligible(sm.SecretIdentifier(secretName, key))
			require.NoError(t, err)

			// The AWS call — and its error — is deferred to callback invocation.
			_, err = cb(ctx)
			assert.Error(t, err)
		})

		t.Run("should resolve the value via callback when invoked", func(t *testing.T) {
			ctx := t.Context()
			client := buildNewClient(t)
			secretName := getTestSecretName()
			key := "db-password"
			insertedValue := "secret-db-pass"

			createSecretOnAws(ctx, t, client, secretName, map[string]string{key: insertedValue})
			t.Cleanup(func() { deleteSecretOnAws(t.Context(), t, client, secretName) })

			sm := buildSM(client)

			cb, err := sm.AccessSecretCallbackIfEligible(sm.SecretIdentifier(secretName, key))
			require.NoError(t, err)

			retrievedValue, err := cb(ctx)
			require.NoError(t, err)
			assert.Equal(t, insertedValue, retrievedValue)
		})

		t.Run("should return ErrAWSKeyNotFound via callback when key is not in the secret", func(t *testing.T) {
			ctx := t.Context()
			client := buildNewClient(t)
			secretName := getTestSecretName()
			nonExistentKey := "api-key"

			createSecretOnAws(ctx, t, client, secretName, nil)
			t.Cleanup(func() { deleteSecretOnAws(t.Context(), t, client, secretName) })

			sm := buildSM(client)

			cb, err := sm.AccessSecretCallbackIfEligible(sm.SecretIdentifier(secretName, nonExistentKey))
			require.NoError(t, err)

			_, err = cb(ctx)
			assert.ErrorIs(t, err, xconfigs.ErrAWSKeyNotFound)
		})
	})
}
