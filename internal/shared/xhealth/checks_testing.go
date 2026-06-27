package xhealth

import "context"

type FuncCheck struct {
	name    string
	checkFn func(ctx context.Context) error
}

func NewFuncCheck(name string, checkFn func(ctx context.Context) error) *FuncCheck {
	return &FuncCheck{
		name:    name,
		checkFn: checkFn,
	}
}

func (c *FuncCheck) Name() string {
	return c.name
}

func (c *FuncCheck) Check(ctx context.Context) error {
	return c.checkFn(ctx)
}
