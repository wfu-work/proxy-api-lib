package transport

import (
	"net/http"
	"net/url"
)

// NewHTTPClient returns an HTTP client configured with an optional proxy URL.
// Empty proxyURL preserves the default Go environment proxy behavior.
func NewHTTPClient(proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Transport: transport}, nil
}

// NewHTTPClientNoProxy returns an HTTP client that bypasses HTTP_PROXY/HTTPS_PROXY.
func NewHTTPClientNoProxy() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport}
}
