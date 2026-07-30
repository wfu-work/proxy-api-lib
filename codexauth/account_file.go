package codexauth

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// AccountFile 是 FreeAI 与 Codex 账号工具共同使用的 OAuth 导入导出格式。
// 文件中包含敏感令牌，调用方必须加密存储并避免写入日志。
type AccountFile struct {
	Tokens AccountTokens `json:"tokens"`
	Meta   AccountMeta   `json:"meta"`
}

// AccountTokens 保存 Codex OAuth 会话及请求路由账号。
type AccountTokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccountID    string `json:"account_id"`
}

// AccountMeta 保存可安全展示的账号元信息；它不参与上游授权判断。
type AccountMeta struct {
	Label                 string `json:"label,omitempty"`
	Issuer                string `json:"issuer,omitempty"`
	Status                string `json:"status,omitempty"`
	WorkspaceID           string `json:"workspaceId,omitempty"`
	ChatGPTAccountID      string `json:"chatgptAccountId,omitempty"`
	ExportedAt            int64  `json:"exportedAt,omitempty"`
	PlanType              string `json:"planType,omitempty"`
	SubscriptionPlan      string `json:"subscriptionPlan,omitempty"`
	SubscriptionExpiresAt int64  `json:"subscriptionExpiresAt,omitempty"`
	SubscriptionRenewsAt  int64  `json:"subscriptionRenewsAt,omitempty"`
	SubscriptionWillRenew *bool  `json:"subscriptionWillRenew,omitempty"`
}

// ParseAccountFile 解析并标准化一个 Codex OAuth 账号文件。
func ParseAccountFile(data []byte) (*AccountFile, error) {
	var file AccountFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if err := file.Normalize(); err != nil {
		return nil, err
	}
	return &file, nil
}

// NewAccountFile 根据 OAuth Token 响应创建规范账号文件。
func NewAccountFile(tokens TokenSet, accountID string) (*AccountFile, error) {
	file := &AccountFile{
		Tokens: AccountTokens{
			AccessToken:  tokens.AccessToken,
			IDToken:      tokens.IDToken,
			RefreshToken: tokens.RefreshToken,
			AccountID:    accountID,
		},
		Meta: AccountMeta{Issuer: DefaultIssuer, Status: "active", ExportedAt: time.Now().UnixMilli()},
	}
	if err := file.Normalize(); err != nil {
		return nil, err
	}
	return file, nil
}

// Normalize 清理字段并从 JWT 展示声明中补齐账号路由信息。
func (f *AccountFile) Normalize() error {
	if f == nil {
		return errors.New("codexauth: account file is nil")
	}
	f.Tokens.AccessToken = strings.TrimSpace(f.Tokens.AccessToken)
	f.Tokens.IDToken = strings.TrimSpace(f.Tokens.IDToken)
	f.Tokens.RefreshToken = strings.TrimSpace(f.Tokens.RefreshToken)
	f.Tokens.AccountID = strings.TrimSpace(f.Tokens.AccountID)
	if f.Tokens.AccessToken == "" {
		return errors.New("codexauth: account file does not contain access_token")
	}

	claims := f.bestClaims()
	accountID := firstAccountValue(
		f.Meta.ChatGPTAccountID,
		f.Tokens.AccountID,
		claims.ResolvedAccountID(),
	)
	if accountID == "" {
		return errors.New("codexauth: account file does not contain chatgpt account id")
	}
	f.Tokens.AccountID = accountID
	f.Meta.ChatGPTAccountID = accountID
	if f.Meta.WorkspaceID = firstWorkspaceValue(f.Meta.WorkspaceID, claims.ResolvedWorkspaceID()); f.Meta.WorkspaceID == "" {
		f.Meta.WorkspaceID = accountID
	}
	f.Meta.Label = strings.TrimSpace(f.Meta.Label)
	if f.Meta.Label == "" {
		f.Meta.Label = claims.ResolvedEmail()
	}
	f.Meta.Issuer = strings.TrimSpace(f.Meta.Issuer)
	if f.Meta.Issuer == "" {
		f.Meta.Issuer = DefaultIssuer
	}
	f.Meta.Status = strings.ToLower(strings.TrimSpace(f.Meta.Status))
	if f.Meta.Status == "" {
		f.Meta.Status = "active"
	}
	if f.Meta.PlanType == "" {
		f.Meta.PlanType = claims.ResolvedPlanType()
	}
	f.Meta.PlanType = strings.ToLower(strings.TrimSpace(f.Meta.PlanType))
	f.Meta.SubscriptionPlan = strings.ToLower(strings.TrimSpace(f.Meta.SubscriptionPlan))
	if f.Meta.ExportedAt <= 0 {
		f.Meta.ExportedAt = time.Now().UnixMilli()
	}
	return nil
}

// ApplyTokenSet 原子刷新前在内存中合并轮换后的 Token 集合。
func (f *AccountFile) ApplyTokenSet(tokens TokenSet) error {
	if f == nil {
		return errors.New("codexauth: account file is nil")
	}
	previousRefresh := f.Tokens.RefreshToken
	f.Tokens.AccessToken = strings.TrimSpace(tokens.AccessToken)
	if value := strings.TrimSpace(tokens.IDToken); value != "" {
		f.Tokens.IDToken = value
	}
	f.Tokens.RefreshToken = tokens.EffectiveRefreshToken(previousRefresh)
	f.Meta.ExportedAt = time.Now().UnixMilli()
	return f.Normalize()
}

// Marshal 输出规范导出 JSON。
func (f *AccountFile) Marshal() ([]byte, error) {
	if err := f.Normalize(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(f, "", "  ")
}

// AccessTokenExpiresAt 返回访问令牌自身的到期时间，不代表订阅到期时间。
func (f *AccountFile) AccessTokenExpiresAt() (time.Time, bool) {
	if f == nil {
		return time.Time{}, false
	}
	claims, err := ParseUnverifiedClaims(f.Tokens.AccessToken)
	if err != nil {
		return time.Time{}, false
	}
	return claims.TokenExpiresAt()
}

// NeedsRefresh 判断访问令牌是否已过期或即将在 skew 内过期。
func (f *AccountFile) NeedsRefresh(now time.Time, skew time.Duration) bool {
	expiresAt, ok := f.AccessTokenExpiresAt()
	return ok && !expiresAt.After(now.Add(skew))
}

func (f *AccountFile) bestClaims() Claims {
	if f == nil {
		return Claims{}
	}
	for _, token := range []string{f.Tokens.IDToken, f.Tokens.AccessToken} {
		if claims, err := ParseUnverifiedClaims(token); err == nil {
			return claims
		}
	}
	return Claims{}
}

func firstAccountValue(values ...string) string {
	for _, value := range values {
		if normalized := normalizeScopedIdentity(value, "cgpt="); normalized != "" {
			return normalized
		}
	}
	return ""
}

func firstWorkspaceValue(values ...string) string {
	for _, value := range values {
		if normalized := normalizeScopedIdentity(value, "ws="); normalized != "" {
			return normalized
		}
	}
	return ""
}
