package proxyapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wfu-work/proxy-api-lib/openai"
)

// Stream 发送流式 Responses 请求，并返回逐事件读取的迭代器。
// 调用方读取结束后应调用 Close；服务端错误事件会通过 Err 返回。
func (s *ResponsesService) Stream(ctx context.Context, req openai.ResponseRequest) (*openai.ResponseStream, error) {
	body, err := marshalResponseRequest(req, true)
	if err != nil {
		return nil, err
	}
	httpReq, err := s.client.newRequest(ctx, http.MethodPost, "/responses", body, "text/event-stream", req.Credential)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("x-request-id"), respBody)
	}

	reader := newSSEReader(resp.Body)
	return openai.NewResponseStream(reader.Next, reader.Event, reader.Err, reader.Close), nil
}

// sseReader 负责读取和解析 OpenAI Responses API 的 SSE 数据。
type sseReader struct {
	body      io.Closer
	scanner   *bufio.Scanner
	eventName string
	dataLines []string
	current   openai.StreamEvent
	err       error
	done      bool
}

// newSSEReader 创建 SSE 读取器，并将单条事件的最大容量设置为 4 MiB。
func newSSEReader(body io.ReadCloser) *sseReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &sseReader{body: body, scanner: scanner}
}

// Next 扫描到下一条完整 SSE 事件。
// 返回 false 表示流结束或发生错误，错误可通过 Err 获取。
func (r *sseReader) Next() bool {
	if r.done {
		return false
	}
	for r.scanner.Scan() {
		line := strings.TrimRight(r.scanner.Text(), "\r")
		if line == "" {
			return r.dispatch()
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			r.eventName = value
		case "data":
			r.dataLines = append(r.dataLines, value)
		}
	}
	if err := r.scanner.Err(); err != nil {
		r.err = err
		_ = r.Close()
		return false
	}
	if len(r.dataLines) > 0 {
		return r.dispatch()
	}
	r.done = true
	_ = r.Close()
	return false
}

// Event 返回最近一次 Next 成功读取的事件。
func (r *sseReader) Event() openai.StreamEvent { return r.current }

// Err 返回 SSE 读取或服务端错误事件产生的错误。
func (r *sseReader) Err() error { return r.err }

// Close 关闭响应体并终止后续读取；重复调用是安全的。
func (r *sseReader) Close() error {
	r.done = true
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

// dispatch 将当前缓存的 SSE 字段组装为一条 OpenAI 流事件。
// 遇到 [DONE] 或错误事件时会结束并关闭底层响应体。
func (r *sseReader) dispatch() bool {
	if len(r.dataLines) == 0 {
		r.eventName = ""
		return r.Next()
	}
	data := strings.Join(r.dataLines, "\n")
	r.dataLines = nil
	eventType := r.eventName
	r.eventName = ""
	if data == "[DONE]" {
		r.done = true
		_ = r.Close()
		return false
	}
	raw := json.RawMessage(data)
	if eventType == "" {
		eventType = eventTypeFromData(raw)
	}
	r.current = openai.StreamEvent{Type: eventType, Data: raw}
	if isStreamErrorEvent(eventType, raw) {
		r.err = streamEventError(eventType, raw)
		r.done = true
		_ = r.Close()
	}
	return true
}

// eventTypeFromData 从 SSE data JSON 中解析事件类型。
func eventTypeFromData(data json.RawMessage) string {
	var payload struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(data, &payload)
	return payload.Type
}

// isStreamErrorEvent 判断事件是否表示 Responses 流执行失败。
func isStreamErrorEvent(eventType string, data json.RawMessage) bool {
	if eventType == "error" || eventType == "response.failed" || eventType == "response.incomplete" {
		return true
	}
	var payload struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	return payload.Error != nil && (payload.Type == "error" || payload.Type == "response.failed")
}

// streamEventError 将流式错误事件转换为可供上层处理的错误。
func streamEventError(eventType string, data json.RawMessage) error {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("openai: stream event %s", eventType)
	}
	if payload.Error.Message != "" {
		return &openai.APIError{Provider: "openai", Type: payload.Error.Type, Code: payload.Error.Code, Message: payload.Error.Message}
	}
	if payload.Message != "" {
		return fmt.Errorf("openai: stream event %s: %s", eventType, payload.Message)
	}
	return fmt.Errorf("openai: stream event %s", eventType)
}
