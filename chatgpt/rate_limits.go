package chatgpt

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitHeaderSnapshot 是普通 Codex 响应头携带的限额快照。
// 官方响应并不保证每次都返回全部字段，因此所有状态字段都使用指针表示缺失。
type RateLimitHeaderSnapshot struct {
	RateLimit *RateLimit
	Captured  http.Header
}

// ParseRateLimitHeaders 从普通 Codex 响应中提取主、次额度窗口。
// 它兼容目前观察到的 x-codex-* 命名变体；完全没有额度信号时返回 false。
func ParseRateLimitHeaders(header http.Header) (RateLimitHeaderSnapshot, bool) {
	if len(header) == 0 {
		return RateLimitHeaderSnapshot{}, false
	}
	primary, primaryOK := parseHeaderWindow(header, "primary")
	secondary, secondaryOK := parseHeaderWindow(header, "secondary")
	allowed, allowedOK := headerBool(header,
		"x-codex-allowed",
		"x-codex-rate-limit-allowed",
	)
	limitReached, reachedOK := headerBool(header,
		"x-codex-limit-reached",
		"x-codex-rate-limit-reached",
	)
	if !primaryOK && !secondaryOK && !allowedOK && !reachedOK {
		return RateLimitHeaderSnapshot{}, false
	}
	captured := make(http.Header)
	for key, values := range header {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "x-codex-") || strings.HasPrefix(lower, "x-ratelimit-") {
			captured[key] = append([]string(nil), values...)
		}
	}
	limit := &RateLimit{PrimaryWindow: primary, SecondaryWindow: secondary}
	if allowedOK {
		limit.Allowed = &allowed
	}
	if reachedOK {
		limit.LimitReached = &limitReached
	}
	return RateLimitHeaderSnapshot{RateLimit: limit, Captured: captured}, true
}

func parseHeaderWindow(header http.Header, kind string) (*RateLimitWindow, bool) {
	prefixes := []string{
		"x-codex-" + kind + "-",
		"x-codex-rate-limit-" + kind + "-",
		"x-codex-" + kind + "-window-",
	}
	used, usedOK := headerFloat(header, headerNames(prefixes, "used-percent")...)
	seconds, secondsOK := headerInt64(header, append(
		headerNames(prefixes, "limit-window-seconds"),
		headerNames(prefixes, "window-seconds")...,
	)...)
	resetAt, resetOK := headerTimestamp(header, append(
		headerNames(prefixes, "reset-at"),
		headerNames(prefixes, "reset")...,
	)...)
	if !usedOK && !secondsOK && !resetOK {
		return nil, false
	}
	window := &RateLimitWindow{}
	if usedOK {
		used = math.Max(0, math.Min(100, used))
		window.UsedPercent = &used
	}
	if secondsOK {
		window.LimitWindowSeconds = &seconds
	}
	if resetOK {
		window.ResetAt = &resetAt
	}
	return window, true
}

func headerNames(prefixes []string, suffix string) []string {
	names := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		names = append(names, prefix+suffix)
	}
	return names
}

func firstHeader(header http.Header, names ...string) (string, bool) {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value, true
		}
	}
	return "", false
}

func headerFloat(header http.Header, names ...string) (float64, bool) {
	value, ok := firstHeader(header, names...)
	if !ok {
		return 0, false
	}
	value = strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}

func headerInt64(header http.Header, names ...string) (int64, bool) {
	value, ok := firstHeader(header, names...)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil && parsed >= 0
}

func headerTimestamp(header http.Header, names ...string) (int64, bool) {
	value, ok := firstHeader(header, names...)
	if !ok {
		return 0, false
	}
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		if parsed > 1_000_000_000_000 {
			parsed /= 1000
		}
		return parsed, parsed > 0
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Unix(), true
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return time.Now().Add(duration).Unix(), true
	}
	return 0, false
}

func headerBool(header http.Header, names ...string) (bool, bool) {
	value, ok := firstHeader(header, names...)
	if !ok {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	return parsed, err == nil
}
