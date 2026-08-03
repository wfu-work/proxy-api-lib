// Package openai 导出 proxyapi 使用的 OpenAI 协议类型。
package openai

import "github.com/wfu-work/proxy-api-lib/domains"

// Credential 是请求鉴权凭据接口。
type Credential = domains.Credential

// ResponseRequest 是 OpenAI Responses API 请求。
type ResponseRequest = domains.ResponseRequest

// Reasoning 是模型推理配置。
type Reasoning = domains.Reasoning

// Response 是 OpenAI Responses API 响应。
type Response = domains.Response

// ResponseItem 是响应中的单个输出项。
type ResponseItem = domains.ResponseItem

// ResponseContent 是输出项中的一段内容。
type ResponseContent = domains.ResponseContent

// Usage 是请求的 Token 用量。
type Usage = domains.Usage

// InputTokenDetails 是输入 Token 的缓存命中明细。
type InputTokenDetails = domains.InputTokenDetails

// OutputTokenDetails 是输出 Token 的推理用量明细。
type OutputTokenDetails = domains.OutputTokenDetails

// JSONSchema 是函数工具参数的 JSON Schema。
type JSONSchema = domains.JSONSchema

// Tool 是 OpenAI 工具定义接口。
type Tool = domains.Tool

// FunctionTool 是函数工具定义。
type FunctionTool = domains.FunctionTool

// RawTool 是支持字段透传的原始工具定义。
type RawTool = domains.RawTool

// ToolCall 是模型发起的函数调用。
type ToolCall = domains.ToolCall

// ResponseStream 是 Responses API 流式迭代器。
type ResponseStream = domains.ResponseStream

// StreamEvent 是 Responses API 流事件。
type StreamEvent = domains.StreamEvent

// OutputTextDeltaEvent 是文本输出增量事件。
type OutputTextDeltaEvent = domains.OutputTextDeltaEvent

// FunctionCallArgumentsDeltaEvent 是函数调用参数增量事件。
type FunctionCallArgumentsDeltaEvent = domains.FunctionCallArgumentsDeltaEvent

// OutputItemAddedEvent 是输出项新增事件。
type OutputItemAddedEvent = domains.OutputItemAddedEvent

// StreamAccumulator 是流事件聚合器。
type StreamAccumulator = domains.StreamAccumulator

// APIError 是 OpenAI API 错误。
type APIError = domains.APIError

const (
	EventResponseCompleted              = domains.EventResponseCompleted
	EventResponseFailed                 = domains.EventResponseFailed
	EventResponseOutputItemAdded        = domains.EventResponseOutputItemAdded
	EventResponseOutputTextDelta        = domains.EventResponseOutputTextDelta
	EventResponseFunctionArgumentsDelta = domains.EventResponseFunctionArgumentsDelta
	EventResponseFunctionArgumentsDone  = domains.EventResponseFunctionArgumentsDone
)

// InputText 构造简单文本输入。
var InputText = domains.InputText

// FunctionCallOutput 构造函数执行结果输入项。
var FunctionCallOutput = domains.FunctionCallOutput

// NewStreamAccumulator 创建流事件聚合器。
var NewStreamAccumulator = domains.NewStreamAccumulator

// NewResponseStream 根据底层回调创建流式迭代器。
var NewResponseStream = domains.NewResponseStream
