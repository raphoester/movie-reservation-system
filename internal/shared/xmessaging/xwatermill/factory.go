package xwatermill

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/raphoester/movie-reservation-system/internal/shared/xbootstrap"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhealth"
	"github.com/raphoester/movie-reservation-system/internal/shared/xmessaging"
)

// NewPublisher creates a message.Publisher for the configured provider,
// registers cleanup with closable, and registers a health check.
func NewPublisher(
	ctx context.Context,
	cfg xmessaging.MessagingConfig,
	logger *slog.Logger,
	closable xbootstrap.FnRegistrar,
	health *xhealth.HealthRegistry,
) (message.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)

	switch cfg.MessagingProvider {
	case xmessaging.MessagingProviderRedis:
		redisClient, err := NewRedisClient(ctx, cfg.Redis)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to redis: %w", err)
		}
		closable.RegisterCloser(redisClient)
		logger.Info("connected to redis")
		health.Register(xhealth.NewRedisCheck("redis", redisClient), xhealth.WithInterval(15*time.Second))

		publisher, err := NewRedisPublisher(redisClient, wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis publisher: %w", err)
		}
		closable.RegisterCloser(publisher)
		logger.Info("using redis publisher")
		return publisher, nil

	case xmessaging.MessagingProviderSNSSQS:
		awsCfg, err := LoadAWSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		publisher, err := NewSNSPublisher(awsCfg, SNSTopicResolver(cfg.TopicARN), wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SNS publisher: %w", err)
		}
		closable.RegisterCloser(publisher)
		logger.Info("using SNS publisher")
		return publisher, nil

	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.MessagingProvider)
	}
}

// NewPlatformEventsPublisher creates a Redis publisher that writes stream entries
// in the format expected by platform-ops consumers (eventType + payload fields).
// Only valid for the Redis messaging provider.
func NewPlatformEventsPublisher(
	ctx context.Context,
	cfg xmessaging.MessagingConfig,
	logger *slog.Logger,
	closable xbootstrap.FnRegistrar,
) (message.Publisher, error) {
	if cfg.MessagingProvider != xmessaging.MessagingProviderRedis {
		return nil, fmt.Errorf("platform events publisher is only supported for the Redis messaging provider")
	}

	wmLogger := watermill.NewSlogLogger(logger)

	redisClient, err := NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	closable.RegisterCloser(redisClient)

	publisher, err := NewPlatformEventsRedisPublisher(redisClient, wmLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create platform events redis publisher: %w", err)
	}
	closable.RegisterCloser(publisher)
	logger.Info("using platform events redis publisher")
	return publisher, nil
}

type RouterOption func(*routerConfig)

type routerConfig struct {
	retry middleware.Retry
	dlq   bool
}

// NewRouter creates a Watermill router pre-wired with Retry (exponential backoff).
// Messages that exhaust retries are NACKed and remain in the PEL for redelivery,
// keeping failures loud in logs until the root cause is resolved.
// Pass WithDLQ() to route exhausted messages to "{original_topic}:dlq" instead.
func NewRouter(
	ctx context.Context,
	cfg xmessaging.MessagingConfig,
	logger *slog.Logger,
	closable xbootstrap.FnRegistrar,
	opts ...RouterOption,
) (*message.Router, error) {
	wmLogger := watermill.NewSlogLogger(logger)

	rc := routerConfig{
		retry: middleware.Retry{
			MaxElapsedTime:  5 * time.Minute,
			InitialInterval: 1 * time.Second,
			MaxInterval:     30 * time.Second,
			Multiplier:      2,
		},
	}
	for _, opt := range opts {
		opt(&rc)
	}
	rc.retry.Logger = wmLogger

	router, err := message.NewRouter(message.RouterConfig{}, wmLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	if rc.dlq {
		innerPublisher, err := newPoisonPublisher(ctx, cfg, logger, closable)
		if err != nil {
			return nil, fmt.Errorf("failed to create poison publisher: %w", err)
		}
		dlqRouter := &dlqRoutingPublisher{inner: innerPublisher}
		poisonQueueMiddleware, err := middleware.PoisonQueue(dlqRouter, ":dlq")
		if err != nil {
			return nil, fmt.Errorf("failed to create poison queue middleware: %w", err)
		}
		router.AddMiddleware(poisonQueueMiddleware)
	}

	router.AddMiddleware(rc.retry.Middleware)

	return router, nil
}

// dlqRoutingPublisher appends the topic argument (used as a suffix) to the original source
// topic from PoisonedTopicKey metadata, routing each message to "{original_topic}{suffix}".
type dlqRoutingPublisher struct {
	inner message.Publisher
}

func (p *dlqRoutingPublisher) Publish(suffix string, msgs ...*message.Message) error {
	for _, msg := range msgs {
		dlqTopic := msg.Metadata.Get(middleware.PoisonedTopicKey) + suffix
		if err := p.inner.Publish(dlqTopic, msg); err != nil {
			return fmt.Errorf("publish to DLQ topic %q: %w", dlqTopic, err)
		}
	}
	return nil
}

func (p *dlqRoutingPublisher) Close() error {
	return p.inner.Close()
}

func newPoisonPublisher(
	ctx context.Context,
	cfg xmessaging.MessagingConfig,
	logger *slog.Logger,
	closable xbootstrap.FnRegistrar,
) (message.Publisher, error) {
	wmLogger := watermill.NewSlogLogger(logger)

	switch cfg.MessagingProvider {
	case xmessaging.MessagingProviderRedis:
		redisClient, err := NewRedisClient(ctx, cfg.Redis)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to redis: %w", err)
		}
		closable.RegisterCloser(redisClient)

		publisher, err := NewRedisPublisher(redisClient, wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis publisher: %w", err)
		}
		closable.RegisterCloser(publisher)
		return publisher, nil

	case xmessaging.MessagingProviderSNSSQS:
		awsCfg, err := LoadAWSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		publisher, err := NewSNSPublisher(awsCfg, SNSTopicResolver(cfg.TopicARN), wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SNS publisher: %w", err)
		}
		closable.RegisterCloser(publisher)
		return publisher, nil

	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.MessagingProvider)
	}
}

// NewSubscriber creates a message.Subscriber for the configured provider,
// registers cleanup with closable, and registers a health check.
func NewSubscriber(
	ctx context.Context,
	cfg xmessaging.MessagingConfig,
	consumerGroup string,
	logger *slog.Logger,
	closable xbootstrap.FnRegistrar,
	health *xhealth.HealthRegistry,
) (message.Subscriber, error) {
	wmLogger := watermill.NewSlogLogger(logger)

	switch cfg.MessagingProvider {
	case xmessaging.MessagingProviderSNSSQS:
		awsCfg, err := LoadAWSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		subscriber, err := NewSNSSQSSubscriber(awsCfg, SNSTopicResolver(cfg.TopicARN), consumerGroup, wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQS subscriber: %w", err)
		}
		closable.RegisterCloser(subscriber)
		logger.Info("using SNS/SQS subscriber")
		return subscriber, nil

	case xmessaging.MessagingProviderRedis:
		redisClient, err := NewRedisClient(ctx, cfg.Redis)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to redis: %w", err)
		}
		closable.RegisterCloser(redisClient)
		logger.Info("connected to redis")
		health.Register(xhealth.NewRedisCheck("redis_sub", redisClient), xhealth.WithInterval(15*time.Second))

		subscriber, err := NewRedisSubscriber(redisClient, consumerGroup, wmLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis subscriber: %w", err)
		}
		closable.RegisterCloser(subscriber)
		logger.Info("using redis subscriber")
		return subscriber, nil

	default:
		return nil, fmt.Errorf("unsupported messaging provider: %s", cfg.MessagingProvider)
	}
}
