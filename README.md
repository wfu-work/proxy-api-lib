# proxy-api-lib

[English](README.md) | [简体中文](README.zh-CN.md)

`proxy-api-lib` is a Go client and protocol toolkit that integrates only official OpenAI services. It contains no intermediary-specific logic for Aiok, FreeModel, Tokeni, CodexZH, or similar third-party relays.

The project keeps two boundaries explicit:

- `proxyapi` and `openai`: the public, stable OpenAI Platform API at `https://api.openai.com/v1`.
- `codexauth` and `chatgpt`: token parsing, token refresh, usage limits, and subscription queries for official ChatGPT/Codex login accounts.

OAuth account-pool applications such as FreeAI use only the second boundary for upstream access. Platform API keys accepted by `proxyapi` are a separate library capability and are not a fallback credential for `chatgpt`.

The `chatgpt` package calls services on `auth.openai.com` and `chatgpt.com`, but the account usage and subscription endpoints are not part of the public, stable OpenAI Platform API. They can change with the ChatGPT/Codex protocol. Application code should use this package instead of depending directly on the internal JSON responses.

Module path:

```text
github.com/wfu-work/proxy-api-lib
```

## What it owns

- Official OpenAI Responses API, including SSE streaming and tool calls.
- Official Embeddings and Models resources.
- OpenAI API key and caller-managed bearer-token credentials.
- ChatGPT/Codex JWT display-field parsing and OAuth refresh-token exchange.
- Canonical OAuth account-file parsing and normalized export.
- ChatGPT/Codex wham limit windows, earned rate-limit reset credits, account lists, merged `accounts/check` + `subscriptions` data, and subscription expiry/renewal queries.
- Official Codex model discovery and Responses calls with `x-codex-*` response-header preservation.
- Codex request normalization and SSE aggregation for callers that request a non-streaming response.
- OpenAI Responses and Chat Completions request/response codecs for gateways.
- OpenAI-style API errors, organization/project headers, custom HTTP clients, and network proxy configuration.

## What applications own

- Initial OAuth token acquisition outside this library, or import of an existing canonical account file.
- Encrypted storage for access, refresh, and ID tokens.
- Refresh scheduling, atomic persistence of rotated tokens, and retry after a 401.
- Account pools, routing, failover, and quota-snapshot persistence.
- External API authentication and authorization.
- Request logging and local usage accounting.

## ChatGPT/Codex account information

Given existing ChatGPT/Codex OAuth tokens, parse account display fields and refresh the access token with:

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

An existing account export can instead be normalized with `codexauth.ParseAccountFile`; applications should encrypt the complete normalized file at rest.

`ParseUnverifiedClaims` only decodes the JWT. It does not verify the signature, issuer, or audience, so its result is suitable for display and account routing, not authorization decisions.

Use the access token to query rate-limit windows and subscription information:

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

`usage.RateLimit` contains the used percentage, window duration, and reset time. It commonly represents 5-hour and 7-day windows and preserves additional Code Review, Spark, and future limit buckets. It is not an exact token count. `Subscription` queries both account sources when available and fills missing plan, subscription state, expiry, renewal time, and `WillRenew` fields without inventing values.

Earned rate-limit reset credits are also handled by this protocol boundary. The library owns endpoint compatibility, response parsing, normalized outcomes, and usage-summary fallback:

```go
credits, err := accountClient.Resets.List(ctx, accountID)
if err != nil {
	return err
}

// Reuse the same UUID when retrying one logical redemption attempt.
result, err := accountClient.Resets.Consume(ctx, accountID, redemptionUUID, creditID)
if err != nil {
	return err
}
if result.Outcome.IsIdempotentSuccess() {
	usage, err = accountClient.Usage.Get(ctx, accountID)
}
```

Applications remain responsible for persisting the idempotency key and refreshing their local quota snapshot after `reset` or `alreadyRedeemed`.

Applications that already implement secure refresh and storage can use `chatgpt.WithTokenSource` to read the latest access token before each request.

> OpenAI Platform organization usage and cost reporting is a separate public API authenticated with an organization Admin Key. It is not the same account or billing model as Codex limits included with a ChatGPT plan.

## Installation

```bash
go get github.com/wfu-work/proxy-api-lib
```

Requires Go 1.26 or newer.

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
		Input: openai.InputText("Say hello in one short sentence."),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.OutputText())
}
```

For an access token managed by the application:

```go
client := proxyapi.NewClient(
	proxyapi.WithBearerToken(accessToken),
)
```

A dynamic token source can implement `auth.TokenSource` and be installed with `auth.FromTokenSource`.

## Streaming

```go
stream, err := client.Responses.Stream(ctx, openai.ResponseRequest{
	Model: "gpt-4.1",
	Input: openai.InputText("Count from one to three."),
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

## Gateway codecs

Incoming OpenAI Responses JSON can be decoded with:

```go
request, err := responses.Decode(body)
```

Incoming Chat Completions JSON can be converted to a Responses request, and a Responses result can be converted back to Chat Completions:

```go
request, err := chatcompletions.Decode(body)
payload := chatcompletions.Response(model, response)
```

The codecs preserve unknown request fields in `ResponseRequest.Extra` so the gateway can remain forward-compatible.

## Errors

```go
var apiErr *openai.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode)
	fmt.Println(apiErr.Code)
	fmt.Println(apiErr.Message)
	fmt.Println(apiErr.RequestID)
}
```

The Platform client's `WithBaseURL` exists for tests and controlled official OpenAI endpoint deployments. The ChatGPT account client has its own `chatgpt.WithBaseURL`; the two configurations remain isolated. Neither is a provider registry or an intermediary preset mechanism.

## Verification

```bash
go test ./...
```
