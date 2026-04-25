package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const httpTimeout = 20 * time.Second

// SendText delivers plain text according to channel kind and credentials.
func SendText(ctx context.Context, kind, webhookURL, appID, appSecret string, extra map[string]string, text string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "lark":
		return sendLark(ctx, webhookURL, appID, appSecret, extra, text)
	case "dingtalk":
		sec := strings.TrimSpace(appSecret)
		if sec == "" {
			sec = strings.TrimSpace(extraVal(extra, "sign_secret"))
		}
		return sendDingTalk(ctx, webhookURL, sec, extra, text)
	case "wecom":
		return sendWeCom(ctx, webhookURL, text)
	default:
		return fmt.Errorf("unknown channel kind: %s", kind)
	}
}

func sendLark(ctx context.Context, webhookURL, appID, appSecret string, extra map[string]string, text string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	plain := strings.EqualFold(strings.TrimSpace(extraVal(extra, "use_plain_text")), "true")
	if plain {
		if webhookURL != "" {
			body := map[string]any{
				"msg_type": "text",
				"content":  map[string]string{"text": text},
			}
			return postJSON(ctx, webhookURL, body)
		}
		if appID == "" || appSecret == "" {
			return fmt.Errorf("lark: set webhook_url or app_id+app_secret with receive_id in extra")
		}
		inner, _ := json.Marshal(map[string]string{"text": text})
		return larkSendIMApp(ctx, appID, appSecret, extra, "text", string(inner))
	}

	title, md := larkCardTitleAndMarkdown(extra, text)
	if strings.TrimSpace(md) == "" {
		md = " "
	}
	card := buildLarkInteractiveCardMap(title, md)
	if webhookURL != "" {
		body := map[string]any{
			"msg_type": "interactive",
			"card":     card,
		}
		return postJSON(ctx, webhookURL, body)
	}
	if appID == "" || appSecret == "" {
		return fmt.Errorf("lark: set webhook_url or app_id+app_secret with receive_id in extra")
	}
	return larkSendIMApp(ctx, appID, appSecret, extra, "interactive", larkInteractiveContentString(title, md))
}

// larkSendIMApp posts im/v1/messages with tenant token (msg_type text or interactive).
func larkSendIMApp(ctx context.Context, appID, appSecret string, extra map[string]string, msgType, content string) error {
	receiveID := strings.TrimSpace(extraVal(extra, "receive_id"))
	if receiveID == "" {
		return fmt.Errorf("lark: extra.receive_id is required when using app credentials")
	}
	receiveIDType := strings.TrimSpace(extraVal(extra, "receive_id_type"))
	if receiveIDType == "" {
		receiveIDType = "open_id"
	}
	base := strings.TrimSuffix(strings.TrimSpace(extraVal(extra, "api_base")), "/")
	if base == "" {
		base = "https://open.feishu.cn"
	}
	token, err := larkTenantAccessToken(ctx, base, appID, appSecret)
	if err != nil {
		return err
	}
	msgURL := fmt.Sprintf("%s/open-apis/im/v1/messages?receive_id_type=%s",
		base, url.QueryEscape(receiveIDType))
	payload := map[string]any{
		"receive_id": receiveID,
		"msg_type":   msgType,
		"content":    content,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msgURL, bytes.NewReader(mustJSON(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lark im message: status %d: %s", resp.StatusCode, truncate(b, 500))
	}
	var wrap struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(b, &wrap); err == nil && wrap.Code != 0 {
		return fmt.Errorf("lark api: code=%d msg=%s", wrap.Code, wrap.Msg)
	}
	return nil
}

func larkTenantAccessToken(ctx context.Context, base, appID, appSecret string) (string, error) {
	u := base + "/open-apis/auth/v3/tenant_access_token/internal"
	body := map[string]string{"app_id": appID, "app_secret": appSecret}
	resp, err := postJSONRaw(ctx, u, body)
	if err != nil {
		return "", err
	}
	var out struct {
		Code             int    `json:"code"`
		Msg              string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", fmt.Errorf("lark token: parse: %w", err)
	}
	if out.Code != 0 {
		return "", fmt.Errorf("lark token: code=%d msg=%s", out.Code, out.Msg)
	}
	if out.TenantAccessToken == "" {
		return "", fmt.Errorf("lark token: empty tenant_access_token")
	}
	return out.TenantAccessToken, nil
}

func sendDingTalk(ctx context.Context, webhookURL, signSecret string, extra map[string]string, text string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("dingtalk: webhook_url is required")
	}
	sec := strings.TrimSpace(signSecret)
	if sec == "" {
		sec = strings.TrimSpace(extraVal(extra, "sign_secret"))
	}
	u := webhookURL
	if sec != "" {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		stringToSign := ts + "\n" + sec
		mac := hmac.New(sha256.New, []byte(sec))
		_, _ = mac.Write([]byte(stringToSign))
		sig := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		join := "?"
		if strings.Contains(webhookURL, "?") {
			join = "&"
		}
		u = webhookURL + join + "timestamp=" + ts + "&sign=" + sig
	}
	body := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	}
	return postJSON(ctx, u, body)
}

func sendWeCom(ctx context.Context, webhookURL, text string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("wecom: webhook_url is required (group robot URL with key)")
	}
	body := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	}
	return postJSON(ctx, webhookURL, body)
}

func postJSON(ctx context.Context, rawURL string, body any) error {
	b, err := postJSONRaw(ctx, rawURL, body)
	if err != nil {
		return err
	}
	s := string(b)
	if strings.Contains(rawURL, "qyapi.weixin.qq.com") {
		var w struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if json.Unmarshal(b, &w) == nil && w.ErrCode != 0 {
			return fmt.Errorf("wecom: errcode=%d errmsg=%s", w.ErrCode, w.ErrMsg)
		}
		return nil
	}
	if strings.Contains(rawURL, "dingtalk") || strings.Contains(rawURL, "alibaba-inc.com") {
		var d struct {
			ErrCode int `json:"errcode"`
		}
		if json.Unmarshal(b, &d) == nil && d.ErrCode != 0 {
			return fmt.Errorf("dingtalk: errcode=%d body=%s", d.ErrCode, truncate(b, 400))
		}
		return nil
	}
	if strings.Contains(rawURL, "feishu.cn") || strings.Contains(rawURL, "larksuite.com") {
		var fs struct {
			Code       int    `json:"code"`
			StatusCode int    `json:"StatusCode"`
			Msg        string `json:"msg"`
			StatusMsg  string `json:"StatusMessage"`
		}
		if json.Unmarshal(b, &fs) == nil {
			code := fs.Code
			if code == 0 {
				code = fs.StatusCode
			}
			msg := fs.Msg
			if msg == "" {
				msg = fs.StatusMsg
			}
			if code != 0 {
				return fmt.Errorf("lark webhook: code=%d msg=%s", code, msg)
			}
		}
		return nil
	}
	_ = s
	return nil
}

func postJSONRaw(ctx context.Context, rawURL string, body any) ([]byte, error) {
	data := mustJSON(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	client := &http.Client{Timeout: httpTimeout}
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
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(b, 500))
	}
	return b, nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func extraVal(extra map[string]string, key string) string {
	if extra == nil {
		return ""
	}
	return extra[key]
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
