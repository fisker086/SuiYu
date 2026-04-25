package schema

import "time"

type MessageChannel struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	AgentID     int64             `json:"agent_id"`
	AgentName   string            `json:"agent_name,omitempty"`
	Kind        string            `json:"kind"`
	Description string            `json:"description"`
	IsPublic    bool              `json:"is_public"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsActive    bool              `json:"is_active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type CreateMessageChannelRequest struct {
	Name        string            `json:"name" validate:"required,min=1,max=200"`
	AgentID     int64             `json:"agent_id" validate:"required"`
	Kind        string            `json:"kind" validate:"required,oneof=direct broadcast topic"`
	Description string            `json:"description"`
	IsPublic    bool              `json:"is_public"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsActive    bool              `json:"is_active"`
}

type UpdateMessageChannelRequest struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	IsPublic    *bool             `json:"is_public,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsActive    *bool             `json:"is_active,omitempty"`
}
