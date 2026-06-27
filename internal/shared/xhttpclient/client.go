package xhttpclient

import (
	"net"
	"net/http"
	"time"
)

const (
	timeout               = 10 * time.Second
	dialTimeout           = 30 * time.Second
	dialKeepAlive         = 30 * time.Second
	maxIdleConns          = 100
	idleConnTimeout       = 90 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
	maxIdleConnsPerHost   = 100
)

// New returns a hardened HTTP client with tuned transport settings suitable
// for production use.
func New() *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: defaultTransport(),
	}
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: dialKeepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
	}
}

type Config struct {
	BaseURL string `validate:"required,url" mapstructure:"baseUrl"`
}
