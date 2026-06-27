package xhealth

import "errors"

var (
	ErrCheckNotFound = errors.New("health check not found")
	ErrUnhealthy     = errors.New("one or more health checks failed")
)
