package xconfigs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// UpsertSecretKey reads the JSON secret identified by an awssm://secretName:key
// identifier, sets key to value, and writes it back via PutSecretValue.
//
// The secret must already exist in AWS (managed by Terraform). AWS returns
// ResourceNotFoundException from GetSecretValue for both "secret missing" and
// "secret exists but has no versions yet". We disambiguate by proceeding to
// PutSecretValue: it succeeds for no-version secrets and returns 404 only when
// the secret truly does not exist.
func UpsertSecretKey(ctx context.Context, cfg *aws.Config, identifier, value string) error {
	secretName, key, err := parseAWSIdentifier(identifier)
	if err != nil {
		return err
	}

	client := secretsmanager.NewFromConfig(*cfg)

	existing := make(map[string]string)
	out, getErr := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})

	var notFound *types.ResourceNotFoundException
	switch {
	case getErr == nil:
		if out.SecretString != nil {
			if err := json.Unmarshal([]byte(*out.SecretString), &existing); err != nil {
				return fmt.Errorf("secret %q contains non-JSON or non-map value; refusing to overwrite: %w", secretName, err)
			}
		}
	case errors.As(getErr, &notFound):
		// May have no versions yet — PutSecretValue will disambiguate.
	default:
		return fmt.Errorf("get secret %q: %w", secretName, getErr)
	}

	existing[key] = value
	data, _ := json.Marshal(existing)

	_, putErr := client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(data)),
	})
	if errors.As(putErr, &notFound) {
		return fmt.Errorf("secret %q does not exist — create it via Terraform before filling values", secretName)
	}
	if putErr != nil {
		return fmt.Errorf("put secret %q: %w", secretName, putErr)
	}
	return nil
}

// parseAWSIdentifier splits an awssm://secretName:key identifier into its parts.
func parseAWSIdentifier(identifier string) (secretName, key string, err error) {
	without := strings.TrimPrefix(identifier, "awssm://")
	if without == identifier {
		return "", "", fmt.Errorf("%w; expected awssm://<secret>:<key>, got %q", ErrAwsMalformedKey, identifier)
	}
	secretName, key, found := strings.Cut(without, ":")
	if !found || secretName == "" || key == "" {
		return "", "", fmt.Errorf("%w; expected awssm://<secret>:<key>, got %q", ErrAwsMalformedKey, identifier)
	}
	return secretName, key, nil
}
