package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) ListAgents() ([]*schema.Agent, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT a.id, a.public_id::text, a.name, a.description, a.category, a.is_builtin, a.is_active, a.created_at, a.updated_at, 
			COALESCE(r.skill_ids, '{}'), COALESCE(r.mcp_config_ids, '{}')
		FROM agents a
		LEFT JOIN agent_runtime r ON a.id = r.agent_id
		ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*schema.Agent
	for rows.Next() {
		var a schema.Agent
		var skillIDs []string
		var mcpIDs []int64
		if err := rows.Scan(&a.ID, &a.PublicID, &a.Name, &a.Desc, &a.Category, &a.IsBuiltin, &a.IsActive, &a.CreatedAt, &a.UpdatedAt, &skillIDs, &mcpIDs); err != nil {
			return nil, err
		}
		a.SkillIDs = skillIDs
		a.MCPConfigIDs = mcpIDs
		agents = append(agents, &a)
	}
	return agents, nil
}

func (s *PostgresStorage) GetAgent(id int64) (*schema.AgentWithRuntime, error) {
	var a schema.Agent
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, public_id::text, name, description, category, is_builtin, is_active, created_at, updated_at FROM agents WHERE id = $1`, id).
		Scan(&a.ID, &a.PublicID, &a.Name, &a.Desc, &a.Category, &a.IsBuiltin, &a.IsActive, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}

	agent := &schema.AgentWithRuntime{Agent: a}

	var rt model.AgentRuntime
	var skillIDs []string
	var mcpIDs []int64
	var imConfigJSON []byte
	err = s.pool.QueryRow(context.Background(),
		`SELECT id, agent_id, source_agent, archetype, role, goal, backstory, system_prompt, llm_model, temperature, stream_enabled, memory_enabled, skill_ids, mcp_config_ids, execution_mode, max_iterations, plan_prompt, reflection_depth, approval_mode, approvers, im_enabled, im_config, created_at, updated_at FROM agent_runtime WHERE agent_id = $1`, id).
		Scan(&rt.ID, &rt.AgentID, &rt.SourceAgent, &rt.Archetype, &rt.Role, &rt.Goal, &rt.Backstory, &rt.SystemPrompt, &rt.LlmModel, &rt.Temperature, &rt.StreamEnabled, &rt.MemoryEnabled, &skillIDs, &mcpIDs, &rt.ExecutionMode, &rt.MaxIterations, &rt.PlanPrompt, &rt.ReflectionDepth, &rt.ApprovalMode, &rt.Approvers, &rt.IMEnabled, &imConfigJSON, &rt.CreatedAt, &rt.UpdatedAt)
	if err == nil {
		var imConfig schema.IMConfig
		if len(imConfigJSON) > 0 {
			_ = json.Unmarshal(imConfigJSON, &imConfig)
		}
		agent.RuntimeProfile = &schema.RuntimeProfile{
			SourceAgent:     rt.SourceAgent,
			Archetype:       rt.Archetype,
			Role:            rt.Role,
			Goal:            rt.Goal,
			Backstory:       rt.Backstory,
			SystemPrompt:    rt.SystemPrompt,
			LlmModel:        rt.LlmModel,
			Temperature:     rt.Temperature,
			StreamEnabled:   rt.StreamEnabled,
			MemoryEnabled:   rt.MemoryEnabled,
			SkillIDs:        skillIDs,
			MCPConfigIDs:    mcpIDs,
			ExecutionMode:   rt.ExecutionMode,
			MaxIterations:   rt.MaxIterations,
			PlanPrompt:      rt.PlanPrompt,
			ReflectionDepth: rt.ReflectionDepth,
			ApprovalMode:    rt.ApprovalMode,
			Approvers:       rt.Approvers,
			IMEnabled:       rt.IMEnabled,
			IMConfig:        imConfig,
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		_ = fmt.Errorf("failed to load agent_runtime row (agent returned without runtime profile): %w", err)
	}

	tree, err := s.GetCapabilityTree(id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		_ = fmt.Errorf("failed to get capability tree: %w", err)
	}
	if tree != nil {
		agent.CapabilityTree = tree
	}

	return agent, nil
}

func (s *PostgresStorage) GetAgentIDByName(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM agents WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *PostgresStorage) CreateAgent(req *schema.CreateAgentRequest) (*schema.Agent, error) {
	var id int64
	var publicID string
	var now time.Time
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO agents (name, description, category, is_builtin, is_active) VALUES ($1, $2, $3, false, true) RETURNING id, public_id::text, created_at`,
		req.Name, req.Description, req.Category).Scan(&id, &publicID, &now)
	if err != nil {
		return nil, err
	}

	if req.RuntimeProfile != nil {
		rp := req.RuntimeProfile
		maxIter := rp.MaxIterations
		if maxIter <= 0 {
			maxIter = 16
		}
		_, err = s.pool.Exec(context.Background(),
			`INSERT INTO agent_runtime (agent_id, source_agent, archetype, role, goal, backstory, system_prompt, llm_model, temperature, stream_enabled, memory_enabled, skill_ids, mcp_config_ids, execution_mode, max_iterations, plan_prompt, reflection_depth, approval_mode, approvers, im_enabled, im_config) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`,
			id, rp.SourceAgent, rp.Archetype, rp.Role, rp.Goal, rp.Backstory, rp.SystemPrompt, rp.LlmModel, rp.Temperature, rp.StreamEnabled, rp.MemoryEnabled, rp.SkillIDs, rp.MCPConfigIDs, rp.ExecutionMode, maxIter, rp.PlanPrompt, rp.ReflectionDepth, rp.ApprovalMode, rp.Approvers, rp.IMEnabled, rp.IMConfig)
		if err != nil {
			return nil, err
		}
	}

	return &schema.Agent{
		ID: id, PublicID: publicID, Name: req.Name, Desc: req.Description, Category: req.Category, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *PostgresStorage) UpdateAgent(id int64, req *schema.UpdateAgentRequest) (*schema.Agent, error) {
	var now time.Time
	_, err := s.pool.Exec(context.Background(),
		`UPDATE agents SET name = COALESCE(NULLIF($1, ''), name), description = COALESCE(NULLIF($2, ''), description), category = COALESCE(NULLIF($3, ''), category), updated_at = NOW() WHERE id = $4`,
		req.Name, req.Description, req.Category, id)
	if err != nil {
		return nil, err
	}

	if req.RuntimeProfile != nil {
		rp := req.RuntimeProfile
		maxIter := rp.MaxIterations
		if maxIter <= 0 {
			maxIter = 16
		}
		var imConfigJSON []byte
		// Persist full im_config whenever an IM platform is selected (matches UI: Telegram uses token/chat_id only).
		if rp.IMEnabled != "" && rp.IMEnabled != "disabled" {
			imConfigJSON, _ = json.Marshal(rp.IMConfig)
		}
		_, err = s.pool.Exec(context.Background(),
			`UPDATE agent_runtime SET source_agent = COALESCE(NULLIF($1, ''), source_agent), archetype = COALESCE(NULLIF($2, ''), archetype), role = COALESCE(NULLIF($3, ''), role), goal = COALESCE(NULLIF($4, ''), goal), backstory = COALESCE(NULLIF($5, ''), backstory), system_prompt = COALESCE(NULLIF($6, ''), system_prompt), llm_model = COALESCE(NULLIF($7, ''), llm_model), temperature = $8, stream_enabled = $9, memory_enabled = $10, skill_ids = $11, mcp_config_ids = $12, execution_mode = COALESCE(NULLIF($13, ''), execution_mode), max_iterations = $14, plan_prompt = COALESCE(NULLIF($15, ''), plan_prompt), reflection_depth = $16, approval_mode = COALESCE(NULLIF($17, ''), approval_mode), approvers = $18, im_enabled = COALESCE(NULLIF($19, ''), im_enabled), im_config = $20, updated_at = NOW() WHERE agent_id = $21`,
			rp.SourceAgent, rp.Archetype, rp.Role, rp.Goal, rp.Backstory, rp.SystemPrompt, rp.LlmModel, rp.Temperature, rp.StreamEnabled, rp.MemoryEnabled, rp.SkillIDs, rp.MCPConfigIDs, rp.ExecutionMode, maxIter, rp.PlanPrompt, rp.ReflectionDepth, rp.ApprovalMode, rp.Approvers, rp.IMEnabled, imConfigJSON, id)
		if err != nil {
			return nil, err
		}
	}

	var publicID string
	var name, desc, category string
	var isBuiltin, isActive bool
	var createdAt time.Time
	err = s.pool.QueryRow(context.Background(),
		`SELECT id, public_id::text, name, description, category, is_builtin, is_active, created_at, updated_at FROM agents WHERE id = $1`, id).
		Scan(&id, &publicID, &name, &desc, &category, &isBuiltin, &isActive, &createdAt, &now)
	if err != nil {
		return nil, err
	}

	return &schema.Agent{ID: id, PublicID: publicID, Name: name, Desc: desc, Category: category, IsBuiltin: isBuiltin, IsActive: isActive, CreatedAt: createdAt, UpdatedAt: now}, nil
}

func (s *PostgresStorage) DeleteAgent(id int64) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, id)
	return err
}

func (s *PostgresStorage) GetCapabilityTree(agentID int64) (*schema.CapabilityTree, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, parent_id, node_type, label, capability_id, rule_json, sort_order, is_active FROM capability_tree_nodes WHERE agent_id = $1 ORDER BY version DESC, sort_order`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []schema.CapabilityTreeNode
	for rows.Next() {
		var n schema.CapabilityTreeNode
		var ruleJSON []byte
		if err := rows.Scan(&n.ID, &n.ParentID, &n.NodeType, &n.Label, &n.CapabilityID, &ruleJSON, &n.SortOrder, &n.IsActive); err != nil {
			return nil, err
		}
		if ruleJSON != nil {
			if err := json.Unmarshal(ruleJSON, &n.RuleJSON); err != nil {
				n.RuleJSON = nil
			}
		}
		nodes = append(nodes, n)
	}

	return &schema.CapabilityTree{AgentID: agentID, Nodes: nodes}, nil
}

func (s *PostgresStorage) UpdateCapabilityTree(agentID int64, nodes []schema.CapabilityTreeNode) (*schema.CapabilityTree, error) {
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	_, _ = tx.Exec(context.Background(), `DELETE FROM capability_tree_nodes WHERE agent_id = $1`, agentID)

	for _, n := range nodes {
		ruleJSON, err := json.Marshal(n.RuleJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal rule JSON for node %d: %w", n.ID, err)
		}
		_, err = tx.Exec(context.Background(),
			`INSERT INTO capability_tree_nodes (agent_id, parent_id, node_type, label, capability_id, rule_json, sort_order, is_active, version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)`,
			agentID, n.ParentID, n.NodeType, n.Label, n.CapabilityID, ruleJSON, n.SortOrder, n.IsActive)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		return nil, err
	}

	return s.GetCapabilityTree(agentID)
}
