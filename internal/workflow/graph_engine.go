package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/model"
)

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

type GraphEngine struct {
	runtime *agent.Runtime
	store   GraphStore
}

type GraphStore interface {
	GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error)
	CreateWorkflowExecution(ctx context.Context, exec *model.WorkflowExecution) (*model.WorkflowExecution, error)
	UpdateWorkflowExecution(ctx context.Context, id int64, exec *model.WorkflowExecution) error
	ListWorkflowExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error)
	GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error)
}

func NewGraphEngine(runtime *agent.Runtime, store GraphStore) *GraphEngine {
	SetAgentRuntime(runtime)
	return &GraphEngine{runtime: runtime, store: store}
}

type ExecutionContext struct {
	Variables   map[string]any
	NodeOutputs map[string]any
	UserMessage string
	Edges       []model.WorkflowEdge
	VarContext  *VariableContext
	Items       map[string][]Item
}

type Item struct {
	JSON       map[string]any `json:"json"`
	PairedItem *PairedItem    `json:"pairedItem,omitempty"`
}

type PairedItem struct {
	Item      int    `json:"item"`
	Reference string `json:"reference,omitempty"`
}

func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Variables:   make(map[string]any),
		NodeOutputs: make(map[string]any),
		VarContext:  NewVariableContext(),
		Items:       make(map[string][]Item),
	}
}

type NodeResult struct {
	NodeID     string    `json:"node_id"`
	Label      string    `json:"label"`
	NodeType   string    `json:"node_type"`
	Input      string    `json:"input,omitempty"`
	Output     any       `json:"output"`
	Error      string    `json:"error,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	DurationMs int64     `json:"duration_ms"`
	RetryCount int       `json:"retry_count,omitempty"`
}

type GraphExecutionResult struct {
	Output      any          `json:"output"`
	NodeResults []NodeResult `json:"node_results"`
	ExecutionID int64        `json:"execution_id,omitempty"`
	DurationMs  int64        `json:"duration_ms"`
}

const (
	NodeTypeInput     = "input"
	NodeTypeOutput    = "output"
	NodeTypeAgent     = "agent"
	NodeTypeCondition = "condition"
	NodeTypeLLM       = "llm"
	NodeTypeTool      = "tool"
	NodeTypeMerge     = "merge"
)

func (e *GraphEngine) Execute(ctx context.Context, workflowID int64, userMessage string, variables map[string]any) (*GraphExecutionResult, error) {
	startTime := time.Now()
	def, err := e.store.GetWorkflowDefinition(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	if !def.IsActive {
		return nil, fmt.Errorf("workflow is not active")
	}

	if len(def.Nodes) == 0 {
		return nil, fmt.Errorf("workflow has no nodes")
	}

	normalizeAgentNodeAgentIDs(def)

	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		userMessage = extractStartUserPrompt(def)
	}

	exec := &model.WorkflowExecution{
		WorkflowID:  workflowID,
		WorkflowKey: def.Key,
		Status:      "running",
		Input:       userMessage,
		StartedAt:   time.Now(),
	}
	createdExec, _ := e.store.CreateWorkflowExecution(ctx, exec)
	if createdExec != nil {
		exec = createdExec
	}

	execCtx := NewExecutionContext()
	execCtx.UserMessage = userMessage
	execCtx.Edges = def.Edges

	// 初始化系统变量
	execCtx.VarContext.SetSystemVariable("sys.query", userMessage)
	execCtx.VarContext.SetSystemVariable("sys.workflow_id", workflowID)
	execCtx.VarContext.SetSystemVariable("sys.workflow_run_id", exec.ID)

	if variables != nil {
		for k, v := range variables {
			execCtx.Variables[k] = v
			execCtx.VarContext.SetGlobalVariable(k, v)
		}
	}
	if def.Variables != nil {
		for k, v := range def.Variables {
			if _, exists := execCtx.Variables[k]; !exists {
				execCtx.Variables[k] = v
				execCtx.VarContext.SetGlobalVariable(k, v)
			}
		}
	}

	if err := e.validateGraph(def); err != nil {
		exec.Status = "failed"
		exec.Error = err.Error()
		ft := time.Now()
		exec.FinishedAt = &ft
		exec.DurationMs = time.Since(startTime).Milliseconds()
		_ = e.store.UpdateWorkflowExecution(ctx, exec.ID, exec)
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	nodeMap := make(map[string]*model.WorkflowNode)
	for i := range def.Nodes {
		nodeMap[def.Nodes[i].ID] = &def.Nodes[i]
	}

	edgeMap := make(map[string][]*model.WorkflowEdge)
	for i := range def.Edges {
		edgeMap[def.Edges[i].SourceNodeID] = append(edgeMap[def.Edges[i].SourceNodeID], &def.Edges[i])
	}

	inDegree := make(map[string]int)
	for _, node := range def.Nodes {
		if _, exists := inDegree[node.ID]; !exists {
			inDegree[node.ID] = 0
		}
	}
	for _, edge := range def.Edges {
		inDegree[edge.TargetNodeID]++
	}

	results := make([]NodeResult, 0, len(def.Nodes))
	processed := make(map[string]bool)
	var resultsMu sync.Mutex
	var outputsMu sync.Mutex

	// 执行就绪节点（支持并行）
	for {
		// 找出所有就绪的节点
		var readyNodes []string
		for _, node := range def.Nodes {
			if !processed[node.ID] && inDegree[node.ID] == 0 {
				readyNodes = append(readyNodes, node.ID)
			}
		}

		if len(readyNodes) == 0 {
			break
		}

		slog.Info("workflow: executing batch",
			"workflow_id", workflowID,
			"execution_id", exec.ID,
			"batch_size", len(readyNodes),
		)

		// 并行执行所有就绪节点
		var wg sync.WaitGroup
		var firstError *NodeResult

		for _, nodeID := range readyNodes {
			wg.Add(1)
			go func(nid string) {
				defer wg.Done()

				node := nodeMap[nid]
				nodeStartTime := time.Now()

				slog.Info("workflow: node started",
					"workflow_id", workflowID,
					"execution_id", exec.ID,
					"node_id", nid,
					"node_label", node.Label,
					"node_type", node.Type,
				)

				inputMsg := e.buildNodeInput(node, execCtx)
				result := e.executeNode(ctx, node, execCtx)
				nodeEndTime := time.Now()

				result.NodeID = nid
				result.Label = node.Label
				result.NodeType = node.Type
				result.Input = truncateForLog(inputMsg, 500)
				result.StartTime = nodeStartTime
				result.EndTime = nodeEndTime
				result.DurationMs = nodeEndTime.Sub(nodeStartTime).Milliseconds()

				// 记录结果
				if result.Error != "" {
					slog.Error("workflow: node failed",
						"workflow_id", workflowID,
						"execution_id", exec.ID,
						"node_id", nid,
						"node_label", node.Label,
						"duration_ms", result.DurationMs,
						"error", result.Error,
					)
				} else {
					slog.Info("workflow: node completed",
						"workflow_id", workflowID,
						"execution_id", exec.ID,
						"node_id", nid,
						"node_label", node.Label,
						"duration_ms", result.DurationMs,
						"output_size", len(fmt.Sprintf("%v", result.Output)),
					)
				}

				resultsMu.Lock()
				results = append(results, result)
				if firstError == nil && result.Error != "" {
					firstError = &result
				}
				resultsMu.Unlock()

				// 更新输出（需要锁保护）
				outputsMu.Lock()
				execCtx.NodeOutputs[nid] = result.Output
				if node.Label != "" {
					execCtx.Variables[node.Label] = result.Output
				}
				execCtx.Variables["last_output"] = result.Output
				execCtx.Items[nid] = convertToItems(result.Output, nid)
				execCtx.VarContext.SetNodeOutput(nid, result.Output)
				if node.Label != "" {
					execCtx.VarContext.SetGlobalVariable(node.Label, result.Output)
				}
				processed[nid] = true
				outputsMu.Unlock()
			}(nodeID)
		}

		wg.Wait()

		// 如果有错误，立即返回
		if firstError != nil {
			exec.Status = "failed"
			exec.Output = fmt.Sprintf("Node %s failed: %s", firstError.Label, firstError.Error)
			exec.Error = firstError.Error
			exec.NodeResults = convertToModelResults(results)
			ft := time.Now()
			exec.FinishedAt = &ft
			exec.DurationMs = time.Since(startTime).Milliseconds()
			_ = e.store.UpdateWorkflowExecution(ctx, exec.ID, exec)
			return &GraphExecutionResult{
				Output:      exec.Output,
				NodeResults: results,
				ExecutionID: exec.ID,
				DurationMs:  exec.DurationMs,
			}, nil
		}

		// 更新入度，准备下一批
		for _, nid := range readyNodes {
			for _, edge := range edgeMap[nid] {
				if edge.Condition != "" {
					outputsMu.Lock()
					if !e.evaluateCondition(edge.Condition, execCtx) {
						outputsMu.Unlock()
						continue
					}
					outputsMu.Unlock()
				}
				inDegree[edge.TargetNodeID]--
			}
		}
	}

	var finalOutput any
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Output != nil {
			finalOutput = results[i].Output
			break
		}
	}

	exec.Status = "success"
	if s, ok := finalOutput.(string); ok {
		exec.Output = s
	} else {
		exec.Output = fmt.Sprintf("%v", finalOutput)
	}
	exec.NodeResults = convertToModelResults(results)
	finishTime := time.Now()
	exec.FinishedAt = &finishTime
	exec.DurationMs = time.Since(startTime).Milliseconds()
	_ = e.store.UpdateWorkflowExecution(ctx, exec.ID, exec)

	return &GraphExecutionResult{
		Output:      finalOutput,
		NodeResults: results,
		ExecutionID: exec.ID,
		DurationMs:  exec.DurationMs,
	}, nil
}

func (e *GraphEngine) buildNodeInput(node *model.WorkflowNode, execCtx *ExecutionContext) string {
	input := ""

	if node.Config != nil {
		if inputMapping, ok := node.Config["input_mapping"].(map[string]any); ok {
			var mappingParts []string
			for targetField, sourcePath := range inputMapping {
				if sourceStr, ok := sourcePath.(string); ok {
					val := e.resolveTemplateValue(sourceStr, execCtx)
					if val != "" {
						mappingParts = append(mappingParts, fmt.Sprintf("%s: %s", targetField, val))
					}
				}
			}
			if len(mappingParts) > 0 {
				input = strings.Join(mappingParts, ", ")
			}
		}

		if promptTemplate, ok := node.Config["prompt_template"]; ok {
			if tmpl, ok := promptTemplate.(string); ok && tmpl != "" {
				if input != "" {
					input = e.renderTemplate(tmpl, execCtx)
				} else {
					input = e.renderTemplate(tmpl, execCtx)
				}
			}
		}
	}

	if input == "" {
		input = execCtx.UserMessage
	}

	if len(execCtx.NodeOutputs) > 0 {
		var prevOutputs []string
		for nodeID, output := range execCtx.NodeOutputs {
			prevOutputs = append(prevOutputs, fmt.Sprintf("[%s]: %v", nodeID, output))
		}
		if len(prevOutputs) > 0 {
			input = fmt.Sprintf("[Previous outputs]\n%s\n\n[Current input]\n%s", strings.Join(prevOutputs, "\n"), input)
		}
	}

	return input
}

func (e *GraphEngine) resolveTemplateValue(path string, execCtx *ExecutionContext) string {
	re := regexp.MustCompile(`\{\{(.?[\w.\[\]]+)\}\}`)
	path = re.ReplaceAllStringFunc(path, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		keyPath := strings.TrimPrefix(submatches[1], ".")

		parts := strings.Split(keyPath, ".")
		if len(parts) == 0 {
			return match
		}

		var val any
		var ok bool

		val, ok = execCtx.Variables[parts[0]]
		if !ok {
			val, ok = execCtx.NodeOutputs[parts[0]]
		}
		if !ok {
			// 支持通过节点ID查找 Items
			if items, hasItems := execCtx.Items[parts[0]]; hasItems && len(items) > 0 {
				if len(parts) == 1 {
					// 只有节点ID，返回第一个item的json
					return formatItemJSON(items[0])
				}
				// 尝试获取json字段
				if len(parts) >= 2 && parts[1] == "json" {
					if jsonMap, ok := items[0].JSON[parts[2]]; ok {
						return formatValue(jsonMap)
					}
				}
				// 尝试获取 items[index].json.field
				if len(parts) >= 4 && parts[1] == "items" {
					var idx int
					fmt.Sscanf(parts[2], "%d", &idx)
					if idx >= 0 && idx < len(items) {
						if parts[3] == "json" && len(parts) >= 5 {
							if jsonMap, ok := items[idx].JSON[parts[4]]; ok {
								return formatValue(jsonMap)
							}
						}
					}
				}
			}
			return match
		}

		for i := 1; i < len(parts) && val != nil; i++ {
			val, ok = getNestedField(val, parts[i])
			if !ok {
				return match
			}
		}

		if val == nil {
			return match
		}

		return formatValue(val)
	})

	return path
}

func formatValueSimple(val any) string {
	if str, ok := val.(string); ok {
		return str
	}
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func formatItemJSON(item Item) string {
	if len(item.JSON) == 0 {
		return ""
	}
	if data, ok := item.JSON["data"]; ok {
		return formatValueSimple(data)
	}
	return formatValueSimple(item.JSON)
}

func (e *GraphEngine) renderTemplate(tmpl string, execCtx *ExecutionContext) string {
	re := regexp.MustCompile(`\{\{(\.?[\w.]+)\}\}`)
	return re.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		path := submatches[1]
		path = strings.TrimPrefix(path, ".")

		parts := strings.Split(path, ".")
		if len(parts) == 0 {
			return match
		}

		var val any
		var ok bool

		if len(parts) == 1 {
			val, ok = execCtx.Variables[parts[0]]
		} else {
			val, ok = execCtx.Variables[parts[0]]
			if !ok {
				val, ok = execCtx.NodeOutputs[parts[0]]
			}
			for i := 1; i < len(parts) && val != nil; i++ {
				val, ok = getNestedField(val, parts[i])
			}
		}

		if val == nil {
			return match
		}

		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	})
}

func (e *GraphEngine) evaluateCondition(condition string, execCtx *ExecutionContext) bool {
	condition = strings.TrimSpace(condition)

	if strings.HasPrefix(condition, "{{.") && strings.HasSuffix(condition, "}}") {
		key := strings.TrimSuffix(strings.TrimPrefix(condition, "{{."), "}}")
		val, ok := execCtx.Variables[key]
		if !ok {
			return false
		}
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v != "" && v != "false" && v != "0"
		case int, int64, float64:
			return v != 0
		default:
			return val != nil
		}
	}

	return condition != ""
}

func convertToModelResults(results []NodeResult) []model.NodeResult {
	out := make([]model.NodeResult, 0, len(results))
	for _, r := range results {
		out = append(out, model.NodeResult{
			NodeID: r.NodeID,
			Label:  r.Label,
			Output: map[string]any{"data": r.Output},
			Error:  r.Error,
		})
	}
	return out
}

func convertToItems(output any, nodeID string) []Item {
	if output == nil {
		return []Item{}
	}
	switch v := output.(type) {
	case []Item:
		return v
	case []map[string]any:
		items := make([]Item, len(v))
		for i, m := range v {
			items[i] = Item{
				JSON: m,
				PairedItem: &PairedItem{
					Item:      i,
					Reference: nodeID,
				},
			}
		}
		return items
	case map[string]any:
		return []Item{{
			JSON: v,
			PairedItem: &PairedItem{
				Item:      0,
				Reference: nodeID,
			},
		}}
	default:
		return []Item{{
			JSON: map[string]any{"value": v},
			PairedItem: &PairedItem{
				Item:      0,
				Reference: nodeID,
			},
		}}
	}
}

func (e *GraphEngine) executeNode(ctx context.Context, node *model.WorkflowNode, execCtx *ExecutionContext) NodeResult {
	result := NodeResult{}

	taskType := TaskType(node.Type)
	taskDef, ok := GetTask(taskType)
	if !ok {
		result.Error = fmt.Sprintf("unknown node type: %s", node.Type)
		return result
	}

	input := &TaskInput{
		NodeID:      node.ID,
		NodeLabel:   node.Label,
		Config:      node.Config,
		Variables:   execCtx.Variables,
		NodeOutputs: execCtx.NodeOutputs,
		UserMessage: execCtx.UserMessage,
		VarContext:  execCtx.VarContext,
	}

	if node.AgentID != nil && taskType == TaskTypeAgent {
		if input.Config == nil {
			input.Config = make(map[string]any)
		}
		input.Config["agent_id"] = float64(*node.AgentID)
	}

	// 获取重试配置
	retryCount := 0
	retryDelayMs := 1000
	if rc, ok := node.Config["retry_count"].(float64); ok {
		retryCount = int(rc)
	}
	if rd, ok := node.Config["retry_delay_ms"].(float64); ok {
		retryDelayMs = int(rd)
	}

	// 执行节点（带重试）
	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			slog.Info("workflow: node retry",
				"node_id", node.ID,
				"node_label", node.Label,
				"attempt", attempt,
				"max_retries", retryCount,
				"delay_ms", retryDelayMs,
			)
			time.Sleep(time.Duration(retryDelayMs) * time.Millisecond)
		}

		output, err := taskDef.Execute(ctx, input)
		if err != nil {
			lastErr = err
			continue
		}

		if output.Error != "" {
			lastErr = fmt.Errorf("%s", output.Error)
			continue
		}

		result.Output = output.Data
		result.RetryCount = attempt
		return result
	}

	if lastErr != nil {
		result.Error = lastErr.Error()
	}
	return result
}

func (e *GraphEngine) validateInputSchema(node *model.WorkflowNode, execCtx *ExecutionContext) error {
	if node.InputSchema == nil {
		return nil
	}

	props, ok := node.InputSchema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	inputMapping := node.Config["input_mapping"].(map[string]any)

	for field, schema := range props {
		schemaMap, ok := schema.(map[string]any)
		if !ok {
			continue
		}

		fieldType, _ := schemaMap["type"].(string)
		required, _ := schemaMap["required"].(bool)

		hasMapping := false
		if inputMapping != nil {
			for targetField := range inputMapping {
				if targetField == field {
					hasMapping = true
					break
				}
			}
		}

		if required && !hasMapping {
			sourcePath := ""
			if mapping, ok := node.Config["input_mapping"].(map[string]any); ok {
				for _, path := range mapping {
					if str, ok := path.(string); ok && str != "" {
						sourcePath = str
						break
					}
				}
			}
			if sourcePath == "" && execCtx.UserMessage == "" {
				return fmt.Errorf("required field '%s' has no mapping from upstream nodes", field)
			}
		}

		_ = fieldType
	}

	return nil
}

func getSourceNodeOutput(sourceNodeID string, execCtx *ExecutionContext) any {
	if val, ok := execCtx.NodeOutputs[sourceNodeID]; ok {
		return val
	}
	if val, ok := execCtx.Variables[sourceNodeID]; ok {
		return val
	}
	return nil
}

func extractFieldFromOutput(output any, field string) any {
	if output == nil {
		return nil
	}
	switch v := output.(type) {
	case map[string]any:
		if val, ok := v[field]; ok {
			return val
		}
	case string:
		return output
	}
	return nil
}

// extractStartUserPrompt returns config.user_prompt from the first "start" node (for API/UI when runtime message is empty).
func extractStartUserPrompt(def *model.WorkflowDefinition) string {
	for _, n := range def.Nodes {
		if n.Type != "start" || n.Config == nil {
			continue
		}
		raw, ok := n.Config["user_prompt"]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		default:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

// normalizeAgentNodeAgentIDs sets node.AgentID from config["agent_id"] when the top-level
// field is missing (e.g. older UI saves or hand-edited JSON that only stored agent_id in config).
func normalizeAgentNodeAgentIDs(def *model.WorkflowDefinition) {
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeTypeAgent || n.AgentID != nil || n.Config == nil {
			continue
		}
		raw, ok := n.Config["agent_id"]
		if !ok || raw == nil {
			raw, ok = n.Config["agentId"]
		}
		if !ok || raw == nil {
			continue
		}
		id, ok := parseWorkflowAgentID(raw)
		if !ok || id < 1 {
			continue
		}
		n.AgentID = &id
	}
}

func parseWorkflowAgentID(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case float64:
		if x < 1 {
			return 0, false
		}
		return int64(x), true
	case float32:
		if x < 1 {
			return 0, false
		}
		return int64(x), true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		id, err := strconv.ParseInt(s, 10, 64)
		return id, err == nil && id >= 1
	case json.Number:
		id, err := x.Int64()
		return id, err == nil && id >= 1
	default:
		return 0, false
	}
}

func (e *GraphEngine) validateGraph(def *model.WorkflowDefinition) error {
	if len(def.Nodes) == 0 {
		return fmt.Errorf("workflow must have at least one node")
	}

	nodeIDs := make(map[string]bool)
	for _, node := range def.Nodes {
		slog.Debug("validating node", "id", node.ID, "type", node.Type, "label", node.Label, "agentID", node.AgentID)

		if node.ID == "" {
			return fmt.Errorf("node ID cannot be empty")
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		nodeIDs[node.ID] = true

		if node.Type == NodeTypeAgent && node.AgentID == nil {
			slog.Error("agent node missing agent_id", "nodeID", node.ID, "label", node.Label)
			return fmt.Errorf("agent node '%s' must have agent_id (bind an agent in the node config and save the workflow)", node.Label)
		}
	}

	for _, edge := range def.Edges {
		if !nodeIDs[edge.SourceNodeID] {
			return fmt.Errorf("edge %s references non-existent source node: %s", edge.ID, edge.SourceNodeID)
		}
		if !nodeIDs[edge.TargetNodeID] {
			return fmt.Errorf("edge %s references non-existent target node: %s", edge.ID, edge.TargetNodeID)
		}
		if edge.SourceNodeID == edge.TargetNodeID {
			return fmt.Errorf("edge %s creates a self-loop on node %s", edge.ID, edge.SourceNodeID)
		}
	}

	if hasCycle(def) {
		return fmt.Errorf("workflow graph contains cycles")
	}

	return nil
}

func hasCycle(def *model.WorkflowDefinition) bool {
	adj := make(map[string][]string)
	for _, edge := range def.Edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, node := range def.Nodes {
		if !visited[node.ID] {
			if dfs(node.ID) {
				return true
			}
		}
	}

	return false
}

func (e *GraphEngine) ListExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error) {
	return e.store.ListWorkflowExecutions(ctx, workflowID, limit)
}

func (e *GraphEngine) GetExecution(ctx context.Context, execID int64) (*model.WorkflowExecution, error) {
	return e.store.GetWorkflowExecution(ctx, execID)
}
