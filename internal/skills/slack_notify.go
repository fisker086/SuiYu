package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolSlackNotify = "builtin_slack_notify"

var allowedSlackOps = map[string]bool{
	"send":    true,
	"webhook": true,
}

func execBuiltinSlackNotify(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "webhook"
	}

	if !allowedSlackOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedSlackOps)
	}

	message := strArg(in, "message", "text", "content")
	if message == "" {
		return "", fmt.Errorf("missing message")
	}

	switch op {
	case "webhook":
		webhookURL := strArg(in, "webhook_url", "url", "webhook")
		if webhookURL == "" {
			return "", fmt.Errorf("missing webhook_url")
		}

		payload := map[string]string{"text": message}
		body, _ := json.Marshal(payload)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Sprintf("Failed to send to Slack webhook: %v", err), nil
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("Slack webhook returned HTTP %d: %s", resp.StatusCode, string(respBody)), nil
		}
		return "Message sent to Slack via webhook", nil

	case "send":
		token := strArg(in, "token", "slack_token", "api_token")
		channel := strArg(in, "channel", "channel_id")
		if token == "" || channel == "" {
			return "", fmt.Errorf("missing token or channel for send operation")
		}

		payload := map[string]string{"channel": channel, "text": message}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Failed to send to Slack API: %v", err), nil
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

		var result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(respBody, &result); err == nil && !result.OK {
			return fmt.Sprintf("Slack API error: %s", result.Error), nil
		}
		return "Message sent to Slack via API", nil
	}

	return "", fmt.Errorf("unknown operation: %s", op)
}

func NewBuiltinSlackNotifyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSlackNotify,
			Desc:  "Send messages to Slack via webhook or API. Supports markdown.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":   {Type: einoschema.String, Desc: "Operation: send, webhook", Required: false},
				"message":     {Type: einoschema.String, Desc: "Message text (supports Slack markdown)", Required: true},
				"channel":     {Type: einoschema.String, Desc: "Slack channel (for send operation)", Required: false},
				"webhook_url": {Type: einoschema.String, Desc: "Incoming webhook URL (for webhook operation)", Required: false},
				"token":       {Type: einoschema.String, Desc: "Slack bot token (for send operation)", Required: false},
			}),
		},
		execBuiltinSlackNotify,
	)
}
