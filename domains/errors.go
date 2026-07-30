package domains

import "fmt"

// APIError 以稳定结构封装 OpenAI API 返回的错误信息。
type APIError struct {
	StatusCode int
	Code       string
	Type       string
	Message    string
	RequestID  string
	Cause      error
}

// Error 返回包含 OpenAI 标识、错误消息和可选错误代码的可读文本。
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("openai: %s (%s)", e.Message, e.Code)
	}
	return fmt.Sprintf("openai: %s", e.Message)
}

// Unwrap 返回底层错误，使 errors.Is 和 errors.As 可以继续匹配。
func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
