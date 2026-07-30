package proxyapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	responsescodec "github.com/wfu-work/proxy-api-lib/codec/responses"
	"github.com/wfu-work/proxy-api-lib/openai"
)

// ResponsesService 提供 OpenAI 官方 Responses API。
type ResponsesService struct {
	client *Client
}

// Create 发送非流式 Responses 请求，并返回完整响应。
// 请求中的 Credential 非空时会覆盖客户端默认凭据。
func (s *ResponsesService) Create(ctx context.Context, req openai.ResponseRequest) (*openai.Response, error) {
	body, err := responsescodec.Encode(req, false)
	if err != nil {
		return nil, err
	}
	httpReq, err := s.client.newRequest(ctx, http.MethodPost, "/responses", body, "application/json", req.Credential)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("x-request-id"), respBody)
	}
	var out openai.Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	out.Raw = append(out.Raw[:0], respBody...)
	out.RequestID = resp.Header.Get("x-request-id")
	return &out, nil
}

// parseAPIError 将 OpenAI 错误响应解析为稳定的 APIError。
// 当响应不符合标准错误结构时，保留状态码和原始响应内容。
func parseAPIError(statusCode int, requestID string, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return &openai.APIError{
			StatusCode: statusCode,
			Code:       envelope.Error.Code,
			Type:       envelope.Error.Type,
			Message:    envelope.Error.Message,
			RequestID:  requestID,
		}
	}
	return &openai.APIError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf("OpenAI returned status %d: %s", statusCode, string(body)),
		RequestID:  requestID,
	}
}
