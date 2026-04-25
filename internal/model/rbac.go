package model

import (
	"time"
)

type Action int

const (
	ActionRead   Action = 1 // 0001 - 读取
	ActionCreate Action = 2 // 0010 - 创建
	ActionUpdate Action = 4 // 0100 - 更新
	ActionDelete Action = 8 // 1000 - 删除
)

func (a Action) Has(action Action) bool {
	return a&action != 0
}

func (a Action) String() string {
	var ops []string
	if a.Has(ActionRead) {
		ops = append(ops, "read")
	}
	if a.Has(ActionCreate) {
		ops = append(ops, "create")
	}
	if a.Has(ActionUpdate) {
		ops = append(ops, "update")
	}
	if a.Has(ActionDelete) {
		ops = append(ops, "delete")
	}
	if len(ops) == 0 {
		return "none"
	}
	if len(ops) == 4 {
		return "all"
	}
	return join(ops, ",")
}

func join(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}

type Permission struct {
	ID           int64     `json:"id"`
	ResourceType string    `json:"resource_type"` // agent, tool, skill
	ResourceName string    `json:"resource_name"` // agent_name, tool_name, *
	Actions      Action    `json:"actions"`       // bitmask: 1=read,2=create,4=update,8=delete
	Description  string    `json:"description"`
	IsSystem     bool      `json:"is_system"`
	CreatedAt    time.Time `json:"created_at"`
}

func (p *Permission) PermissionKey() string {
	return p.ResourceType + ":" + p.ResourceName + ":" + p.Actions.String()
}

type Role struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	IsActive    bool         `json:"is_active"`
	Permissions []Permission `json:"permissions,omitempty"`
	IsApprover  bool         `json:"is_approver"`
	UserCount   int64        `json:"user_count,omitempty"`
	AgentCount  int64        `json:"agent_count,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type UserRole struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	RoleID    int64      `json:"role_id"`
	IsActive  bool       `json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Role      *Role      `json:"role,omitempty"`
}
