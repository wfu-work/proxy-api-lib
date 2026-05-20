package compatible_test

import (
	"context"
	"os"
	"testing"
	"time"

	proxyapi "github.com/wfu-work/proxy-api-lib"
	"github.com/wfu-work/proxy-api-lib/compatible"
	"github.com/wfu-work/proxy-api-lib/domains"
)

func TestIntegrationResponsesCreate(t *testing.T) {
	if os.Getenv("PROXYAPI_INTEGRATION") != "1" {
		t.Skip("set PROXYAPI_INTEGRATION=1 to run real upstream integration tests")
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is required")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		t.Skip("OPENAI_MODEL is required")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = compatible.DefaultOpenAIBaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "integration",
			BaseURL: baseURL,
			WireAPI: compatible.WireAPIResponses,
		})),
		proxyapi.WithAPIKey(apiKey),
	)

	resp, err := client.Responses.Create(ctx, domains.ResponseRequest{
		Model: model,
		Input: "Reply with the word ok.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.ID == "" {
		t.Fatalf("response id is empty: %#v", resp)
	}
}
