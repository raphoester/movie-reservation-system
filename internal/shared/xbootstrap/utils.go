package xbootstrap

import (
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"
)

func NewClosableRegistry() *ClosableRegistry {
	return &ClosableRegistry{
		fns: []func() error{},
	}
}

type ClosableRegistry struct {
	fns []func() error
}

func (cr *ClosableRegistry) Register(f func()) {
	cr.fns = append(cr.fns, func() error {
		f()
		return nil
	})
}

func (cr *ClosableRegistry) RegisterCloser(closer io.Closer) {
	cr.fns = append(cr.fns, closer.Close)
}

func (cr *ClosableRegistry) Close() error {
	var errGroup errgroup.Group
	for _, cleanupFunc := range cr.fns {
		errGroup.Go(cleanupFunc)
	}
	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}
	return nil
}

type FnRegistrar interface {
	Register(f func())
	RegisterCloser(closer io.Closer)
}

type ProductionSuitabilityChecker interface {
	IsSuitableForProduction() bool
}

type NotSuitableForProduction struct{}

func (NotSuitableForProduction) IsSuitableForProduction() bool {
	return false
}

type SuitableForProduction struct{}

func (SuitableForProduction) IsSuitableForProduction() bool {
	return true
}

type NopCloser struct{}

func (NopCloser) Close() error {
	return nil
}
