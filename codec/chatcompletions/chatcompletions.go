// Package chatcompletions 将 OpenAI Chat Completions 请求转换为网关核心使用的
// Responses API 请求，并提供反向响应转换。
package chatcompletions

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/wfu-work/proxy-api-lib/openai"
)

// Decode 解码 Chat Completions JSON 请求，并保留 JSON 数字精度。
func Decode(data []byte) (openai.ResponseRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return openai.ResponseRequest{}, err
	}
	return FromMap(payload), nil
}

// FromMap 将 Chat Completions 字段映射为 Responses 请求。
// 未识别字段会保存到 Extra，传输层专属字段不会继续上送。
func FromMap(payload map[string]any) openai.ResponseRequest {
	req := openai.ResponseRequest{Extra: map[string]any{}}
	for key, value := range payload {
		switch key {
		case "model":
			req.Model, _ = value.(string)
		case "messages":
			req.Input = value
		case "tools":
			req.Tools = convertTools(value)
		case "tool_choice":
			req.ToolChoice = value
		case "temperature":
			if number, ok := asFloat64(value); ok {
				req.Temperature = &number
			}
		case "max_tokens", "max_completion_tokens":
			if number, ok := asInt(value); ok {
				req.MaxOutputTokens = &number
			}
		case "reasoning_effort":
			if effort, ok := value.(string); ok && effort != "" {
				req.Reasoning = &openai.Reasoning{Effort: effort}
			}
		case "response_format":
			req.ResponseFormat = convertResponseFormat(value)
		case "metadata":
			if metadata, ok := value.(map[string]any); ok {
				req.Metadata = metadata
			}
		case "stream", "stream_options", "n":
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

// convertResponseFormat 将 Chat Completions 的 response_format
// 转换为 Responses API 的 text.format 结构。
func convertResponseFormat(value any) any {
	payload, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	formatType, _ := payload["type"].(string)
	if formatType == "json_schema" {
		if schema, ok := payload["json_schema"].(map[string]any); ok {
			format := map[string]any{"type": "json_schema"}
			for key, item := range schema {
				format[key] = item
			}
			return map[string]any{"format": format}
		}
	}
	if formatType == "json_object" || formatType == "text" {
		return map[string]any{"format": map[string]any{"type": formatType}}
	}
	return nil
}

// Response 将 Responses API 结果转换为 OpenAI Chat Completions 响应。
// 当响应包含函数调用时，content 为 nil，finish_reason 为 tool_calls。
func Response(model string, resp *openai.Response) map[string]any {
	id := "chatcmpl"
	created := time.Now().Unix()
	if resp != nil && resp.ID != "" {
		id = resp.ID
	}
	if resp != nil && resp.CreatedAt > 0 {
		created = resp.CreatedAt
	}
	message := map[string]any{"role": "assistant", "content": ""}
	finishReason := "stop"
	if resp != nil {
		message["content"] = resp.OutputText()
		if calls := resp.ToolCalls(); len(calls) > 0 {
			toolCalls := make([]map[string]any, 0, len(calls))
			for _, call := range calls {
				callID := call.CallID
				if callID == "" {
					callID = call.ID
				}
				toolCalls = append(toolCalls, map[string]any{
					"id": callID, "type": "function",
					"function": map[string]any{"name": call.Name, "arguments": call.Arguments},
				})
			}
			message["tool_calls"] = toolCalls
			message["content"] = nil
			finishReason = "tool_calls"
		}
	}
	return map[string]any{
		"id": id, "object": "chat.completion", "created": created, "model": model,
		"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finishReason}},
		"usage":   usage(resp),
	}
}

// convertTools 将 Chat Completions 的嵌套函数工具结构
// 转换为 Responses API 使用的扁平工具结构。
func convertTools(value any) []openai.Tool {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	tools := make([]openai.Tool, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if payload["type"] == "function" {
			if fn, ok := payload["function"].(map[string]any); ok {
				tool := openai.RawTool{"type": "function"}
				for key, value := range fn {
					tool[key] = value
				}
				tools = append(tools, tool)
				continue
			}
		}
		tools = append(tools, openai.RawTool(payload))
	}
	return tools
}

// usage 将 Responses Token 用量映射为 Chat Completions 字段名称。
func usage(resp *openai.Response) map[string]int {
	if resp == nil || resp.Usage == nil {
		return map[string]int{}
	}
	return map[string]int{"prompt_tokens": resp.Usage.InputTokens, "completion_tokens": resp.Usage.OutputTokens, "total_tokens": resp.Usage.TotalTokens}
}

// asFloat64 将 JSON 或 Go 数值转换为 float64。
func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

// asInt 将 JSON 或 Go 数值转换为 int。
func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), true
	case json.Number:
		number, err := typed.Int64()
		return int(number), err == nil
	default:
		return 0, false
	}
}
