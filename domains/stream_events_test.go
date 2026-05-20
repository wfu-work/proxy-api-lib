package domains_test

import (
	"encoding/json"
	"testing"

	"github.com/wfu-work/proxy-api-lib/domains"
)

func TestStreamAccumulatorAggregatesTextToolCallsAndCompletion(t *testing.T) {
	acc := domains.NewStreamAccumulator()

	events := []domains.StreamEvent{
		rawEvent("response.output_text.delta", `{"type":"response.output_text.delta","delta":"hel"}`),
		rawEvent("response.output_text.delta", `{"type":"response.output_text.delta","delta":"lo"}`),
		rawEvent("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"get_weather"}}`),
		rawEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"city\""}`),
		rawEvent("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":":\"Shanghai\"}"}`),
		rawEvent("response.completed", `{"type":"response.completed","response":{"id":"resp_done","status":"completed"}}`),
	}

	for _, event := range events {
		acc.Add(event)
	}

	if acc.Text() != "hello" {
		t.Fatalf("Text = %q", acc.Text())
	}
	calls := acc.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("ToolCalls len = %d", len(calls))
	}
	if calls[0].ID != "fc_1" || calls[0].CallID != "call_1" || calls[0].Name != "get_weather" {
		t.Fatalf("ToolCall metadata = %#v", calls[0])
	}
	if calls[0].Arguments != `{"city":"Shanghai"}` {
		t.Fatalf("ToolCall arguments = %q", calls[0].Arguments)
	}
	if acc.CompletedResponse() == nil || acc.CompletedResponse().ID != "resp_done" {
		t.Fatalf("CompletedResponse = %#v", acc.CompletedResponse())
	}
}

func rawEvent(eventType string, data string) domains.StreamEvent {
	return domains.StreamEvent{
		Type: eventType,
		Data: json.RawMessage(data),
	}
}
