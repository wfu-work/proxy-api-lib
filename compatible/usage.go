package compatible

import (
	"net/url"
	"strings"
)

// DefaultUsageURL builds the conventional OpenAI-compatible usage endpoint.
// If baseURL already contains a /v1 path segment, /usage is appended.
// Otherwise /v1/usage is appended.
func DefaultUsageURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "/v1/usage"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		base := strings.TrimRight(baseURL, "/")
		if pathHasV1(base) {
			return base + "/usage"
		}
		return base + "/v1/usage"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if pathHasV1(parsed.Path) {
		parsed.Path += "/usage"
	} else {
		parsed.Path += "/v1/usage"
	}
	return parsed.String()
}

func pathHasV1(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == "v1" {
			return true
		}
	}
	return false
}
