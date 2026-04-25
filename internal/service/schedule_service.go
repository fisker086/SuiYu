package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
)

type ScheduleService struct {
	store         ScheduleStore
	scheduler     SchedulerWrapper
	workflowStore WorkflowStore
	agentStore    AgentStore
}

func NewScheduleService(store ScheduleStore, scheduler SchedulerWrapper, workflowStore WorkflowStore, agentStore AgentStore) *ScheduleService {
	return &ScheduleService{store: store, scheduler: scheduler, workflowStore: workflowStore, agentStore: agentStore}
}

type SchedulerWrapper interface {
	Start() error
	Stop() error
	AddOrUpdateSchedule(schedule *model.Schedule) error
	RemoveSchedule(scheduleID int64) error
	TriggerSchedule(ctx context.Context, scheduleID int64, triggerUserID string) error
}

type ScheduleStore interface {
	ListSchedules() ([]*model.Schedule, error)
	GetSchedule(id int64) (*model.Schedule, error)
	CreateSchedule(schedule *model.Schedule) (*model.Schedule, error)
	UpdateSchedule(id int64, schedule *model.Schedule) (*model.Schedule, error)
	DeleteSchedule(id int64) error
	ListScheduleExecutions(scheduleID int64, limit int) ([]*model.ScheduleExecution, error)
	CreateScheduleExecution(exec *model.ScheduleExecution) (*model.ScheduleExecution, error)
	UpdateScheduleExecution(id int64, status string, result, err string, durationMs int64) error
	GetChannel(id int64) (*model.Channel, error)
}

type WorkflowStore interface {
	GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error)
}

type AgentStore interface {
	GetAgent(id int64) (*schema.AgentWithRuntime, error)
}

func (s *ScheduleService) ListSchedules() ([]*schema.Schedule, error) {
	schedules, err := s.store.ListSchedules()
	if err != nil {
		return nil, err
	}
	result := make([]*schema.Schedule, 0, len(schedules))
	for _, sch := range schedules {
		out := toSchemaSchedule(sch)
		if sch.ChannelID != nil && *sch.ChannelID > 0 {
			if ch, err := s.store.GetChannel(*sch.ChannelID); err == nil && ch != nil {
				out.ChannelName = ch.Name
			}
		}
		if sch.WorkflowID > 0 && s.workflowStore != nil {
			if wf, err := s.workflowStore.GetWorkflowDefinition(context.Background(), sch.WorkflowID); err == nil && wf != nil {
				out.WorkflowName = wf.Name
			}
		}
		result = append(result, out)
	}
	return result, nil
}

func (s *ScheduleService) GetSchedule(id int64) (*schema.Schedule, error) {
	schedule, err := s.store.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	out := toSchemaSchedule(schedule)
	if schedule.ChannelID != nil && *schedule.ChannelID > 0 {
		if ch, err := s.store.GetChannel(*schedule.ChannelID); err == nil && ch != nil {
			out.ChannelName = ch.Name
		}
	}
	if schedule.WorkflowID > 0 && s.workflowStore != nil {
		if wf, err := s.workflowStore.GetWorkflowDefinition(context.Background(), schedule.WorkflowID); err == nil && wf != nil {
			out.WorkflowName = wf.Name
		}
	}
	return out, nil
}

// ownerUserID is the same string as chat_sessions.user_id (e.g. decimal DB user id); empty when unauthenticated or no user store.
func (s *ScheduleService) CreateSchedule(req *schema.CreateScheduleRequest, ownerUserID string) (*schema.Schedule, error) {
	hasAgent := req.AgentID != nil && *req.AgentID > 0
	hasWorkflow := req.WorkflowID != nil && *req.WorkflowID > 0
	hasCode := req.CodeLanguage != ""

	if !hasAgent && !hasWorkflow && !hasCode {
		return nil, fmt.Errorf("either agent_id, workflow_id, or code_language is required")
	}
	if (hasAgent && hasWorkflow) || (hasAgent && hasCode) || (hasWorkflow && hasCode) {
		return nil, fmt.Errorf("specify only one of agent_id, workflow_id, or code_language")
	}

	schedule := &model.Schedule{
		Name:          req.Name,
		Description:   req.Description,
		ScheduleKind:  req.ScheduleKind,
		CronExpr:      req.CronExpr,
		At:            req.At,
		EveryMs:       req.EveryMs,
		Timezone:      req.Timezone,
		WakeMode:      req.WakeMode,
		SessionTarget: req.SessionTarget,
		Prompt:        req.Prompt,
		CodeLanguage:  req.CodeLanguage,
		StaggerMs:     req.StaggerMs,
		Enabled:       req.Enabled,
		OwnerUserID:   strings.TrimSpace(ownerUserID),
	}
	if req.AgentID != nil {
		schedule.AgentID = *req.AgentID
	}
	if req.WorkflowID != nil {
		schedule.WorkflowID = *req.WorkflowID
	}
	if req.ChannelID != nil && *req.ChannelID > 0 {
		v := *req.ChannelID
		schedule.ChannelID = &v
	}

	if schedule.WakeMode == "" {
		schedule.WakeMode = "now"
	}
	if schedule.SessionTarget == "" {
		schedule.SessionTarget = "main"
	}
	if !schedule.Enabled {
		schedule.Enabled = true
	}

	created, err := s.store.CreateSchedule(schedule)
	if err != nil {
		return nil, err
	}

	if s.scheduler != nil && created.Enabled {
		if err := s.scheduler.AddOrUpdateSchedule(created); err != nil {
			logger.Warn("scheduler sync failed after create schedule", "schedule_id", created.ID, "err", err)
		}
	}

	return toSchemaSchedule(created), nil
}

// ownerUserID is the same as CreateSchedule (JWT user id string); when non-empty, persists schedules.owner_user_id so isolated 等模式能关联会话与审计。
func (s *ScheduleService) UpdateSchedule(id int64, req *schema.UpdateScheduleRequest, ownerUserID string) (*schema.Schedule, error) {
	existing, err := s.store.GetSchedule(id)
	if err != nil {
		return nil, err
	}

	if ou := strings.TrimSpace(ownerUserID); ou != "" {
		existing.OwnerUserID = ou
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.AgentID != nil && *req.AgentID > 0 {
		existing.AgentID = *req.AgentID
	}
	if req.WorkflowID != nil && *req.WorkflowID > 0 {
		existing.WorkflowID = *req.WorkflowID
	}
	if req.ScheduleKind != nil {
		existing.ScheduleKind = *req.ScheduleKind
	}
	if req.CronExpr != nil {
		existing.CronExpr = *req.CronExpr
	}
	if req.At != nil {
		existing.At = *req.At
	}
	if req.EveryMs != nil {
		existing.EveryMs = *req.EveryMs
	}
	if req.Timezone != nil {
		existing.Timezone = *req.Timezone
	}
	if req.WakeMode != nil {
		existing.WakeMode = *req.WakeMode
	}
	if req.SessionTarget != nil {
		existing.SessionTarget = *req.SessionTarget
	}
	if req.Prompt != nil {
		existing.Prompt = *req.Prompt
	}
	if req.CodeLanguage != nil {
		existing.CodeLanguage = *req.CodeLanguage
	}
	if req.StaggerMs != nil {
		existing.StaggerMs = *req.StaggerMs
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.ChannelID != nil {
		if *req.ChannelID == 0 {
			existing.ChannelID = nil
		} else {
			v := *req.ChannelID
			existing.ChannelID = &v
		}
	}

	updated, err := s.store.UpdateSchedule(id, existing)
	if err != nil {
		return nil, err
	}

	if s.scheduler != nil {
		if err := s.scheduler.AddOrUpdateSchedule(updated); err != nil {
			logger.Warn("scheduler sync failed after update schedule", "schedule_id", updated.ID, "err", err)
		}
	}

	return toSchemaSchedule(updated), nil
}

func (s *ScheduleService) DeleteSchedule(id int64) error {
	if s.scheduler != nil {
		s.scheduler.RemoveSchedule(id)
	}
	return s.store.DeleteSchedule(id)
}

// triggerUserID is JWT user id when calling POST .../trigger; empty for cron so runtime uses schedule.OwnerUserID.
func (s *ScheduleService) TriggerSchedule(ctx context.Context, scheduleID int64, triggerUserID string) error {
	if s.scheduler == nil {
		return nil
	}
	return s.scheduler.TriggerSchedule(ctx, scheduleID, triggerUserID)
}

func (s *ScheduleService) ListExecutions(scheduleID int64, limit int) ([]*schema.ScheduleExecution, error) {
	execs, err := s.store.ListScheduleExecutions(scheduleID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]*schema.ScheduleExecution, 0, len(execs))
	for _, e := range execs {
		result = append(result, toSchemaExecution(e))
	}
	return result, nil
}

func (s *ScheduleService) StartScheduler() error {
	if s.scheduler == nil {
		return nil
	}
	return s.scheduler.Start()
}

func (s *ScheduleService) StopScheduler() error {
	if s.scheduler == nil {
		return nil
	}
	return s.scheduler.Stop()
}

func toSchemaSchedule(m *model.Schedule) *schema.Schedule {
	out := &schema.Schedule{
		ID:            m.ID,
		Name:          m.Name,
		Description:   m.Description,
		AgentID:       m.AgentID,
		WorkflowID:    m.WorkflowID,
		ChannelID:     m.ChannelID,
		ScheduleKind:  m.ScheduleKind,
		CronExpr:      m.CronExpr,
		At:            m.At,
		EveryMs:       m.EveryMs,
		Timezone:      m.Timezone,
		WakeMode:      m.WakeMode,
		SessionTarget: m.SessionTarget,
		ChatSessionID: m.ChatSessionID,
		Prompt:        m.Prompt,
		CodeLanguage:  m.CodeLanguage,
		Enabled:       m.Enabled,
		StaggerMs:     m.StaggerMs,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	return out
}

func toSchemaExecution(m *model.ScheduleExecution) *schema.ScheduleExecution {
	return &schema.ScheduleExecution{
		ID:         m.ID,
		ScheduleID: m.ScheduleID,
		Status:     m.Status,
		Result:     m.Result,
		Error:      m.Error,
		DurationMs: m.DurationMs,
		StartedAt:  m.StartedAt,
		FinishedAt: m.FinishedAt,
	}
}
