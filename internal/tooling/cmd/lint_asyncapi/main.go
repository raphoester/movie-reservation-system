package main

import (
	"fmt"
	"os"

	"github.com/raphoester/movie-reservation-system/internal/tooling/cmd/lint_asyncapi/internal/lintasyncapi"
)

func main() {
	linter := lintasyncapi.NewLinter(os.DirFS("contracts/asyncapi"))

	violations, err := linter.Lint()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-asyncapi: %v\n", err)
		os.Exit(1)
	}

	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "lint-asyncapi: %s\n", v)
	}

	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "lint-asyncapi: %d violation(s) found\n", len(violations))
		os.Exit(1)
	}

	fmt.Println("lint-asyncapi: all checks passed.")
}
