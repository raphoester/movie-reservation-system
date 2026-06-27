package xwatermill

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
)

// TopicRoutingPublisher dispatches messages to topic-specific publishers,
// falling back to a default publisher for topics without an explicit mapping.
type TopicRoutingPublisher struct {
	routes           map[string]message.Publisher
	defaultPublisher message.Publisher
}

func NewTopicRoutingPublisher(
	routes map[string]message.Publisher,
	defaultPublisher message.Publisher,
) *TopicRoutingPublisher {
	return &TopicRoutingPublisher{
		routes:           routes,
		defaultPublisher: defaultPublisher,
	}
}

func (p *TopicRoutingPublisher) Publish(topic string, messages ...*message.Message) error {
	pub := p.defaultPublisher
	if specific, ok := p.routes[topic]; ok {
		pub = specific
	}
	if err := pub.Publish(topic, messages...); err != nil {
		return fmt.Errorf("publish to topic %q: %w", topic, err)
	}
	return nil
}

func (p *TopicRoutingPublisher) Close() error { return nil }

var _ message.Publisher = (*TopicRoutingPublisher)(nil)
