package cliproxyapi

import (
	"encoding/json"
	"fmt"

	"github.com/free-model/proxy-api-lib/domains"
)

// ConvertResponsesPayload converts a CLIProxyAPI-style Responses payload into a ResponseRequest.
func ConvertResponsesPayload(payload map[string]any) domains.ResponseRequest {
	req := domains.ResponseRequest{
		Extra: map[string]any{},
	}
	for key, value := range payload {
		switch key {
		case "model":
			req.Model, _ = value.(string)
		case "input":
			req.Input = value
		case "instructions":
			req.Instructions, _ = value.(string)
		case "tools":
			req.Tools = convertTools(value)
		case "tool_choice":
			req.ToolChoice = value
		case "temperature":
			if number, ok := asFloat64(value); ok {
				req.Temperature = &number
			}
		case "max_output_tokens":
			if number, ok := asInt(value); ok {
				req.MaxOutputTokens = &number
			}
		case "reasoning":
			req.Reasoning = convertReasoning(value)
		case "model_reasoning_effort":
			if effort, ok := value.(string); ok && effort != "" {
				req.Reasoning = &domains.Reasoning{Effort: effort}
			}
		case "store":
			if store, ok := value.(bool); ok {
				req.Store = &store
			}
		case "disable_response_storage":
			if disabled, ok := value.(bool); ok && disabled {
				store := false
				req.Store = &store
			}
		case "metadata":
			req.Metadata = convertStringAnyMap(value)
		case "text":
			req.ResponseFormat = value
		case "previous_response_id":
			req.PreviousResponseID, _ = value.(string)
		case "stream":
			continue
		default:
			req.Extra[key] = value
		}
	}
	if len(req.Extra) == 0 {
		req.Extra = nil
	}
	return req
}

// ConvertResponsesJSON converts raw JSON into a ResponseRequest.
func ConvertResponsesJSON(data []byte) (domains.ResponseRequest, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return domains.ResponseRequest{}, err
	}
	return ConvertResponsesPayload(payload), nil
}

// MustConvertResponsesJSON is a convenience for tests and static examples.
func MustConvertResponsesJSON(data []byte) domains.ResponseRequest {
	req, err := ConvertResponsesJSON(data)
	if err != nil {
		panic(err)
	}
	return req
}

func convertTools(value any) []domains.Tool {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	tools := make([]domains.Tool, 0, len(items))
	for _, item := range items {
		if payload := convertStringAnyMap(item); len(payload) > 0 {
			tools = append(tools, domains.RawTool(payload))
		}
	}
	return tools
}

func convertReasoning(value any) *domains.Reasoning {
	payload := convertStringAnyMap(value)
	if len(payload) == 0 {
		return nil
	}
	effort, _ := payload["effort"].(string)
	if effort == "" {
		return nil
	}
	return &domains.Reasoning{Effort: effort}
}

func convertStringAnyMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = value
		}
		return out
	default:
		return nil
	}
}

func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	default:
		return 0, false
	}
}
