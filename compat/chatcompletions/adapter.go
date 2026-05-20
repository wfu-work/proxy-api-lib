package chatcompletions

import (
	"encoding/json"

	"github.com/wfu-work/proxy-api-lib/domains"
)

// ConvertPayload converts a Chat Completions-style payload into a Responses request.
func ConvertPayload(payload map[string]any) domains.ResponseRequest {
	req := domains.ResponseRequest{Extra: map[string]any{}}
	for key, value := range payload {
		switch key {
		case "model":
			req.Model, _ = value.(string)
		case "messages":
			req.Input = value
		case "tools":
			req.Tools = convertChatTools(value)
		case "tool_choice":
			req.ToolChoice = value
		case "temperature":
			if number, ok := asFloat64(value); ok {
				req.Temperature = &number
			}
		case "max_tokens":
			if number, ok := asInt(value); ok {
				req.MaxOutputTokens = &number
			}
		case "max_completion_tokens":
			if number, ok := asInt(value); ok {
				req.MaxOutputTokens = &number
			}
		case "metadata":
			if metadata, ok := value.(map[string]any); ok {
				req.Metadata = metadata
			}
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

// ConvertJSON converts Chat Completions JSON into a Responses request.
func ConvertJSON(data []byte) (domains.ResponseRequest, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return domains.ResponseRequest{}, err
	}
	return ConvertPayload(payload), nil
}

func convertChatTools(value any) []domains.Tool {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	tools := make([]domains.Tool, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if payload["type"] == "function" {
			if fn, ok := payload["function"].(map[string]any); ok {
				tool := domains.RawTool{
					"type": "function",
				}
				for key, value := range fn {
					tool[key] = value
				}
				tools = append(tools, tool)
				continue
			}
		}
		tools = append(tools, domains.RawTool(payload))
	}
	return tools
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
