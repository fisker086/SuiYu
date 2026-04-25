package agent

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
)

type AuditStore interface {
	CreateAuditLog(log *model.AuditLog) (*model.AuditLog, error)
}

type AuditLogger struct {
	store AuditStore
}

func NewAuditLogger(store AuditStore) *AuditLogger {
	return &AuditLogger{store: store}
}

func (a *AuditLogger) Log(ctx context.Context, log *model.AuditLog) {
	if a == nil || a.store == nil {
		return
	}
	go func() {
		if _, err := a.store.CreateAuditLog(log); err != nil {
			logger.Error("failed to create audit log", "err", err)
		}
	}()
}

func (a *AuditLogger) LogSync(ctx context.Context, log *model.AuditLog) {
	if a == nil || a.store == nil {
		return
	}
	if _, err := a.store.CreateAuditLog(log); err != nil {
		logger.Error("failed to create audit log", "err", err)
	}
}

func (a *AuditLogger) LogToolCall(agentID int64, sessionID, userID, toolName, action, riskLevel, input, output, errMsg, status string, durationMs int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.Log(ctx, &model.AuditLog{
		AgentID:    agentID,
		SessionID:  sessionID,
		UserID:     userID,
		ToolName:   toolName,
		Action:     action,
		RiskLevel:  riskLevel,
		Input:      truncateStr(input, 4000),
		Output:     truncateStr(output, 8000),
		Error:      truncateStr(errMsg, 4000),
		Status:     status,
		DurationMs: durationMs,
	})
}

// truncateStr limits length by **bytes** but never splits a UTF-8 code point (PostgreSQL TEXT rejects invalid UTF-8).
func truncateStr(s string, max int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "...[truncated]"
}
