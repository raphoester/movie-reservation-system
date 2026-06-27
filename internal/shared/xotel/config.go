package xotel

type Config struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string
	ExporterType   string
	OTLPEndpoint   string
	OTLPInsecure   bool
	SamplingRatio  float64
}
