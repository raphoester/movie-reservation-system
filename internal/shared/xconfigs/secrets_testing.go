package xconfigs

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type InMemorySecretManager struct {
	secrets map[string]string
	mu      sync.Mutex
}

func NewInMemorySecretManager(secrets map[string]string) *InMemorySecretManager {
	return &InMemorySecretManager{secrets: secrets, mu: sync.Mutex{}}
}

func (m *InMemorySecretManager) AddSecret(name, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[name] = value
}

func (m *InMemorySecretManager) secretScheme() string {
	return "inmem://"
}

func (m *InMemorySecretManager) AccessSecretCallbackIfEligible(identifier string) (StringCallback, error) {
	secretName, err := m.parseIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	return func(context.Context) (string, error) {
		return m.getValue(secretName)
	}, nil
}

func (m *InMemorySecretManager) getValue(secretName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, ok := m.secrets[secretName]
	if !ok {
		return "", fmt.Errorf("secret not found for name: %s", secretName)
	}
	return secret, nil
}

func (m *InMemorySecretManager) AccessSecretIfEligible(_ context.Context, identifier string) (string, error) {
	secretName, err := m.parseIdentifier(identifier)
	if err != nil {
		return "", err
	}

	return m.getValue(secretName)
}

func (m *InMemorySecretManager) parseIdentifier(identifier string) (secretName string, err error) {
	secretName = strings.TrimPrefix(identifier, m.secretScheme())
	if secretName == identifier {
		return "", fmt.Errorf("identifier does not start with expected scheme %s: %w", m.secretScheme(), ErrNotEligible)
	}
	return secretName, nil
}
