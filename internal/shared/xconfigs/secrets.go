package xconfigs

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
)

func resolveSecretManager(sms []SecretManager) mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, rawData any) (any, error) {
		if f.Kind() != reflect.String {
			return rawData, nil
		}
		identifier := rawData.(string)
		switch {
		case t.Kind() == reflect.String:
			return resolveEager(sms, identifier)
		case t == lazyStringType:
			return resolveLazy(sms, identifier)
		default:
			return rawData, nil
		}
	}
}

// resolveEager iterates over secret managers and returns the first resolved value for the given identifier.
// If the identifier matches an SM's scheme but the secret can't be fetched, it returns an error immediately.
// If no SM matches, the identifier is returned as-is (plain string pass-through).
func resolveEager(sms []SecretManager, identifier string) (any, error) {
	for _, sm := range sms {
		val, err := sm.AccessSecretIfEligible(context.Background(), identifier)
		if err == nil {
			return val, nil
		}
		if !errors.Is(err, ErrNotEligible) {
			return nil, fmt.Errorf("%w: %s: %w", ErrSecretResolution, identifier, err)
		}
	}
	return identifier, nil
}

// resolveLazy iterates over secret managers and returns a LazyString wrapping the first matching callback.
// If the identifier matches an SM's scheme but the callback can't be created, it returns an error immediately.
// If no SM matches, the identifier is returned as-is so resolveLazyString can wrap it as a plain value.
func resolveLazy(sms []SecretManager, identifier string) (any, error) {
	for _, sm := range sms {
		cb, err := sm.AccessSecretCallbackIfEligible(identifier)
		if err == nil {
			return NewLazyString(cb), nil
		}
		if !errors.Is(err, ErrNotEligible) {
			return nil, fmt.Errorf("%w: %s: %w", ErrSecretResolution, identifier, err)
		}
	}
	return identifier, nil
}

func resolveLazyString() mapstructure.DecodeHookFuncType {
	return func(f reflect.Type, t reflect.Type, rawData any) (any, error) {
		if t != lazyStringType {
			return rawData, nil
		}
		if _, ok := rawData.(*LazyString); ok {
			return rawData, nil
		}

		var val string
		if f.Kind() == reflect.String {
			val = rawData.(string)
		} else {
			val = fmt.Sprintf("%v", rawData)
		}
		// return the string as a LazyString with a callback that just returns the value, since it wasn't resolved by
		// any SecretManager. This allows the rest of the config to load successfully even if there are some unresolved
		// secrets, and lets the application handle missing secrets at runtime when the LazyString is accessed.
		return NewLazyString(func(_ context.Context) (string, error) { return val, nil }), nil
	}
}

var lazyStringType = reflect.TypeOf(LazyString{fn: nil})

type SecretManager interface {
	AccessSecretIfEligible(ctx context.Context, identifier string) (string, error)
	AccessSecretCallbackIfEligible(identifier string) (StringCallback, error)
}

type StringCallback func(context.Context) (string, error)
