package compatible

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/free-model/proxy-api-lib/domains"
)

// ResponsesProvider implements the OpenAI-compatible Responses API.
type ResponsesProvider struct {
	name       string
	baseURL    string
	wireAPI    string
	httpClient *http.Client
	headers    map[string]string
	initErr    error
}

func (p *ResponsesProvider) Name() string {
	return p.name
}

func (p *ResponsesProvider) CreateResponse(ctx context.Context, req domains.ResponseRequest) (*domains.Response, error) {
	if p.initErr != nil {
		return nil, p.initErr
	}
	if p.wireAPI != WireAPIResponses {
		return nil, fmt.Errorf("compatible: unsupported wire api %q", p.wireAPI)
	}
	if req.Credential == nil {
		return nil, errors.New("compatible: credential is required")
	}

	body, err := marshalResponseRequest(req, false)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
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
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(p.name, resp.StatusCode, resp.Header.Get("x-request-id"), respBody)
	}

	var out domains.Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	out.Raw = append(out.Raw[:0], respBody...)
	out.RequestID = resp.Header.Get("x-request-id")
	return &out, nil
}

func marshalResponseRequest(req domains.ResponseRequest, stream bool) ([]byte, error) {
	payload := map[string]any{}
	if req.Model != "" {
		payload["model"] = req.Model
	}
	if req.Input != nil {
		payload["input"] = req.Input
	}
	if req.Instructions != "" {
		payload["instructions"] = req.Instructions
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if tool == nil {
				continue
			}
			payload, err := toolPayload(tool)
			if err != nil {
				return nil, err
			}
			tools = append(tools, payload)
		}
		payload["tools"] = tools
	}
	if req.ToolChoice != nil {
		payload["tool_choice"] = req.ToolChoice
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxOutputTokens != nil {
		payload["max_output_tokens"] = *req.MaxOutputTokens
	}
	if req.Reasoning != nil {
		payload["reasoning"] = req.Reasoning
	}
	if req.Store != nil {
		payload["store"] = *req.Store
	}
	if len(req.Metadata) > 0 {
		payload["metadata"] = req.Metadata
	}
	if req.ResponseFormat != nil {
		payload["text"] = req.ResponseFormat
	}
	if req.PreviousResponseID != "" {
		payload["previous_response_id"] = req.PreviousResponseID
	}
	if stream {
		payload["stream"] = true
	}
	for key, value := range req.Extra {
		payload[key] = value
	}
	return json.Marshal(payload)
}

func toolPayload(tool domains.Tool) (map[string]any, error) {
	data, err := json.Marshal(tool)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseAPIError(provider string, statusCode int, requestID string, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return &domains.APIError{
			Provider:   provider,
			StatusCode: statusCode,
			Code:       envelope.Error.Code,
			Type:       envelope.Error.Type,
			Message:    envelope.Error.Message,
			RequestID:  requestID,
		}
	}
	return &domains.APIError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    string(body),
		RequestID:  requestID,
	}
}
