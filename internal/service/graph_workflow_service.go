package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/workflow"
)

type GraphWorkflowService struct {
	engine      *workflow.GraphEngine
	store       GraphWorkflowStore
	agentStore  AgentStore
	auditLogger *agent.AuditLogger
}

type GraphWorkflowStore interface {
	GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error)
	GetWorkflowDefinitionByKey(ctx context.Context, key string) (*model.WorkflowDefinition, error)
	ListWorkflowDefinitions(ctx context.Context) ([]*model.WorkflowDefinition, error)
	CreateWorkflowDefinition(ctx context.Context, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error)
	UpdateWorkflowDefinition(ctx context.Context, id int64, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error)
	DeleteWorkflowDefinition(ctx context.Context, id int64) error
}

func NewGraphWorkflowService(engine *workflow.GraphEngine, store GraphWorkflowStore, agentStore AgentStore, auditLogger *agent.AuditLogger) *GraphWorkflowService {
	return &GraphWorkflowService{engine: engine, store: store, agentStore: agentStore, auditLogger: auditLogger}
}

func (s *GraphWorkflowService) ListDefinitions(ctx context.Context) ([]*schema.WorkflowDefinitionPublic, error) {
	defs, err := s.store.ListWorkflowDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*schema.WorkflowDefinitionPublic, 0, len(defs))
	for _, def := range defs {
		if s.agentStore != nil {
			canUse, err := s.canWorkflowBeUsedInSchedule(def)
			if err != nil || !canUse {
				continue
			}
		}
		result = append(result, s.toPublic(def))
	}
	return result, nil
}

func (s *GraphWorkflowService) canWorkflowBeUsedInSchedule(def *model.WorkflowDefinition) (bool, error) {
	for _, node := range def.Nodes {
		if node.AgentID != nil {
			agent, err := s.agentStore.GetAgent(*node.AgentID)
			if err != nil {
				continue
			}
			if agent.RuntimeProfile != nil && agent.RuntimeProfile.ExecutionMode == schema.ExecutionModeClient {
				return false, nil
			}
		}
	}
	return true, nil
}

func (s *GraphWorkflowService) GetDefinition(ctx context.Context, id int64) (*schema.WorkflowDefinitionPublic, error) {
	def, err := s.store.GetWorkflowDefinition(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toPublic(def), nil
}

func (s *GraphWorkflowService) GetDefinitionByKey(ctx context.Context, key string) (*schema.WorkflowDefinitionPublic, error) {
	def, err := s.store.GetWorkflowDefinitionByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.toPublic(def), nil
}

func (s *GraphWorkflowService) CreateDefinition(ctx context.Context, req *schema.CreateWorkflowDefinitionRequest) (*schema.WorkflowDefinitionPublic, error) {
	def := &model.WorkflowDefinition{
		Key:          req.Key,
		Name:         req.Name,
		Description:  req.Description,
		Kind:         req.Kind,
		Nodes:        nodesToModel(req.Nodes),
		Edges:        edgesToModel(req.Edges),
		Variables:    req.Variables,
		InputSchema:  req.InputSchema,
		OutputSchema: req.OutputSchema,
		IsActive:     true,
	}
	if req.IsActive != nil {
		def.IsActive = *req.IsActive
	}

	created, err := s.store.CreateWorkflowDefinition(ctx, def)
	if err != nil {
		return nil, err
	}
	return s.toPublic(created), nil
}

func (s *GraphWorkflowService) UpdateDefinition(ctx context.Context, id int64, req *schema.UpdateWorkflowDefinitionRequest) (*schema.WorkflowDefinitionPublic, error) {
	existing, err := s.store.GetWorkflowDefinition(ctx, id)
	if err != nil {
		return nil, err
	}

	def := &model.WorkflowDefinition{
		Key:          existing.Key,
		Name:         req.Name,
		Description:  req.Description,
		Kind:         req.Kind,
		Nodes:        nodesToModel(req.Nodes),
		Edges:        edgesToModel(req.Edges),
		Variables:    req.Variables,
		InputSchema:  req.InputSchema,
		OutputSchema: req.OutputSchema,
		Version:      existing.Version + 1,
		IsActive:     existing.IsActive,
	}
	if req.IsActive != nil {
		def.IsActive = *req.IsActive
	}

	updated, err := s.store.UpdateWorkflowDefinition(ctx, id, def)
	if err != nil {
		return nil, err
	}
	return s.toPublic(updated), nil
}

func (s *GraphWorkflowService) DeleteDefinition(ctx context.Context, id int64) error {
	return s.store.DeleteWorkflowDefinition(ctx, id)
}

func (s *GraphWorkflowService) ListExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error) {
	return s.engine.ListExecutions(ctx, workflowID, limit)
}

func (s *GraphWorkflowService) GetExecution(ctx context.Context, execID int64) (*model.WorkflowExecution, error) {
	return s.engine.GetExecution(ctx, execID)
}

func (s *GraphWorkflowService) Execute(ctx context.Context, req *schema.ExecuteWorkflowRequest) (*schema.ExecuteWorkflowResponse, error) {
	start := time.Now()
	slog.Info("GraphWorkflowService.Execute called", "workflowID", req.WorkflowID, "message", req.Message, "variables", req.Variables)

	execStartTime := time.Now()
	if s.auditLogger != nil {
		s.auditLogger.LogToolCall(
			0,
			fmt.Sprintf("workflow_%d", req.WorkflowID),
			req.UserID,
			"workflow",
			"start",
			"low",
			req.Message,
			"",
			"",
			"running",
			0,
		)
	}

	result, err := s.engine.Execute(ctx, req.WorkflowID, req.Message, req.Variables)
	if err != nil {
		slog.Error("engine execution failed", "workflowID", req.WorkflowID, "error", err)
		if s.auditLogger != nil {
			s.auditLogger.LogToolCall(
				0,
				fmt.Sprintf("workflow_%d", req.WorkflowID),
				req.UserID,
				"workflow",
				"execute",
				"low",
				req.Message,
				"",
				err.Error(),
				"failed",
				time.Since(execStartTime).Milliseconds(),
			)
		}
		return nil, err
	}

	nodeResults := make(map[string]any)
	nodeOrder := make([]string, 0, len(result.NodeResults))
	for _, nr := range result.NodeResults {
		nodeOrder = append(nodeOrder, nr.NodeID)
		nodeResults[nr.NodeID] = map[string]any{
			"label":  nr.Label,
			"output": nr.Output,
			"error":  nr.Error,
		}
	}

	duration := time.Since(start).Milliseconds()
	slog.Info("engine execution completed", "workflowID", req.WorkflowID, "durationMS", duration)

	if s.auditLogger != nil {
		var output string
		if result.Output != nil {
			output = fmt.Sprintf("%v", result.Output)
		}
		s.auditLogger.LogToolCall(
			0,
			fmt.Sprintf("workflow_%d", req.WorkflowID),
			req.UserID,
			"workflow",
			"execute",
			"low",
			req.Message,
			output,
			"",
			"success",
			time.Since(execStartTime).Milliseconds(),
		)
	}

	return &schema.ExecuteWorkflowResponse{
		Output:          result.Output,
		NodeResult:      nodeResults,
		NodeResultOrder: nodeOrder,
		DurationMS:      duration,
	}, nil
}

func (s *GraphWorkflowService) toPublic(def *model.WorkflowDefinition) *schema.WorkflowDefinitionPublic {
	return &schema.WorkflowDefinitionPublic{
		ID:           def.ID,
		Key:          def.Key,
		Name:         def.Name,
		Description:  def.Description,
		Kind:         def.Kind,
		Nodes:        nodesToSchema(def.Nodes),
		Edges:        edgesToSchema(def.Edges),
		Variables:    def.Variables,
		InputSchema:  def.InputSchema,
		OutputSchema: def.OutputSchema,
		Version:      def.Version,
		IsActive:     def.IsActive,
		CreatedAt:    def.CreatedAt,
		UpdatedAt:    def.UpdatedAt,
	}
}

func nodesToModel(nodes []schema.WorkflowNode) []model.WorkflowNode {
	result := make([]model.WorkflowNode, len(nodes))
	for i, n := range nodes {
		result[i] = model.WorkflowNode{
			ID:           n.ID,
			Type:         n.Type,
			Label:        n.Label,
			AgentID:      n.AgentID,
			Config:       n.Config,
			Position:     (*model.NodePosition)(n.Position),
			InputSchema:  n.InputSchema,
			OutputSchema: n.OutputSchema,
		}
	}
	return result
}

func nodesToSchema(nodes []model.WorkflowNode) []schema.WorkflowNode {
	result := make([]schema.WorkflowNode, len(nodes))
	for i, n := range nodes {
		result[i] = schema.WorkflowNode{
			ID:           n.ID,
			Type:         n.Type,
			Label:        n.Label,
			AgentID:      n.AgentID,
			Config:       n.Config,
			Position:     (*schema.NodePosition)(n.Position),
			InputSchema:  n.InputSchema,
			OutputSchema: n.OutputSchema,
		}
	}
	return result
}

func edgesToModel(edges []schema.WorkflowEdge) []model.WorkflowEdge {
	result := make([]model.WorkflowEdge, len(edges))
	for i, e := range edges {
		result[i] = model.WorkflowEdge{
			ID:           e.ID,
			SourceNodeID: e.SourceNodeID,
			SourcePort:   e.SourcePort,
			TargetNodeID: e.TargetNodeID,
			TargetPort:   e.TargetPort,
			Condition:    e.Condition,
			Label:        e.Label,
		}
	}
	return result
}

func edgesToSchema(edges []model.WorkflowEdge) []schema.WorkflowEdge {
	result := make([]schema.WorkflowEdge, len(edges))
	for i, e := range edges {
		result[i] = schema.WorkflowEdge{
			ID:           e.ID,
			SourceNodeID: e.SourceNodeID,
			SourcePort:   e.SourcePort,
			TargetNodeID: e.TargetNodeID,
			TargetPort:   e.TargetPort,
			Condition:    e.Condition,
			Label:        e.Label,
		}
	}
	return result
}
