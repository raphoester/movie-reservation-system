package xgrpcclient

// Config holds the connection settings for a gRPC client.
// BaseURL must be in host:port form — do not include a scheme (http://, https://).
type Config struct {
	BaseURL string `mapstructure:"baseURL" validate:"required"`
}
