package compatible

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/free-model/proxy-api-lib/domains"
)

func (p *ResponsesProvider) StreamResponse(ctx context.Context, req domains.ResponseRequest) (*domains.ResponseStream, error) {
	if p.initErr != nil {
		return nil, p.initErr
	}
	if p.wireAPI != WireAPIResponses {
		return nil, fmt.Errorf("compatible: unsupported wire api %q", p.wireAPI)
	}
	if req.Credential == nil {
		return nil, errors.New("compatible: credential is required")
	}

	body, err := marshalResponseRequest(req, true)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	for key, value := range p.headers {
		httpReq.Header.Set(key, value)
	}

	authHeader, err := req.Credential.AuthorizationHeader(ctx)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", authHeader)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, parseAPIError(p.name, resp.StatusCode, resp.Header.Get("x-request-id"), respBody)
	}

	reader := newSSEReader(p.name, resp.Body)
	return domains.NewResponseStream(reader.Next, reader.Event, reader.Err, reader.Close), nil
}

type sseReader struct {
	provider  string
	body      io.Closer
	scanner   *bufio.Scanner
	eventName string
	dataLines []string
	current   domains.StreamEvent
	err       error
	done      bool
}

func newSSEReader(provider string, body io.ReadCloser) *sseReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &sseReader{
		provider: provider,
		body:     body,
		scanner:  scanner,
	}
}

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

func (r *sseReader) Event() domains.StreamEvent {
	return r.current
}

func (r *sseReader) Err() error {
	return r.err
}

func (r *sseReader) Close() error {
	r.done = true
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

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
	r.current = domains.StreamEvent{
		Type: eventType,
		Data: raw,
	}
	if isStreamErrorEvent(eventType, raw) {
		r.err = streamEventError(r.provider, eventType, raw)
		r.done = true
		_ = r.Close()
	}
	return true
}

func eventTypeFromData(data json.RawMessage) string {
	var payload struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload.Type
}

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

func streamEventError(provider, eventType string, data json.RawMessage) error {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("compatible: stream event %s", eventType)
	}
	if payload.Error.Message != "" {
		return &domains.APIError{
			Provider: provider,
			Type:     payload.Error.Type,
			Code:     payload.Error.Code,
			Message:  payload.Error.Message,
		}
	}
	if payload.Message != "" {
		return fmt.Errorf("compatible: stream event %s: %s", eventType, payload.Message)
	}
	return fmt.Errorf("compatible: stream event %s", eventType)
}
