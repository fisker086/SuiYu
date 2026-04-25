package agent

import (
	"context"
	"time"

	agentSchema "github.com/fisk086/sya/internal/schema"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func normalizeToolToSkillKey(toolName string) string {
	if len(toolName) >= len("builtin_skill.") && toolName[:len("builtin_skill.")] == "builtin_skill." {
		return toolName
	}
	if len(toolName) >= len("builtin_") && toolName[:len("builtin_")] == "builtin_" {
		return "builtin_skill." + toolName[len("builtin_"):]
	}
	return toolName
}

func getToolRiskLevel(toolName string) string {
	skillKey := normalizeToolToSkillKey(toolName)
	if risk, ok := agentSchema.DefaultSkillRiskLevel[skillKey]; ok {
		return risk
	}
	return agentSchema.RiskLevelLow
}

func GetToolRiskLevel(toolName string) string {
	return getToolRiskLevel(toolName)
}

type auditHITLWrapper struct {
	inner           tool.InvokableTool
	toolName        string
	riskLevel       string
	agentID         int64
	sessionID       string
	userID          string
	approvalMode    string
	auditLogger     *AuditLogger
	approvalChecker func(agentID int64, sessionID, toolName, riskLevel, input string) (approved bool, approvalID int64, err error)
}

func (w *auditHITLWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w *auditHITLWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if w.needsApproval() {
		if w.approvalChecker != nil {
			approved, approvalID, err := w.approvalChecker(w.agentID, w.sessionID, w.toolName, w.riskLevel, argumentsInJSON)
			if err != nil {
				return w.recordAndReturn(ctx, argumentsInJSON, "", err.Error(), "blocked", 0, 0)
			}
			if !approved {
				return w.recordAndReturn(ctx, argumentsInJSON, "approval_pending", "正在审批中，请稍候...", "pending", 0, approvalID)
			}
		}
	}

	start := time.Now()
	result, err := w.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	durationMs := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}

	return w.recordAudit(ctx, argumentsInJSON, result, errMsg, status, durationMs)
}

func (w *auditHITLWrapper) needsApproval() bool {
	switch w.approvalMode {
	case "all":
		return true
	case "high_and_above":
		return w.riskLevel == "high" || w.riskLevel == "critical"
	default:
		return false
	}
}

func (w *auditHITLWrapper) recordAudit(ctx context.Context, input, output, errMsg, status string, durationMs int64) (string, error) {
	if w.auditLogger != nil {
		w.auditLogger.LogToolCall(
			w.agentID, w.sessionID, w.userID,
			w.toolName, "invoke", w.riskLevel,
			input, output, errMsg, status, durationMs,
		)
	}
	if errMsg != "" {
		return output, &toolExecutionError{Tool: w.toolName, Message: errMsg}
	}
	return output, nil
}

func (w *auditHITLWrapper) recordAndReturn(ctx context.Context, input, output, errMsg, status string, durationMs int64, approvalID int64) (string, error) {
	if w.auditLogger != nil {
		w.auditLogger.LogToolCall(
			w.agentID, w.sessionID, w.userID,
			w.toolName, "invoke", w.riskLevel,
			input, output, errMsg, status, durationMs,
		)
	}
	if status == "pending" && approvalID > 0 {
		return output, &approvalPendingError{
			toolExecutionError: toolExecutionError{Tool: w.toolName, Message: errMsg},
			ApprovalID:         approvalID,
		}
	}
	return output, &toolExecutionError{Tool: w.toolName, Message: errMsg}
}

type toolExecutionError struct {
	Tool    string
	Message string
}

func (e *toolExecutionError) Error() string {
	return "tool " + e.Tool + ": " + e.Message
}

type approvalPendingError struct {
	toolExecutionError
	ApprovalID int64
}

func (e *approvalPendingError) IsApprovalPending() bool {
	return true
}

func (e *approvalPendingError) Error() string {
	return e.Message
}

func WrapToolWithAuditHITL(t tool.InvokableTool, agentID int64, sessionID, userID, approvalMode string, auditLogger *AuditLogger, approvalChecker func(agentID int64, sessionID, toolName, riskLevel, input string) (bool, int64, error)) tool.BaseTool {
	info, err := t.Info(context.Background())
	if err != nil {
		return t
	}
	return &auditHITLWrapper{
		inner:           t,
		toolName:        info.Name,
		riskLevel:       getToolRiskLevel(info.Name),
		agentID:         agentID,
		sessionID:       sessionID,
		userID:          userID,
		approvalMode:    approvalMode,
		auditLogger:     auditLogger,
		approvalChecker: approvalChecker,
	}
}
