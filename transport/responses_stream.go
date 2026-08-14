package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wfu-work/proxy-api-lib/openai"
)

const (
	responseSSEInitialBufferBytes = 64 << 10
	responseSSEMaxLineBytes       = 64 << 20
)

// NewResponseStream 将 Responses SSE 响应体包装为统一事件迭代器。
func NewResponseStream(body io.ReadCloser) *openai.ResponseStream {
	reader := newResponseSSEReader(body)
	return openai.NewResponseStream(reader.Next, reader.Event, reader.Err, reader.Close)
}

type responseSSEReader struct {
	body      io.Closer
	scanner   *bufio.Scanner
	eventName string
	dataLines []string
	current   openai.StreamEvent
	err       error
	done      bool
}

func newResponseSSEReader(body io.ReadCloser) *responseSSEReader {
	scanner := bufio.NewScanner(body)
	// Compaction output contains encrypted state, and the final completed event
	// can repeat the full response in one SSE data line. Keep the reader bounded
	// while allowing those events to grow beyond the old 4 MiB limit.
	scanner.Buffer(make([]byte, 0, responseSSEInitialBufferBytes), responseSSEMaxLineBytes)
	return &responseSSEReader{body: body, scanner: scanner}
}

func (r *responseSSEReader) Next() bool {
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

func (r *responseSSEReader) Event() openai.StreamEvent { return r.current }
func (r *responseSSEReader) Err() error                { return r.err }

func (r *responseSSEReader) Close() error {
	r.done = true
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

func (r *responseSSEReader) dispatch() bool {
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
		eventType = responseEventType(raw)
	}
	r.current = openai.StreamEvent{Type: eventType, Data: raw}
	if responseStreamError(eventType, raw) {
		r.err = responseStreamEventError(eventType, raw)
		r.done = true
		_ = r.Close()
	}
	return true
}

func responseEventType(data json.RawMessage) string {
	var payload struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(data, &payload)
	return payload.Type
}

func responseStreamError(eventType string, data json.RawMessage) bool {
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

func responseStreamEventError(eventType string, data json.RawMessage) error {
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
		return &openai.APIError{Type: payload.Error.Type, Code: payload.Error.Code, Message: payload.Error.Message}
	}
	if payload.Message != "" {
		return fmt.Errorf("openai: stream event %s: %s", eventType, payload.Message)
	}
	return fmt.Errorf("openai: stream event %s", eventType)
}
