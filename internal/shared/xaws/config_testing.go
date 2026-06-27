package xaws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Session returns an aws.Config for the given SSO profile, skipping the test if credentials
// cannot be exported (e.g. SSO session expired).
// Requires `aws sso login --sso-session default` (see docs/configuration.md).
func Session(t *testing.T, profile string) aws.Config {
	t.Helper()
	cfg, err := ConfigForProfile(t.Context(), profile)
	if err != nil {
		t.Skipf("could not export AWS credentials for %q (run `aws sso login --sso-session default`): %v", profile, err)
	}
	return *cfg
}

// DevSession returns an aws.Config for the mrs-dev SSO profile.
func DevSession(t *testing.T) aws.Config {
	t.Helper()
	return Session(t, "mrs-dev")
}
