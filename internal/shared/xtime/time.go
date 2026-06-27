package xtime

import "time"

type Provider interface {
	Now() time.Time
}

type RealProvider struct{}

func (RealProvider) Now() time.Time {
	return time.Now().UTC()
}
