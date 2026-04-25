package model

import "time"

type WorkflowDefinition struct {
	ID           int64          `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Kind         string         `json:"kind"`
	Nodes        []WorkflowNode `json:"nodes"`
	Edges        []WorkflowEdge `json:"edges"`
	Variables    map[string]any `json:"variables,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	Version      int            `json:"version"`
	IsActive     bool           `json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type WorkflowNode struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Label        string         `json:"label"`
	AgentID      *int64         `json:"agent_id,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	Position     *NodePosition  `json:"position,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type WorkflowEdge struct {
	ID           string `json:"id"`
	SourceNodeID string `json:"source_node_id"`
	SourcePort   string `json:"source_port,omitempty"`
	TargetNodeID string `json:"target_node_id"`
	TargetPort   string `json:"target_port,omitempty"`
	Condition    string `json:"condition,omitempty"`
	Label        string `json:"label,omitempty"`
}

type WorkflowExecution struct {
	ID          int64          `json:"id"`
	WorkflowID  int64          `json:"workflow_id"`
	WorkflowKey string         `json:"workflow_key"`
	Status      string         `json:"status"`
	Input       string         `json:"input"`
	Output      string         `json:"output"`
	Error       string         `json:"error,omitempty"`
	NodeResults []NodeResult   `json:"node_results,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	DurationMs  int64          `json:"duration_ms"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
}

type NodeResult struct {
	NodeID     string         `json:"node_id"`
	Label      string         `json:"label"`
	NodeType   string         `json:"node_type"`
	Input      string         `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	StartTime  string         `json:"start_time,omitempty"`
	EndTime    string         `json:"end_time,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	RetryCount int            `json:"retry_count,omitempty"`
}
