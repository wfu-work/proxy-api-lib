package codex_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/free-model/proxy-api-lib/compat/codex"
	"github.com/free-model/proxy-api-lib/compatible"
)

func TestLoadCodexConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	authPath := filepath.Join(dir, "auth.json")

	if err := os.WriteFile(configPath, []byte(`
model_provider = "freemodel"
model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
preferred_auth_method = "apikey"

[model_providers.freemodel]
name = "freemodel"
base_url = "https://api.freemodel.dev"
wire_api = "responses"
proxy_url = "http://127.0.0.1:7890"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authPath, []byte(`{"OPENAI_API_KEY":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := codex.Load(configPath, authPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	provider := cfg.Provider("freemodel")
	if provider.Name != "freemodel" || provider.BaseURL != "https://api.freemodel.dev" || provider.WireAPI != compatible.WireAPIResponses {
		t.Fatalf("provider = %#v", provider)
	}
	defaults := cfg.RequestDefaults()
	if defaults.Model != "gpt-5.5" {
		t.Fatalf("model = %q", defaults.Model)
	}
	if defaults.Reasoning == nil || defaults.Reasoning.Effort != "xhigh" {
		t.Fatalf("reasoning = %#v", defaults.Reasoning)
	}
	if defaults.Store == nil || *defaults.Store {
		t.Fatalf("store = %#v", defaults.Store)
	}
	if cfg.OpenAIAPIKey() != "secret" {
		t.Fatalf("api key = %q", cfg.OpenAIAPIKey())
	}
	if cfg.Credential() == nil {
		t.Fatal("Credential = nil")
	}
}
