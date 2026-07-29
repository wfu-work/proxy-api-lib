package domains

import "encoding/json"

// Tool 是所有 OpenAI 工具定义需要实现的接口。
type Tool interface {
	json.Marshaler
}

// FunctionTool 描述可供模型调用的函数工具。
type FunctionTool struct {
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Parameters  JSONSchema `json:"parameters,omitempty"`
	Strict      *bool      `json:"strict,omitempty"`
}

// toolPayload 将函数工具转换为 OpenAI Responses API 接受的字段结构。
func (t FunctionTool) toolPayload() map[string]any {
	payload := map[string]any{
		"type":        "function",
		"name":        t.Name,
		"description": t.Description,
		"parameters":  t.Parameters,
	}
	if t.Strict != nil {
		payload["strict"] = *t.Strict
	}
	return payload
}

// MarshalJSON 将函数工具编码为 OpenAI 所需的 JSON 结构。
func (t FunctionTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.toolPayload())
}

// RawTool 允许调用方透传尚未被库类型化的 OpenAI 工具定义。
type RawTool map[string]any

// MarshalJSON 原样编码调用方提供的工具字段。
func (t RawTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any(t))
}

// ToolCall 描述模型请求执行的一次函数调用。
type ToolCall struct {
	ID          string
	CallID      string
	Name        string
	Arguments   string
	OutputIndex int
}

// FunctionCallOutput 构造向 Responses API 回传函数执行结果的输入项。
// 非字符串结果会优先编码为 JSON 字符串。
func FunctionCallOutput(callID string, output any) map[string]any {
	outputString, ok := output.(string)
	if !ok {
		data, err := json.Marshal(output)
		if err == nil {
			outputString = string(data)
		}
	}
	return map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  outputString,
	}
}
