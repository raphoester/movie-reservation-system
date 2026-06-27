package xwatermill

import "github.com/ThreeDotsLabs/watermill/message/router/middleware"

func WithRetry(r middleware.Retry) RouterOption {
	return func(c *routerConfig) { c.retry = r }
}

// WithDLQ enables the poison queue middleware: messages that exhaust retries are
// published to "{original_topic}:dlq" instead of being NACKed back to the PEL.
func WithDLQ() RouterOption {
	return func(c *routerConfig) { c.dlq = true }
}
