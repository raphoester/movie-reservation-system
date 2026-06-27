package xaws

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// ProfileForEnv returns the standard SSO profile name for the given environment.
func ProfileForEnv(env string) (string, error) {
	profiles := map[string]string{
		"dev":     "mrs-dev",
		"staging": "mrs-stg",
		"prod":    "mrs-prod",
	}
	p, ok := profiles[env]
	if !ok {
		return "", fmt.Errorf("unknown environment %q — expected dev, staging, or prod", env)
	}
	return p, nil
}

// ConfigForEnv builds an aws.Config for the given environment using its standard SSO profile.
func ConfigForEnv(ctx context.Context, env string) (*aws.Config, error) {
	profile, err := ProfileForEnv(env)
	if err != nil {
		return nil, err
	}
	return ConfigForProfile(ctx, profile)
}

// ConfigForProfile builds an aws.Config by exporting credentials for the named AWS SSO profile.
func ConfigForProfile(ctx context.Context, profile string) (*aws.Config, error) {
	out, err := exec.CommandContext(
		ctx,
		"aws", "configure", "export-credentials",
		"--profile", profile,
		"--format", "process",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("export credentials for profile %q (did you run `aws sso login --sso-session default`? see docs/configuration.md): %w", profile, err)
	}
	var crd struct {
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken    string `json:"SessionToken"`
	}
	if err := json.Unmarshal(out, &crd); err != nil {
		return nil, fmt.Errorf("parse exported credentials: %w", err)
	}
	creds := credentials.NewStaticCredentialsProvider(crd.AccessKeyID, crd.SecretAccessKey, crd.SessionToken)
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("us-east-2"),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &cfg, nil
}
