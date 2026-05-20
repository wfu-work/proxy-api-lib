package aiok_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/aiok"
)

func TestUsageURLDefault(t *testing.T) {
	if aiok.UsageURL != "https://aiok.club/v1/usage" {
		t.Fatalf("UsageURL = %q", aiok.UsageURL)
	}
}

func TestParseUsageResponse(t *testing.T) {
	stats, err := aiok.ParseUsageResponse([]byte(`{"data":{"balance":"12.34"}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stats.Balance != 12.34 {
		t.Fatalf("balance = %v", stats.Balance)
	}
}

func TestUsageClientFetch(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"balance":5.67}`))
	}))
	defer server.Close()
	client := aiok.NewUsageClient(
		aiok.WithUsageBaseURL(server.URL),
		aiok.WithUsageHTTPClient(server.Client()),
	)
	stats, err := client.Fetch(context.Background(), "secret")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if stats.Balance != 5.67 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestWithUsageProviderBaseURL(t *testing.T) {
	client := aiok.NewUsageClient(aiok.WithUsageProviderBaseURL("https://example.com"))
	if client.BaseURL != "https://example.com/v1/usage" {
		t.Fatalf("BaseURL = %q", client.BaseURL)
	}
	client = aiok.NewUsageClient(aiok.WithUsageProviderBaseURL("https://example.com/api/v1"))
	if client.BaseURL != "https://example.com/api/v1/usage" {
		t.Fatalf("BaseURL = %q", client.BaseURL)
	}
}
