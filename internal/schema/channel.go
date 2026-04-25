package schema

import "time"

// Channel is an outbound notification endpoint (Lark / DingTalk / WeCom).
// Secrets are never returned in full; use HasAppSecret to indicate configuration.
type Channel struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	WebhookURL   string            `json:"webhook_url,omitempty"`
	AppID        string            `json:"app_id,omitempty"`
	HasAppSecret bool              `json:"has_app_secret"`
	Extra        map[string]string `json:"extra,omitempty"`
	IsActive     bool              `json:"is_active"`
	CreatedAt    time.Time         `json:"created_at"`
}

type CreateChannelRequest struct {
	Name       string            `json:"name" validate:"required,min=1,max=200"`
	Kind       string            `json:"kind" validate:"required,oneof=lark dingtalk wecom"`
	WebhookURL string            `json:"webhook_url"`
	AppID      string            `json:"app_id"`
	AppSecret  string            `json:"app_secret"`
	Extra      map[string]string `json:"extra,omitempty"`
	IsActive   bool              `json:"is_active"`
}

type UpdateChannelRequest struct {
	Name       string            `json:"name,omitempty"`
	WebhookURL string            `json:"webhook_url,omitempty"`
	AppID      string            `json:"app_id,omitempty"`
	AppSecret  string            `json:"app_secret,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
	IsActive   *bool             `json:"is_active,omitempty"`
}

type TestChannelRequest struct {
	Message string `json:"message"`
}
