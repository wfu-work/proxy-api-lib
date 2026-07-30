# proxy-api-lib

[English](README.md) | [简体中文](README.zh-CN.md)

`proxy-api-lib` 是只集成 OpenAI 官方服务的 Go 客户端与协议工具库，不包含 Aiok、FreeModel、Tokeni、CodexZH 等第三方中转站逻辑。

工程明确区分两条边界：

- `proxyapi` 和 `openai`：公开稳定的 OpenAI Platform API，默认地址为 `https://api.openai.com/v1`。
- `codexauth` 和 `chatgpt`：OpenAI 官方 ChatGPT/Codex 登录账号的 Token 解析、刷新、额度和订阅查询。

FreeAI 这类 OAuth 账号池只使用第二条边界访问上游。`proxyapi` 支持的 Platform API Key 是基础库面向其他调用方的独立能力，不会作为 `chatgpt` 的备用凭据。

`chatgpt` 调用的服务位于 `auth.openai.com` 和 `chatgpt.com`，但账号额度与订阅端点不是公开稳定的 OpenAI Platform API，可能随 ChatGPT/Codex 协议调整。业务代码应通过本库访问，不应直接依赖内部响应 JSON。

模块路径：

```text
github.com/wfu-work/proxy-api-lib
```

## 工程边界

本库负责：

- OpenAI 官方 Responses API，包括 SSE 流式事件和工具调用。
- OpenAI 官方 Embeddings 和 Models 资源。
- OpenAI API Key 与调用方管理的 Bearer Token。
- ChatGPT/Codex JWT 展示字段解析和 OAuth Refresh Token 刷新。
- 规范 OAuth 账号文件解析与标准化导出。
- ChatGPT/Codex wham 额度窗口、账号列表、`accounts/check` + `subscriptions` 合并，以及订阅到期/续费查询。
- 官方 Codex 模型发现、Responses 请求和 `x-codex-*` 响应头保留。
- Codex 请求形态规范化，以及面向非流式调用方的上游 SSE 聚合。
- 面向网关的 OpenAI Responses、Chat Completions 编解码。
- OpenAI 风格错误、Organization/Project 请求头、自定义 HTTP Client 和网络代理。

应用层负责：

- 在本库之外完成首次 OAuth Token 获取，或导入已有规范账号文件。
- Access Token、Refresh Token 和 ID Token 的加密存储。
- Token 刷新调度、刷新结果的原子持久化和 401 后重试。
- 账号池、路由、故障切换以及额度快照持久化。
- 对外 API 的身份认证与授权。
- 请求日志和本地用量统计。

## ChatGPT/Codex 账号信息

已有 ChatGPT/Codex OAuth Token 时，可以解析账号展示字段并刷新 Access Token：

```go
claims, err := codexauth.ParseUnverifiedClaims(accessToken)
if err != nil {
	return err
}
accountID := claims.ResolvedAccountID()

oauth := codexauth.NewOAuthClient()
tokens, err := oauth.Refresh(ctx, refreshToken)
if err != nil {
	return err
}
accessToken = tokens.AccessToken
refreshToken = tokens.EffectiveRefreshToken(refreshToken)
```

已有账号导出文件也可以通过 `codexauth.ParseAccountFile` 解析和标准化；应用应对完整规范文件进行静态加密。

`ParseUnverifiedClaims` 只解码 JWT，不验证签名、签发者或受众，结果只能用于展示和账号路由，不能用于授权判断。

使用 Access Token 查询额度窗口和订阅信息：

```go
accountClient := chatgpt.NewClient(
	chatgpt.WithAccessToken(accessToken),
)

usage, err := accountClient.Usage.Get(ctx, accountID)
if err != nil {
	return err
}

subscription, err := accountClient.Accounts.Subscription(ctx, accountID)
if err != nil {
	return err
}

models, err := accountClient.Codex.Models(ctx, accountID, clientVersion)
```

`usage.RateLimit` 返回已用比例、窗口长度和重置时间，通常对应 5 小时和 7 天窗口，并保留 Code Review、Spark 等附加窗口。它不是精确 Token 数量。`Subscription` 会在可用时同时查询两类账号接口，补齐套餐、订阅状态、到期、续费时间和 `WillRenew`，不会猜测上游未提供的值。

应用已经实现安全的刷新与存储时，可通过 `chatgpt.WithTokenSource` 在每次请求前读取最新 Access Token。

> OpenAI Platform 的组织用量与费用是另一套公开 API，使用组织 Admin Key。它与 ChatGPT 套餐包含的 Codex 额度不是同一个账户或计费模型。

## 安装

```bash
go get github.com/wfu-work/proxy-api-lib
```

需要 Go 1.26 或更高版本。

## Responses API

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wfu-work/proxy-api-lib/openai"
	"github.com/wfu-work/proxy-api-lib/proxyapi"
)

func main() {
	client := proxyapi.NewClient(
		proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	resp, err := client.Responses.Create(context.Background(), openai.ResponseRequest{
		Model: "gpt-4.1",
		Input: openai.InputText("用一句话问好。"),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.OutputText())
}
```

如果应用层管理的是访问令牌：

```go
client := proxyapi.NewClient(
	proxyapi.WithBearerToken(accessToken),
)
```

动态令牌可实现 `auth.TokenSource`，再通过 `auth.FromTokenSource` 交给客户端。

## 流式调用

```go
stream, err := client.Responses.Stream(ctx, openai.ResponseRequest{
	Model: "gpt-4.1",
	Input: openai.InputText("从一数到三。"),
})
if err != nil {
	return err
}
defer stream.Close()

acc := openai.NewStreamAccumulator()
for stream.Next() {
	event := stream.Event()
	acc.Add(event)
	fmt.Print(event.TextDelta())
}
if err := stream.Err(); err != nil {
	return err
}
```

## 网关协议转换

OpenAI Responses 请求可直接解码：

```go
request, err := responses.Decode(body)
```

Chat Completions 请求可以转换成 Responses 请求，Responses 结果也可以转换回 Chat Completions：

```go
request, err := chatcompletions.Decode(body)
payload := chatcompletions.Response(model, response)
```

未知请求字段会保存在 `ResponseRequest.Extra`，便于网关兼容 OpenAI 后续新增字段。

## 错误处理

```go
var apiErr *openai.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode)
	fmt.Println(apiErr.Code)
	fmt.Println(apiErr.Message)
	fmt.Println(apiErr.RequestID)
}
```

Platform 客户端的 `WithBaseURL` 仅用于测试或受控的 OpenAI 官方端点部署，不承担 Provider 注册或第三方中转预设职责。ChatGPT 账号客户端也提供独立的 `chatgpt.WithBaseURL`，两个地址配置不会相互污染。

## 验证

```bash
go test ./...
```
