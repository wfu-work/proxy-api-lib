package domains

// JSONSchema is a minimal JSON Schema representation for function tools.
type JSONSchema struct {
	Type                 string                `json:"type,omitempty"`
	Description          string                `json:"description,omitempty"`
	Properties           map[string]JSONSchema `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	Items                *JSONSchema           `json:"items,omitempty"`
	Enum                 []string              `json:"enum,omitempty"`
	AdditionalProperties any                   `json:"additionalProperties,omitempty"`
}
