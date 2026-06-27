package xhttpsrv

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/contracts"
)

func registerSwaggerHandlersIfNeeded(mux *http.ServeMux, swagger *Swagger, logger Logger) error {
	if swagger == nil {
		return nil
	}
	tmplContent, err := fs.ReadFile(assets.HTML(), "swagger.html")
	if err != nil {
		return fmt.Errorf("failed to read swagger template: %w", err)
	}

	swaggerTmpl, err := template.New("swagger").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse swagger template: %w", err)
	}

	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, contracts.OAPI(), swagger.OpenAPI)
	})

	mux.HandleFunc("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := swaggerTmpl.Execute(w, swagger); err != nil {
			logger.Error("failed to execute swagger template", "error", err)
		}
	})

	return nil
}
