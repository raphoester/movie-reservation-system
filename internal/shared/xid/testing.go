package xid

func GetTestStringID() string {
	return "deadbeef-cafe-cafe-cafe-deadbeef0000"
}

type TestStringProvider struct{}

func (p TestStringProvider) ProvideID() string {
	return GetTestStringID()
}
