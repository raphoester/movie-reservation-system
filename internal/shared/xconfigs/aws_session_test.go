//go:build integration

package xconfigs_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/raphoester/movie-reservation-system/internal/shared/xaws"
)

func awsSession(t *testing.T) aws.Config {
	t.Helper()
	return xaws.DevSession(t)
}
