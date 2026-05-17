package domains

import "encoding/json"

// Response is the provider-neutral Responses API result.
type Response struct {
	ID        string          `json:"id,omitempty"`
	Model     string          `json:"model,omitempty"`
	Status    string          `json:"status,omitempty"`
	Output    []ResponseItem  `json:"output,omitempty"`
	Usage     *Usage          `json:"usage,omitempty"`
	Raw       json.RawMessage `json:"-"`
	RequestID string          `json:"-"`
}

// OutputText returns concatenated text found in output items.
func (r *Response) OutputText() string {
	if r == nil {
		return ""
	}
	var out string
	for _, item := range r.Output {
		for _, content := range item.Content {
			if content.Text != "" {
				out += content.Text
			}
		}
	}
	return out
}

// ToolCalls returns function tool calls found in output items.
func (r *Response) ToolCalls() []ToolCall {
	if r == nil {
		return nil
	}
	var calls []ToolCall
	for _, item := range r.Output {
		if item.Type != "function_call" {
			continue
		}
		calls = append(calls, ToolCall{
			ID:        item.ID,
			CallID:    item.CallID,
			Name:      item.Name,
			Arguments: item.Arguments,
		})
	}
	return calls
}

// ResponseItem is a single output item.
type ResponseItem struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Role      string            `json:"role,omitempty"`
	Status    string            `json:"status,omitempty"`
	Content   []ResponseContent `json:"content,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
	Raw       json.RawMessage   `json:"-"`
}

// ResponseContent is a content part in an output item.
type ResponseContent struct {
	Type        string          `json:"type,omitempty"`
	Text        string          `json:"text,omitempty"`
	Annotations []any           `json:"annotations,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// Usage captures token usage when supplied by the provider.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
