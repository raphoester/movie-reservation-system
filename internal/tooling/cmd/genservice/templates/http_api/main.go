// {{.Header}}

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/raphoester/movie-reservation-system/internal/shared/xbootstrap"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

func main() {
	if err := runServer(); err != nil {
		xlog.QuickNew().Error("failed to run service", "error", err)
		os.Exit(1)
	}
}

func runServer() error {
	if err := xbootstrap.OAPIHTTPServer(
		&Config{},
		func(_ context.Context, _ xbootstrap.OAPIHTTPDiSequenceProps[*Config]) (any, error) {
			// TODO: Replace 'any' with your generated StrictServerInterface type and
			// return your implementation. Example:
			//   func(...) (generated.StrictServerInterface, error) {
			//       return myServer, nil
			//   }
			return nil, fmt.Errorf("not implemented")
		},
		func(_ any, _ *http.ServeMux) {
			// TODO: Replace with generated.RegisterRoutes once your spec package exists.
		},
	); err != nil {
		return fmt.Errorf("failed to run HTTP server: %w", err)
	}

	return nil
}
