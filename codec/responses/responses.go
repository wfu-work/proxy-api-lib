// Package responses 将 OpenAI 官方 Responses 请求解码为库内类型，
// 同时保留尚未类型化的扩展字段以支持协议向前兼容。
package responses

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/wfu-work/proxy-api-lib/openai"
)

// Decode 解码 Responses API JSON 请求，并保留 JSON 数字精度。
func Decode(data []byte) (openai.ResponseRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return openai.ResponseRequest{}, err
	}
	return FromMap(payload), nil
}

// FromMap 将通用字段映射转换为类型化 Responses 请求。
// 未识别字段会保存到 Extra，stream 字段由调用方的传输方式决定。
func FromMap(payload map[string]any) openai.ResponseRequest {
	req := openai.ResponseRequest{Extra: map[string]any{}}
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
		case "store":
			if store, ok := value.(bool); ok {
				req.Store = &store
			}
		case "metadata":
			req.Metadata = stringMap(value)
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

// convertTools 将工具数组转换为支持字段透传的 RawTool 列表。
func convertTools(value any) []openai.Tool {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	tools := make([]openai.Tool, 0, len(items))
	for _, item := range items {
		if payload := stringMap(item); len(payload) > 0 {
			tools = append(tools, openai.RawTool(payload))
		}
	}
	return tools
}

// convertReasoning 解析 Responses API 的推理强度配置。
func convertReasoning(value any) *openai.Reasoning {
	payload := stringMap(value)
	effort, _ := payload["effort"].(string)
	if effort == "" {
		return nil
	}
	return &openai.Reasoning{Effort: effort}
}

// stringMap 将常见的动态 map 结构统一转换为 string 键。
func stringMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprint(key)] = item
		}
		return out
	default:
		return nil
	}
}

// asFloat64 将 JSON 或 Go 数值转换为 float64。
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

// asInt 将 JSON 或 Go 数值转换为 int。
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
