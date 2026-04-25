package model

import "time"

// Channel is the persisted row including secrets (only used server-side).
type Channel struct {
	ID         int64             `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	WebhookURL string            `json:"webhook_url"`
	AppID      string            `json:"app_id"`
	AppSecret  string            `json:"app_secret"`
	Extra      map[string]string `json:"extra"`
	IsActive   bool              `json:"is_active"`
	CreatedAt  time.Time         `json:"created_at"`
}
