// Package catalog 定义官方账号模型目录的统一读取边界。
//
// 该包只描述官方远端模型，不包含数据库、账号池路由或第三方中转站配置。
package catalog

import (
	"context"
	"encoding/json"
)

const (
	// VendorOpenAI 表示 OpenAI 官方服务。
	VendorOpenAI = "openai"
	// VendorAnthropic 为未来的 Anthropic 官方账号集成预留稳定标识。
	VendorAnthropic = "anthropic"

	// ProductCodex 表示 OpenAI Codex/ChatGPT 订阅产品。
	ProductCodex = "codex"
	// ProductClaudeCode 为未来的 Claude Code 官方账号集成预留稳定标识。
	ProductClaudeCode = "claude_code"

	// ProtocolOpenAIResponses 表示上游使用 OpenAI Responses 协议。
	ProtocolOpenAIResponses = "openai_responses"
	// ProtocolAnthropicMessages 表示上游使用 Anthropic Messages 协议。
	ProtocolAnthropicMessages = "anthropic_messages"
)

// SourceIdentity 描述模型目录来源的官方厂商、产品和上游协议。
type SourceIdentity struct {
	Vendor   string `json:"vendor"`
	Product  string `json:"product"`
	Protocol string `json:"protocol"`
}

// RemoteModel 是不同官方产品返回模型信息的最小公共结构。
// Raw 保留官方原始数据，便于调用方在不升级接口的情况下读取新增能力字段。
type RemoteModel struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"displayName,omitempty"`
	Description  string          `json:"description,omitempty"`
	OwnedBy      string          `json:"ownedBy,omitempty"`
	Created      int64           `json:"created,omitempty"`
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

// ModelCapabilities 是官方模型目录在不同厂商之间共享的结构化能力。
//
// Capabilities 原文仍会保存在 RemoteModel 中；这里仅固定业务层需要稳定读取的公共字段。
type ModelCapabilities struct {
	ReasoningEfforts       []string `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
}

// DecodeModelCapabilities 从统一能力 JSON 中读取结构化能力。
// 无效或未知 JSON 会返回空能力，调用方仍可通过 RemoteModel.Raw 检查官方原始数据。
func DecodeModelCapabilities(raw json.RawMessage) ModelCapabilities {
	var capabilities ModelCapabilities
	if len(raw) == 0 {
		return capabilities
	}
	if err := json.Unmarshal(raw, &capabilities); err != nil {
		return ModelCapabilities{}
	}
	return capabilities
}

// Source 统一不同官方账号产品的模型目录读取行为。
// 实现不得要求调用方提供任意第三方 Base URL。
type Source interface {
	// Identity 返回稳定的官方来源标识。
	Identity() SourceIdentity
	// ListModels 获取当前官方账号实际可见的远端模型。
	ListModels(ctx context.Context) ([]RemoteModel, error)
}
