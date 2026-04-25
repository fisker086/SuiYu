package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolAlertSender = "builtin_alert_sender"

func execBuiltinAlertSender(_ context.Context, in map[string]any) (string, error) {
	channel := strArg(in, "channel", "platform", "type")
	if channel == "" {
		return "", fmt.Errorf("missing channel (lark, dingtalk, wecom)")
	}

	webhookURL := strArg(in, "webhook_url", "webhook", "url")
	if webhookURL == "" {
		return "", fmt.Errorf("missing webhook_url")
	}

	title := strArg(in, "title", "subject", "alert_title")
	if title == "" {
		title = "Alert Notification"
	}

	content := strArg(in, "content", "message", "body", "text")
	if content == "" {
		return "", fmt.Errorf("missing content")
	}

	level := strArg(in, "level", "severity", "priority")
	if level == "" {
		level = "info"
	}

	atUsers := strArg(in, "at_users", "mention", "notify_users")

	var payload any
	var err error

	switch strings.ToLower(channel) {
	case "lark", "feishu":
		payload = buildLarkPayload(title, content, level, atUsers)
	case "dingtalk", "dingding":
		payload = buildDingTalkPayload(title, content, level, atUsers)
	case "wecom", "wechat", "enterprise_wechat":
		payload = buildWeComPayload(title, content, level, atUsers)
	default:
		return "", fmt.Errorf("unsupported channel: %s (supported: lark, dingtalk, wecom)", channel)
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("alert webhook returned HTTP %d", resp.StatusCode)
	}

	return fmt.Sprintf("Alert sent successfully via %s\nTitle: %s\nLevel: %s", channel, title, level), nil
}

func buildLarkPayload(title, content, level string, atUsers string) map[string]any {
	color := "blue"
	switch level {
	case "warning":
		color = "orange"
	case "critical", "error", "fatal":
		color = "red"
	}

	var atList []string
	if atUsers != "" {
		for _, u := range strings.Split(atUsers, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				atList = append(atList, fmt.Sprintf("<at email=%q></at>", u))
			}
		}
	}

	contentWithAt := ""
	if len(atList) > 0 {
		contentWithAt = strings.Join(atList, " ") + "\n"
	}
	contentWithAt += content

	return map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"schema": "2.0",
			"config": map[string]any{
				"update_multi": true,
				"width_mode":   "fill",
			},
			"header": map[string]any{
				"title": map[string]any{
					"tag":     "plain_text",
					"content": title,
				},
				"template": color,
			},
			"body": map[string]any{
				"elements": []map[string]any{
					{
						"tag":        "markdown",
						"element_id": "md_alert",
						"content":    contentWithAt,
					},
				},
			},
		},
	}
}

func buildDingTalkPayload(title, content, level string, atUsers string) map[string]any {
	var atMobiles []string
	var atAll bool

	if atUsers != "" {
		for _, u := range strings.Split(atUsers, ",") {
			u = strings.TrimSpace(u)
			if u == "all" {
				atAll = true
			} else {
				atMobiles = append(atMobiles, u)
			}
		}
	}

	text := fmt.Sprintf("## %s\n**Level:** %s\n\n%s", title, level, content)

	return map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
		"at": map[string]any{
			"atMobiles": atMobiles,
			"isAtAll":   atAll,
		},
	}
}

func buildWeComPayload(title, content, level string, atUsers string) map[string]any {
	text := fmt.Sprintf("## %s\n**Level:** %s\n\n%s", title, level, content)

	if atUsers != "" {
		for _, u := range strings.Split(atUsers, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				text += fmt.Sprintf("\n<@%s>", u)
			}
		}
	}

	return map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": text,
		},
	}
}

func NewBuiltinAlertSenderTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolAlertSender,
			Desc: "Send alert notifications to Lark, DingTalk, or WeCom via webhook. Supports markdown content and user mentions.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"channel":     {Type: einoschema.String, Desc: "Channel: lark, dingtalk, wecom", Required: true},
				"webhook_url": {Type: einoschema.String, Desc: "Webhook URL for the target channel", Required: true},
				"title":       {Type: einoschema.String, Desc: "Alert title", Required: false},
				"content":     {Type: einoschema.String, Desc: "Alert content (supports markdown)", Required: true},
				"level":       {Type: einoschema.String, Desc: "Severity: info, warning, critical (default: info)", Required: false},
				"at_users":    {Type: einoschema.String, Desc: "Comma-separated user IDs/phones to mention", Required: false},
			}),
		},
		execBuiltinAlertSender,
	)
}
