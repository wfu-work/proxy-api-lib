package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	responsescodec "github.com/wfu-work/proxy-api-lib/codec/responses"
	"github.com/wfu-work/proxy-api-lib/openai"
	"github.com/wfu-work/proxy-api-lib/transport"
)

const (
	codexModelsPath    = "/codex/models"
	codexResponsesPath = "/codex/responses"
)

// CodexService 提供 ChatGPT 订阅账号可用的模型清单和 Responses 传输。
type CodexService struct{ client *Client }

// CodexModelCatalog 保留官方模型清单原文和响应元数据。
type CodexModelCatalog struct {
	Models    []CodexModel    `json:"models"`
	Raw       json.RawMessage `json:"-"`
	Header    http.Header     `json:"-"`
	RequestID string          `json:"-"`
}

// CodexModel 仅固定模型标识；其余官方字段保存在 Raw 中。
type CodexModel struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug,omitempty"`
	DisplayName string          `json:"display_name,omitempty"`
	Raw         json.RawMessage `json:"-"`
}

// CodexResponse 是非流式 Responses 调用结果，包含可用于额度采样的原始响应头。
type CodexResponse struct {
	Response   *openai.Response
	Header     http.Header
	StatusCode int
	RequestID  string
}

// CodexResponseStream 是流式 Responses 调用结果。
type CodexResponseStream struct {
	Stream     *openai.ResponseStream
	Header     http.Header
	StatusCode int
	RequestID  string
}

// Models 获取指定 OAuth 账号可用的官方 Codex 模型清单。
func (s *CodexService) Models(ctx context.Context, accountID, clientVersion string) (*CodexModelCatalog, error) {
	path := codexModelsPath
	if clientVersion = strings.TrimSpace(clientVersion); clientVersion != "" {
		path += "?" + url.Values{"client_version": []string{clientVersion}}.Encode()
	}
	req, err := s.client.newRequest(ctx, http.MethodGet, path, accountID, map[string]string{
		"Accept-Encoding": "identity",
	})
	if err != nil {
		return nil, err
	}
	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	requestID := responseRequestID(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, req.URL.Path, requestID, body)
	}
	catalog, err := decodeCodexModelCatalog(body)
	if err != nil {
		return nil, err
	}
	catalog.Raw = append(catalog.Raw[:0], body...)
	catalog.Header = resp.Header.Clone()
	catalog.RequestID = requestID
	return catalog, nil
}

// Create 对调用方提供非流式结果，但上游始终使用 Codex 要求的 SSE 传输并在本地聚合。
func (s *CodexService) Create(ctx context.Context, accountID string, input openai.ResponseRequest) (*CodexResponse, error) {
	upstream, err := s.Stream(ctx, accountID, input)
	if err != nil {
		return nil, err
	}
	defer upstream.Stream.Close()
	var completed *openai.Response
	for upstream.Stream.Next() {
		event := upstream.Stream.Event()
		response, ok := event.CompletedResponse()
		if !ok {
			continue
		}
		completed = response
		var envelope struct {
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(event.Data, &envelope) == nil && len(envelope.Response) > 0 {
			completed.Raw = append(completed.Raw[:0], envelope.Response...)
		}
	}
	if err := upstream.Stream.Err(); err != nil {
		return nil, err
	}
	if completed == nil {
		return nil, errors.New("chatgpt: Codex response stream ended without response.completed")
	}
	if len(completed.Raw) == 0 {
		if raw, marshalErr := json.Marshal(completed); marshalErr == nil {
			completed.Raw = raw
		}
	}
	completed.RequestID = upstream.RequestID
	return &CodexResponse{
		Response: completed, Header: upstream.Header.Clone(), StatusCode: upstream.StatusCode, RequestID: upstream.RequestID,
	}, nil
}

// Stream 发送流式官方 Codex Responses 请求。
func (s *CodexService) Stream(ctx context.Context, accountID string, input openai.ResponseRequest) (*CodexResponseStream, error) {
	input = prepareCodexRequest(input)
	body, err := responsescodec.Encode(input, true)
	if err != nil {
		return nil, err
	}
	resp, err := s.doResponses(ctx, accountID, body, "text/event-stream", nil)
	if err != nil {
		return nil, err
	}
	requestID := responseRequestID(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if readErr != nil {
			return nil, readErr
		}
		return nil, parseAPIError(resp.StatusCode, codexResponsesPath, requestID, responseBody)
	}
	return &CodexResponseStream{
		Stream: transport.NewResponseStream(resp.Body), Header: resp.Header.Clone(), StatusCode: resp.StatusCode, RequestID: requestID,
	}, nil
}

// prepareCodexRequest 将公开 Responses/Chat Completions 常见输入收敛为 Codex 内部端点接受的请求形态。
func prepareCodexRequest(input openai.ResponseRequest) openai.ResponseRequest {
	input.Input = normalizeCodexInput(input.Input)
	input.MaxOutputTokens = nil
	input.Temperature = nil
	store := false
	input.Store = &store

	extra := make(map[string]any, len(input.Extra)+1)
	for key, value := range input.Extra {
		extra[key] = value
	}
	for _, unsupported := range []string{"max_output_tokens", "max_tokens", "max_completion_tokens", "temperature", "stream", "store"} {
		delete(extra, unsupported)
	}
	if strings.TrimSpace(input.Instructions) == "" {
		extra["instructions"] = ""
	}
	if len(extra) == 0 {
		extra = nil
	}
	input.Extra = extra
	return input
}

func normalizeCodexInput(input any) any {
	switch value := input.(type) {
	case string:
		return []any{codexTextMessage("user", value)}
	case map[string]any:
		return []any{normalizeCodexInputItem(value)}
	case []map[string]any:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, normalizeCodexInputItem(item))
		}
		return items
	case []any:
		items := make([]any, 0, len(value))
		for _, item := range value {
			switch typed := item.(type) {
			case string:
				items = append(items, codexTextMessage("user", typed))
			case map[string]any:
				items = append(items, normalizeCodexInputItem(typed))
			default:
				items = append(items, item)
			}
		}
		return items
	default:
		return input
	}
}

func normalizeCodexInputItem(item map[string]any) map[string]any {
	itemType, _ := item["type"].(string)
	role, hasRole := item["role"].(string)
	if itemType != "" && itemType != "message" || !hasRole && itemType != "message" {
		return item
	}
	if strings.TrimSpace(role) == "" {
		role = "user"
	}
	out := make(map[string]any, len(item)+1)
	for key, value := range item {
		out[key] = value
	}
	out["type"] = "message"
	out["role"] = role
	out["content"] = normalizeCodexContent(role, item["content"])
	return out
}

func normalizeCodexContent(role string, content any) any {
	blockType := "input_text"
	if role == "assistant" {
		blockType = "output_text"
	}
	switch value := content.(type) {
	case string:
		return []any{map[string]any{"type": blockType, "text": value}}
	case []map[string]any:
		blocks := make([]any, 0, len(value))
		for _, block := range value {
			blocks = append(blocks, normalizeCodexContentBlock(blockType, block))
		}
		return blocks
	case []any:
		blocks := make([]any, 0, len(value))
		for _, block := range value {
			switch typed := block.(type) {
			case string:
				blocks = append(blocks, map[string]any{"type": blockType, "text": typed})
			case map[string]any:
				blocks = append(blocks, normalizeCodexContentBlock(blockType, typed))
			default:
				blocks = append(blocks, block)
			}
		}
		return blocks
	default:
		return content
	}
}

func normalizeCodexContentBlock(defaultType string, block map[string]any) map[string]any {
	blockType, _ := block["type"].(string)
	if blockType != "" && blockType != "text" {
		return block
	}
	if _, ok := block["text"].(string); !ok {
		return block
	}
	out := make(map[string]any, len(block)+1)
	for key, value := range block {
		out[key] = value
	}
	out["type"] = defaultType
	return out
}

func codexTextMessage(role, text string) map[string]any {
	return map[string]any{
		"type": "message", "role": role,
		"content": []any{map[string]any{"type": "input_text", "text": text}},
	}
}

// DoResponses 执行原始官方 Codex Responses 请求。
// 调用方负责关闭返回响应体；此方法保留非 2xx 状态，便于网关决定账号切换策略。
func (s *CodexService) DoResponses(ctx context.Context, accountID string, body []byte, stream bool, extraHeaders map[string]string) (*http.Response, error) {
	accept := "application/json"
	if stream {
		accept = "text/event-stream"
	}
	return s.doResponses(ctx, accountID, body, accept, extraHeaders)
}

func (s *CodexService) doResponses(ctx context.Context, accountID string, body []byte, accept string, extraHeaders map[string]string) (*http.Response, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("chatgpt: Codex service is nil")
	}
	headers := map[string]string{"Content-Type": "application/json", "Accept": accept}
	for key, value := range extraHeaders {
		headers[key] = value
	}
	req, err := s.client.newRequestBody(ctx, http.MethodPost, codexResponsesPath, accountID, body, headers)
	if err != nil {
		return nil, err
	}
	return s.client.httpClient.Do(req)
}

func decodeCodexModelCatalog(body []byte) (*CodexModelCatalog, error) {
	var envelope struct {
		Models []json.RawMessage `json:"models"`
		Data   []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	items := envelope.Models
	if len(items) == 0 {
		items = envelope.Data
	}
	catalog := &CodexModelCatalog{Models: make([]CodexModel, 0, len(items))}
	for _, raw := range items {
		var model CodexModel
		if err := json.Unmarshal(raw, &model); err != nil {
			return nil, err
		}
		if model.ID == "" {
			model.ID = model.Slug
		}
		model.Raw = append(model.Raw[:0], raw...)
		catalog.Models = append(catalog.Models, model)
	}
	return catalog, nil
}
