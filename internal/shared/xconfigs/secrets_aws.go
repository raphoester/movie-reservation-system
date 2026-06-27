package xconfigs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smithy "github.com/aws/smithy-go"
)

// NewAWSSecretManagerFromConfig creates an AWSSecretManager backed by the given aws.Config.
func NewAWSSecretManagerFromConfig(cfg *aws.Config) *AWSSecretManager {
	return NewAWSSecretManager(NewAwsCache(NewAWSClientAdapter(secretsmanager.NewFromConfig(*cfg))))
}

type awsSecretDataReader interface {
	getSecret(ctx context.Context, secretName string) (map[string]string, error)
}

type AWSClientAdapter struct {
	client *secretsmanager.Client
}

func NewAWSClientAdapter(client *secretsmanager.Client) *AWSClientAdapter {
	return &AWSClientAdapter{client: client}
}

func (a *AWSClientAdapter) getSecret(ctx context.Context, secretName string) (map[string]string, error) {
	result, err := a.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AccessDeniedException" {
			return nil, fmt.Errorf("%w: %s", ErrAWSAccessDenied, secretName)
		}
		return nil, fmt.Errorf("failed to retrieve secret from AWS Secrets Manager: %w", err)
	}

	if result.SecretString == nil {
		return nil, fmt.Errorf("secret value is empty for secret %q", secretName)
	}

	var secretData map[string]string
	if err := json.Unmarshal([]byte(*result.SecretString), &secretData); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	return secretData, nil
}

type AwsSecretManagerCache struct {
	inner   awsSecretDataReader
	mu      sync.Mutex
	secrets map[string]map[string]string
}

func NewAwsCache(inner awsSecretDataReader) *AwsSecretManagerCache {
	return &AwsSecretManagerCache{
		inner:   inner,
		secrets: make(map[string]map[string]string),
		mu:      sync.Mutex{},
	}
}

func (c *AwsSecretManagerCache) getSecret(ctx context.Context, secretName string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.secrets[secretName]; ok {
		return maps.Clone(cached), nil
	}

	data, err := c.inner.getSecret(ctx, secretName)
	if err != nil {
		return nil, err
	}

	c.secrets[secretName] = data
	return maps.Clone(data), nil
}

type AWSSecretManager struct {
	reader awsSecretDataReader
}

func (s *AWSSecretManager) AccessSecretCallbackIfEligible(identifier string) (StringCallback, error) {
	secretName, secretKey, err := s.parseIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) (string, error) {
		return s.readSecretValueForKey(ctx, secretName, secretKey)
	}, nil
}

var _ SecretManager = (*AWSSecretManager)(nil)

var _ SecretManager = (*InMemorySecretManager)(nil)

func (s *AWSSecretManager) parseIdentifier(identifier string) (secretName, secretKey string, err error) {
	withoutScheme := strings.TrimPrefix(identifier, s.secretScheme())
	if withoutScheme == identifier {
		return "", "", fmt.Errorf("identifier does not start with expected scheme %s: %w", s.secretScheme(), ErrNotEligible)
	}

	secretName, secretKey, found := strings.Cut(withoutScheme, ":")
	if !found || secretName == "" || secretKey == "" {
		return "", "", fmt.Errorf("%w; got %s", ErrAwsMalformedKey, identifier)
	}

	return secretName, secretKey, nil
}

func (s *AWSSecretManager) readSecretValueForKey(ctx context.Context, secretName, secretKey string) (string, error) {
	secretData, err := s.reader.getSecret(ctx, secretName)
	if err != nil {
		return "", err
	}

	value, ok := secretData[secretKey]
	if !ok {
		return "", fmt.Errorf("%w: key %q not found in secret %q", ErrAWSKeyNotFound, secretKey, secretName)
	}

	return value, nil
}

func (s *AWSSecretManager) AccessSecretIfEligible(ctx context.Context, identifier string) (string, error) {
	secretName, secretKey, err := s.parseIdentifier(identifier)
	if err != nil {
		return "", err
	}

	return s.readSecretValueForKey(ctx, secretName, secretKey)
}

var (
	ErrAWSKeyNotFound  = errors.New("key not found in AWS secret")
	ErrAwsMalformedKey = errors.New("malformed key for AWS secret; expected format is awssm://<secretName>:<key>")
	ErrAWSAccessDenied = errors.New("access denied to AWS Secrets Manager")
)

func (s *AWSSecretManager) SecretIdentifier(secretName, key string) string {
	return fmt.Sprintf("%s%s:%s", s.secretScheme(), secretName, key)
}

func (s *AWSSecretManager) secretScheme() string {
	return "awssm://"
}

func NewAWSSecretManager(reader awsSecretDataReader) *AWSSecretManager {
	return &AWSSecretManager{reader: reader}
}
