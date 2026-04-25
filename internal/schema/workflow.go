package schema

import "time"

const (
	WorkflowKindSingle     = "single"
	WorkflowKindSequential = "sequential"
	WorkflowKindParallel   = "parallel"
	WorkflowKindSupervisor = "supervisor"
	WorkflowKindLoop       = "loop"
	WorkflowKindGraph      = "graph"
)

const (
	NodeTypeInput     = "input"
	NodeTypeOutput    = "output"
	NodeTypeAgent     = "agent"
	NodeTypeCondition = "condition"
	NodeTypeLLM       = "llm"
	NodeTypeTool      = "tool"
	NodeTypeMerge     = "merge"
	NodeTypeLoop      = "loop"
	NodeTypeParallel  = "parallel"
	NodeTypeBranch    = "branch"
	NodeTypeWait      = "wait"
)

type AgentWorkflow struct {
	ID           int64          `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind"`
	StepAgentIDs []int64        `json:"step_agent_ids"`
	Config       map[string]any `json:"config,omitempty"`
	IsActive     bool           `json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateWorkflowRequest struct {
	Key          string         `json:"key" validate:"required,min=1,max=100"`
	Name         string         `json:"name" validate:"required,min=1,max=200"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind" validate:"required"`
	StepAgentIDs []int64        `json:"step_agent_ids" validate:"required,min=1"`
	Config       map[string]any `json:"config,omitempty"`
	IsActive     *bool          `json:"is_active,omitempty"`
}

type UpdateWorkflowRequest struct {
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind,omitempty"`
	StepAgentIDs []int64        `json:"step_agent_ids,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	IsActive     *bool          `json:"is_active,omitempty"`
}

type WorkflowDefinitionPublic struct {
	ID           int64          `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
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

type CreateWorkflowDefinitionRequest struct {
	Key          string         `json:"key" validate:"required,min=1,max=100"`
	Name         string         `json:"name" validate:"required,min=1,max=200"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind" validate:"required,oneof=graph sequential"`
	Nodes        []WorkflowNode `json:"nodes" validate:"required,min=1"`
	Edges        []WorkflowEdge `json:"edges"`
	Variables    map[string]any `json:"variables,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	IsActive     *bool          `json:"is_active,omitempty"`
}

type UpdateWorkflowDefinitionRequest struct {
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind,omitempty"`
	Nodes        []WorkflowNode `json:"nodes,omitempty"`
	Edges        []WorkflowEdge `json:"edges,omitempty"`
	Variables    map[string]any `json:"variables,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	IsActive     *bool          `json:"is_active,omitempty"`
}

type ExecuteWorkflowRequest struct {
	WorkflowID int64          `json:"workflow_id" validate:"required"`
	// Message is the runtime user text. If empty after trim, the engine uses the first Start node’s config.user_prompt from the saved workflow.
	Message   string         `json:"message"`
	Variables map[string]any `json:"variables,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
}

type ExecuteWorkflowResponse struct {
	Output     any            `json:"output"`
	NodeResult map[string]any `json:"node_results,omitempty"`
	// NodeResultOrder is the order nodes finished (for UI edge animation).
	NodeResultOrder []string `json:"node_result_order,omitempty"`
	DurationMS      int64    `json:"duration_ms"`
}
