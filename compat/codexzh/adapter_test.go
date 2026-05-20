package codexzh_test

import (
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/codexzh"
	"github.com/wfu-work/proxy-api-lib/compatible"
)

func TestConfigDefaults(t *testing.T) {
	cfg := codexzh.Config()
	if cfg.Name != "codexzh" || cfg.BaseURL != "https://api.codexzh.com/v1" || cfg.WireAPI != compatible.WireAPIResponses {
		t.Fatalf("config = %#v", cfg)
	}
}
