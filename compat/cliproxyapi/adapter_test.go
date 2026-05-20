package cliproxyapi_test

import (
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/cliproxyapi"
)

func TestConvertResponsesJSON(t *testing.T) {
	req, err := cliproxyapi.ConvertResponsesJSON([]byte(`{
		"model":"gpt-5.5",
		"input":"hi",
		"tools":[{"type":"function","name":"get_weather"}],
		"model_reasoning_effort":"xhigh",
		"disable_response_storage":true,
		"previous_response_id":"resp_1",
		"custom":"kept"
	}`))
	if err != nil {
		t.Fatalf("ConvertResponsesJSON: %v", err)
	}
	if req.Model != "gpt-5.5" || req.Input != "hi" {
		t.Fatalf("request = %#v", req)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("tools len = %d", len(req.Tools))
	}
	if req.Reasoning == nil || req.Reasoning.Effort != "xhigh" {
		t.Fatalf("reasoning = %#v", req.Reasoning)
	}
	if req.Store == nil || *req.Store {
		t.Fatalf("store = %#v", req.Store)
	}
	if req.PreviousResponseID != "resp_1" {
		t.Fatalf("previous response id = %q", req.PreviousResponseID)
	}
	if req.Extra["custom"] != "kept" {
		t.Fatalf("extra = %#v", req.Extra)
	}
}
