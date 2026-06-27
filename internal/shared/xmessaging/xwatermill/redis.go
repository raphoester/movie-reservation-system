package xwatermill

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/raphoester/movie-reservation-system/internal/shared/xmessaging"
	"github.com/redis/go-redis/v9"
)

func NewRedisPublisher(client redis.UniversalClient, logger watermill.LoggerAdapter) (*redisstream.Publisher, error) {
	pub, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:        client,
			DefaultMaxlen: 1000,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis publisher: %w", err)
	}
	return pub, nil
}

// PlatformEventsMarshaller writes Redis stream entries in the format expected by
// platform-ops consumers: a top-level "eventType" field and a "payload" field
// containing the JSON body. The eventType is read from the Watermill message
// metadata key "eventType".
type PlatformEventsMarshaller struct{}

func (PlatformEventsMarshaller) Marshal(_ string, msg *message.Message) (map[string]interface{}, error) {
	return map[string]interface{}{
		redisstream.UUIDHeaderKey: msg.UUID,
		"eventType":               msg.Metadata.Get("eventType"),
		"payload":                 []byte(msg.Payload),
	}, nil
}

func NewPlatformEventsRedisPublisher(client redis.UniversalClient, logger watermill.LoggerAdapter) (*redisstream.Publisher, error) {
	pub, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:        client,
			DefaultMaxlen: 1000,
			Marshaller:    PlatformEventsMarshaller{},
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OPM redis publisher: %w", err)
	}
	return pub, nil
}

func NewRedisSubscriber(
	client redis.UniversalClient,
	consumerGroup string,
	logger watermill.LoggerAdapter,
) (*redisstream.Subscriber, error) {
	sub, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:        client,
			ConsumerGroup: consumerGroup,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis subscriber: %w", err)
	}
	return sub, nil
}

func NewRedisClient(ctx context.Context, cfg xmessaging.RedisConfig) (*redis.Client, error) {
	opts := &redis.Options{
		Network:  cfg.Protocol,
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Username: cfg.Username,
		Password: cfg.Password,
	}
	if cfg.TLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}
