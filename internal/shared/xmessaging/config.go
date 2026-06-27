package xmessaging

import (
	"errors"
	"fmt"
)

const (
	MessagingProviderSNSSQS = "sns_sqs"
	MessagingProviderRedis  = "redis"
)

// RedisConfig holds structured Redis connection parameters.
type RedisConfig struct {
	Protocol string
	Host     string
	Port     int
	Username string
	Password string
	TLS      bool
}

// MessagingConfig holds messaging provider configuration shared across services.
// Embed this struct with mapstructure:",squash" to keep YAML keys flat.
type MessagingConfig struct {
	MessagingProvider string
	Redis             RedisConfig
	TopicARN          map[string]string
}

func (c MessagingConfig) Validate() error {
	switch c.MessagingProvider {
	case MessagingProviderSNSSQS:
		if len(c.TopicARN) == 0 {
			return errors.New("topic ARN map is required for sns_sqs messaging provider")
		}

	case MessagingProviderRedis:
		if c.Redis.Protocol != "tcp" && c.Redis.Protocol != "unix" {
			return fmt.Errorf("redis protocol must be tcp or unix, got %q", c.Redis.Protocol)
		}
		if c.Redis.Host == "" {
			return errors.New("redis host is required for redis messaging provider")
		}
		if c.Redis.Port == 0 {
			return errors.New("redis port is required for redis messaging provider")
		}

	default:
		return fmt.Errorf("unknown messaging provider: %s", c.MessagingProvider)
	}

	return nil
}
