package model

import "time"

// MessageChannel represents a communication channel between agents.
// Each agent owns one or more channels for receiving messages.
type MessageChannel struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	AgentID     int64             `json:"agent_id"`
	Kind        string            `json:"kind"`
	Description string            `json:"description"`
	IsPublic    bool              `json:"is_public"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsActive    bool              `json:"is_active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AgentMessage represents a single message in the inter-agent messaging system.
type AgentMessage struct {
	ID          int64          `json:"id"`
	FromAgentID int64          `json:"from_agent_id"`
	ToAgentID   int64          `json:"to_agent_id"`
	ChannelID   int64          `json:"channel_id"`
	SessionID   string         `json:"session_id"`
	Kind        string         `json:"kind"`
	Content     string         `json:"content"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Status      string         `json:"status"`
	Priority    int            `json:"priority"`
	CreatedAt   time.Time      `json:"created_at"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
}

// A2ACard represents an Agent Card for A2A protocol compatibility.
type A2ACard struct {
	ID           int64     `json:"id"`
	AgentID      int64     `json:"agent_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	URL          string    `json:"url"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}
