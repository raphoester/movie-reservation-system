// {{.Header}}

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/raphoester/movie-reservation-system/internal/shared/xbootstrap"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

func main() {
	if err := runJob(); err != nil {
		xlog.QuickNew().Error("failed to run job", "error", err)
		os.Exit(1)
	}
}

func runJob() error {
	if err := xbootstrap.Job(
		&Config{},
		func(ctx context.Context, props xbootstrap.JobProps[*Config]) error {
			// TODO: implement job logic
			_ = ctx
			_ = props
			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to run job: %w", err)
	}

	return nil
}
