package transport

import (
	"net/http"
	"net/url"
)

// NewHTTPClient 创建支持可选代理地址的 HTTP 客户端。
// proxyURL 为空时保留 Go 默认的环境变量代理行为。
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

// NewHTTPClientNoProxy 创建忽略 HTTP_PROXY 和 HTTPS_PROXY 的直连客户端。
func NewHTTPClientNoProxy() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport}
}
