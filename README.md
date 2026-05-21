# proxy-api-lib

[English](README.md) | [简体中文](README.zh-CN.md)

`proxy-api-lib` is a small Go library for calling OpenAI-compatible AI providers through one normalized API. It focuses on the Responses API, streaming events, tool calls, OpenAI-compatible provider presets, and compatibility adapters that are useful when building local proxy services, CLI tools, agents, or backend gateways.

The module path is:

```text
github.com/wfu-work/proxy-api-lib
```

## Features

- OpenAI Responses API wrapper with non-streaming and streaming calls.
- OpenAI-compatible provider support through configurable `base_url`, provider name, headers, proxy URL, and HTTP client.
- Built-in provider presets for OpenAI, FreeModel, CodexZH, Aiok, and Tokeni.
- API key and bearer token credentials.
- Function tool schema support and function call output helpers.
- SSE stream reader with typed stream events and accumulator helpers.
- OpenAI-style API error mapping.
- Chat Completions payload to Responses request adapter.
- CLIProxyAPI-style Responses payload adapter.
- Minimal Codex-style `config.toml` / `auth.json` loader.
- Optional usage clients for CodexZH, Aiok, and Tokeni.

## Installation

```bash
go get github.com/wfu-work/proxy-api-lib
```

Requires Go 1.22 or newer.

## Quick Start

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

## Provider Presets

The provider preset packages are thin wrappers around the OpenAI-compatible Responses implementation.

| Package | Provider name | Default base URL |
| --- | --- | --- |
| `openai` | `openai` | `https://api.openai.com/v1` |
| `compat/freemodel` | `freemodel` | `https://api.freemodel.dev` |
| `compat/codexzh` | `codexzh` | `https://api.codexzh.com/v1` |
| `compat/aiok` | `aiok` | `https://aiok.club/v1` |
| `compat/tokeni` | `tokeni` | `https://api.tokeni.top` |

Example with Tokeni:

```go
client := proxyapi.NewClient(
	proxyapi.WithProvider(tokeni.New()),
	proxyapi.WithBearerToken(os.Getenv("TOKENI_API_KEY")),
)
```

Import:

```go
import "github.com/wfu-work/proxy-api-lib/compat/tokeni"
```

You can override the upstream endpoint, proxy URL, or HTTP client:

```go
provider := tokeni.New(
	tokeni.WithBaseURL("https://example.com/v1"),
	tokeni.WithProxyURL("http://127.0.0.1:7890"),
)
```

For any OpenAI-compatible provider without a preset:

```go
provider := compatible.OpenAIResponses(compatible.Config{
	Name:    "custom",
	BaseURL: "https://example.com/v1",
	WireAPI: compatible.WireAPIResponses,
})
```

## Streaming

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

To aggregate streamed text and tool call arguments:

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

## Tool Calls

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

## Usage APIs

Some third-party providers expose balance or usage endpoints. This library includes small usage clients for the providers already used by the presets.

CodexZH has a dedicated usage endpoint:

```go
stats, err := codexzh.NewUsageClient().Fetch(ctx, os.Getenv("CODEXZH_API_KEY"))
if err != nil {
	return err
}

fmt.Println(stats.TodayUsed, stats.WeekUsed)
```

Aiok and Tokeni use the conventional OpenAI-compatible usage endpoint:

```go
stats, err := tokeni.NewUsageClient().Fetch(ctx, os.Getenv("TOKENI_API_KEY"))
if err != nil {
	return err
}

fmt.Println(stats.Balance)
```

Default usage URL generation follows this rule:

- If `base_url` already contains a `/v1` path segment, append `/usage`.
- Otherwise append `/v1/usage`.

Examples:

```text
https://aiok.club/v1       -> https://aiok.club/v1/usage
https://api.tokeni.top     -> https://api.tokeni.top/v1/usage
https://example.com/api/v1 -> https://example.com/api/v1/usage
```

## Codex Config Loader

`compat/codex` can load a minimal Codex-style provider configuration without reading or writing user files implicitly.

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

Supported fields include:

- `model`
- `model_provider`
- `model_reasoning_effort`
- `disable_response_storage`
- `preferred_auth_method`
- `[model_providers.<name>]`
- `base_url`
- `wire_api`
- `proxy_url`

## Compatibility Adapters

The `compat` directory contains optional adapters for common proxy and CLI payload shapes:

- `compat/chatcompletions`: converts Chat Completions JSON payloads into normalized Responses requests.
- `compat/cliproxyapi`: converts CLIProxyAPI-style Responses payloads.
- `compat/codex`: loads minimal Codex-style config and credential data.
- `compat/freemodel`, `compat/codexzh`, `compat/aiok`, `compat/tokeni`: provider presets.

These adapters are explicit and opt-in. The core package stays provider-neutral.

## Error Handling

Upstream API failures are mapped to `domains.APIError`.

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

## Examples

Runnable examples live in:

- `examples/basic`
- `examples/freemodel`
- `examples/stream`

Run one with:

```bash
OPENAI_API_KEY=... OPENAI_MODEL=gpt-4.1 go run ./examples/basic
```

## Testing

Run the unit test suite:

```bash
go test ./...
```

Integration tests that hit real upstream providers are disabled by default. Enable them explicitly:

```bash
PROXYAPI_INTEGRATION=1 go test ./...
```

## Project Layout

```text
.
├── auth/                 # Credential implementations
├── compat/               # Optional compatibility adapters and provider presets
├── compatible/           # OpenAI-compatible Responses provider
├── domains/              # Provider-neutral request, response, stream, tool, and error types
├── examples/             # Runnable examples
├── openai/               # OpenAI provider preset
├── provider/             # Provider registry
└── transport/            # HTTP proxy transport helpers
```

## Design Goals

- Keep core types stable and provider-neutral.
- Avoid exposing upstream SDK types in public core APIs.
- Keep provider-specific behavior in provider packages or `compat` packages.
- Make compatibility behavior explicit instead of hidden global behavior.
- Prefer small, testable adapters over a large framework.

## Status

This project currently implements the practical OpenAI-compatible Responses path:

- Non-streaming Responses calls.
- SSE streaming Responses calls.
- Function tool schemas and function call output helpers.
- API key and bearer token authentication.
- Provider-level proxy configuration.
- OpenAI, FreeModel, CodexZH, Aiok, and Tokeni presets.
- Codex, Chat Completions, and CLIProxyAPI compatibility helpers.

APIs may still evolve while the library is being integrated with downstream proxy services.

## Contributing

Issues and pull requests are welcome. For changes that affect public API shape, include tests and a short explanation of the compatibility impact.

Before opening a PR:

```bash
go fmt ./...
go test ./...
```

## License

This project is open source based on the MIT License. See [LICENSE](LICENSE) for details.
