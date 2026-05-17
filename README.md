# proxy-api-lib

`proxy-api-lib` 是一个 Go 库，用来封装不同 AI Provider 的接口请求、流式响应、工具调用与兼容转换逻辑。项目第一阶段优先实现 OpenAI API，同时兼容 OpenAI-compatible / Codex-compatible 的第三方上游，例如 [FreeModel](https://freemodel.dev/dashboard/docs) 这类通过 API Key、`base_url` 与 Responses wire API 接入的服务。

本项目会参考两个开源项目的设计经验：

- [OpenAI Codex](https://github.com/openai/codex)：参考其请求链路、登录语义、上游兼容行为与源码组织方式。
- [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)：参考其 Responses 请求转换、工具调用约定、CLI 客户端兼容与代理行为。
- [FreeModel](https://freemodel.dev/dashboard/docs)：参考其 Codex 配置形态，支持第三方 API Key、`model_providers`、`base_url` 与 `wire_api = "responses"` 的接入方式。

参考不等于复制。本库目标是沉淀一个可嵌入 Go 应用的通用请求层，而不是复刻 CLI 或代理服务本身。

## 项目目标

- 提供统一的 Go API，屏蔽不同 Provider 的请求、鉴权、流式输出、错误格式与工具调用差异。
- 优先支持 OpenAI Responses API，并支持 OpenAI-compatible / Codex-compatible 第三方 Responses 上游。
- 支持工具调用约定的标准化，便于上层实现 agent、CLI、IDE 插件或服务端代理。
- 支持非流式与流式响应，流式接口优先按事件序列建模。
- 提供清晰的请求链路拦截点，方便记录日志、注入 headers、重试、限流与兼容转换。
- 将登录/鉴权语义抽象出来，既支持 API Key，也为 OAuth、会话 token、代理 token 预留扩展点。

## 非目标

- 不在第一阶段实现完整代理服务器。
- 不在第一阶段实现所有 Provider。
- 不把某个上游 SDK 的类型直接暴露为本库核心类型。
- 不在核心包里绑定具体数据库、Web 框架、CLI 框架或日志框架。

## 第一阶段范围

第一阶段以 OpenAI 官方 API 为主线，同时支持 OpenAI-compatible / Codex-compatible 的第三方 Responses 上游。

计划支持：

- `POST /v1/responses` 请求构造、发送与结果解析。
- 自定义 Provider 名称、Base URL 与 wire API，例如 `freemodel` + `https://api.freemodel.dev` + `responses`。
- 流式 Responses 事件解析。
- 文本输入、消息输入、工具定义、工具调用结果回传。
- 基础模型参数：`model`、`input`、`instructions`、`tools`、`tool_choice`、`temperature`、`max_output_tokens` 等。
- OpenAI 风格错误解析与统一错误包装。
- API Key / Bearer Token 鉴权：`Authorization: Bearer <token>`。
- 可插拔 HTTP Client、Base URL、默认 Header、超时与重试策略。
- 面向 CPA/Codex/FreeModel 兼容行为的请求转换层，但保持为可选模块。

暂不承诺：

- 文件上传、Batch、Fine-tuning、Realtime、Assistants、Vector Stores 等完整资源 API。
- 复杂 OAuth 登录流程。
- 完整的 Chat Completions 双向兼容。
- 所有 Responses event 类型的高级语义封装。

## 设计原则

1. **核心类型稳定**
   核心包只暴露项目自己的请求、响应、事件、工具调用、错误类型。上游 Provider 类型只能出现在 provider 内部或 adapter 包里。

2. **请求链路可组合**
   请求从上层 `Client` 进入后，依次经过规范化、Provider 适配、鉴权注入、HTTP 发送、响应解析、错误映射、事件聚合等阶段。每个阶段都应该可以测试。

3. **登录语义独立于 Provider**
   Provider 只消费 `Credential`，不直接知道凭证来自环境变量、配置文件、OAuth 登录还是代理服务。

4. **兼容行为显式化**
   Codex/CPA 兼容逻辑放在独立 adapter 中，通过选项开启，避免核心行为变得隐式。

5. **流式优先按事件处理**
   不把流式响应强行拼成字符串。上层可以选择直接消费事件，也可以使用 helper 聚合成最终文本。

## 建议包结构

```text
proxy-api-lib/
  go.mod
  README.md
  client.go
  domains/
    request.go
    response.go
    stream.go
    stream_events.go
    tool.go
    schema.go
    credential.go
    errors.go
  auth/
    credential.go
  provider/
    registry.go
  openai/
    client.go
  compatible/
    openai.go
    responses.go
    stream.go
  compat/
    chatcompletions/
      adapter.go
    codex/
      config.go
    cliproxyapi/
      adapter.go
    freemodel/
      adapter.go
  transport/
    proxy.go
  examples/
    basic/
    freemodel/
    stream/
```

## 请求链路规划

```text
App Code
  -> proxyapi.Client
  -> Request normalization
  -> Compat adapter, optional
  -> Provider adapter, OpenAI / OpenAI-compatible first
  -> Credential resolver
  -> HTTP transport middleware
  -> Upstream API
  -> Provider response parser
  -> Unified response / stream events
```

关键扩展点：

- `Provider`：定义模型请求、流式请求、能力声明与错误转换。
- `CredentialProvider`：负责返回当前请求可用的凭证。
- `TransportMiddleware`：负责 headers、日志、重试、限流、trace id 等横切逻辑。
- `CompatAdapter`：负责 Codex/CPA 风格请求到本库统一请求的转换。

## 登录与鉴权语义

第一阶段支持 API Key 与显式 Bearer Token。OpenAI 官方 API Key 请求本质上也是 Bearer Token header：

```go
client := proxyapi.NewClient(
    proxyapi.WithProvider(openai.New()),
    proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
)
```

也可以直接传入 token，适合 OAuth access token、Codex 登录 token、代理服务下发 token 或第三方兼容服务 token：

```go
client := proxyapi.NewClient(
    proxyapi.WithProvider(openai.New()),
    proxyapi.WithBearerToken(token),
)
```

最终 HTTP header：

```http
Authorization: Bearer <token>
```

建议的 credential 类型：

- `APIKeyCredential`：OpenAI / 第三方 API Key，按 Bearer header 发送。
- `BearerTokenCredential`：显式 bearer token，适合 OAuth、Codex、代理 token。
- `EnvCredential`：从环境变量读取。
- `RotatingCredential`：支持 token 轮换。
- `OAuthCredential`：支持 OAuth 登录结果注入。
- `ProxyCredential`：支持代理服务下发或转发 token。

同一个请求只应选择一种主鉴权方式。默认优先级建议：

1. Request 级 credential。
2. Provider 级 credential。
3. Client 级 credential。
4. 环境变量或显式配置文件 loader。

鉴权模块只负责提供凭证，不负责决定业务是否允许调用模型。权限、配额、租户隔离等应由调用方或代理层处理。

## 第三方 API Key 兼容

很多第三方服务并不要求新的协议，而是提供 OpenAI-compatible endpoint，再通过 Codex 风格配置声明 Provider、模型、Base URL 与 wire API。FreeModel 就属于这类接入形态。

典型 `~/.codex/auth.json`：

```json
{
  "OPENAI_API_KEY": "YOUR_API_KEY"
}
```

典型 `~/.codex/config.toml`：

```toml
model_provider = "freemodel"
model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
preferred_auth_method = "apikey"

[model_providers.freemodel]
name = "freemodel"
base_url = "https://api.freemodel.dev"
wire_api = "responses"
proxy_url = "http://127.0.0.1:7890"
```

本库需要兼容这种语义，但不应该默认读写用户的 `~/.codex` 文件。建议提供显式 loader：

```go
cfg, err := codexconfig.Load("~/.codex/config.toml", "~/.codex/auth.json")
if err != nil {
    return err
}

client := proxyapi.NewClient(
    proxyapi.WithProvider(compatible.OpenAIResponses(cfg.Provider("freemodel"))),
    proxyapi.WithAPIKey(cfg.OpenAIAPIKey()),
)
```

兼容要求：

- 支持 `model_provider` 选择当前 Provider。
- 支持 `[model_providers.<name>]` 多 Provider 配置。
- 支持 `base_url` 覆盖上游地址。
- 支持 `wire_api = "responses"`，明确使用 Responses 请求链路。
- 支持 `preferred_auth_method = "apikey"` 或 `"token"`，从 auth 配置或调用方注入 API Key / Bearer Token。
- 支持 `model_reasoning_effort` 映射到 Responses 请求中的 reasoning 配置。
- 支持 `disable_response_storage` 映射到 OpenAI Responses 的存储开关语义。

这类 Provider 在内部可以复用 OpenAI Responses adapter，不需要为每个第三方服务复制一套完整实现。

## 网络代理

OpenAI 官方 API 在部分网络环境下可能需要通过本地 VPN/代理访问，而 FreeModel 等第三方上游可能可以直连。代理配置应该支持 Provider 级别覆盖。

Codex 风格配置示例：

```toml
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"
wire_api = "responses"
proxy_url = "http://127.0.0.1:7890"

[model_providers.freemodel]
name = "freemodel"
base_url = "https://api.freemodel.dev"
wire_api = "responses"
```

Go 代码示例：

```go
client := proxyapi.NewClient(
    proxyapi.WithProvider(openai.New(
        openai.WithProxyURL("http://127.0.0.1:7890"),
    )),
    proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
)
```

建议优先级：

1. Provider 级 `proxy_url`。
2. Provider 级自定义 `http.Client`。
3. Go 标准库环境变量：`HTTP_PROXY`、`HTTPS_PROXY`、`NO_PROXY`。
4. 直连。

`proxy_url` 应支持 `http://`、`https://` 与 `socks5://` 形式。

## Responses API 规划

基础调用示例：

```go
package main

import (
    "context"
    "fmt"
    "os"

    proxyapi "github.com/free-model/proxy-api-lib"
    "github.com/free-model/proxy-api-lib/domains"
    "github.com/free-model/proxy-api-lib/openai"
)

func main() {
    client := proxyapi.NewClient(
        proxyapi.WithProvider(openai.New()),
        proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    )

    resp, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
        Model: "gpt-4.1",
        Input: domains.InputText("写一个 Go HTTP client 示例"),
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.OutputText())
}
```

流式调用示例：

```go
stream, err := client.Responses.Stream(ctx, domains.ResponseRequest{
    Model: "gpt-4.1",
    Input: domains.InputText("逐步解释 Go interface 的用途"),
})
if err != nil {
    return err
}
defer stream.Close()

acc := domains.NewStreamAccumulator()
for stream.Next() {
    event := stream.Event()
    acc.Add(event)
    fmt.Print(event.TextDelta())
}
if err := stream.Err(); err != nil {
    return err
}

for _, call := range acc.ToolCalls() {
    // handle streamed tool calls
    _ = call
}
```

工具调用示例：

```go
resp, err := client.Responses.Create(ctx, domains.ResponseRequest{
    Model: "gpt-4.1",
    Input: domains.InputText("查询上海今天的天气"),
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

以上 API 名称是规划稿，实际实现时会根据 Go 可用性与测试反馈调整。

示例中的模型名只用于说明 API 形态，实际默认模型应由调用方配置，或在实现时参考 OpenAI 当前官方模型文档。

## 兼容策略

### Codex 兼容

关注点：

- 请求链路组织。
- 登录状态、凭证来源与上游转发边界。
- Responses API 事件与工具调用的处理方式。
- 针对上游 API 的兼容性保护。

规划：

- `compat/codex` 只做请求/响应语义适配。
- 不绑定 Codex CLI 的交互 UI。
- 不假设调用方一定是命令行工具。
- `compat/chatcompletions` 提供 Chat Completions payload 到 Responses request 的兼容转换。

### CLIProxyAPI 兼容

关注点：

- Responses 请求转换。
- 工具调用约定。
- CLI 客户端行为兼容。
- 代理转发时的 header、stream 与错误行为。

规划：

- `compat/cliproxyapi` 提供显式 adapter。
- 对转换前后的请求结构做 golden tests。
- 将代理服务相关能力保留为上层应用职责，本库只提供请求转换与 HTTP 调用能力。

### FreeModel / 第三方 OpenAI-compatible 兼容

关注点：

- 使用第三方 `base_url`，但请求体仍走 Responses API。
- 使用 API Key 或 Bearer Token 鉴权，header 仍保持 OpenAI-compatible 的 bearer token 形式。
- 支持 Codex 风格 `model_providers` 配置。
- 支持第三方模型名，不在核心层强校验模型名称。

规划：

- 将 FreeModel 视为 OpenAI-compatible Responses Provider。
- `compat/freemodel` 只提供预设与配置映射，例如默认 `base_url`、`wire_api` 与 provider name。
- 具体 HTTP 请求、流式解析、工具调用仍复用 `compatible` 或 `openai` 的 Responses 实现。
- 对第三方返回的未知字段与未知事件保持宽容解析，避免兼容上游轻微差异时直接失败。

## 错误模型

统一错误类型建议：

```go
type APIError struct {
    Provider   string
    StatusCode int
    Code       string
    Type       string
    Message    string
    RequestID  string
    Cause      error
}
```

错误处理目标：

- 保留上游原始错误信息。
- 暴露统一的 `StatusCode`、`Code`、`Type`、`Message`。
- 支持 `errors.Is` / `errors.As`。
- 支持读取 request id，方便排查上游问题。

## 测试计划

- Unit tests：请求转换、工具 schema、错误解析、credential resolver。
- Golden tests：OpenAI 官方请求 JSON、FreeModel 风格请求 JSON、CPA/Codex 兼容转换结果。
- Stream tests：SSE 分帧、事件顺序、异常中断、未知事件容错。
- Transport tests：headers、超时、重试、base URL、request id。
- Integration tests：用 mock server 验证完整请求链路，真实上游测试默认跳过。

## 里程碑

### M0: 项目骨架

- 初始化 Go module。
- 建立核心类型、Provider 接口、Client options。
- 建立 OpenAI provider 包。
- 建立 OpenAI-compatible Responses provider 基础包。
- 建立 mock server 测试工具。

### M1: OpenAI Responses 非流式

- 实现 `Responses.Create`。
- 支持 API Key、Base URL、HTTP Client。
- 支持第三方 Provider 名称与 `base_url` 配置。
- 支持基础请求字段与统一错误模型。
- 加入 golden tests。

### M2: OpenAI Responses 流式

- 实现 SSE reader。
- 暴露统一 `StreamEvent`。
- 支持文本增量、工具调用增量、完成与错误事件。
- 加入中断与未知事件测试。

### M3: 工具调用

- 定义统一 `Tool` / `ToolCall` 类型。
- 支持 function tool schema。
- 支持工具结果回传请求。
- 补齐工具调用 golden tests。

### M4: 兼容 adapter

- 增加 `compat/codex`。
- 增加 `compat/cliproxyapi`。
- 增加 `compat/freemodel` 或 OpenAI-compatible provider preset。
- 支持显式加载 Codex 风格 `config.toml` / `auth.json`。
- 将兼容行为做成显式选项。
- 编写转换测试与边界文档。

### M5: Provider 扩展准备

- 完成 provider registry。
- 提炼能力声明。
- 评估 Anthropic、Gemini、OpenAI-compatible endpoint 的接入方式。

## 开发约定

- 核心包不直接依赖具体 Provider 的 SDK。
- JSON 请求使用结构体编码，避免手写字符串拼接。
- 新字段优先加测试，再开放公共 API。
- 兼容逻辑必须有独立测试，不和默认 OpenAI 行为混在一起。
- 默认行为应贴近上游官方 API，兼容行为通过选项启用。

## 状态

当前状态：已完成 M0-M3 的最小可用实现。

已实现：

- Go module 初始化。
- 核心 `Client`、`Provider`、`Credential`、`ResponsesService`。
- API Key 与 Bearer Token 鉴权。
- OpenAI-compatible Responses 非流式请求。
- OpenAI-compatible Responses SSE 流式请求。
- 流式事件原样透出，支持 `StreamEvent.TextDelta()` 提取文本增量。
- Responses stream event 强类型 helper。
- `StreamAccumulator` 聚合文本增量、流式 function call arguments 与完成响应。
- function tool schema 请求。
- `Response.ToolCalls()` 解析模型返回的 function call。
- `FunctionCallOutput()` 构造工具结果回传 input item。
- OpenAI 官方 provider preset。
- FreeModel provider preset。
- Codex 风格 `config.toml` / `auth.json` 最小 loader。
- CLIProxyAPI 风格 Responses payload 转换。
- Chat Completions 风格 payload 到 Responses request 转换。
- Provider 级 `proxy_url` / `WithProxyURL`。
- 统一 `APIError`。
- mock server 单元测试。
- `examples/basic`、`examples/freemodel`、`examples/stream` 示例。
- `PROXYAPI_INTEGRATION=1` 真实上游集成测试开关。

下一步建议：

1. 扩展 Codex/CLIProxyAPI 兼容 adapter 的 golden tests。
2. 补齐更多 Provider 能力声明与 registry 使用示例。
3. 根据真实上游测试结果调整 FreeModel/Codex 兼容边界。
4. 评估 Anthropic、Gemini 等非 OpenAI-compatible Provider 的统一类型映射。

## 参考资料

- [OpenAI Responses API Reference](https://platform.openai.com/docs/api-reference/responses/create)
- [OpenAI Codex](https://github.com/openai/codex)
- [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)
- [FreeModel Docs](https://freemodel.dev/dashboard/docs)
