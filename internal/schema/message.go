package schema

import "time"

type AgentMessage struct {
	ID            int64          `json:"id"`
	FromAgentID   int64          `json:"from_agent_id"`
	FromAgentName string         `json:"from_agent_name,omitempty"`
	ToAgentID     int64          `json:"to_agent_id"`
	ToAgentName   string         `json:"to_agent_name,omitempty"`
	ChannelID     int64          `json:"channel_id"`
	SessionID     string         `json:"session_id"`
	Kind          string         `json:"kind"`
	Content       string         `json:"content"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Status        string         `json:"status"`
	Priority      int            `json:"priority"`
	CreatedAt     time.Time      `json:"created_at"`
	DeliveredAt   *time.Time     `json:"delivered_at,omitempty"`
}

type SendMessageRequest struct {
	FromAgentID int64          `json:"from_agent_id" validate:"required"`
	ToAgentID   int64          `json:"to_agent_id"`
	ChannelID   int64          `json:"channel_id"`
	SessionID   string         `json:"session_id"`
	Kind        string         `json:"kind" validate:"required,oneof=text command event result"`
	Content     string         `json:"content" validate:"required"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Priority    int            `json:"priority"`
}

type MessageSendResponse struct {
	MessageID   int64  `json:"message_id"`
	Status      string `json:"status"`
	DeliveredAt string `json:"delivered_at,omitempty"`
}

type MessageSpanRequest struct {
	FromAgentID int64          `json:"from_agent_id" validate:"required"`
	ToAgentID   int64          `json:"to_agent_id"`
	ChannelID   int64          `json:"channel_id"`
	SessionID   string         `json:"session_id"`
	Content     string         `json:"content" validate:"required"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type MessageSpanResponse struct {
	SpanID    string `json:"span_id"`
	MessageID int64  `json:"message_id"`
	Status    string `json:"status"`
	TraceID   string `json:"trace_id,omitempty"`
}

type ListMessagesRequest struct {
	ChannelID int64  `json:"channel_id"`
	AgentID   int64  `json:"agent_id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type A2ACard struct {
	ID           int64    `json:"id"`
	AgentID      int64    `json:"agent_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	URL          string   `json:"url"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities,omitempty"`
	IsActive     bool     `json:"is_active"`
	CreatedAt    string   `json:"created_at"`
}

type CreateA2ACardRequest struct {
	AgentID      int64    `json:"agent_id" validate:"required"`
	Name         string   `json:"name" validate:"required"`
	Description  string   `json:"description"`
	URL          string   `json:"url"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities,omitempty"`
	IsActive     bool     `json:"is_active"`
}
