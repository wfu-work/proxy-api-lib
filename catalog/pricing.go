package catalog

import "context"

const (
	// PricingScopeAPIReference 表示价格仅用于估算 OpenAI Platform API 成本。
	PricingScopeAPIReference = "api_reference"

	// PricingTierStandard 表示标准实时处理价格。
	PricingTierStandard = "standard"
	// PricingTierBatch 表示 Batch API 价格。
	PricingTierBatch = "batch"
	// PricingTierFlex 表示 Flex 处理价格。
	PricingTierFlex = "flex"
	// PricingTierPriority 表示 Priority 处理价格。
	PricingTierPriority = "priority"

	// PricingContextShort 表示短上下文价格区间。
	PricingContextShort = "short"
	// PricingContextLong 表示长上下文价格区间。
	PricingContextLong = "long"

	// PricingUnitPerMillionTokens 表示价格单位为每一百万 Token。
	PricingUnitPerMillionTokens = "per_1m_tokens"
	// CurrencyUSD 表示美元币种。
	CurrencyUSD = "USD"

	// PricingSourceOfficialDocsLive 表示数据来自本次实时读取的官方文档。
	PricingSourceOfficialDocsLive = "official_docs_live"
	// PricingSourceOfficialDocsSnapshot 表示实时读取失败后使用了内置官方快照。
	PricingSourceOfficialDocsSnapshot = "official_docs_snapshot"
)

// ModelPrice 描述一个模型在指定服务层级和上下文区间的官方 API 参考价。
// 金额使用微美元整数保存，避免浮点数造成货币精度损失。
type ModelPrice struct {
	VendorCode               string `json:"vendorCode"`
	RemoteModelID            string `json:"remoteModelId"`
	Scope                    string `json:"scope"`
	ServiceTier              string `json:"serviceTier"`
	ContextTier              string `json:"contextTier"`
	Currency                 string `json:"currency"`
	Unit                     string `json:"unit"`
	InputMicrousdPer1M       *int64 `json:"inputMicrousdPer1M,omitempty"`
	CachedInputMicrousdPer1M *int64 `json:"cachedInputMicrousdPer1M,omitempty"`
	CacheWriteMicrousdPer1M  *int64 `json:"cacheWriteMicrousdPer1M,omitempty"`
	OutputMicrousdPer1M      *int64 `json:"outputMicrousdPer1M,omitempty"`
}

// HasPrice 判断当前价格行是否至少包含一个有效金额。
func (p ModelPrice) HasPrice() bool {
	return p.InputMicrousdPer1M != nil || p.CachedInputMicrousdPer1M != nil ||
		p.CacheWriteMicrousdPer1M != nil || p.OutputMicrousdPer1M != nil
}

// PricingSnapshot 保存一次官方定价同步的完整结果和数据来源。
type PricingSnapshot struct {
	Prices        []ModelPrice `json:"prices"`
	SourceURL     string       `json:"sourceUrl"`
	SourceKind    string       `json:"sourceKind"`
	SourceVersion string       `json:"sourceVersion,omitempty"`
	FetchedAt     int64        `json:"fetchedAt"`
	Warning       string       `json:"warning,omitempty"`
}

// PricingSource 统一不同官方厂商的公开定价读取行为。
type PricingSource interface {
	// Vendor 返回稳定的官方厂商标识。
	Vendor() string
	// Fetch 获取官方参考定价；实现可以在实时来源不可用时返回带警告的可信快照。
	Fetch(ctx context.Context) (*PricingSnapshot, error)
}
