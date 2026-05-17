package domains

import "encoding/json"

// Tool is implemented by concrete tool definitions.
type Tool interface {
	json.Marshaler
}

// FunctionTool describes a function tool available to the model.
type FunctionTool struct {
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Parameters  JSONSchema `json:"parameters,omitempty"`
	Strict      *bool      `json:"strict,omitempty"`
}

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

// MarshalJSON keeps the public Tool interface ergonomic while producing OpenAI-compatible JSON.
func (t FunctionTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.toolPayload())
}

// RawTool allows callers to pass provider-specific tool payloads.
type RawTool map[string]any

func (t RawTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any(t))
}

// ToolCall describes a function call requested by the model.
type ToolCall struct {
	ID          string
	CallID      string
	Name        string
	Arguments   string
	OutputIndex int
}

// FunctionCallOutput builds a Responses API input item for returning tool output.
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
