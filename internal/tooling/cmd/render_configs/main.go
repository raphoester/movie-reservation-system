package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

var allEnvs = []string{"local", "docker", "dev", "staging", "prod"}

func main() {
	logger := xlog.NewForLocalConsole(xlog.Config{Level: "info"})
	if err := run(); err != nil {
		logger.Error("render-configs failed", "error", err)
		os.Exit(1)
	}
}

type FlagsConfig struct{}

func parseFlags() FlagsConfig {
	flag.Parse()
	return FlagsConfig{}
}

func run() error {
	parseFlags()

	services, err := discoverServices("internal")
	if err != nil {
		return fmt.Errorf("could not discover services: %w", err)
	}

	results := renderAll(services)

	ok := displayResults(results)
	if !ok {
		return fmt.Errorf("one or more services failed to render")
	}

	return nil
}

// discoverServices walks root and returns every directory that contains both a
// Dockerfile and a main.go, matching the same discovery logic as the Makefile SERVICES variable.
func discoverServices(root string) ([]string, error) {
	var services []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if fileExists(filepath.Join(path, "Dockerfile")) && fileExists(filepath.Join(path, "main.go")) {
			services = append(services, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}
	return services, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// renderKey identifies a single (service, env) combination to render.
type renderKey struct {
	ServiceDir string
	Env        string
}

type RenderResult struct {
	renderKey
	OutputPath string
	Err        error
}

// renderAll renders every (service × env) combination concurrently.
// Results are returned in the same order as the input pairs.
func renderAll(services []string) []RenderResult {
	pairs := make([]renderKey, 0, len(services)*len(allEnvs))
	for _, svc := range services {
		for _, env := range allEnvs {
			pairs = append(pairs, renderKey{ServiceDir: svc, Env: env})
		}
	}

	results := make([]RenderResult, len(pairs))
	var wg sync.WaitGroup
	for i, p := range pairs {
		wg.Go(func() {
			results[i] = renderOne(p)
		})
	}
	wg.Wait()
	return results
}

// renderOne merges the config for a single (service, env) pair and writes it to
// {serviceDir}/configs/{env}.resolved.yaml. Missing service configs (e.g. a service
// that has no dev.yaml) are skipped rather than treated as fatal errors.
func renderOne(key renderKey) RenderResult {
	configPath := filepath.Join(key.ServiceDir, "configs", key.Env+".yaml")
	if !fileExists(configPath) {
		// Not every service has a config for every environment — skip silently.
		return RenderResult{
			renderKey:  key,
			OutputPath: "",
			Err:        nil,
		}
	}

	combined, err := xconfigs.BuildCombinedYAML(assets.SharedConfigs(), key.Env, configPath)
	if err != nil {
		return RenderResult{
			renderKey:  key,
			OutputPath: "",
			Err:        fmt.Errorf("error building combined YAML: %w", err),
		}
	}

	outputPath := filepath.Join(key.ServiceDir, "configs", key.Env+".resolved.yaml")
	if err := os.WriteFile(outputPath, []byte(combined), 0o600); err != nil {
		return RenderResult{
			renderKey:  key,
			OutputPath: outputPath,
			Err:        fmt.Errorf("error writing file: %w", err),
		}
	}

	return RenderResult{
		renderKey:  key,
		OutputPath: outputPath,
		Err:        nil,
	}
}

// displayResults prints a ✓/✗ summary grouped by environment and returns false if any failed.
func displayResults(results []RenderResult) (ok bool) {
	ok = true
	var sb strings.Builder
	for _, r := range results {
		switch {
		case r.Err != nil:
			fmt.Fprintf(&sb, "  ✗ [%s] %-65s %s\n", r.Env, r.ServiceDir, r.Err)
			ok = false
		case r.OutputPath != "":
			fmt.Fprintf(&sb, "  ✓ [%s] %-65s → %s\n", r.Env, r.ServiceDir, r.OutputPath)
			// OutputPath == "" means the service config didn't exist for this env — silently skip.
		}
	}
	fmt.Print(sb.String())
	return ok
}
