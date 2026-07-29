package domains

// ResponseRequest 描述 OpenAI 官方 Responses API 请求。
// Extra 用于透传库尚未建模的新字段，Credential 可覆盖客户端默认凭据。
type ResponseRequest struct {
	Model              string         `json:"model,omitempty"`
	Input              any            `json:"input,omitempty"`
	Instructions       string         `json:"instructions,omitempty"`
	Tools              []Tool         `json:"tools,omitempty"`
	ToolChoice         any            `json:"tool_choice,omitempty"`
	Temperature        *float64       `json:"temperature,omitempty"`
	MaxOutputTokens    *int           `json:"max_output_tokens,omitempty"`
	Reasoning          *Reasoning     `json:"reasoning,omitempty"`
	Store              *bool          `json:"store,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	Extra              map[string]any `json:"-"`
	Credential         Credential     `json:"-"`
	ResponseFormat     any            `json:"text,omitempty"`
	PreviousResponseID string         `json:"previous_response_id,omitempty"`
}

// InputText 将简单文本转换为可直接用作 Responses input 的值。
func InputText(text string) string {
	return text
}

// Reasoning 描述模型推理强度等相关请求参数。
type Reasoning struct {
	Effort string `json:"effort,omitempty"`
}
