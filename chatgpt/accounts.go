package chatgpt

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const accountsCheckPath = "/accounts/check/v4-2023-04-27"

// AccountsService 提供 ChatGPT 账号与订阅权益查询。
type AccountsService struct{ client *Client }

// AccountsCheckResponse 是 ChatGPT accounts/check 接口返回的账号集合。
type AccountsCheckResponse struct {
	Accounts  map[string]AccountEntry `json:"accounts"`
	Raw       json.RawMessage         `json:"-"`
	RequestID string                  `json:"-"`
}

// AccountEntry 聚合一个 ChatGPT 账号的基础信息和订阅权益。
type AccountEntry struct {
	Account     *AccountInfo        `json:"account,omitempty"`
	Entitlement *AccountEntitlement `json:"entitlement,omitempty"`
}

// AccountInfo 是 accounts/check 返回的 ChatGPT 账号基础信息。
type AccountInfo struct {
	Name                     string `json:"name,omitempty"`
	Email                    string `json:"email,omitempty"`
	PlanType                 string `json:"plan_type,omitempty"`
	IsDefault                *bool  `json:"is_default,omitempty"`
	HasSubscription          *bool  `json:"has_subscription,omitempty"`
	HasActiveSubscription    *bool  `json:"has_active_subscription,omitempty"`
	IsPaidSubscriptionActive *bool  `json:"is_paid_subscription_active,omitempty"`
}

// AccountEntitlement 描述 ChatGPT 账号的订阅权益和续费时间。
type AccountEntitlement struct {
	SubscriptionPlan      string     `json:"subscription_plan,omitempty"`
	ExpiresAt             *Timestamp `json:"expires_at,omitempty"`
	RenewsAt              *Timestamp `json:"renews_at,omitempty"`
	NextRenewalAt         *Timestamp `json:"next_renewal_at,omitempty"`
	NextCreditGrantUpdate *Timestamp `json:"next_credit_grant_update,omitempty"`
	RenewalDate           *Timestamp `json:"renewal_date,omitempty"`
	WillRenew             *bool      `json:"will_renew,omitempty"`
	HasActiveSubscription *bool      `json:"has_active_subscription,omitempty"`
}

// Timestamp 兼容 RFC3339、Unix 秒和 Unix 毫秒格式的账号时间字段。
type Timestamp struct {
	Time time.Time
	Raw  string
}

// UnmarshalJSON 解析 ChatGPT 账号接口可能返回的字符串或数字时间戳。
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if t == nil {
		return nil
	}
	*t = Timestamp{}
	if string(data) == "null" {
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = strings.TrimSpace(string(data))
	}
	t.Raw = strings.TrimSpace(text)
	if t.Raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, t.Raw); err == nil {
		t.Time = parsed
		return nil
	}
	if timestamp, err := strconv.ParseInt(t.Raw, 10, 64); err == nil {
		if timestamp > 1_000_000_000_000 {
			timestamp /= 1000
		}
		t.Time = time.Unix(timestamp, 0)
	}
	return nil
}

// Valid 报告时间字段是否成功解析。
func (t *Timestamp) Valid() bool {
	return t != nil && !t.Time.IsZero()
}

// SubscriptionSnapshot 是选定 ChatGPT 账号的标准化订阅摘要。
// HasSubscription 为 nil 表示上游没有提供足够信息，而不是明确未订阅。
type SubscriptionSnapshot struct {
	AccountID        string
	HasSubscription  *bool
	AccountPlanType  string
	SubscriptionPlan string
	ExpiresAt        *time.Time
	RenewsAt         *time.Time
	Entry            AccountEntry
	Raw              json.RawMessage
	RequestID        string
}

// Check 获取当前 OAuth Token 可访问的全部 ChatGPT 账号和权益。
func (s *AccountsService) Check(ctx context.Context) (*AccountsCheckResponse, error) {
	headers := map[string]string{
		"Origin":  DefaultBaseURL,
		"Referer": DefaultBaseURL + "/",
	}
	req, err := s.client.newRequest(ctx, http.MethodGet, accountsCheckPath, "", headers)
	if err != nil {
		return nil, err
	}
	body, requestID, err := s.client.doJSON(req)
	if err != nil {
		return nil, err
	}
	var response AccountsCheckResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Accounts == nil {
		response.Accounts = map[string]AccountEntry{}
	}
	response.Raw = append(response.Raw[:0], body...)
	response.RequestID = requestID
	return &response, nil
}

// Subscription 查询并标准化指定账号的订阅信息。
// accountID 未匹配时依次选择默认账号、付费账号和第一个账号。
func (s *AccountsService) Subscription(ctx context.Context, accountID string) (*SubscriptionSnapshot, error) {
	response, err := s.Check(ctx)
	if err != nil {
		return nil, err
	}
	selectedID, entry, ok := selectAccount(response.Accounts, accountID)
	if !ok {
		return &SubscriptionSnapshot{Raw: response.Raw, RequestID: response.RequestID}, nil
	}
	snapshot := buildSubscriptionSnapshot(selectedID, entry)
	snapshot.Raw = response.Raw
	snapshot.RequestID = response.RequestID
	return &snapshot, nil
}

// selectAccount 按明确 ID、默认、付费和稳定排序选择账号。
func selectAccount(accounts map[string]AccountEntry, requestedID string) (string, AccountEntry, bool) {
	if requestedID = strings.TrimSpace(requestedID); requestedID != "" {
		if entry, ok := accounts[requestedID]; ok {
			return requestedID, entry, true
		}
	}
	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := accounts[key]
		if entry.Account != nil && entry.Account.IsDefault != nil && *entry.Account.IsDefault {
			return key, entry, true
		}
	}
	for _, key := range keys {
		entry := accounts[key]
		if hasPaidPlan(entry) {
			return key, entry, true
		}
	}
	if len(keys) == 0 {
		return "", AccountEntry{}, false
	}
	return keys[0], accounts[keys[0]], true
}

// buildSubscriptionSnapshot 将内部账号结构转换为稳定的订阅摘要。
func buildSubscriptionSnapshot(accountID string, entry AccountEntry) SubscriptionSnapshot {
	snapshot := SubscriptionSnapshot{AccountID: accountID, Entry: entry}
	if entry.Account != nil {
		snapshot.AccountPlanType = normalizePlan(entry.Account.PlanType)
	}
	if entry.Entitlement != nil {
		snapshot.SubscriptionPlan = normalizePlan(entry.Entitlement.SubscriptionPlan)
		snapshot.ExpiresAt = timestampTime(entry.Entitlement.ExpiresAt)
		for _, candidate := range []*Timestamp{
			entry.Entitlement.RenewsAt,
			entry.Entitlement.NextRenewalAt,
			entry.Entitlement.NextCreditGrantUpdate,
			entry.Entitlement.RenewalDate,
		} {
			if snapshot.RenewsAt = timestampTime(candidate); snapshot.RenewsAt != nil {
				break
			}
		}
		if snapshot.RenewsAt == nil && entry.Entitlement.WillRenew != nil && *entry.Entitlement.WillRenew {
			snapshot.RenewsAt = snapshot.ExpiresAt
		}
	}
	snapshot.HasSubscription = resolvedSubscriptionState(entry, snapshot)
	return snapshot
}

// resolvedSubscriptionState 优先采用上游显式状态，缺失时再从套餐和时间推断。
func resolvedSubscriptionState(entry AccountEntry, snapshot SubscriptionSnapshot) *bool {
	if entry.Entitlement != nil && entry.Entitlement.HasActiveSubscription != nil {
		return boolPointer(*entry.Entitlement.HasActiveSubscription)
	}
	if entry.Account != nil {
		for _, candidate := range []*bool{
			entry.Account.HasSubscription,
			entry.Account.HasActiveSubscription,
			entry.Account.IsPaidSubscriptionActive,
		} {
			if candidate != nil {
				return boolPointer(*candidate)
			}
		}
	}
	if snapshot.AccountPlanType != "" && snapshot.AccountPlanType != "free" ||
		snapshot.SubscriptionPlan != "" && snapshot.SubscriptionPlan != "free" ||
		snapshot.ExpiresAt != nil || snapshot.RenewsAt != nil {
		return boolPointer(true)
	}
	if snapshot.AccountPlanType == "free" || snapshot.SubscriptionPlan == "free" {
		return boolPointer(false)
	}
	return nil
}

// hasPaidPlan 报告账号或权益中是否存在明确的非免费套餐。
func hasPaidPlan(entry AccountEntry) bool {
	if entry.Account != nil {
		if plan := normalizePlan(entry.Account.PlanType); plan != "" && plan != "free" {
			return true
		}
	}
	if entry.Entitlement != nil {
		if plan := normalizePlan(entry.Entitlement.SubscriptionPlan); plan != "" && plan != "free" {
			return true
		}
	}
	return false
}

// normalizePlan 统一套餐类型的空白和大小写。
func normalizePlan(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// timestampTime 将有效的兼容时间转换为独立的 time.Time 指针。
func timestampTime(value *Timestamp) *time.Time {
	if !value.Valid() {
		return nil
	}
	result := value.Time
	return &result
}

// boolPointer 创建不与响应结构共享地址的布尔值指针。
func boolPointer(value bool) *bool {
	return &value
}
