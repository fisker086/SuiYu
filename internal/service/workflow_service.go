package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

type WorkflowService struct {
	store storage.Storage
}

func NewWorkflowService(store storage.Storage) *WorkflowService {
	return &WorkflowService{store: store}
}

func (s *WorkflowService) List(ctx context.Context) ([]schema.AgentWorkflow, error) {
	return s.store.ListWorkflows(ctx)
}

func (s *WorkflowService) Get(ctx context.Context, id int64) (*schema.AgentWorkflow, error) {
	return s.store.GetWorkflow(ctx, id)
}

func (s *WorkflowService) Create(ctx context.Context, req *schema.CreateWorkflowRequest) (*schema.AgentWorkflow, error) {
	if err := validateWorkflowKind(req.Kind); err != nil {
		return nil, err
	}
	if err := s.validateStepAgents(ctx, req.StepAgentIDs); err != nil {
		return nil, err
	}
	return s.store.CreateWorkflow(ctx, req)
}

func (s *WorkflowService) Update(ctx context.Context, id int64, req *schema.UpdateWorkflowRequest) (*schema.AgentWorkflow, error) {
	if req.Kind != "" {
		if err := validateWorkflowKind(req.Kind); err != nil {
			return nil, err
		}
	}
	if req.StepAgentIDs != nil {
		if err := s.validateStepAgents(ctx, req.StepAgentIDs); err != nil {
			return nil, err
		}
	}
	return s.store.UpdateWorkflow(ctx, id, req)
}

func (s *WorkflowService) Delete(ctx context.Context, id int64) error {
	return s.store.DeleteWorkflow(ctx, id)
}

func validateWorkflowKind(kind string) error {
	k := strings.TrimSpace(strings.ToLower(kind))
	switch k {
	case schema.WorkflowKindSingle, schema.WorkflowKindSequential, schema.WorkflowKindParallel,
		schema.WorkflowKindSupervisor, schema.WorkflowKindLoop:
		return nil
	default:
		return fmt.Errorf("unsupported workflow kind: %s", kind)
	}
}

func (s *WorkflowService) validateStepAgents(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("step_agent_ids must not be empty")
	}
	for _, id := range ids {
		if id < 1 {
			return fmt.Errorf("invalid agent id: %d", id)
		}
	}

	agents, err := s.store.ListAgents()
	if err != nil {
		return fmt.Errorf("failed to list agents for validation: %w", err)
	}
	agentMap := make(map[int64]bool, len(agents))
	for _, a := range agents {
		agentMap[a.ID] = true
	}

	for _, id := range ids {
		if !agentMap[id] {
			return fmt.Errorf("agent not found: %d", id)
		}
	}
	return nil
}

// GetForChat loads workflow and checks it is active (used by ChatService).
func (s *WorkflowService) GetForChat(ctx context.Context, id int64) (*schema.AgentWorkflow, error) {
	wf, err := s.store.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	if !wf.IsActive {
		return nil, fmt.Errorf("workflow is inactive")
	}
	return wf, nil
}
