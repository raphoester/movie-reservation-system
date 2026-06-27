package xid

import "github.com/google/uuid"

type StringProvider interface {
	ProvideID() string
}

type UUIDStringProvider struct{}

func (p UUIDStringProvider) ProvideID() string {
	return uuid.Must(uuid.NewV7()).String()
}
