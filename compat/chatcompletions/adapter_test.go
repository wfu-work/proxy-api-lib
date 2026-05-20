package chatcompletions_test

import (
	"encoding/json"
	"testing"

	"github.com/wfu-work/proxy-api-lib/compat/chatcompletions"
)

func TestConvertJSON(t *testing.T) {
	req, err := chatcompletions.ConvertJSON([]byte(`{
		"model":"gpt-chat",
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{
			"type":"function",
			"function":{
				"name":"get_weather",
				"description":"Get weather",
				"parameters":{"type":"object"}
			}
		}],
		"max_tokens":64,
		"temperature":0.2,
		"custom":"kept"
	}`))
	if err != nil {
		t.Fatalf("ConvertJSON: %v", err)
	}
	if req.Model != "gpt-chat" || req.Input == nil {
		t.Fatalf("request = %#v", req)
	}
	if req.MaxOutputTokens == nil || *req.MaxOutputTokens != 64 {
		t.Fatalf("max output tokens = %#v", req.MaxOutputTokens)
	}
	if req.Temperature == nil || *req.Temperature != 0.2 {
		t.Fatalf("temperature = %#v", req.Temperature)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("tools len = %d", len(req.Tools))
	}
	toolJSON, err := json.Marshal(req.Tools[0])
	if err != nil {
		t.Fatal(err)
	}
	var tool map[string]any
	if err := json.Unmarshal(toolJSON, &tool); err != nil {
		t.Fatal(err)
	}
	if tool["type"] != "function" || tool["name"] != "get_weather" {
		t.Fatalf("tool = %#v", tool)
	}
	if req.Extra["custom"] != "kept" {
		t.Fatalf("extra = %#v", req.Extra)
	}
}
