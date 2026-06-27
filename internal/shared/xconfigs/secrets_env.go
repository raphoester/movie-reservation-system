package xconfigs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrEnvVarNotFound = errors.New("environment variable not set")

type EnvSecretManager struct{}

var _ SecretManager = (*EnvSecretManager)(nil)

func NewEnvSecretManager() *EnvSecretManager {
	return &EnvSecretManager{}
}

func (e *EnvSecretManager) secretScheme() string {
	return "env://"
}

func (e *EnvSecretManager) parseIdentifier(identifier string) (string, error) {
	key := strings.TrimPrefix(identifier, e.secretScheme())
	if key == identifier {
		return "", fmt.Errorf("identifier does not start with expected scheme %s: %w", e.secretScheme(), ErrNotEligible)
	}
	return key, nil
}

func (e *EnvSecretManager) AccessSecretIfEligible(_ context.Context, identifier string) (string, error) {
	key, err := e.parseIdentifier(identifier)
	if err != nil {
		return "", err
	}

	val, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrEnvVarNotFound, key)
	}

	return val, nil
}

func (e *EnvSecretManager) AccessSecretCallbackIfEligible(identifier string) (StringCallback, error) {
	key, err := e.parseIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return func(_ context.Context) (string, error) {
		val, ok := os.LookupEnv(key)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrEnvVarNotFound, key)
		}
		return val, nil
	}, nil
}
