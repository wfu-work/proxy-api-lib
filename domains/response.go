package domains

import "encoding/json"

// Response 描述 OpenAI 官方 Responses API 返回结果。
// Raw 保留原始 JSON，RequestID 保存响应头中的 OpenAI 请求 ID。
type Response struct {
	ID        string          `json:"id,omitempty"`
	CreatedAt int64           `json:"created_at,omitempty"`
	Model     string          `json:"model,omitempty"`
	Status    string          `json:"status,omitempty"`
	Output    []ResponseItem  `json:"output,omitempty"`
	Usage     *Usage          `json:"usage,omitempty"`
	Raw       json.RawMessage `json:"-"`
	RequestID string          `json:"-"`
}

// OutputText 按输出项顺序拼接所有文本内容。
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

// ToolCalls 提取响应中模型发起的全部函数工具调用。
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

// ResponseItem 表示 Responses 响应中的单个输出项。
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

// ResponseContent 表示输出项中的一段内容。
type ResponseContent struct {
	Type        string          `json:"type,omitempty"`
	Text        string          `json:"text,omitempty"`
	Annotations []any           `json:"annotations,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// Usage 记录 OpenAI 返回的输入、输出和总 Token 用量。
type Usage struct {
	InputTokens         int                 `json:"input_tokens,omitempty"`
	InputTokensDetails  *InputTokenDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int                 `json:"output_tokens,omitempty"`
	OutputTokensDetails *OutputTokenDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int                 `json:"total_tokens,omitempty"`
}

// InputTokenDetails 记录输入 Token 中命中上游缓存的数量。
//
// OpenAI 的 input_tokens 已经包含 cached_tokens，计算费用时需要先从普通输入中扣除。
type InputTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// OutputTokenDetails 记录输出 Token 中属于推理过程的数量。
// 推理 Token 已包含在 output_tokens 中，此字段主要用于观测，不重复计费。
type OutputTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}
