// {{.Header}}

package main

import (
	"context"
	"fmt"

	"github.com/raphoester/movie-reservation-system/internal/shared/xbootstrap"
)

func main() {
	if err := runWorker(); err != nil {
		panic(err)
	}
}

func runWorker() error {
	if err := xbootstrap.Worker(
		&Config{},
		func(ctx context.Context, props xbootstrap.WorkerDiSequenceProps[*Config]) error {
			// TODO: Connect to Postgres
			// TODO: Create repositories and application layer
			// TODO: Set up watermill subscriber
			// TODO: Build router with AddConsumerHandler(...)
			// TODO: go router.Run(ctx); props.Closable.RegisterErr(router.Close)
			_ = ctx
			_ = props
			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to run worker: %w", err)
	}

	return nil
}
