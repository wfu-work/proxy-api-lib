package compatible_test

import (
	"testing"

	"github.com/wfu-work/proxy-api-lib/compatible"
)

func TestDefaultUsageURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "base without v1", baseURL: "https://api.tokeni.top", want: "https://api.tokeni.top/v1/usage"},
		{name: "base with v1", baseURL: "https://aiok.club/v1", want: "https://aiok.club/v1/usage"},
		{name: "base with nested v1", baseURL: "https://example.com/api/v1/", want: "https://example.com/api/v1/usage"},
		{name: "v1 is path segment only", baseURL: "https://example.com/v10", want: "https://example.com/v10/v1/usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatible.DefaultUsageURL(tt.baseURL); got != tt.want {
				t.Fatalf("DefaultUsageURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}
