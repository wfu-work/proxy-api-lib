package aiok_test

import (
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/aiok"
	"github.com/wfu-work/proxy-api-lib/compatible"
)

func TestConfigDefaults(t *testing.T) {
	cfg := aiok.Config()
	if cfg.Name != "aiok" || cfg.BaseURL != "https://aiok.club/v1" || cfg.WireAPI != compatible.WireAPIResponses {
		t.Fatalf("config = %#v", cfg)
	}
}
