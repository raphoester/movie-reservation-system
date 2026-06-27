package xruntime

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"
)

// Characteristics describes the runtime environment of the current process.
type Characteristics struct {
	IsCloudDeployment bool
}

// imdsClient is a dedicated HTTP client for the IMDS probe that explicitly disables
// proxy usage. http.DefaultClient respects HTTP_PROXY / NO_PROXY, which can cause
// requests to 169.254.169.254 to be routed through a developer proxy and
// incorrectly succeed, giving a false IsRunningOnAWS=true.
var imdsClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil, // bypass any proxy configuration
	},
}

var (
	once         sync.Once
	cachedResult Characteristics
)

// Detect returns the cached runtime characteristics, detecting them on first call.
// Safe for concurrent use.
func Detect(ctx context.Context) Characteristics {
	once.Do(func() {
		cachedResult = detect(ctx)
	})
	return cachedResult
}

// detect returns characteristics of the current runtime environment.
// It returns quickly: env var checks are instant; the IMDS probe has a 300ms timeout.
func detect(ctx context.Context) Characteristics {
	if os.Getenv("ECS_CONTAINER_METADATA_URI_V4") != "" || os.Getenv("ECS_CONTAINER_METADATA_URI") != "" {
		return Characteristics{IsCloudDeployment: true}
	}

	probe, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(probe, http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	if err != nil {
		return Characteristics{IsCloudDeployment: false}
	}

	resp, err := imdsClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		return Characteristics{IsCloudDeployment: true}
	}

	return Characteristics{IsCloudDeployment: false}
}
