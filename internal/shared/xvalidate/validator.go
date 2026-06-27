package xvalidate

import "github.com/go-playground/validator/v10"

// NewValidator creates and returns a new validator instance.
// We wrap it so that we can easily add custom types and validation rules in the future.
func NewValidator() *validator.Validate {
	return validator.New()
}
