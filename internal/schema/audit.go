package schema

import "time"

// AuditLogCreateRequest is the body for POST /audit/logs (desktop / client audit events).
type AuditLogCreateRequest struct {
	AgentID    int64  `json:"agent_id"`
	SessionID  string `json:"session_id"`
	ToolName   string `json:"tool_name"`
	Action     string `json:"action"`
	RiskLevel  string `json:"risk_level"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
}

type AuditLog struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	AgentID    int64     `json:"agent_id"`
	AgentName  string    `json:"agent_name,omitempty"`
	SessionID  string    `json:"session_id"`
	ToolName   string    `json:"tool_name"`
	Action     string    `json:"action"`
	RiskLevel  string    `json:"risk_level"`
	Input      string    `json:"input"`
	Output     string    `json:"output"`
	Error      string    `json:"error"`
	Status     string    `json:"status"`
	DurationMs int64     `json:"duration_ms"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
}

type ApprovalRequest struct {
	ID         int64      `json:"id"`
	AgentID    int64      `json:"agent_id"`
	SessionID  string     `json:"session_id"`
	UserID     string     `json:"user_id"`
	ToolName   string     `json:"tool_name"`
	RiskLevel  string     `json:"risk_level"`
	Input      string     `json:"input"`
	Status     string     `json:"status"`
	ApproverID string     `json:"approver_id"`
	Comment    string     `json:"comment"`
	ApprovedAt *time.Time `json:"approved_at"`
	CreatedAt  time.Time  `json:"created_at"`

	ApprovalType  string     `json:"approval_type,omitempty"`
	ExternalID    string     `json:"external_id,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	NotifyWebhook string     `json:"notify_webhook,omitempty"`

	// Enriched from agent + viewer; not stored in approval_requests.
	AgentName           string   `json:"agent_name,omitempty"`
	DesignatedApprovers []string `json:"designated_approvers,omitempty"`
	CanApprove          bool     `json:"can_approve"`
}
