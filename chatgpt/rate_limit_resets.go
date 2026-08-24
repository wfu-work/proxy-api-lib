package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	rateLimitResetCreditsPath        = "/wham/rate-limit-reset-credits"
	rateLimitResetCreditsConsumePath = "/wham/rate-limit-reset-credits/consume"
	rateLimitResetCreditsLegacyPath  = "/api/codex/rate-limit-reset-credits"
	rateLimitResetConsumeLegacyPath  = "/api/codex/rate-limit-reset-credits/consume"
)

// RateLimitResetOutcome 是官方额度重置券兑换结果。
type RateLimitResetOutcome string

const (
	RateLimitResetOutcomeReset           RateLimitResetOutcome = "reset"
	RateLimitResetOutcomeAlreadyRedeemed RateLimitResetOutcome = "alreadyRedeemed"
	RateLimitResetOutcomeNothingToReset  RateLimitResetOutcome = "nothingToReset"
	RateLimitResetOutcomeNoCredit        RateLimitResetOutcome = "noCredit"
)

// RateLimitResetService 提供 ChatGPT/Codex 官方额度重置券查询和兑换能力。
// 这些端点属于 ChatGPT 账号协议，不是 OpenAI Platform 计费 API。
type RateLimitResetService struct{ client *Client }

// RateLimitResetCredits 是账号当前持有的额度重置券快照。
// ApplicableAvailableCount 可能缺失；nil 表示上游只返回了总数。
type RateLimitResetCredits struct {
	AvailableCount           int                     `json:"available_count"`
	ApplicableAvailableCount *int                    `json:"applicable_available_count,omitempty"`
	Credits                  []*RateLimitResetCredit `json:"credits,omitempty"`
	Raw                      json.RawMessage         `json:"-"`
	RequestID                string                  `json:"-"`
}

// RateLimitResetCredit 描述一张可用于重置 Codex 限额窗口的官方重置券。
type RateLimitResetCredit struct {
	ID          string `json:"id"`
	ResetType   string `json:"reset_type"`
	Status      string `json:"status"`
	GrantedAt   *int64 `json:"granted_at,omitempty"`
	ExpiresAt   *int64 `json:"expires_at,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// UnmarshalJSON 兼容官方接口在不同版本中返回的 Unix 数字时间戳和字符串时间。
// granted_at / expires_at 只是展示元数据；无法识别时按缺失处理，不能阻断券的查询和兑换。
func (c *RateLimitResetCredit) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.New("chatgpt: reset credit is nil")
	}
	var wire struct {
		ID          string          `json:"id"`
		ResetType   string          `json:"reset_type"`
		Status      string          `json:"status"`
		GrantedAt   json.RawMessage `json:"granted_at"`
		ExpiresAt   json.RawMessage `json:"expires_at"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	grantedAt, _ := parseResetCreditWireTimestamp(wire.GrantedAt)
	expiresAt, _ := parseResetCreditWireTimestamp(wire.ExpiresAt)
	*c = RateLimitResetCredit{
		ID: wire.ID, ResetType: wire.ResetType, Status: wire.Status,
		GrantedAt: grantedAt, ExpiresAt: expiresAt,
		Title: wire.Title, Description: wire.Description,
	}
	return nil
}

// ConsumeRateLimitResetCreditResult 是一次幂等兑换的归一化结果。
type ConsumeRateLimitResetCreditResult struct {
	Outcome         RateLimitResetOutcome `json:"outcome"`
	CreditID        string                `json:"credit_id,omitempty"`
	RedeemRequestID string                `json:"redeem_request_id,omitempty"`
	WindowsReset    json.RawMessage       `json:"windows_reset,omitempty"`
	Raw             json.RawMessage       `json:"-"`
	RequestID       string                `json:"-"`
}

// DetailsAvailable 区分“上游只返回数量”与“已获取详情但列表为空”。
func (c *RateLimitResetCredits) DetailsAvailable() bool {
	return c != nil && c.Credits != nil
}

// SortedCredits 返回按到期时间升序排列的副本；没有到期时间的券排在最后。
func (c *RateLimitResetCredits) SortedCredits() []*RateLimitResetCredit {
	if c == nil || c.Credits == nil {
		return nil
	}
	credits := append([]*RateLimitResetCredit(nil), c.Credits...)
	sort.SliceStable(credits, func(i, j int) bool {
		left, right := credits[i], credits[j]
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		leftExpires, leftOK := left.ExpiresTime()
		rightExpires, rightOK := right.ExpiresTime()
		if !leftOK {
			return false
		}
		if !rightOK {
			return true
		}
		return leftExpires.Before(rightExpires)
	})
	return credits
}

// NextAvailableCredit 返回最早到期的可用券；没有可用详情时返回 nil。
func (c *RateLimitResetCredits) NextAvailableCredit() *RateLimitResetCredit {
	for _, credit := range c.SortedCredits() {
		if credit != nil && (credit.Status == "" || strings.EqualFold(credit.Status, "available")) {
			return credit
		}
	}
	return nil
}

// GrantedTime 返回券发放时间，并兼容秒和毫秒时间戳。
func (c *RateLimitResetCredit) GrantedTime() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	return resetCreditTimestamp(c.GrantedAt)
}

// ExpiresTime 返回券到期时间，并兼容秒和毫秒时间戳。
func (c *RateLimitResetCredit) ExpiresTime() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	return resetCreditTimestamp(c.ExpiresAt)
}

// GrantedAtMillis 返回适合应用 JSON DTO 使用的毫秒时间戳。
func (c *RateLimitResetCredit) GrantedAtMillis() int64 {
	value, ok := c.GrantedTime()
	if !ok {
		return 0
	}
	return value.UnixMilli()
}

// ExpiresAtMillis 返回适合应用 JSON DTO 使用的毫秒时间戳。
func (c *RateLimitResetCredit) ExpiresAtMillis() int64 {
	value, ok := c.ExpiresTime()
	if !ok {
		return 0
	}
	return value.UnixMilli()
}

// IsKnown 表示结果是否为当前官方协议定义的四种状态之一。
func (o RateLimitResetOutcome) IsKnown() bool {
	switch o {
	case RateLimitResetOutcomeReset, RateLimitResetOutcomeAlreadyRedeemed, RateLimitResetOutcomeNothingToReset, RateLimitResetOutcomeNoCredit:
		return true
	default:
		return false
	}
}

// IsIdempotentSuccess 表示券已在本次或同一幂等操作中成功兑换。
func (o RateLimitResetOutcome) IsIdempotentSuccess() bool {
	return o == RateLimitResetOutcomeReset || o == RateLimitResetOutcomeAlreadyRedeemed
}

// List 获取重置券详情。账号只返回汇总时，Credits 可以为空，但 AvailableCount 仍有效。
func (s *RateLimitResetService) List(ctx context.Context, accountID string) (*RateLimitResetCredits, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("chatgpt: rate-limit reset service is nil")
	}
	credits, err := s.listFromDetails(ctx, accountID, false)
	if !isAPIStatus(err, http.StatusNotFound) {
		return credits, err
	}
	credits, legacyErr := s.listFromDetails(ctx, accountID, true)
	if !isAPIStatus(legacyErr, http.StatusNotFound) {
		return credits, legacyErr
	}

	// 某些账号或服务版本只在 /wham/usage 中提供权威数量汇总。
	// 这是 ChatGPT 协议兼容行为，因此由库透明处理。
	usage, usageErr := s.client.Usage.Get(ctx, accountID)
	if usageErr != nil {
		return nil, legacyErr
	}
	if usage.RateLimitResetCredits == nil {
		return nil, legacyErr
	}
	credits = cloneRateLimitResetCredits(usage.RateLimitResetCredits)
	credits.Raw = append(credits.Raw[:0], usage.Raw...)
	credits.RequestID = usage.RequestID
	return credits, nil
}

// Consume 幂等消耗一张额度重置券。redeemRequestID 必须为同一逻辑操作稳定复用的 UUID。
func (s *RateLimitResetService) Consume(ctx context.Context, accountID, redeemRequestID, creditID string) (*ConsumeRateLimitResetCreditResult, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("chatgpt: rate-limit reset service is nil")
	}
	redeemRequestID = strings.TrimSpace(redeemRequestID)
	if redeemRequestID == "" {
		return nil, errors.New("chatgpt: redeem request ID is required")
	}
	payload := struct {
		RedeemRequestID string `json:"redeem_request_id"`
		CreditID        string `json:"credit_id,omitempty"`
	}{RedeemRequestID: redeemRequestID, CreditID: strings.TrimSpace(creditID)}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result, err := s.consumeAt(ctx, accountID, body, false)
	if !isAPIStatus(err, http.StatusNotFound) {
		return result, err
	}
	return s.consumeAt(ctx, accountID, body, true)
}

func (s *RateLimitResetService) listFromDetails(ctx context.Context, accountID string, legacy bool) (*RateLimitResetCredits, error) {
	var req *http.Request
	var err error
	if legacy {
		req, err = s.client.newRootRequest(ctx, http.MethodGet, rateLimitResetCreditsLegacyPath, accountID, nil, nil)
	} else {
		req, err = s.client.newRequest(ctx, http.MethodGet, rateLimitResetCreditsPath, accountID, nil)
	}
	if err != nil {
		return nil, err
	}
	body, requestID, err := s.client.doJSON(req)
	if err != nil {
		return nil, err
	}
	credits, err := decodeRateLimitResetCredits(body)
	if err != nil {
		return nil, err
	}
	credits.Raw = append(credits.Raw[:0], body...)
	credits.RequestID = requestID
	return credits, nil
}

func (s *RateLimitResetService) consumeAt(ctx context.Context, accountID string, body []byte, legacy bool) (*ConsumeRateLimitResetCreditResult, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	var req *http.Request
	var err error
	if legacy {
		req, err = s.client.newRootRequest(ctx, http.MethodPost, rateLimitResetConsumeLegacyPath, accountID, body, headers)
	} else {
		req, err = s.client.newRequestBody(ctx, http.MethodPost, rateLimitResetCreditsConsumePath, accountID, body, headers)
	}
	if err != nil {
		return nil, err
	}
	responseBody, requestID, err := s.client.doJSON(req)
	if err != nil {
		return nil, err
	}
	result, err := decodeConsumeRateLimitResetCreditResult(responseBody)
	if err != nil {
		return nil, err
	}
	result.RequestID = requestID
	return result, nil
}

func decodeRateLimitResetCredits(body []byte) (*RateLimitResetCredits, error) {
	var envelope struct {
		RateLimitResetCredits *RateLimitResetCredits  `json:"rate_limit_reset_credits"`
		Credits               []*RateLimitResetCredit `json:"credits"`
		AvailableCount        int                     `json:"available_count"`
		ApplicableCount       *int                    `json:"applicable_available_count"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.RateLimitResetCredits != nil {
		return normalizeRateLimitResetCredits(envelope.RateLimitResetCredits), nil
	}
	return normalizeRateLimitResetCredits(&RateLimitResetCredits{
		AvailableCount:           envelope.AvailableCount,
		ApplicableAvailableCount: envelope.ApplicableCount,
		Credits:                  envelope.Credits,
	}), nil
}

func decodeConsumeRateLimitResetCreditResult(body []byte) (*ConsumeRateLimitResetCreditResult, error) {
	var wire struct {
		Outcome         string          `json:"outcome"`
		Output          string          `json:"output"`
		CreditID        string          `json:"credit_id"`
		RedeemRequestID string          `json:"redeem_request_id"`
		WindowsReset    json.RawMessage `json:"windows_reset"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, err
	}
	outcome := strings.TrimSpace(wire.Outcome)
	if outcome == "" {
		outcome = strings.TrimSpace(wire.Output)
	}
	if outcome == "" {
		return nil, errors.New("chatgpt: reset credit response has no outcome")
	}
	return &ConsumeRateLimitResetCreditResult{
		Outcome:         normalizeResetCreditOutcome(outcome),
		CreditID:        strings.TrimSpace(wire.CreditID),
		RedeemRequestID: strings.TrimSpace(wire.RedeemRequestID),
		WindowsReset:    append(json.RawMessage(nil), wire.WindowsReset...),
		Raw:             append(json.RawMessage(nil), body...),
	}, nil
}

func normalizeResetCreditOutcome(value string) RateLimitResetOutcome {
	value = strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(value)))
	switch value {
	case "nothing_to_reset", "nothingtoreset":
		return RateLimitResetOutcomeNothingToReset
	case "no_credit", "nocredit":
		return RateLimitResetOutcomeNoCredit
	case "already_redeemed", "alreadyredeemed":
		return RateLimitResetOutcomeAlreadyRedeemed
	case "reset":
		return RateLimitResetOutcomeReset
	default:
		return RateLimitResetOutcome(value)
	}
}

func normalizeRateLimitResetCredits(credits *RateLimitResetCredits) *RateLimitResetCredits {
	if credits == nil {
		return nil
	}
	if credits.Credits != nil {
		items := make([]*RateLimitResetCredit, 0, len(credits.Credits))
		for _, credit := range credits.Credits {
			if credit == nil {
				continue
			}
			credit.ID = strings.TrimSpace(credit.ID)
			credit.ResetType = strings.TrimSpace(credit.ResetType)
			credit.Status = strings.TrimSpace(credit.Status)
			credit.Title = strings.TrimSpace(credit.Title)
			credit.Description = strings.TrimSpace(credit.Description)
			items = append(items, credit)
		}
		credits.Credits = items
	}
	return credits
}

func cloneRateLimitResetCredits(credits *RateLimitResetCredits) *RateLimitResetCredits {
	if credits == nil {
		return nil
	}
	clone := *credits
	if credits.Credits != nil {
		clone.Credits = append([]*RateLimitResetCredit{}, credits.Credits...)
	}
	clone.Raw = append(json.RawMessage(nil), credits.Raw...)
	return &clone
}

func resetCreditTimestamp(value *int64) (time.Time, bool) {
	if value == nil || *value <= 0 {
		return time.Time{}, false
	}
	seconds, nanoseconds := *value, int64(0)
	if seconds >= 1_000_000_000_000 {
		nanoseconds = (seconds % 1000) * int64(time.Millisecond)
		seconds /= 1000
	}
	return time.Unix(seconds, nanoseconds), true
}

func parseResetCreditWireTimestamp(raw json.RawMessage) (*int64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return nil, nil
	}
	if strings.HasPrefix(value, "\"") {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		if timestamp, err := strconv.ParseInt(text, 10, 64); err == nil {
			return &timestamp, nil
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05.999999999Z07",
			"2006-01-02T15:04:05.999999999Z07",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999",
		} {
			if parsed, err := time.Parse(layout, text); err == nil {
				timestamp := parsed.UnixMilli()
				return &timestamp, nil
			}
		}
		return nil, fmt.Errorf("unsupported timestamp %q", text)
	}
	timestamp, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("must be an integer or time string: %w", err)
	}
	return &timestamp, nil
}

func isAPIStatus(err error, statusCode int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == statusCode
}
