package chatgpt

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UsageService 提供 ChatGPT/Codex 账号的限额窗口查询。
type UsageService struct{ client *Client }

// UsageSnapshot 是 ChatGPT/Codex 用量接口返回的账号限额快照。
// RateLimit 通常包含 5 小时和 7 天窗口；具体窗口长度以服务端返回值为准。
type UsageSnapshot struct {
	PlanType             string
	RateLimit            *RateLimit
	Credits              json.RawMessage
	AdditionalRateLimits []AdditionalRateLimit
	Extra                map[string]json.RawMessage
	Raw                  json.RawMessage
	RequestID            string
}

// RateLimit 描述一组主、次限额窗口及可用状态。
type RateLimit struct {
	Allowed         *bool            `json:"allowed,omitempty"`
	LimitReached    *bool            `json:"limit_reached,omitempty"`
	PrimaryWindow   *RateLimitWindow `json:"primary_window,omitempty"`
	SecondaryWindow *RateLimitWindow `json:"secondary_window,omitempty"`
}

// RateLimitWindow 描述一个滚动限额窗口。
// UsedPercent 是已使用比例，不是剩余比例或精确 Token 数量。
type RateLimitWindow struct {
	UsedPercent        *float64 `json:"used_percent,omitempty"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds,omitempty"`
	ResetAt            *int64   `json:"reset_at,omitempty"`
}

// RemainingPercent 根据服务端返回的已用比例计算剩余比例。
func (w *RateLimitWindow) RemainingPercent() (float64, bool) {
	if w == nil || w.UsedPercent == nil || math.IsNaN(*w.UsedPercent) || math.IsInf(*w.UsedPercent, 0) {
		return 0, false
	}
	return math.Max(0, math.Min(100, 100-*w.UsedPercent)), true
}

// WindowDuration 返回当前限额窗口长度。
func (w *RateLimitWindow) WindowDuration() (time.Duration, bool) {
	if w == nil || w.LimitWindowSeconds == nil || *w.LimitWindowSeconds < 0 {
		return 0, false
	}
	seconds := *w.LimitWindowSeconds
	if seconds > int64(math.MaxInt64)/int64(time.Second) {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

// ResetTime 返回当前限额窗口的下次重置时间。
func (w *RateLimitWindow) ResetTime() (time.Time, bool) {
	if w == nil || w.ResetAt == nil || *w.ResetAt <= 0 {
		return time.Time{}, false
	}
	return time.Unix(*w.ResetAt, 0), true
}

// AdditionalRateLimit 描述 Code Review、Spark 等额外限额窗口。
type AdditionalRateLimit struct {
	SourceKey      string
	LimitID        string `json:"limit_id,omitempty"`
	LimitName      string `json:"limit_name,omitempty"`
	MeteredFeature string `json:"metered_feature,omitempty"`
	RateLimit      *RateLimit
	Raw            json.RawMessage
}

// Get 获取 ChatGPT/Codex 账号的最新限额窗口。
// accountID 会写入 ChatGPT-Account-ID 请求头；多工作区账号应显式传入。
func (s *UsageService) Get(ctx context.Context, accountID string) (*UsageSnapshot, error) {
	req, err := s.client.newRequest(ctx, http.MethodGet, "/wham/usage", accountID, nil)
	if err != nil {
		return nil, err
	}
	body, requestID, err := s.client.doJSON(req)
	if err != nil {
		return nil, err
	}
	snapshot, err := decodeUsageSnapshot(body)
	if err != nil {
		return nil, err
	}
	snapshot.Raw = append(snapshot.Raw[:0], body...)
	snapshot.RequestID = requestID
	return snapshot, nil
}

// decodeUsageSnapshot 解析标准限额窗口，并保留未知字段和附加窗口。
func decodeUsageSnapshot(body []byte) (*UsageSnapshot, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	snapshot := &UsageSnapshot{Extra: map[string]json.RawMessage{}}
	if raw := fields["plan_type"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &snapshot.PlanType)
		delete(fields, "plan_type")
	}
	if raw, ok := fields["rate_limit"]; ok {
		if len(raw) > 0 && string(raw) != "null" {
			var limit RateLimit
			if err := json.Unmarshal(raw, &limit); err != nil {
				return nil, err
			}
			snapshot.RateLimit = &limit
		}
		delete(fields, "rate_limit")
	}
	if raw, ok := fields["credits"]; ok {
		if len(raw) > 0 && string(raw) != "null" {
			snapshot.Credits = append(snapshot.Credits[:0], raw...)
		}
		delete(fields, "credits")
	}
	if raw, ok := fields["additional_rate_limits"]; ok {
		if len(raw) > 0 && string(raw) != "null" {
			additional, err := decodeAdditionalRateLimits(raw)
			if err != nil {
				return nil, err
			}
			snapshot.AdditionalRateLimits = append(snapshot.AdditionalRateLimits, additional...)
		}
		delete(fields, "additional_rate_limits")
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := fields[key]
		if key != "rate_limit" && strings.HasSuffix(key, "_rate_limit") && string(raw) != "null" {
			additional, err := decodeAdditionalRateLimit(key, raw)
			if err != nil {
				return nil, err
			}
			snapshot.AdditionalRateLimits = append(snapshot.AdditionalRateLimits, additional)
			continue
		}
		snapshot.Extra[key] = append(json.RawMessage(nil), raw...)
	}
	if len(snapshot.Extra) == 0 {
		snapshot.Extra = nil
	}
	return snapshot, nil
}

// decodeAdditionalRateLimits 兼容 additional_rate_limits 的数组和对象两种返回格式。
func decodeAdditionalRateLimits(raw json.RawMessage) ([]AdditionalRateLimit, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err == nil {
		items := make([]AdditionalRateLimit, 0, len(array))
		for index, item := range array {
			sourceKey := "additional_rate_limits[" + strconv.Itoa(index) + "]"
			additional, err := decodeAdditionalRateLimit(sourceKey, item)
			if err != nil {
				return nil, err
			}
			if identifier := additionalIdentifier(additional); identifier != "" {
				additional.SourceKey = identifier
			}
			items = append(items, additional)
		}
		return items, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]AdditionalRateLimit, 0, len(keys))
	for _, key := range keys {
		additional, err := decodeAdditionalRateLimit(key, object[key])
		if err != nil {
			return nil, err
		}
		items = append(items, additional)
	}
	return items, nil
}

// decodeAdditionalRateLimit 兼容限额字段直接出现和嵌套在 rate_limit 中两种结构。
func decodeAdditionalRateLimit(sourceKey string, raw json.RawMessage) (AdditionalRateLimit, error) {
	var wire struct {
		LimitID         string           `json:"limit_id,omitempty"`
		LimitName       string           `json:"limit_name,omitempty"`
		MeteredFeature  string           `json:"metered_feature,omitempty"`
		Allowed         *bool            `json:"allowed,omitempty"`
		LimitReached    *bool            `json:"limit_reached,omitempty"`
		PrimaryWindow   *RateLimitWindow `json:"primary_window,omitempty"`
		SecondaryWindow *RateLimitWindow `json:"secondary_window,omitempty"`
		RateLimit       *RateLimit       `json:"rate_limit,omitempty"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return AdditionalRateLimit{}, err
	}
	limit := wire.RateLimit
	if limit == nil && (wire.PrimaryWindow != nil || wire.SecondaryWindow != nil || wire.Allowed != nil || wire.LimitReached != nil) {
		limit = &RateLimit{
			Allowed:         wire.Allowed,
			LimitReached:    wire.LimitReached,
			PrimaryWindow:   wire.PrimaryWindow,
			SecondaryWindow: wire.SecondaryWindow,
		}
	}
	return AdditionalRateLimit{
		SourceKey:      sourceKey,
		LimitID:        firstNonEmpty(wire.LimitID, wire.MeteredFeature),
		LimitName:      strings.TrimSpace(wire.LimitName),
		MeteredFeature: strings.TrimSpace(wire.MeteredFeature),
		RateLimit:      limit,
		Raw:            append(json.RawMessage(nil), raw...),
	}, nil
}

// additionalIdentifier 返回额外限额最稳定的展示或路由标识。
func additionalIdentifier(limit AdditionalRateLimit) string {
	return firstNonEmpty(limit.LimitID, limit.MeteredFeature, limit.LimitName)
}

// firstNonEmpty 返回第一个去除空白后不为空的字符串。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
