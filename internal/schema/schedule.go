package schema

import "time"

type Schedule struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	AgentID       int64     `json:"agent_id,omitempty"`
	WorkflowID    int64     `json:"workflow_id,omitempty"`
	WorkflowName  string    `json:"workflow_name,omitempty"`
	AgentName     string    `json:"agent_name,omitempty"`
	ChannelID     *int64    `json:"channel_id,omitempty"`
	ChannelName   string    `json:"channel_name,omitempty"`
	ScheduleKind  string    `json:"schedule_kind"`
	CronExpr      string    `json:"cron_expr,omitempty"`
	At            string    `json:"at,omitempty"`
	EveryMs       int64     `json:"every_ms,omitempty"`
	Timezone      string    `json:"timezone,omitempty"`
	WakeMode      string    `json:"wake_mode"`
	SessionTarget string    `json:"session_target"`
	ChatSessionID string    `json:"chat_session_id,omitempty"`
	Prompt        string    `json:"prompt"`
	CodeLanguage  string    `json:"code_language,omitempty"` // python, shell, javascript
	Enabled       bool      `json:"enabled"`
	StaggerMs     int64     `json:"stagger_ms"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateScheduleRequest struct {
	Name          string `json:"name" validate:"required,min=1,max=200"`
	Description   string `json:"description"`
	AgentID       *int64 `json:"agent_id,omitempty"`
	WorkflowID    *int64 `json:"workflow_id,omitempty"`
	ChannelID     *int64 `json:"channel_id,omitempty"`
	ScheduleKind  string `json:"schedule_kind" validate:"required,oneof=at every cron"`
	CronExpr      string `json:"cron_expr"`
	At            string `json:"at"`
	EveryMs       int64  `json:"every_ms"`
	Timezone      string `json:"timezone"`
	WakeMode      string `json:"wake_mode" validate:"oneof=now next_heartbeat"`
	SessionTarget string `json:"session_target" validate:"oneof=main isolated"`
	Prompt        string `json:"prompt"`
	CodeLanguage  string `json:"code_language,omitempty" validate:"oneof=python shell javascript"`
	StaggerMs     int64  `json:"stagger_ms"`
	Enabled       bool   `json:"enabled"`
}

type UpdateScheduleRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	AgentID       *int64  `json:"agent_id,omitempty"`
	WorkflowID    *int64  `json:"workflow_id,omitempty"`
	ChannelID     *int64  `json:"channel_id,omitempty"`
	ScheduleKind  *string `json:"schedule_kind,omitempty"`
	CronExpr      *string `json:"cron_expr,omitempty"`
	At            *string `json:"at,omitempty"`
	EveryMs       *int64  `json:"every_ms,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
	WakeMode      *string `json:"wake_mode,omitempty"`
	SessionTarget *string `json:"session_target,omitempty"`
	Prompt        *string `json:"prompt,omitempty"`
	CodeLanguage  *string `json:"code_language,omitempty"`
	StaggerMs     *int64  `json:"stagger_ms,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

type ScheduleExecution struct {
	ID         int64      `json:"id"`
	ScheduleID int64      `json:"schedule_id"`
	Status     string     `json:"status"`
	Result     string     `json:"result,omitempty"`
	Error      string     `json:"error,omitempty"`
	DurationMs int64      `json:"duration_ms"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}
