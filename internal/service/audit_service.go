package service

import (
	"strconv"
	"strings"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

type AuditService struct {
	store  AuditStore
	users  UserLookup
	agents AgentLookup
}

// UserLookup resolves numeric user_id strings to display names (optional).
type UserLookup interface {
	GetUserByID(id int64) (*model.User, error)
}

// AgentLookup resolves agent_id to display name (optional).
type AgentLookup interface {
	GetAgent(id int64) (*schema.AgentWithRuntime, error)
}

type AuditStore interface {
	CreateAuditLog(log *model.AuditLog) (*model.AuditLog, error)
	ListAuditLogs(filter *storage.AuditLogFilter) ([]*model.AuditLog, int64, error)
	GetAuditLog(id int64) (*model.AuditLog, error)
	CountAuditLogs(filter *storage.AuditLogFilter) (int64, error)
	DeleteAuditLogs(filter *storage.AuditLogFilter) (int64, error)
}

func NewAuditService(store AuditStore, users UserLookup, agents AgentLookup) *AuditService {
	return &AuditService{store: store, users: users, agents: agents}
}

func (s *AuditService) CreateLog(req *model.AuditLog) (*schema.AuditLog, error) {
	log, err := s.store.CreateAuditLog(req)
	if err != nil {
		return nil, err
	}
	out := toSchemaAuditLog(log)
	s.enrichUsernames([]*schema.AuditLog{out})
	s.enrichAgentNames([]*schema.AuditLog{out})
	return out, nil
}

func (s *AuditService) ListLogs(filter *storage.AuditLogFilter) ([]*schema.AuditLog, int64, error) {
	logs, total, err := s.store.ListAuditLogs(filter)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*schema.AuditLog, 0, len(logs))
	for _, log := range logs {
		result = append(result, toSchemaAuditLog(log))
	}
	s.enrichUsernames(result)
	s.enrichAgentNames(result)
	return result, total, nil
}

func (s *AuditService) GetLog(id int64) (*schema.AuditLog, error) {
	log, err := s.store.GetAuditLog(id)
	if err != nil {
		return nil, err
	}
	out := toSchemaAuditLog(log)
	s.enrichUsernames([]*schema.AuditLog{out})
	s.enrichAgentNames([]*schema.AuditLog{out})
	return out, nil
}

func (s *AuditService) CountLogs(filter *storage.AuditLogFilter) (int64, error) {
	return s.store.CountAuditLogs(filter)
}

func (s *AuditService) DeleteLogs(filter *storage.AuditLogFilter) (int64, error) {
	return s.store.DeleteAuditLogs(filter)
}

func (s *AuditService) enrichUsernames(logs []*schema.AuditLog) {
	if s.users == nil || len(logs) == 0 {
		return
	}
	ids := make(map[int64]struct{})
	for _, log := range logs {
		if log == nil {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(log.UserID), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		ids[id] = struct{}{}
	}
	if len(ids) == 0 {
		return
	}
	names := make(map[int64]string, len(ids))
	for id := range ids {
		u, err := s.users.GetUserByID(id)
		if err != nil || u == nil {
			continue
		}
		if strings.TrimSpace(u.Username) != "" {
			names[id] = strings.TrimSpace(u.Username)
		}
	}
	for _, log := range logs {
		if log == nil {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(log.UserID), 10, 64)
		if err != nil {
			continue
		}
		if n, ok := names[id]; ok {
			log.Username = n
		}
	}
}

func (s *AuditService) enrichAgentNames(logs []*schema.AuditLog) {
	if s.agents == nil || len(logs) == 0 {
		return
	}
	ids := make(map[int64]struct{})
	for _, log := range logs {
		if log == nil || log.AgentID <= 0 {
			continue
		}
		ids[log.AgentID] = struct{}{}
	}
	if len(ids) == 0 {
		return
	}
	names := make(map[int64]string, len(ids))
	for id := range ids {
		ag, err := s.agents.GetAgent(id)
		if err != nil || ag == nil {
			continue
		}
		if strings.TrimSpace(ag.Name) != "" {
			names[id] = strings.TrimSpace(ag.Name)
		}
	}
	for _, log := range logs {
		if log == nil || log.AgentID <= 0 {
			continue
		}
		if n, ok := names[log.AgentID]; ok {
			log.AgentName = n
		}
	}
}

func toSchemaAuditLog(m *model.AuditLog) *schema.AuditLog {
	return &schema.AuditLog{
		ID:         m.ID,
		UserID:     m.UserID,
		AgentID:    m.AgentID,
		SessionID:  m.SessionID,
		ToolName:   m.ToolName,
		Action:     m.Action,
		RiskLevel:  m.RiskLevel,
		Input:      m.Input,
		Output:     m.Output,
		Error:      m.Error,
		Status:     m.Status,
		DurationMs: m.DurationMs,
		IPAddress:  m.IPAddress,
		CreatedAt:  m.CreatedAt,
	}
}
