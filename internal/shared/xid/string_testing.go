package xid

func NewFixedStringProvider(id string) StringProvider {
	return FixedProvider{id: id}
}

type FixedProvider struct {
	id string
}

func (p FixedProvider) ProvideID() string { return p.id }
