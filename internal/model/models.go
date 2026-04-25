package model

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

type Agent struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	IsBuiltin   bool      `json:"is_builtin"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentRuntime struct {
	ID              int64     `json:"id"`
	AgentID         int64     `json:"agent_id"`
	SourceAgent     string    `json:"source_agent"`
	Archetype       string    `json:"archetype"`
	Role            string    `json:"role"`
	Goal            string    `json:"goal"`
	Backstory       string    `json:"backstory"`
	SystemPrompt    string    `json:"system_prompt"`
	LlmModel        string    `json:"llm_model"`
	Temperature     float64   `json:"temperature"`
	StreamEnabled   bool      `json:"stream_enabled"`
	MemoryEnabled   bool      `json:"memory_enabled"`
	SkillIDs        []string  `json:"skill_ids"`
	MCPConfigIDs    []int64   `json:"mcp_config_ids"`
	ExecutionMode   string    `json:"execution_mode"`
	MaxIterations   int       `json:"max_iterations"`
	PlanPrompt      string    `json:"plan_prompt"`
	ReflectionDepth int       `json:"reflection_depth"`
	ApprovalMode    string    `json:"approval_mode"`
	Approvers       []string  `json:"approvers"`
	IMEnabled       string    `json:"im_enabled"`
	IMConfig        []byte    `json:"im_config"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Capability struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	SourceType  string    `json:"source_type"`
	SourceRef   string    `json:"source_ref"`
	ToolName    string    `json:"tool_name"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type CapabilityTreeNode struct {
	ID           int64          `json:"id"`
	AgentID      int64          `json:"agent_id"`
	ParentID     *int64         `json:"parent_id,omitempty"`
	NodeType     string         `json:"node_type"`
	Label        string         `json:"label"`
	CapabilityID *int64         `json:"capability_id,omitempty"`
	RuleJSON     map[string]any `json:"rule_json,omitempty"`
	SortOrder    int            `json:"sort_order"`
	IsActive     bool           `json:"is_active"`
	Version      int            `json:"version"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Skill struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SourceRef   string    `json:"source_ref"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type MCPConfig struct {
	ID           int64          `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Transport    string         `json:"transport"`
	Endpoint     string         `json:"endpoint"`
	Config       map[string]any `json:"config,omitempty"`
	IsActive     bool           `json:"is_active"`
	HealthStatus string         `json:"health_status"`
	ToolCount    int            `json:"tool_count"`
	CreatedAt    time.Time      `json:"created_at"`
}

type MCPServer struct {
	ID          int64          `json:"id"`
	ConfigID    int64          `json:"config_id"`
	ToolName    string         `json:"tool_name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	IsActive    bool           `json:"is_active"`
}

type AgentMemory struct {
	ID        int64           `json:"id"`
	AgentID   int64           `json:"agent_id"`
	UserID    string          `json:"user_id"`
	SessionID string          `json:"session_id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Extra     map[string]any  `json:"extra,omitempty"`
	Embedding pgvector.Vector `json:"embedding"`
	CreatedAt time.Time       `json:"created_at"`
}

type SemanticMemory struct {
	ID        int64           `json:"id"`
	AgentID   int64           `json:"agent_id"`
	UserID    string          `json:"user_id"`
	Content   string          `json:"content"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
	Embedding pgvector.Vector `json:"embedding"`
	CreatedAt time.Time       `json:"created_at"`
}

type UserProfile struct {
	ID        int64           `json:"id"`
	UserID    string          `json:"user_id"`
	AgentID   int64           `json:"agent_id"`
	Profile   map[string]any  `json:"profile"`
	Embedding pgvector.Vector `json:"embedding"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Schedule struct {
	ID            int64     `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name" gorm:"size:200;not null"`
	Description   string    `json:"description" gorm:"size:500"`
	AgentID       int64     `json:"agent_id" gorm:"index"`
	WorkflowID    int64     `json:"workflow_id" gorm:"index"`
	ChannelID     *int64    `json:"channel_id,omitempty" gorm:"index"`        // optional notify channel (channels.id)
	OwnerUserID   string    `json:"owner_user_id,omitempty" gorm:"size:64"`   // login user id string; chat session owner
	ChatSessionID string    `json:"chat_session_id,omitempty" gorm:"size:36"` // last run session (isolated: each run; main: latest used)
	ScheduleKind  string    `json:"schedule_kind" gorm:"size:20;not null"`
	CronExpr      string    `json:"cron_expr" gorm:"size:100"`
	At            string    `json:"at" gorm:"size:100"`
	EveryMs       int64     `json:"every_ms"`
	Timezone      string    `json:"timezone" gorm:"size:50"`
	WakeMode      string    `json:"wake_mode" gorm:"size:20"`
	SessionTarget string    `json:"session_target" gorm:"size:20"`
	Prompt        string    `json:"prompt" gorm:"type:text"`
	CodeLanguage  string    `json:"code_language" gorm:"size:20"` // python, shell, javascript for code execution
	StaggerMs     int64     `json:"stagger_ms"`
	Enabled       bool      `json:"enabled" gorm:"default:true"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ScheduleExecution struct {
	ID         int64      `json:"id" gorm:"primaryKey"`
	ScheduleID int64      `json:"schedule_id" gorm:"not null;index"`
	Status     string     `json:"status" gorm:"size:20;not null"`
	Result     string     `json:"result" gorm:"type:text"`
	Error      string     `json:"error" gorm:"type:text"`
	DurationMs int64      `json:"duration_ms"`
	StartedAt  time.Time  `json:"started_at" gorm:"not null"`
	FinishedAt *time.Time `json:"finished_at"`
}

type AuditLog struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	UserID     string    `json:"user_id" gorm:"size:100;index"`
	AgentID    int64     `json:"agent_id" gorm:"not null;index"`
	SessionID  string    `json:"session_id" gorm:"size:100;index"`
	ToolName   string    `json:"tool_name" gorm:"size:100;not null"`
	Action     string    `json:"action" gorm:"size:50;not null"`
	RiskLevel  string    `json:"risk_level" gorm:"size:20;not null"`
	Input      string    `json:"input" gorm:"type:text"`
	Output     string    `json:"output" gorm:"type:text"`
	Error      string    `json:"error" gorm:"type:text"`
	Status     string    `json:"status" gorm:"size:20;not null"`
	DurationMs int64     `json:"duration_ms"`
	IPAddress  string    `json:"ip_address" gorm:"size:50"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null;index"`
}

type ApprovalRequest struct {
	ID         int64      `json:"id" gorm:"primaryKey"`
	AgentID    int64      `json:"agent_id" gorm:"not null;index"`
	SessionID  string     `json:"session_id" gorm:"size:100;index"`
	UserID     string     `json:"user_id" gorm:"size:100"`
	ToolName   string     `json:"tool_name" gorm:"size:100;not null"`
	RiskLevel  string     `json:"risk_level" gorm:"size:20;not null"`
	Input      string     `json:"input" gorm:"type:text"`
	Status     string     `json:"status" gorm:"size:20;not null;index"`
	ApproverID string     `json:"approver_id" gorm:"size:100"`
	Comment    string     `json:"comment" gorm:"type:text"`
	ApprovedAt *time.Time `json:"approved_at"`
	CreatedAt  time.Time  `json:"created_at" gorm:"not null;index"`

	ApprovalType  string     `json:"approval_type" gorm:"size:20"`
	ExternalID    string     `json:"external_id" gorm:"size:100"`
	ExpiresAt     *time.Time `json:"expires_at"` // DB NULL → nil; do not use non-pointer (Scan fails on NULL)
	NotifyWebhook string     `json:"notify_webhook" gorm:"size:500"`

	// AgentName is filled on read (JOIN agents / memory lookup), not a table column.
	AgentName string `json:"agent_name,omitempty" gorm:"-"`
}

type TokenUsage struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"user_id" gorm:"size:100;index"`
	UserName     string    `json:"user_name" gorm:"size:100"`
	AgentID      int64     `json:"agent_id" gorm:"index"`
	AgentName    string    `json:"agent_name" gorm:"size:200"`
	Model        string    `json:"model" gorm:"size:100;index"`
	PromptTokens int64     `json:"prompt_tokens"`
	Completion   int64     `json:"completion_tokens" gorm:"column:completion"` // DB column is `completion`, not completion_tokens
	TotalTokens  int64     `json:"total_tokens"`
	Cost         float64   `json:"cost"`
	Date         string    `json:"date" gorm:"size:10;index"` // YYYY-MM-DD
	CreatedAt    time.Time `json:"created_at"`
}

// ChatGroup represents a group chat with multiple agents.
type ChatGroup struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	CreatedBy string    `json:"created_by" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is last activity: max(chat_sessions.updated_at) for this group, else CreatedAt.
	UpdatedAt time.Time         `json:"updated_at"`
	Members   []ChatGroupMember `json:"members" gorm:"foreignKey:GroupID"`
}

// ChatGroupMember represents an agent member in a group.
type ChatGroupMember struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	GroupID   int64     `json:"group_id" gorm:"index;not null"`
	AgentID   int64     `json:"agent_id" gorm:"index;not null"`
	AgentName string    `json:"agent_name,omitempty" gorm:"-"` // filled from agents join, not a DB column
	CreatedAt time.Time `json:"created_at"`
}
