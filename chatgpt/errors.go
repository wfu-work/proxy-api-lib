package chatgpt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError 描述 ChatGPT/Codex 账号接口返回的错误。
type APIError struct {
	StatusCode int
	Endpoint   string
	Code       string
	Type       string
	Message    string
	RequestID  string
	Cause      error
}

// Error 返回账号接口错误的可读文本。
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = fmt.Sprintf("status %d", e.StatusCode)
	}
	if e.Code != "" {
		return fmt.Sprintf("chatgpt: %s (%s)", message, e.Code)
	}
	return "chatgpt: " + message
}

// Unwrap 返回账号请求的底层错误。
func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// parseAPIError 兼容 ChatGPT 常见的顶层和嵌套错误结构。
func parseAPIError(statusCode int, endpoint, requestID string, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Code    string `json:"code"`
	}
	_ = json.Unmarshal(body, &envelope)
	message := strings.TrimSpace(envelope.Error.Message)
	if message == "" {
		message = strings.TrimSpace(envelope.Message)
	}
	if message == "" {
		message = strings.TrimSpace(envelope.Detail)
	}
	if message == "" {
		message = compactBody(body)
	}
	code := strings.TrimSpace(envelope.Error.Code)
	if code == "" {
		code = strings.TrimSpace(envelope.Code)
	}
	return &APIError{
		StatusCode: statusCode,
		Endpoint:   endpoint,
		Code:       code,
		Type:       strings.TrimSpace(envelope.Error.Type),
		Message:    message,
		RequestID:  requestID,
	}
}

// responseRequestID 读取 OpenAI 常用的请求追踪头。
func responseRequestID(header http.Header) string {
	if value := strings.TrimSpace(header.Get("x-request-id")); value != "" {
		return value
	}
	return strings.TrimSpace(header.Get("x-oai-request-id"))
}

// compactBody 压缩并截断非标准错误响应。
func compactBody(body []byte) string {
	message := strings.Join(strings.Fields(string(body)), " ")
	const maxLength = 512
	if len(message) > maxLength {
		message = message[:maxLength] + "..."
	}
	return message
}
