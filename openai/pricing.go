package openai

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wfu-work/proxy-api-lib/catalog"
)

const (
	// OfficialPricingURL 是 OpenAI 官方公开定价页面。
	OfficialPricingURL = "https://developers.openai.com/api/docs/pricing"
	// OfficialPricingMarkdownURL 是定价页面的 Markdown 版本，便于稳定解析表格。
	OfficialPricingMarkdownURL = OfficialPricingURL + ".md"
	// officialPricingSnapshotVersion 标识内置快照对应的官方文档日期。
	officialPricingSnapshotVersion = "2026-07-30"
	maxPricingDocumentBytes        = 8 << 20
)

//go:embed official_pricing_snapshot.md
var officialPricingSnapshot []byte

// OfficialPricingSource 从 OpenAI 官方定价文档读取模型参考价。
type OfficialPricingSource struct {
	httpClient *http.Client
}

// NewOfficialPricingSource 创建只访问 OpenAI 官方定价地址的数据源。
func NewOfficialPricingSource(httpClient *http.Client) *OfficialPricingSource {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OfficialPricingSource{httpClient: httpClient}
}

// Vendor 返回 OpenAI 官方厂商标识。
func (s *OfficialPricingSource) Vendor() string { return catalog.VendorOpenAI }

// Fetch 优先读取实时官方文档；网络或文档异常时返回内置官方快照并携带警告。
func (s *OfficialPricingSource) Fetch(ctx context.Context) (*catalog.PricingSnapshot, error) {
	if s == nil || s.httpClient == nil {
		return nil, errors.New("openai: official pricing source is nil")
	}
	fetchedAt := time.Now().UnixMilli()
	body, err := s.fetchDocument(ctx)
	if err == nil {
		prices, parseErr := ParseOfficialPricingMarkdown(body)
		if parseErr == nil {
			return &catalog.PricingSnapshot{
				Prices: prices, SourceURL: OfficialPricingURL, SourceKind: catalog.PricingSourceOfficialDocsLive,
				FetchedAt: fetchedAt,
			}, nil
		}
		err = parseErr
	}
	prices, snapshotErr := ParseOfficialPricingMarkdown(officialPricingSnapshot)
	if snapshotErr != nil {
		return nil, fmt.Errorf("openai: fetch official pricing: %v; parse bundled snapshot: %w", err, snapshotErr)
	}
	return &catalog.PricingSnapshot{
		Prices: prices, SourceURL: OfficialPricingURL, SourceKind: catalog.PricingSourceOfficialDocsSnapshot,
		SourceVersion: officialPricingSnapshotVersion, FetchedAt: fetchedAt,
		Warning: "实时读取 OpenAI 官方定价失败，已使用 " + officialPricingSnapshotVersion + " 官方快照: " + err.Error(),
	}, nil
}

// fetchDocument 下载官方定价 Markdown，并限制响应体大小。
func (s *OfficialPricingSource) fetchDocument(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OfficialPricingMarkdownURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/markdown, text/plain;q=0.9")
	req.Header.Set("User-Agent", "proxy-api-lib/official-pricing")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
		return nil, fmt.Errorf("official pricing returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPricingDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPricingDocumentBytes {
		return nil, errors.New("official pricing document is too large")
	}
	return body, nil
}

// ParseOfficialPricingMarkdown 解析官方旗舰模型定价表中的服务层级和上下文阶梯。
func ParseOfficialPricingMarkdown(document []byte) ([]catalog.ModelPrice, error) {
	lines := make([]string, 0, bytes.Count(document, []byte{'\n'})+1)
	scanner := bufio.NewScanner(bytes.NewReader(document))
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	prices := make([]catalog.ModelPrice, 0)
	serviceTier := ""
	for index := 0; index < len(lines); index++ {
		if tier := pricingTierFromHeading(lines[index]); tier != "" {
			serviceTier = tier
			continue
		}
		if serviceTier == "" || !isFlagshipPricingHeader(lines[index]) {
			continue
		}
		index++
		if index < len(lines) && isMarkdownSeparator(lines[index]) {
			index++
		}
		for ; index < len(lines) && strings.HasPrefix(lines[index], "|"); index++ {
			rowPrices, err := parseOfficialPricingRow(serviceTier, lines[index])
			if err != nil {
				return nil, err
			}
			prices = append(prices, rowPrices...)
		}
		index--
	}
	if len(prices) == 0 {
		return nil, errors.New("openai: official pricing table was not found")
	}
	return prices, nil
}

// pricingTierFromHeading 从官方表格标题提取标准化服务层级。
func pricingTierFromHeading(line string) string {
	lower := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "###")))
	for _, tier := range []string{
		catalog.PricingTierStandard, catalog.PricingTierBatch,
		catalog.PricingTierFlex, catalog.PricingTierPriority,
	} {
		if lower == tier+" pricing data" {
			return tier
		}
	}
	return ""
}

// isFlagshipPricingHeader 判断表格是否为包含短、长上下文价格的旗舰模型表。
func isFlagshipPricingHeader(line string) bool {
	cells := markdownCells(line)
	return len(cells) >= 5 && strings.EqualFold(cells[0], "Model") &&
		strings.EqualFold(cells[1], "Short context input")
}

// isMarkdownSeparator 判断当前行是否为 Markdown 表头分隔线。
func isMarkdownSeparator(line string) bool {
	cells := markdownCells(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if strings.Trim(cell, " :-") != "" {
			return false
		}
	}
	return true
}

// parseOfficialPricingRow 把一行官方价格拆成短上下文和可选长上下文两条记录。
func parseOfficialPricingRow(serviceTier, line string) ([]catalog.ModelPrice, error) {
	cells := markdownCells(line)
	if len(cells) < 5 {
		return nil, fmt.Errorf("openai: invalid pricing row %q", line)
	}
	modelID := normalizePricingModelID(cells[0])
	if modelID == "" {
		return nil, fmt.Errorf("openai: pricing row has no model id: %q", line)
	}
	values := make([]*int64, 8)
	for index := 1; index < len(cells) && index <= 8; index++ {
		value, err := parseUSDMicrousd(cells[index])
		if err != nil {
			return nil, fmt.Errorf("openai: parse %s %s pricing: %w", modelID, serviceTier, err)
		}
		values[index-1] = value
	}
	base := catalog.ModelPrice{
		VendorCode: catalog.VendorOpenAI, RemoteModelID: modelID, Scope: catalog.PricingScopeAPIReference,
		ServiceTier: serviceTier, Currency: catalog.CurrencyUSD, Unit: catalog.PricingUnitPerMillionTokens,
	}
	short := base
	short.ContextTier = catalog.PricingContextShort
	short.InputMicrousdPer1M, short.CachedInputMicrousdPer1M = values[0], values[1]
	short.CacheWriteMicrousdPer1M, short.OutputMicrousdPer1M = values[2], values[3]
	result := make([]catalog.ModelPrice, 0, 2)
	if short.HasPrice() {
		result = append(result, short)
	}
	long := base
	long.ContextTier = catalog.PricingContextLong
	long.InputMicrousdPer1M, long.CachedInputMicrousdPer1M = values[4], values[5]
	long.CacheWriteMicrousdPer1M, long.OutputMicrousdPer1M = values[6], values[7]
	if long.HasPrice() {
		result = append(result, long)
	}
	return result, nil
}

// markdownCells 把 Markdown 表格行切分为去除空白的单元格。
func markdownCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

// normalizePricingModelID 去除官方价格行中的上下文说明，仅保留可匹配目录的模型 ID。
func normalizePricingModelID(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "`*")
	if index := strings.Index(value, " ("); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

// parseUSDMicrousd 把美元小数字符串精确转换为微美元整数。
func parseUSDMicrousd(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return nil, nil
	}
	value = strings.ReplaceAll(strings.TrimPrefix(value, "$"), ",", "")
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid USD amount %q", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return nil, fmt.Errorf("invalid USD amount %q", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 6 {
		return nil, fmt.Errorf("USD amount has more than six decimals %q", value)
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid USD amount %q", value)
		}
	}
	result := whole*1_000_000 + fractionValue
	return &result, nil
}
