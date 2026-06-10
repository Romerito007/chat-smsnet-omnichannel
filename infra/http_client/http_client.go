// Package http_client provides a small, shared HTTP client with sane timeouts
// for outbound integrations (providerhub, monitoring, channels, webhooks).
package http_client

import (
	"net"
	"net/http"
	"time"
)

// New builds an *http.Client with connection pooling and bounded timeouts so a
// slow upstream cannot hang a request indefinitely.
func New(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
