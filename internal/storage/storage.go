package storage

import (
	"context"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
)

type Storage interface {
	ListAgents() ([]*schema.Agent, error)
	GetAgent(id int64) (*schema.AgentWithRuntime, error)
	GetAgentIDByName(ctx context.Context, name string) (int64, error)
	CreateAgent(agent *schema.CreateAgentRequest) (*schema.Agent, error)
	UpdateAgent(id int64, req *schema.UpdateAgentRequest) (*schema.Agent, error)
	DeleteAgent(id int64) error

	GetCapabilityTree(agentID int64) (*schema.CapabilityTree, error)
	UpdateCapabilityTree(agentID int64, nodes []schema.CapabilityTreeNode) (*schema.CapabilityTree, error)

	ListSkills() ([]*schema.Skill, error)
	GetSkill(id int64) (*schema.Skill, error)
	GetSkillByKey(key string) (*schema.Skill, error)
	CreateSkill(skill *schema.CreateSkillRequest) (*schema.Skill, error)
	UpsertSkill(skill *schema.CreateSkillRequest) (*schema.Skill, error)
	UpdateSkill(id int64, req *schema.UpdateSkillRequest) (*schema.Skill, error)
	DeleteSkill(id int64) error

	ListMCPConfigs() ([]*schema.MCPConfig, error)
	GetMCPConfig(id int64) (*schema.MCPConfig, error)
	CreateMCPConfig(cfg *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error)
	UpdateMCPConfig(id int64, cfg *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error)
	DeleteMCPConfig(id int64) error

	// Learnings
	CreateLearning(req *schema.CreateLearningRequest) (*schema.Learning, error)
	ListLearnings(userID *int64) ([]*schema.Learning, error)
	GetLearning(userID *int64, errorType string) (*schema.Learning, error)
	DeleteLearning(id int64) error
	SyncMCPServer(id int64, req *schema.SyncMCPServerRequest) error
	ListMCPTools(configID int64) ([]schema.MCPServer, error)

	StoreMemory(ctx context.Context, agentID int64, userID, sessionID, role, content string, embedding []float32, extra map[string]any) error
	SearchMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.AgentMemory, error)
	StoreSemanticMemory(ctx context.Context, agentID int64, userID, content string, metadata map[string]any, embedding []float32) error
	SearchSemanticMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.SemanticMemory, error)

	GetUserProfile(ctx context.Context, userID string, agentID int64) (*model.UserProfile, error)
	UpsertUserProfile(ctx context.Context, userID string, agentID int64, profile map[string]any, embedding []float32) error
	SearchUserProfile(ctx context.Context, userID string, agentID int64, embedding []float32) (*model.UserProfile, error)

	// RBAC
	ListRoles() ([]model.Role, error)
	GetRole(id int64) (*model.Role, error)
	CreateRole(role *model.Role) (*model.Role, error)
	UpdateRole(id int64, role *model.Role) (*model.Role, error)
	DeleteRole(id int64) error
	GetRoleAgentPermissions(ctx context.Context, roleID int64) (map[int64]bool, error)
	SetRoleAgentPermissions(ctx context.Context, roleID int64, agentIDs []int64) error

	GetUserAgentPermissions(ctx context.Context, userID int64) (map[int64]bool, error)

	ListUserRoles(userID int64) ([]model.UserRole, error)
	GetUserRolesByRoleID(ctx context.Context, roleID int64) ([]model.UserRole, error)
	AssignRole(userID int64, roleID int64) error
	RevokeRole(userID int64, roleID int64) error

	CreateChatSession(ctx context.Context, agentID int64, userID string, groupID int64) (*schema.ChatSession, error)
	GetChatSession(ctx context.Context, sessionID string) (*schema.ChatSession, error)
	UpdateChatSessionTitle(ctx context.Context, sessionID, userID, title string) error
	DeleteChatSession(ctx context.Context, sessionID string) error
	ListChatSessions(ctx context.Context, agentID int64, userID string, limit, offset int) ([]schema.ChatSession, error)
	// ListRecentSessionMessages returns up to limit most recent messages (chronological ASC).
	ListRecentSessionMessages(ctx context.Context, sessionID string, limit int) ([]schema.ChatHistoryMessage, error)
	// ListSessionMessagesPage returns messages in created_at order with OFFSET/LIMIT (full-history paging).
	ListSessionMessagesPage(ctx context.Context, sessionID string, offset, limit int) ([]schema.ChatHistoryMessage, error)

	ListWorkflows(ctx context.Context) ([]schema.AgentWorkflow, error)
	GetWorkflow(ctx context.Context, id int64) (*schema.AgentWorkflow, error)
	GetWorkflowByKey(ctx context.Context, key string) (*schema.AgentWorkflow, error)
	CreateWorkflow(ctx context.Context, req *schema.CreateWorkflowRequest) (*schema.AgentWorkflow, error)
	UpdateWorkflow(ctx context.Context, id int64, req *schema.UpdateWorkflowRequest) (*schema.AgentWorkflow, error)
	DeleteWorkflow(ctx context.Context, id int64) error

	ListChannels() ([]*model.Channel, error)
	GetChannel(id int64) (*model.Channel, error)
	CreateChannel(req *schema.CreateChannelRequest) (*model.Channel, error)
	UpdateChannel(id int64, req *schema.UpdateChannelRequest) (*model.Channel, error)
	DeleteChannel(id int64) error

	CreateMessage(ctx context.Context, msg *model.AgentMessage) (*model.AgentMessage, error)
	ListMessages(ctx context.Context, req *schema.ListMessagesRequest) ([]*model.AgentMessage, int64, error)
	UpdateMessageStatus(ctx context.Context, id int64, status string) error
	CreateMessageChannel(ctx context.Context, req *schema.CreateMessageChannelRequest) (*model.MessageChannel, error)
	GetMessageChannel(ctx context.Context, id int64) (*model.MessageChannel, error)
	ListMessageChannels(ctx context.Context, agentID int64) ([]*model.MessageChannel, error)
	UpdateMessageChannel(ctx context.Context, id int64, req *schema.UpdateMessageChannelRequest) (*model.MessageChannel, error)
	DeleteMessageChannel(ctx context.Context, id int64) error
	CreateA2ACard(ctx context.Context, req *schema.CreateA2ACardRequest) (*model.A2ACard, error)
	ListA2ACards(ctx context.Context, agentID int64) ([]*model.A2ACard, error)
	GetA2ACard(ctx context.Context, id int64) (*model.A2ACard, error)
	DeleteA2ACard(ctx context.Context, id int64) error

	GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error)
	GetWorkflowDefinitionByKey(ctx context.Context, key string) (*model.WorkflowDefinition, error)
	ListWorkflowDefinitions(ctx context.Context) ([]*model.WorkflowDefinition, error)
	CreateWorkflowDefinition(ctx context.Context, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error)
	UpdateWorkflowDefinition(ctx context.Context, id int64, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error)
	DeleteWorkflowDefinition(ctx context.Context, id int64) error

	CreateWorkflowExecution(ctx context.Context, exec *model.WorkflowExecution) (*model.WorkflowExecution, error)
	UpdateWorkflowExecution(ctx context.Context, id int64, exec *model.WorkflowExecution) error
	ListWorkflowExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error)
	GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error)

	GetChatStats(ctx context.Context, userID string, isAdmin bool) (map[string]int64, error)
	GetRecentChats(ctx context.Context, userID string, limit int) ([]map[string]any, error)
	GetChatActivity(ctx context.Context, userID string, days int) ([]map[string]any, error)

	// Schedules
	ListSchedules() ([]*model.Schedule, error)
	GetSchedule(id int64) (*model.Schedule, error)
	CreateSchedule(schedule *model.Schedule) (*model.Schedule, error)
	UpdateSchedule(id int64, schedule *model.Schedule) (*model.Schedule, error)
	UpdateScheduleChatSessionID(id int64, chatSessionID string) error
	DeleteSchedule(id int64) error

	ListScheduleExecutions(scheduleID int64, limit int) ([]*model.ScheduleExecution, error)
	CreateScheduleExecution(exec *model.ScheduleExecution) (*model.ScheduleExecution, error)
	UpdateScheduleExecution(id int64, status string, result, err string, durationMs int64) error

	CreateAuditLog(log *model.AuditLog) (*model.AuditLog, error)
	ListAuditLogs(filter *AuditLogFilter) ([]*model.AuditLog, int64, error)
	GetAuditLog(id int64) (*model.AuditLog, error)
	CountAuditLogs(filter *AuditLogFilter) (int64, error)
	DeleteAuditLogs(filter *AuditLogFilter) (int64, error)

	CreateApprovalRequest(req *model.ApprovalRequest) (*model.ApprovalRequest, error)
	ListApprovalRequests(filter *ApprovalRequestFilter) ([]*model.ApprovalRequest, int64, error)
	GetApprovalRequest(id int64) (*model.ApprovalRequest, error)
	UpdateApprovalRequest(id int64, status, approverID, comment string) error

	// Chat Groups
	CreateChatGroup(ctx context.Context, req *schema.CreateGroupRequest, userID string) (*model.ChatGroup, error)
	GetChatGroup(ctx context.Context, id int64) (*model.ChatGroup, error)
	ListChatGroups(ctx context.Context, userID string) ([]*model.ChatGroup, error)
	UpdateChatGroup(ctx context.Context, id int64, req *schema.UpdateGroupRequest) (*model.ChatGroup, error)
	DeleteChatGroup(ctx context.Context, id int64) error
}

type AuditLogFilter struct {
	UserID    string
	AgentID   int64
	SessionID string
	ToolName  string
	RiskLevel string
	Status    string
	Page      int
	PageSize  int
}

type ApprovalRequestFilter struct {
	AgentID    int64
	SessionID  string
	Status     string
	ExternalID string
	UserID     string
	Page       int
	PageSize   int
}
