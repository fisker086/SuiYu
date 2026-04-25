package larkauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultOpenAPIBase is the Feishu Open Platform API root (国内飞书).
//
// Naming: 代码里 AUTH_TYPE 常用 "lark"（与开放平台 query channel=lark 一致）；国内域名是 feishu.cn，
// 国际版 Lark 为 https://open.larksuite.com/open-apis 。请勿把 SSO_OPEN_API_BASE 设成 accounts.feishu.cn
//（登录页会重定向到该域，但授权入口与 token 接口均在 open.* / open-apis 下）。
const DefaultOpenAPIBase = "https://open.feishu.cn/open-apis"

// Client calls Feishu OAuth and authen APIs (aligned with aiops lark_auth.py).
type Client struct {
	HTTP        *http.Client
	OpenAPIBase string
	AppID       string
	AppSecret   string
	RedirectURI string
}

// UserInfo holds normalized fields from GET /authen/v1/user_info.
type UserInfo struct {
	Email     string
	Name      string
	OpenID    string
	UnionID   string
	AvatarURL string
}

// LoginURL builds the first-party authorize URL: {OpenAPIBase}/authen/v1/index?...
// 浏览器随后可能跳到 accounts.feishu.cn 展示登录页，属飞书侧行为；本函数只生成 open.feishu.cn 上的入口。
func (c *Client) LoginURL(state string) string {
	base := strings.TrimSuffix(c.OpenAPIBase, "/")
	q := url.Values{}
	q.Set("app_id", c.AppID)
	q.Set("redirect_uri", c.RedirectURI)
	// 开放平台约定：飞书应用仍使用 channel=lark，与域名 feishu / larksuite 无关。
	q.Set("channel", "lark")
	if state != "" {
		q.Set("state", state)
	}
	return base + "/authen/v1/index?" + q.Encode()
}

type feishuEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lark http %d: %s", resp.StatusCode, truncate(b, 500))
	}
	return b, nil
}

func (c *Client) postJSON(path string, headers map[string]string, body any) ([]byte, error) {
	base := strings.TrimSuffix(c.OpenAPIBase, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.doRequest(req)
}

func (c *Client) getJSON(path string, headers map[string]string) ([]byte, error) {
	base := strings.TrimSuffix(c.OpenAPIBase, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.doRequest(req)
}

// feishuCodeMsg is common top-level code (0 = OK).
type feishuCodeMsg struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func checkFeishuCode(b []byte) error {
	var cm feishuCodeMsg
	if err := json.Unmarshal(b, &cm); err != nil {
		return err
	}
	if cm.Code != 0 {
		return fmt.Errorf("lark api code=%d msg=%s", cm.Code, cm.Msg)
	}
	return nil
}

type appTokenFlat struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	AppAccessToken    string `json:"app_access_token"`
}

// AppAccessToken calls POST /auth/v3/app_access_token/internal.
func (c *Client) AppAccessToken() (string, error) {
	b, err := c.postJSON("/auth/v3/app_access_token/internal", nil, map[string]string{
		"app_id":     c.AppID,
		"app_secret": c.AppSecret,
	})
	if err != nil {
		return "", err
	}
	var flat appTokenFlat
	if err := json.Unmarshal(b, &flat); err != nil {
		return "", err
	}
	if flat.Code != 0 {
		return "", fmt.Errorf("lark api code=%d msg=%s", flat.Code, flat.Msg)
	}
	tok := strings.TrimSpace(flat.TenantAccessToken)
	if tok == "" {
		tok = strings.TrimSpace(flat.AppAccessToken)
	}
	if tok == "" {
		return "", fmt.Errorf("lark: empty app/tenant access token")
	}
	return tok, nil
}

type userAccessTokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// UserInfoFromCode exchanges OAuth code for user profile (authen/v1/access_token + authen/v1/user_info).
func (c *Client) UserInfoFromCode(code string) (*UserInfo, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("empty oauth code")
	}
	appTok, err := c.AppAccessToken()
	if err != nil {
		return nil, err
	}
	b, err := c.postJSON("/authen/v1/access_token", map[string]string{
		"Authorization": "Bearer " + appTok,
	}, map[string]any{
		"grant_type": "authorization_code",
		"code":       code,
		"jti":        uuid.NewString(),
	})
	if err != nil {
		return nil, err
	}
	if err := checkFeishuCode(b); err != nil {
		return nil, err
	}
	var env feishuEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	var tok userAccessTokenData
	if err := json.Unmarshal(env.Data, &tok); err != nil {
		return nil, fmt.Errorf("lark access_token parse: %w", err)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return nil, fmt.Errorf("lark: empty user access token")
	}
	ub, err := c.getJSON("/authen/v1/user_info", map[string]string{
		"Authorization": "Bearer " + tok.AccessToken,
	})
	if err != nil {
		return nil, err
	}
	if err := checkFeishuCode(ub); err != nil {
		return nil, err
	}
	var uenv feishuEnvelope
	if err := json.Unmarshal(ub, &uenv); err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	if len(uenv.Data) > 0 {
		if err := json.Unmarshal(uenv.Data, &raw); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(ub, &raw); err != nil {
			return nil, err
		}
	}
	return normalizeUserInfo(raw)
}

func normalizeUserInfo(raw map[string]any) (*UserInfo, error) {
	email := strField(raw, "email")
	if email == "" {
		email = strField(raw, "enterprise_email")
	}
	if email == "" {
		return nil, fmt.Errorf("lark account has no email (check app scopes / user visibility)")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	name := strField(raw, "name")
	if name == "" {
		name = email
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return &UserInfo{
		Email:     email,
		Name:      name,
		OpenID:    strField(raw, "open_id"),
		UnionID:   strField(raw, "union_id"),
		AvatarURL: strField(raw, "avatar_url"),
	}, nil
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// NewClient returns a client with sane defaults.
func NewClient(appID, appSecret, redirectURI, openAPIBase string) *Client {
	base := strings.TrimSpace(openAPIBase)
	if base == "" {
		base = DefaultOpenAPIBase
	}
	return &Client{
		HTTP:        &http.Client{Timeout: 15 * time.Second},
		OpenAPIBase: base,
		AppID:       appID,
		AppSecret:   appSecret,
		RedirectURI: redirectURI,
	}
}
