// Package httpclient provides shared outbound HTTP client construction.
package httpclient

import (
	"kiro-go/config"
	"net/http"
	"net/url"
	"time"
)

// New creates an HTTP client whose transport reads the configured outbound
// proxy on every request, so admin changes apply without restarting.
func New(timeout time.Duration, maxIdleConns, maxIdleConnsPerHost int) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 configuredProxy,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			DisableCompression:    false,
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: timeout,
		},
	}
}

func configuredProxy(req *http.Request) (*url.URL, error) {
	proxyURL := config.GetOutboundProxy()
	if proxyURL == "" {
		return nil, nil
	}
	return url.Parse(proxyURL)
}
