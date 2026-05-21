# proxy-api-lib

[English](README.md) | [简体中文](README.zh-CN.md)

`proxy-api-lib` 是一个用于调用 OpenAI-compatible AI Provider 的 Go 库。它提供统一的 API，封装 Responses API、流式事件、工具调用、Provider 预设，以及构建本地代理服务、CLI 工具、Agent 或后端网关时常用的兼容适配逻辑。

模块路径：

```text
github.com/wfu-work/proxy-api-lib
```

## 功能特性

- 支持 OpenAI Responses API 的非流式与流式调用。
- 支持通过 `base_url`、Provider 名称、headers、代理 URL、自定义 HTTP client 接入 OpenAI-compatible 上游。
- 内置 OpenAI、FreeModel、CodexZH、Aiok、Tokeni Provider 预设。
- 支持 API key 和 bearer token 凭证。
- 支持 function tool schema 和工具调用结果回传。
- 支持 SSE 流式读取、强类型流式事件和流聚合 helper。
- 支持 OpenAI 风格 API 错误映射。
- 支持 Chat Completions payload 转 Responses request。
- 支持 CLIProxyAPI 风格 Responses payload 适配。
- 支持最小 Codex 风格 `config.toml` / `auth.json` 加载。
- 提供 CodexZH、Aiok、Tokeni 的可选 usage client。

## 安装

```bash
go get github.com/wfu-work/proxy-api-lib
```

需要 Go 1.22 或更高版本。

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"os"

	proxyapi "github.com/wfu-work/proxy-api-lib"
	"github.com/wfu-work/proxy-api-lib/domains"
	"github.com/wfu-work/proxy-api-lib/openai"
)

func main() {
	client := proxyapi.NewClient(
		proxyapi.WithProvider(openai.New()),
		proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	resp, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model: "gpt-4.1",
		Input: domains.InputText("Say hello in one short sentence."),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.OutputText())
}
```

## Provider 预设

Provider 预设包是 OpenAI-compatible Responses 实现上的轻量封装。

| Package | Provider name | 默认 base URL |
| --- | --- | --- |
| `openai` | `openai` | `https://api.openai.com/v1` |
| `compat/freemodel` | `freemodel` | `https://api.freemodel.dev` |
| `compat/codexzh` | `codexzh` | `https://api.codexzh.com/v1` |
| `compat/aiok` | `aiok` | `https://aiok.club/v1` |
| `compat/tokeni` | `tokeni` | `https://api.tokeni.top` |

Tokeni 示例：

```go
client := proxyapi.NewClient(
	proxyapi.WithProvider(tokeni.New()),
	proxyapi.WithBearerToken(os.Getenv("TOKENI_API_KEY")),
)
```

导入：

```go
import "github.com/wfu-work/proxy-api-lib/compat/tokeni"
```

可以覆盖上游地址、代理 URL 或 HTTP client：

```go
provider := tokeni.New(
	tokeni.WithBaseURL("https://example.com/v1"),
	tokeni.WithProxyURL("http://127.0.0.1:7890"),
)
```

没有预设的 OpenAI-compatible Provider 可以直接使用 `compatible.OpenAIResponses`：

```go
provider := compatible.OpenAIResponses(compatible.Config{
	Name:    "custom",
	BaseURL: "https://example.com/v1",
	WireAPI: compatible.WireAPIResponses,
})
```

## 流式调用

```go
stream, err := client.Responses.Stream(ctx, domains.ResponseRequest{
	Model: "gpt-4.1",
	Input: domains.InputText("Explain Go interfaces step by step."),
})
if err != nil {
	return err
}
defer stream.Close()

for stream.Next() {
	event := stream.Event()
	fmt.Print(event.TextDelta())
}
if err := stream.Err(); err != nil {
	return err
}
```

聚合流式文本和工具调用参数：

```go
acc := domains.NewStreamAccumulator()
for stream.Next() {
	acc.Add(stream.Event())
}

fmt.Println(acc.OutputText())
for _, call := range acc.ToolCalls() {
	_ = call
}
```

## 工具调用

```go
resp, err := client.Responses.Create(ctx, domains.ResponseRequest{
	Model: "gpt-4.1",
	Input: domains.InputText("Check the weather in Shanghai."),
	Tools: []domains.Tool{
		domains.FunctionTool{
			Name:        "get_weather",
			Description: "Get weather by city name.",
			Parameters: domains.JSONSchema{
				Type: "object",
				Properties: map[string]domains.JSONSchema{
					"city": {Type: "string"},
				},
				Required: []string{"city"},
			},
		},
	},
})
if err != nil {
	return err
}

for _, call := range resp.ToolCalls() {
	result := runTool(call.Name, call.Arguments)
	resp, err = client.Responses.Create(ctx, domains.ResponseRequest{
		Model:              "gpt-4.1",
		PreviousResponseID: resp.ID,
		Input: []any{
			domains.FunctionCallOutput(call.CallID, result),
		},
	})
	if err != nil {
		return err
	}
}
```

## Usage API

部分第三方 Provider 提供余额或用量查询接口。本库为已有预设 Provider 提供了轻量 usage client。

CodexZH 使用专用 usage endpoint：

```go
stats, err := codexzh.NewUsageClient().Fetch(ctx, os.Getenv("CODEXZH_API_KEY"))
if err != nil {
	return err
}

fmt.Println(stats.TodayUsed, stats.WeekUsed)
```

Aiok 和 Tokeni 使用常规 OpenAI-compatible usage endpoint：

```go
stats, err := tokeni.NewUsageClient().Fetch(ctx, os.Getenv("TOKENI_API_KEY"))
if err != nil {
	return err
}

fmt.Println(stats.Balance)
```

默认 usage URL 生成规则：

- 如果 `base_url` 已经包含 `/v1` 路径段，则追加 `/usage`。
- 否则追加 `/v1/usage`。

示例：

```text
https://aiok.club/v1       -> https://aiok.club/v1/usage
https://api.tokeni.top     -> https://api.tokeni.top/v1/usage
https://example.com/api/v1 -> https://example.com/api/v1/usage
```

## Codex 配置加载

`compat/codex` 可以加载最小 Codex 风格 Provider 配置。库不会隐式读写用户文件，调用方需要显式传入路径。

```go
cfg, err := codex.Load("~/.codex/config.toml", "~/.codex/auth.json")
if err != nil {
	return err
}

client := proxyapi.NewClient(
	proxyapi.WithProvider(compatible.OpenAIResponses(cfg.Provider("freemodel"))),
	proxyapi.WithCredential(cfg.Credential()),
)
```

支持字段：

- `model`
- `model_provider`
- `model_reasoning_effort`
- `disable_response_storage`
- `preferred_auth_method`
- `[model_providers.<name>]`
- `base_url`
- `wire_api`
- `proxy_url`

## 兼容适配器

`compat` 目录提供可选适配器：

- `compat/chatcompletions`：将 Chat Completions JSON payload 转为统一 Responses request。
- `compat/cliproxyapi`：转换 CLIProxyAPI 风格 Responses payload。
- `compat/codex`：加载最小 Codex 风格配置和凭证。
- `compat/freemodel`、`compat/codexzh`、`compat/aiok`、`compat/tokeni`：Provider 预设。

这些适配器都是显式使用的，核心包保持 Provider-neutral。

## 错误处理

上游 API 错误会映射为 `domains.APIError`。

```go
var apiErr *domains.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Provider)
	fmt.Println(apiErr.StatusCode)
	fmt.Println(apiErr.Code)
	fmt.Println(apiErr.Message)
	fmt.Println(apiErr.RequestID)
}
```

## 示例

可运行示例位于：

- `examples/basic`
- `examples/freemodel`
- `examples/stream`

运行示例：

```bash
OPENAI_API_KEY=... OPENAI_MODEL=gpt-4.1 go run ./examples/basic
```

## 测试

运行单元测试：

```bash
go test ./...
```

真实上游集成测试默认关闭，需要显式开启：

```bash
PROXYAPI_INTEGRATION=1 go test ./...
```

## 项目结构

```text
.
├── auth/                 # Credential 实现
├── compat/               # 可选兼容适配器和 Provider 预设
├── compatible/           # OpenAI-compatible Responses Provider
├── domains/              # Provider-neutral 请求、响应、流、工具和错误类型
├── examples/             # 可运行示例
├── openai/               # OpenAI Provider 预设
├── provider/             # Provider registry
└── transport/            # HTTP proxy transport helper
```

## 设计目标

- 核心类型稳定，并保持 Provider-neutral。
- 不在核心公开 API 中暴露上游 SDK 类型。
- Provider-specific 行为放在 Provider 包或 `compat` 包中。
- 兼容行为显式启用，不做隐藏全局行为。
- 偏向小而可测试的 adapter，而不是大型框架。

## 当前状态

当前项目已经实现实用的 OpenAI-compatible Responses 调用链路：

- 非流式 Responses 调用。
- SSE 流式 Responses 调用。
- Function tool schema 和工具调用结果回传。
- API key 和 bearer token 鉴权。
- Provider 级代理配置。
- OpenAI、FreeModel、CodexZH、Aiok、Tokeni 预设。
- Codex、Chat Completions、CLIProxyAPI 兼容 helper。

API 仍可能随着下游代理服务集成继续演进。

## 贡献

欢迎提交 issue 和 pull request。涉及公开 API 形状的改动，请附带测试并说明兼容性影响。

提交 PR 前建议运行：

```bash
go fmt ./...
go test ./...
```

## License

本项目基于 MIT License 开源，详见 [LICENSE](LICENSE)。
