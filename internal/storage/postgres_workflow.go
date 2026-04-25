package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) ListWorkflows(ctx context.Context) ([]schema.AgentWorkflow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, name, description, kind, step_agent_ids, config, is_active, created_at, updated_at FROM agent_workflows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []schema.AgentWorkflow
	for rows.Next() {
		w, err := s.scanWorkflow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *w)
	}
	return list, rows.Err()
}

func (s *PostgresStorage) scanWorkflow(rows interface {
	Scan(dest ...any) error
}) (*schema.AgentWorkflow, error) {
	var w schema.AgentWorkflow
	var configJSON []byte
	err := rows.Scan(&w.ID, &w.Key, &w.Name, &w.Description, &w.Kind, &w.StepAgentIDs, &configJSON, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &w.Config); err != nil {
			w.Config = nil
		}
	}
	return &w, nil
}

func (s *PostgresStorage) GetWorkflow(ctx context.Context, id int64) (*schema.AgentWorkflow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, key, name, description, kind, step_agent_ids, config, is_active, created_at, updated_at FROM agent_workflows WHERE id = $1`, id)
	w, err := s.scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}
	return w, nil
}

func (s *PostgresStorage) GetWorkflowByKey(ctx context.Context, key string) (*schema.AgentWorkflow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, key, name, description, kind, step_agent_ids, config, is_active, created_at, updated_at FROM agent_workflows WHERE key = $1`, key)
	w, err := s.scanWorkflow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}
	return w, nil
}

func (s *PostgresStorage) CreateWorkflow(ctx context.Context, req *schema.CreateWorkflowRequest) (*schema.AgentWorkflow, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var w schema.AgentWorkflow
	var configOut []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO agent_workflows (key, name, description, kind, step_agent_ids, config, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, key, name, description, kind, step_agent_ids, config, is_active, created_at, updated_at`,
		req.Key, req.Name, req.Description, req.Kind, req.StepAgentIDs, configJSON, active,
	).Scan(&w.ID, &w.Key, &w.Name, &w.Description, &w.Kind, &w.StepAgentIDs, &configOut, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(configOut) > 0 {
		if err := json.Unmarshal(configOut, &w.Config); err != nil {
			w.Config = nil
		}
	}
	return &w, nil
}

func (s *PostgresStorage) UpdateWorkflow(ctx context.Context, id int64, req *schema.UpdateWorkflowRequest) (*schema.AgentWorkflow, error) {
	cur, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		cur.Name = req.Name
	}
	if req.Description != "" {
		cur.Description = req.Description
	}
	if req.Kind != "" {
		cur.Kind = req.Kind
	}
	if req.StepAgentIDs != nil {
		cur.StepAgentIDs = req.StepAgentIDs
	}
	if req.Config != nil {
		cur.Config = req.Config
	}
	if req.IsActive != nil {
		cur.IsActive = *req.IsActive
	}
	configJSON, err := json.Marshal(cur.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE agent_workflows SET name = $1, description = $2, kind = $3, step_agent_ids = $4, config = $5, is_active = $6, updated_at = NOW() WHERE id = $7`,
		cur.Name, cur.Description, cur.Kind, cur.StepAgentIDs, configJSON, cur.IsActive, id)
	if err != nil {
		return nil, err
	}
	return s.GetWorkflow(ctx, id)
}

func (s *PostgresStorage) DeleteWorkflow(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM agent_workflows WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

func (s *PostgresStorage) GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error) {
	var def model.WorkflowDefinition
	err := s.pool.QueryRow(ctx,
		`SELECT id, key, name, description, kind, nodes, edges, variables, input_schema, output_schema, version, is_active, created_at, updated_at 
		 FROM workflow_definitions WHERE id = $1`,
		id,
	).Scan(&def.ID, &def.Key, &def.Name, &def.Description, &def.Kind, &def.Nodes, &def.Edges, &def.Variables, &def.InputSchema, &def.OutputSchema, &def.Version, &def.IsActive, &def.CreatedAt, &def.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func (s *PostgresStorage) GetWorkflowDefinitionByKey(ctx context.Context, key string) (*model.WorkflowDefinition, error) {
	var def model.WorkflowDefinition
	err := s.pool.QueryRow(ctx,
		`SELECT id, key, name, description, kind, nodes, edges, variables, input_schema, output_schema, version, is_active, created_at, updated_at 
		 FROM workflow_definitions WHERE key = $1`,
		key,
	).Scan(&def.ID, &def.Key, &def.Name, &def.Description, &def.Kind, &def.Nodes, &def.Edges, &def.Variables, &def.InputSchema, &def.OutputSchema, &def.Version, &def.IsActive, &def.CreatedAt, &def.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func (s *PostgresStorage) ListWorkflowDefinitions(ctx context.Context) ([]*model.WorkflowDefinition, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, name, description, kind, nodes, edges, variables, input_schema, output_schema, version, is_active, created_at, updated_at 
		 FROM workflow_definitions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.WorkflowDefinition
	for rows.Next() {
		var def model.WorkflowDefinition
		if err := rows.Scan(&def.ID, &def.Key, &def.Name, &def.Description, &def.Kind, &def.Nodes, &def.Edges, &def.Variables, &def.InputSchema, &def.OutputSchema, &def.Version, &def.IsActive, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &def)
	}
	return list, nil
}

func (s *PostgresStorage) CreateWorkflowDefinition(ctx context.Context, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO workflow_definitions (key, name, description, kind, nodes, edges, variables, input_schema, output_schema, version, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at, updated_at`,
		def.Key, def.Name, def.Description, def.Kind, def.Nodes, def.Edges, def.Variables, def.InputSchema, def.OutputSchema, def.Version, def.IsActive,
	).Scan(&id, &def.CreatedAt, &def.UpdatedAt)
	if err != nil {
		return nil, err
	}
	def.ID = id
	return def, nil
}

func (s *PostgresStorage) UpdateWorkflowDefinition(ctx context.Context, id int64, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error) {
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_definitions SET key = $1, name = $2, description = $3, kind = $4, nodes = $5, edges = $6, variables = $7, 
		 input_schema = $8, output_schema = $9, version = $10, is_active = $11, updated_at = NOW() WHERE id = $12`,
		def.Key, def.Name, def.Description, def.Kind, def.Nodes, def.Edges, def.Variables, def.InputSchema, def.OutputSchema, def.Version, def.IsActive, id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetWorkflowDefinition(ctx, id)
}

func (s *PostgresStorage) DeleteWorkflowDefinition(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM workflow_definitions WHERE id = $1", id)
	return err
}

func (s *PostgresStorage) CreateWorkflowExecution(ctx context.Context, exec *model.WorkflowExecution) (*model.WorkflowExecution, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO workflow_executions (workflow_id, workflow_key, status, input, output, error, node_results, variables, duration_ms, started_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		exec.WorkflowID, exec.WorkflowKey, exec.Status, exec.Input, exec.Output, exec.Error, exec.NodeResults, exec.Variables, exec.DurationMs, exec.StartedAt, exec.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	exec.ID = id
	return exec, nil
}

func (s *PostgresStorage) UpdateWorkflowExecution(ctx context.Context, id int64, exec *model.WorkflowExecution) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_executions SET status = $1, output = $2, error = $3, node_results = $4, variables = $5, 
		 duration_ms = $6, finished_at = $7 WHERE id = $8`,
		exec.Status, exec.Output, exec.Error, exec.NodeResults, exec.Variables, exec.DurationMs, exec.FinishedAt, id,
	)
	return err
}

func (s *PostgresStorage) ListWorkflowExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, workflow_id, workflow_key, status, input, output, error, node_results, variables, duration_ms, started_at, finished_at, created_by
		 FROM workflow_executions WHERE workflow_id = $1 ORDER BY started_at DESC LIMIT $2`,
		workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.WorkflowExecution
	for rows.Next() {
		var exec model.WorkflowExecution
		if err := rows.Scan(&exec.ID, &exec.WorkflowID, &exec.WorkflowKey, &exec.Status, &exec.Input, &exec.Output, &exec.Error, &exec.NodeResults, &exec.Variables, &exec.DurationMs, &exec.StartedAt, &exec.FinishedAt, &exec.CreatedBy); err != nil {
			return nil, err
		}
		list = append(list, &exec)
	}
	return list, nil
}

func (s *PostgresStorage) GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error) {
	var exec model.WorkflowExecution
	err := s.pool.QueryRow(ctx,
		`SELECT id, workflow_id, workflow_key, status, input, output, error, node_results, variables, duration_ms, started_at, finished_at, created_by
		 FROM workflow_executions WHERE id = $1`,
		id,
	).Scan(&exec.ID, &exec.WorkflowID, &exec.WorkflowKey, &exec.Status, &exec.Input, &exec.Output, &exec.Error, &exec.NodeResults, &exec.Variables, &exec.DurationMs, &exec.StartedAt, &exec.FinishedAt, &exec.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &exec, nil
}
