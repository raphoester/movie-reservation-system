package xwatermill

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	watermillsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/delay"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

// PostgreSQLOption configures PostgreSQL publisher and subscriber constructors.
type PostgreSQLOption func(*postgresConfig)

type postgresConfig struct {
	schema string
}

// WithSchema places outbox tables in the given PostgreSQL schema instead of the default
// public schema. Table names are simplified to outbox, outbox_offsets, and delayed_<topic>.
func WithSchema(schema string) PostgreSQLOption {
	return func(c *postgresConfig) {
		c.schema = schema
	}
}

func applyPostgreSQLOptions(opts []PostgreSQLOption) postgresConfig {
	c := postgresConfig{}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func NewOutboxPublisher(tx *sql.Tx, logger watermill.LoggerAdapter, opts ...PostgreSQLOption) (*watermillsql.Publisher, error) {
	cfg := applyPostgreSQLOptions(opts)

	adapter := watermillsql.DefaultPostgreSQLSchema{
		GenerateMessagesTableName:          nil,
		GeneratePayloadType:                nil,
		SubscribeBatchSize:                 0,
		InitializeSchemaWithoutTransaction: false,
		InitializeSchemaLock:               0,
	}
	if cfg.schema != "" {
		adapter.GenerateMessagesTableName = func(_ string) string {
			return fmt.Sprintf(`"%s"."outbox"`, cfg.schema)
		}
	}

	pub, err := watermillsql.NewPublisher(
		watermillsql.TxFromStdSQL(tx),
		watermillsql.PublisherConfig{
			SchemaAdapter:        adapter,
			AutoInitializeSchema: false,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox publisher: %w", err)
	}
	return pub, nil
}

func WrapWithForwarder(publisher message.Publisher, forwarderTopic string) *forwarder.Publisher {
	return forwarder.NewPublisher(publisher, forwarder.PublisherConfig{
		ForwarderTopic: forwarderTopic,
	})
}

func NewDelayedQueuePublisher(tx *sql.Tx, logger watermill.LoggerAdapter, opts ...PostgreSQLOption) (*watermillsql.Publisher, error) {
	cfg := applyPostgreSQLOptions(opts)

	pub, err := watermillsql.NewPublisher(
		watermillsql.TxFromStdSQL(tx),
		watermillsql.PublisherConfig{
			SchemaAdapter: watermillsql.PostgreSQLQueueSchema{
				GenerateMessagesTableName: delayedQueueTableName(cfg),
				GenerateWhereClause:       nil,
				GeneratePayloadType:       nil,
				SubscribeBatchSize:        0,
			},
			AutoInitializeSchema: false,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create delayed queue publisher: %w", err)
	}
	return pub, nil
}

func NewOutboxSubscriber(conn watermillsql.SQLBeginner, logger watermill.LoggerAdapter, opts ...PostgreSQLOption) (*watermillsql.Subscriber, error) {
	cfg := applyPostgreSQLOptions(opts)

	adapter := watermillsql.DefaultPostgreSQLSchema{
		GenerateMessagesTableName:          nil,
		GeneratePayloadType:                nil,
		SubscribeBatchSize:                 0,
		InitializeSchemaWithoutTransaction: false,
		InitializeSchemaLock:               0,
	}
	offsetsAdapter := watermillsql.DefaultPostgreSQLOffsetsAdapter{
		GenerateMessagesOffsetsTableName: nil,
	}
	if cfg.schema != "" {
		adapter.GenerateMessagesTableName = func(_ string) string {
			return fmt.Sprintf(`"%s"."outbox"`, cfg.schema)
		}
		offsetsAdapter.GenerateMessagesOffsetsTableName = func(_ string) string {
			return fmt.Sprintf(`"%s"."outbox_offsets"`, cfg.schema)
		}
	}

	sub, err := watermillsql.NewSubscriber(
		watermillsql.BeginnerFromStdSQL(conn),
		watermillsql.SubscriberConfig{
			SchemaAdapter:    adapter,
			OffsetsAdapter:   offsetsAdapter,
			InitializeSchema: false,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbox subscriber: %w", err)
	}
	return sub, nil
}

// NewDelayedQueueSubscriber creates a subscriber for delayed queue tables. Messages are
// delivered per-topic only once their _watermill_delayed_until metadata timestamp has passed.
// Tables must be created via a database migration.
func NewDelayedQueueSubscriber(conn watermillsql.SQLBeginner, logger watermill.LoggerAdapter, opts ...PostgreSQLOption) (*watermillsql.Subscriber, error) {
	cfg := applyPostgreSQLOptions(opts)

	sub, err := watermillsql.NewSubscriber(
		watermillsql.BeginnerFromStdSQL(conn),
		watermillsql.SubscriberConfig{
			SchemaAdapter: watermillsql.PostgreSQLQueueSchema{
				GenerateMessagesTableName: delayedQueueTableName(cfg),
				GenerateWhereClause: func(_ watermillsql.GenerateWhereClauseParams) (string, []any) {
					return fmt.Sprintf("(metadata->>'%s')::timestamptz <= NOW()", delay.DelayedUntilKey), nil
				},
				GeneratePayloadType: nil,
				SubscribeBatchSize:  0,
			},
			OffsetsAdapter:   watermillsql.PostgreSQLQueueOffsetsAdapter{DeleteOnAck: true, GenerateMessagesTableName: delayedQueueTableName(cfg)},
			InitializeSchema: false,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create delayed queue subscriber: %w", err)
	}
	return sub, nil
}

// delayedQueueTableName returns a function mapping a topic to its delayed queue table name,
// e.g. topic "reservations.seat-hold.expiring" in schema "booking"
// becomes "booking"."delayed_reservations_seat_hold_expiring".
func delayedQueueTableName(cfg postgresConfig) func(topic string) string {
	return func(topic string) string {
		safeName := strings.NewReplacer(".", "_", "-", "_").Replace(topic)
		if cfg.schema != "" {
			return fmt.Sprintf(`"%s"."delayed_%s"`, cfg.schema, safeName)
		}
		return fmt.Sprintf(`"delayed_%s"`, safeName)
	}
}

func NewForwarder(
	outboxSubscriber message.Subscriber,
	targetPublisher message.Publisher,
	forwarderTopic string,
	logger watermill.LoggerAdapter,
) (*forwarder.Forwarder, error) {
	fwd, err := forwarder.NewForwarder(
		outboxSubscriber,
		targetPublisher,
		logger,
		forwarder.Config{
			ForwarderTopic:      forwarderTopic,
			AckWhenCannotUnwrap: false,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create forwarder: %w", err)
	}
	return fwd, nil
}

type Event interface {
	Marshalled() ([]byte, error)
	Topic() string
}

func CreateMessage(event Event) (*Message, error) {
	payload, err := event.Marshalled()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := message.NewMessage(uuid.New().String(), payload)
	msg.Metadata.Set("topic", event.Topic())

	return NewWatermillMessage(event.Topic(), msg), nil
}
