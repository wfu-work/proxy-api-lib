package tokeni_test

import (
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/tokeni"
	"github.com/wfu-work/proxy-api-lib/compatible"
)

func TestConfigDefaults(t *testing.T) {
	cfg := tokeni.Config()
	if cfg.Name != "tokeni" || cfg.BaseURL != "https://api.tokeni.top" || cfg.WireAPI != compatible.WireAPIResponses {
		t.Fatalf("config = %#v", cfg)
	}
}
