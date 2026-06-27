package xtime

import "time"

type CustomProvider struct {
	NowFunc func() time.Time
}

func (f CustomProvider) Now() time.Time {
	return f.NowFunc()
}

func NewDefaultFixedProvider() *CustomProvider {
	return &CustomProvider{
		NowFunc: GetTestTime,
	}
}
