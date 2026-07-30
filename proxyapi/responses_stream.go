package proxyapi

import (
	"context"
	"io"
	"net/http"

	responsescodec "github.com/wfu-work/proxy-api-lib/codec/responses"
	"github.com/wfu-work/proxy-api-lib/openai"
	"github.com/wfu-work/proxy-api-lib/transport"
)

// Stream 发送流式 Responses 请求，并返回逐事件读取的迭代器。
// 调用方读取结束后应调用 Close；服务端错误事件会通过 Err 返回。
func (s *ResponsesService) Stream(ctx context.Context, req openai.ResponseRequest) (*openai.ResponseStream, error) {
	body, err := responsescodec.Encode(req, true)
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
	return transport.NewResponseStream(resp.Body), nil
}
