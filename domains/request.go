package domains

// ResponseRequest is the provider-neutral request shape for the Responses API.
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

// InputText is a convenience helper for simple text prompts.
func InputText(text string) string {
	return text
}

// Reasoning contains reasoning-specific request knobs.
type Reasoning struct {
	Effort string `json:"effort,omitempty"`
}
