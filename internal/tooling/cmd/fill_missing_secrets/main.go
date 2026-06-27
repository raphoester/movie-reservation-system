package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xaws"
	"github.com/raphoester/movie-reservation-system/internal/shared/xcli"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

func main() {
	logger := xlog.NewForLocalConsole(xlog.Config{Level: "info"})
	ctx := context.TODO()
	if err := run(ctx); err != nil {
		logger.Error("fill-missing-secrets failed", "error", err)
		os.Exit(1)
	}
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

type FlagsConfig struct {
	ServicePaths []string
	Environments []string
}

func parseFlags() (*FlagsConfig, error) {
	var services stringList
	flag.Var(&services, "service", "path to service from repo root (repeatable); e.g. --service internal/reservations/cmd/grpc_api")
	envList := flag.String("env", "dev", "comma-separated environments to fill, or 'all' for dev,staging,prod")
	flag.Parse()

	if len(services) == 0 {
		flag.Usage()
		return nil, fmt.Errorf("--service is required")
	}

	return &FlagsConfig{
		ServicePaths: []string(services),
		Environments: parseEnvList(*envList),
	}, nil
}

func parseEnvList(s string) []string {
	if s == "all" {
		return []string{"dev", "staging", "prod"}
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// missingRef is a failed awssm:// reference that can be filled interactively.
type missingRef struct {
	configPath string // YAML property path, e.g. "database.password"
	identifier string // full awssm://secretName:key
}

// skippedRef is a "replace-me" placeholder that requires a manual config edit.
type skippedRef struct {
	configPath string
	configFile string
}

// envRefs holds the gathered refs for a single (service, env) pair.
type envRefs struct {
	service string
	env     string
	missing []missingRef
	skipped []skippedRef
	err     error
}

func run(ctx context.Context) error {
	flags, err := parseFlags()
	if err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if err := validateEnvs(ctx, flags.Environments); err != nil {
		return fmt.Errorf("AWS config validation failed: %w", err)
	}

	pairs := buildPairs(flags.ServicePaths, flags.Environments)

	gathered, err := gatherAll(ctx, pairs)
	if err != nil {
		return fmt.Errorf("gather missing secrets: %w", err)
	}

	if err := fillAll(ctx, gathered); err != nil {
		return fmt.Errorf("fill secrets: %w", err)
	}
	return nil
}

func validateEnvs(ctx context.Context, envs []string) error {
	var invalid []string
	for _, env := range envs {
		if _, err := xaws.ConfigForEnv(ctx, env); err != nil {
			invalid = append(invalid, env)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid AWS config for envs: %s, please ensure you have run "+
			"`aws sso login --sso-session default` (see docs/configuration.md)", strings.Join(invalid, ", "))
	}
	return nil
}

// buildPairs returns all (service, env) combinations in service-major order.
func buildPairs(services, envs []string) []envRefs {
	pairs := make([]envRefs, 0, len(services)*len(envs))
	for _, svc := range services {
		for _, env := range envs {
			pairs = append(pairs, envRefs{
				service: svc,
				env:     env,
				missing: nil,
				skipped: nil,
				err:     nil,
			})
		}
	}
	return pairs
}

// gatherAll runs collectMissingRefs for every pair concurrently and returns
// results in the original order (service-major, then env).
func gatherAll(ctx context.Context, pairs []envRefs) ([]envRefs, error) {
	results := make([]envRefs, len(pairs))
	var wg sync.WaitGroup
	for i, p := range pairs {
		wg.Go(func() {
			awsCfg, err := xaws.ConfigForEnv(ctx, p.env)
			if err != nil {
				results[i] = envRefs{service: p.service, env: p.env, missing: nil, skipped: nil, err: fmt.Errorf("could not get AWS config: %w", err)}
				return
			}
			missing, skipped, err := collectMissingRefs(ctx, p.service, p.env, awsCfg)
			results[i] = envRefs{service: p.service, env: p.env, missing: missing, skipped: skipped, err: err}
		})
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("verify %s (%s): %w", r.service, r.env, r.err)
		}
	}
	return results, nil
}

// fillAll prompts and writes secrets sequentially across all gathered results.
func fillAll(ctx context.Context, gathered []envRefs) error {
	anyFailed := false
	for _, r := range gathered {
		awsCfg, err := xaws.ConfigForEnv(ctx, r.env)
		if err != nil {
			return fmt.Errorf("could not get AWS config for %s: %w", r.env, err)
		}
		failed, stopped := doFill(ctx, r.service, r.env, r.missing, r.skipped, awsCfg)
		if failed {
			anyFailed = true
		}
		if stopped {
			fmt.Println("(stopped)")
			break
		}
	}
	if anyFailed {
		return fmt.Errorf("one or more secrets failed to write")
	}
	return nil
}

func collectMissingRefs(
	ctx context.Context,
	servicePath, env string,
	awsCfg *aws.Config,
) ([]missingRef, []skippedRef, error) {
	sm := xconfigs.NewAWSSecretManagerFromConfig(awsCfg)
	serviceConfigPath := filepath.Join(servicePath, "configs", env+".yaml")
	results, err := xconfigs.VerifyConfigSecrets(ctx, assets.SharedConfigs(), env, serviceConfigPath, sm)
	if err != nil {
		return nil, nil, fmt.Errorf("verify config secrets: %w", err)
	}

	var missing []missingRef
	var skipped []skippedRef
	for _, r := range results {
		if r.OK {
			continue
		}
		if !strings.HasPrefix(r.Value, "awssm://") {
			skipped = append(skipped, skippedRef{configPath: r.ConfigPath, configFile: serviceConfigPath})
			continue
		}
		missing = append(missing, missingRef{
			configPath: r.ConfigPath,
			identifier: r.Value,
		})
	}
	return missing, skipped, nil
}

func doFill(
	ctx context.Context,
	servicePath, env string,
	missing []missingRef,
	skipped []skippedRef,
	awsCfg *aws.Config,
) (anyFailed, stopped bool) {
	if len(missing) == 0 && len(skipped) == 0 {
		return
	}

	fmt.Printf("\n == %s ==\n", env)
	anyFailed, stopped = promptAndWrite(ctx, servicePath, env, missing, awsCfg)

	displaySkipped(skipped)
	return
}

// promptAndWrite prompts for each missing ref in order, writing to AWS immediately
// after each entry. Returns (anyFailed, stopped); stopped is true when the user
// pressed Ctrl-C to exit early.
func promptAndWrite(
	ctx context.Context,
	servicePath, env string,
	missing []missingRef,
	cfg *aws.Config,
) (anyFailed, stopped bool) {
	for _, m := range missing {
		fmt.Printf("\n%s — %s (%s)\n", bold(servicePath), m.configPath, env)
		fmt.Printf("  Secret: %s\n", m.identifier)
		value, err := xcli.ReadMasked("  Value (leave blank to skip): ")
		if err != nil {
			stopped = true
			return
		}
		if strings.TrimSpace(value) == "" {
			fmt.Println("  (skipped)")
			continue
		}
		if err := xconfigs.UpsertSecretKey(ctx, cfg, m.identifier, strings.TrimSpace(value)); err != nil {
			fmt.Printf("  ✗ %s\n", err)
			anyFailed = true
		} else {
			fmt.Println("  ✓ saved")
		}
	}
	return
}

func bold(s string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", s)
}

func displaySkipped(skipped []skippedRef) {
	if len(skipped) == 0 {
		return
	}
	fmt.Printf("\n%d yaml fields are set to plain \"replace-me\" values in the config files. "+
		"Please use an awssm:// reference or set a plain value in the config file if it's not a secret:\n", len(skipped))
	for _, s := range skipped {
		fmt.Printf("%s: %s\n", s.configFile, s.configPath)
	}
}
