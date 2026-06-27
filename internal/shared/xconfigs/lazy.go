package xconfigs

import (
	"context"
	"errors"
)

// NewLazyString creates a LazyString that will invoke fn when Value is called.
// The fn callback must be non-nil and should return the string value or an error.
func NewLazyString(fn func(ctx context.Context) (string, error)) *LazyString {
	return &LazyString{fn: fn}
}

// LazyString defers secret resolution until Value(ctx) is called,
// allowing config fields to be populated without immediately fetching
// expensive or potentially unavailable secrets.
type LazyString struct {
	fn func(ctx context.Context) (string, error)
}

func (ls *LazyString) Value(ctx context.Context) (string, error) {
	if ls == nil || ls.fn == nil {
		return "", ErrLazyStringNotInitialized
	}

	return ls.fn(ctx)
}

var ErrLazyStringNotInitialized = errors.New("LazyString is not initialized")
