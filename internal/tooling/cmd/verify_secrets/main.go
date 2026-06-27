package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xaws"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/samber/lo"
)

func main() {
	logger := xlog.NewForLocalConsole(xlog.Config{Level: "info"})
	ctx := context.TODO()
	if err := run(ctx); err != nil {
		logger.Error("verify-secrets failed", "error", err)
		os.Exit(1)
	}
}

type FlagsConfig struct {
	ServicePath  string
	Environments []string
}

func parseFlags() (*FlagsConfig, error) {
	servicePath := flag.String("service", "", "path to service from repo root (e.g. internal/reservations/cmd/grpc_api)")
	envList := flag.String("env", "dev,staging,prod", "comma-separated list of environments to check")
	flag.Parse()

	if *servicePath == "" {
		flag.Usage()
		return nil, fmt.Errorf("--service is required")
	}

	envs := splitTrimmed(*envList, ",")
	return &FlagsConfig{
		ServicePath:  *servicePath,
		Environments: envs,
	}, nil
}

func run(ctx context.Context) error {
	flags, err := parseFlags()
	if err != nil {
		return err
	}

	if err := validateEnvs(ctx, flags.Environments); err != nil {
		return err
	}

	res, err := runChecks(ctx, flags.ServicePath, flags.Environments)
	if err != nil {
		return fmt.Errorf("error running checks: %w", err)
	}

	ok := displayResults(res)
	if !ok {
		return fmt.Errorf("one or more secrets failed verification")
	}

	return nil
}

func validateEnvs(ctx context.Context, envs []string) error {
	missingEnvs := lo.Filter(envs, func(env string, _ int) bool {
		_, err := xaws.ConfigForEnv(ctx, env)
		return err != nil
	})
	if len(missingEnvs) > 0 {
		return fmt.Errorf("invalid AWS config for envs: %s, please ensure you have run "+
			"`aws sso login --sso-session default` (see docs/configuration.md)", strings.Join(missingEnvs, ", "))
	}
	return nil
}

func splitTrimmed(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

type envResult struct {
	env         string
	results     []xconfigs.SecretCheckResult
	err         error
	skipped     bool
	servicePath string
}

func runChecks(
	ctx context.Context,
	servicePath string,
	envs []string,
) ([]envResult, error) {
	// Pre-index the slice so each goroutine writes to its own position without a mutex.
	allResults := make([]envResult, len(envs))
	var wg sync.WaitGroup
	for i, env := range envs {
		wg.Go(func() {
			allResults[i] = checkOneEnv(ctx, env, servicePath)
		})
	}

	wg.Wait()

	return allResults, nil
}

func checkOneEnv(
	ctx context.Context,
	env string,
	servicePath string,
) envResult {
	awsCfg, err := xaws.ConfigForEnv(ctx, env)
	if err != nil {
		return envResult{
			env:         env,
			results:     nil,
			skipped:     false,
			err:         fmt.Errorf("could not create AWS config: %w", err),
			servicePath: servicePath,
		}
	}

	sm := xconfigs.NewAWSSecretManagerFromConfig(awsCfg)
	serviceConfigPath := filepath.Join(servicePath, "configs", env+".yaml")
	results, err := xconfigs.VerifyConfigSecrets(ctx, assets.SharedConfigs(), env, serviceConfigPath, sm)
	if errors.Is(err, xconfigs.ErrAWSAccessDenied) {
		return envResult{env: env, results: nil, skipped: true, err: nil, servicePath: servicePath}
	}
	return envResult{
		env:         env,
		results:     results,
		skipped:     false,
		servicePath: servicePath,
		err: lo.Ternary(
			err != nil,
			fmt.Errorf("error verifying secrets: %w", err),
			nil,
		),
	}
}

func displayResults(allResults []envResult) (ok bool) {
	ok = true
	var sb strings.Builder
	for _, r := range allResults {
		fmt.Fprintf(&sb, "\n=== %s (%s) ===\n", r.servicePath, r.env)
		if r.skipped {
			fmt.Fprintf(&sb, "  ~ skipped (no Secrets Manager access for this environment)\n")
			continue
		}
		if r.err != nil {
			fmt.Fprintf(&sb, "  ERROR: %s\n", r.err)
			ok = false
			continue
		}
		for _, res := range r.results {
			mark := "✓"
			if !res.OK {
				mark = "✗"
				ok = false
			}
			fmt.Fprintf(&sb, "  %s %-55s %s\n", mark, res.ConfigPath, res.Message)
		}
	}

	fmt.Print(sb.String())
	return ok
}
