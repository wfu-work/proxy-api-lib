package compatible_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	proxyapi "github.com/free-model/proxy-api-lib"
	"github.com/free-model/proxy-api-lib/compatible"
	"github.com/free-model/proxy-api-lib/domains"
)

func TestCreateResponseSendsOpenAICompatibleRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("x-request-id", "req_test")
		_, _ = w.Write([]byte(`{
			"id":"resp_123",
			"model":"gpt-test",
			"status":"completed",
			"output":[{
				"type":"message",
				"role":"assistant",
				"content":[{"type":"output_text","text":"hello"}]
			}],
			"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	store := false
	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
			WireAPI: compatible.WireAPIResponses,
		})),
		proxyapi.WithAPIKey("test-key"),
	)

	resp, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: domains.InputText("hi"),
		Reasoning: &domains.Reasoning{
			Effort: "xhigh",
		},
		Store: &store,
		Tools: []domains.Tool{
			domains.FunctionTool{
				Name:        "get_weather",
				Description: "Get weather.",
				Parameters: domains.JSONSchema{
					Type: "object",
					Properties: map[string]domains.JSONSchema{
						"city": {Type: "string"},
					},
					Required: []string{"city"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if gotPath != "/responses" {
		t.Fatalf("path = %q, want /responses", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth = %q, want bearer api key", gotAuth)
	}
	if gotPayload["model"] != "gpt-test" {
		t.Fatalf("model = %#v", gotPayload["model"])
	}
	if gotPayload["store"] != false {
		t.Fatalf("store = %#v, want false", gotPayload["store"])
	}
	if resp.OutputText() != "hello" {
		t.Fatalf("OutputText = %q", resp.OutputText())
	}
	if resp.RequestID != "req_test" {
		t.Fatalf("RequestID = %q", resp.RequestID)
	}
}

func TestCreateResponseMapsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_bad")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad token","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}))
	defer server.Close()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
		})),
		proxyapi.WithBearerToken("token"),
	)

	_, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: "hi",
	})
	if err == nil {
		t.Fatal("Create error = nil")
	}
	var apiErr *domains.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Code != "invalid_api_key" || apiErr.RequestID != "req_bad" {
		t.Fatalf("api error = %#v", apiErr)
	}
}

func TestCreateResponseParsesToolCallsAndSendsToolOutput(t *testing.T) {
	var gotPayloads []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotPayloads = append(gotPayloads, payload)
		if len(gotPayloads) == 1 {
			_, _ = w.Write([]byte(`{
				"id":"resp_tool",
				"model":"gpt-test",
				"status":"completed",
				"output":[{
					"id":"fc_1",
					"type":"function_call",
					"call_id":"call_1",
					"name":"get_weather",
					"arguments":"{\"city\":\"Shanghai\"}"
				}]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id":"resp_done",
			"model":"gpt-test",
			"status":"completed",
			"output":[{
				"type":"message",
				"role":"assistant",
				"content":[{"type":"output_text","text":"sunny"}]
			}]
		}`))
	}))
	defer server.Close()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
		})),
		proxyapi.WithAPIKey("key"),
	)

	resp, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: "weather?",
		Tools: []domains.Tool{
			domains.FunctionTool{Name: "get_weather"},
		},
	})
	if err != nil {
		t.Fatalf("Create tool call: %v", err)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 || calls[0].CallID != "call_1" || calls[0].Name != "get_weather" {
		t.Fatalf("tool calls = %#v", calls)
	}

	resp, err = client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model:              "gpt-test",
		PreviousResponseID: "resp_tool",
		Input: []any{
			domains.FunctionCallOutput(calls[0].CallID, map[string]string{"weather": "sunny"}),
		},
	})
	if err != nil {
		t.Fatalf("Create tool output: %v", err)
	}
	if resp.OutputText() != "sunny" {
		t.Fatalf("OutputText = %q", resp.OutputText())
	}
	if len(gotPayloads) != 2 {
		t.Fatalf("payload count = %d", len(gotPayloads))
	}
	input, ok := gotPayloads[1]["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("tool output input = %#v", gotPayloads[1]["input"])
	}
	item, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("tool output item = %#v", input[0])
	}
	if item["type"] != "function_call_output" || item["call_id"] != "call_1" || item["output"] != `{"weather":"sunny"}` {
		t.Fatalf("tool output item = %#v", item)
	}
}

func TestStreamResponseReadsSSEEvents(t *testing.T) {
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, "response.output_text.delta", `{"type":"response.output_text.delta","delta":"hel"}`)
		writeSSE(t, w, "response.output_text.delta", `{"type":"response.output_text.delta","delta":"lo"}`)
		writeSSE(t, w, "response.completed", `{"type":"response.completed","response":{"id":"resp_123"}}`)
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
		})),
		proxyapi.WithBearerToken("token"),
	)

	stream, err := client.Responses.Stream(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: "hi",
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()

	var eventTypes []string
	var text string
	for stream.Next() {
		event := stream.Event()
		eventTypes = append(eventTypes, event.Type)
		text += event.TextDelta()
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream err: %v", err)
	}
	if gotPayload["stream"] != true {
		t.Fatalf("stream payload = %#v, want true", gotPayload["stream"])
	}
	if text != "hello" {
		t.Fatalf("text = %q, want hello", text)
	}
	wantTypes := []string{"response.output_text.delta", "response.output_text.delta", "response.completed"}
	if fmt.Sprint(eventTypes) != fmt.Sprint(wantTypes) {
		t.Fatalf("event types = %#v, want %#v", eventTypes, wantTypes)
	}
}

func TestStreamResponseMapsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
	}))
	defer server.Close()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
		})),
		proxyapi.WithAPIKey("key"),
	)

	_, err := client.Responses.Stream(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: "hi",
	})
	if err == nil {
		t.Fatal("Stream error = nil")
	}
	var apiErr *domains.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Code != "rate_limit_exceeded" {
		t.Fatalf("api error = %#v", apiErr)
	}
}

func TestStreamResponseSurfacesErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(t, w, "response.failed", `{"type":"response.failed","error":{"message":"bad stream","type":"invalid_request_error","code":"bad_stream"}}`)
	}))
	defer server.Close()

	client := proxyapi.NewClient(
		proxyapi.WithProvider(compatible.OpenAIResponses(compatible.Config{
			Name:    "freemodel",
			BaseURL: server.URL,
		})),
		proxyapi.WithAPIKey("key"),
	)

	stream, err := client.Responses.Stream(context.Background(), domains.ResponseRequest{
		Model: "gpt-test",
		Input: "hi",
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if !stream.Next() {
		t.Fatal("Next = false, want error event")
	}
	if stream.Event().Type != "response.failed" {
		t.Fatalf("event type = %q", stream.Event().Type)
	}
	var apiErr *domains.APIError
	if !errors.As(stream.Err(), &apiErr) {
		t.Fatalf("stream err = %v, want *APIError", stream.Err())
	}
	if apiErr.Provider != "freemodel" || apiErr.Code != "bad_stream" {
		t.Fatalf("api error = %#v", apiErr)
	}
	if stream.Next() {
		t.Fatal("Next after error = true")
	}
}

func writeSSE(t *testing.T, w http.ResponseWriter, event, data string) {
	t.Helper()
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		t.Fatalf("write SSE: %v", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
