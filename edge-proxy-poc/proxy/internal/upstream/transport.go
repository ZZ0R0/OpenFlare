package upstream

import (
	"net"
	"net/http"
	"time"

	"github.com/openflare/edge-proxy/internal/config"
)

// NewTransport creates an http.Transport configured from upstream settings.
func NewTransport(cfg config.UpstreamConfig) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout(),
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout(),
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:  10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     false,
	}
}
