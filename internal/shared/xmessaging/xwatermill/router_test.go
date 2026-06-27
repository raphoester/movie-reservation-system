package xwatermill_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/stretchr/testify/suite"

	"github.com/raphoester/movie-reservation-system/internal/shared/xbootstrap"
	"github.com/raphoester/movie-reservation-system/internal/shared/xmessaging"
	"github.com/raphoester/movie-reservation-system/internal/shared/xmessaging/xwatermill"
	"github.com/raphoester/movie-reservation-system/internal/shared/xtestc"
)

func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(routerSuite))
}

type routerSuite struct {
	suite.Suite
	redis *xtestc.Redis
	cfg   xmessaging.MessagingConfig
}

func (s *routerSuite) SetupSuite() {
	s.redis = xtestc.BootstrapRedis(s.T())
	s.cfg = xmessaging.MessagingConfig{
		MessagingProvider: xmessaging.MessagingProviderRedis,
		Redis:             s.redis.Config(),
	}
}

func (s *routerSuite) SetupTest() {
	s.redis.SetupTest(s.T())
}

func (s *routerSuite) TearDownSuite() {
	s.redis.TearDownSuite(s.T())
}

func (s *routerSuite) TestSuccessfulMessage_NotSentToDLQ() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
	defer cancel()

	const topic = "test-topic"

	closable := &xbootstrap.ClosableRegistry{}
	defer func() { _ = closable.Close() }()

	router, err := xwatermill.NewRouter(ctx, s.cfg, slog.Default(), closable, xwatermill.WithDLQ(), xwatermill.WithRetry(middleware.Retry{
		MaxRetries:      3,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		Multiplier:      2,
	}))
	s.Require().NoError(err)

	subscriber, err := xwatermill.NewRedisSubscriber(s.redis.Client(), "consumer", watermill.NopLogger{})
	s.Require().NoError(err)

	processed := make(chan struct{})
	router.AddConsumerHandler("handler", topic, subscriber, func(_ *message.Message) error {
		close(processed)
		return nil
	})

	go func() { _ = router.Run(ctx) }()
	<-router.Running()

	publisher, err := xwatermill.NewRedisPublisher(s.redis.Client(), watermill.NopLogger{})
	s.Require().NoError(err)

	err = publisher.Publish(topic, message.NewMessage(watermill.NewUUID(), []byte("payload")))
	s.Require().NoError(err)

	select {
	case <-processed:
	case <-ctx.Done():
		s.Fail("timed out waiting for message to be processed")
	}

	exists, err := s.redis.Client().Exists(ctx, topic+":dlq").Result()
	s.Require().NoError(err)
	s.EqualValues(0, exists, "DLQ stream should not exist for a successfully processed message")
}

func (s *routerSuite) TestTransientFailure_SucceedsAfterRetries_NotSentToDLQ() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
	defer cancel()

	const topic = "test-topic"

	closable := &xbootstrap.ClosableRegistry{}
	defer func() { _ = closable.Close() }()

	router, err := xwatermill.NewRouter(ctx, s.cfg, slog.Default(), closable, xwatermill.WithDLQ(), xwatermill.WithRetry(middleware.Retry{
		MaxRetries:      3,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		Multiplier:      2,
	}))
	s.Require().NoError(err)

	subscriber, err := xwatermill.NewRedisSubscriber(s.redis.Client(), "consumer", watermill.NopLogger{})
	s.Require().NoError(err)

	var attempts atomic.Int32
	processed := make(chan struct{})
	router.AddConsumerHandler("handler", topic, subscriber, func(_ *message.Message) error {
		if attempts.Add(1) < 3 {
			return errors.New("transient error")
		}
		close(processed)
		return nil
	})

	go func() { _ = router.Run(ctx) }()
	<-router.Running()

	publisher, err := xwatermill.NewRedisPublisher(s.redis.Client(), watermill.NopLogger{})
	s.Require().NoError(err)

	err = publisher.Publish(topic, message.NewMessage(watermill.NewUUID(), []byte("payload")))
	s.Require().NoError(err)

	select {
	case <-processed:
	case <-ctx.Done():
		s.Fail("timed out waiting for message to succeed after retries")
	}

	s.EqualValues(3, attempts.Load())

	exists, err := s.redis.Client().Exists(ctx, topic+":dlq").Result()
	s.Require().NoError(err)
	s.EqualValues(0, exists, "DLQ stream should not exist when the handler eventually succeeds")
}

func (s *routerSuite) TestPoisonMessage_RoutedToDLQAfterExhaustingRetries() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 5*time.Second)
	defer cancel()

	const topic = "test-topic"

	closable := &xbootstrap.ClosableRegistry{}
	defer func() { _ = closable.Close() }()

	router, err := xwatermill.NewRouter(ctx, s.cfg, slog.Default(), closable, xwatermill.WithDLQ(), xwatermill.WithRetry(middleware.Retry{
		MaxRetries:      3,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		Multiplier:      2,
	}))
	s.Require().NoError(err)

	subscriber, err := xwatermill.NewRedisSubscriber(s.redis.Client(), "consumer", watermill.NopLogger{})
	s.Require().NoError(err)

	router.AddConsumerHandler("handler", topic, subscriber, func(_ *message.Message) error {
		return errors.New("unrecoverable error")
	})

	dlqSubscriber, err := xwatermill.NewRedisSubscriber(s.redis.Client(), "dlq-consumer", watermill.NopLogger{})
	s.Require().NoError(err)

	dlqMessages, err := dlqSubscriber.Subscribe(ctx, topic+":dlq")
	s.Require().NoError(err)

	go func() { _ = router.Run(ctx) }()
	<-router.Running()

	publisher, err := xwatermill.NewRedisPublisher(s.redis.Client(), watermill.NopLogger{})
	s.Require().NoError(err)

	original := message.NewMessage(watermill.NewUUID(), []byte("poison payload"))
	err = publisher.Publish(topic, original)
	s.Require().NoError(err)

	select {
	case dlqMsg := <-dlqMessages:
		dlqMsg.Ack()
		s.Equal(topic, dlqMsg.Metadata.Get(middleware.PoisonedTopicKey))
		s.Equal("poison payload", string(dlqMsg.Payload))
	case <-ctx.Done():
		s.Fail("timed out waiting for message to appear in DLQ")
	}
}
