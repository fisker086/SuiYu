package workflow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterTask(&TaskDefinition{
		Type:        TaskTypeStart,
		Name:        "Start",
		Description: "Workflow entry point",
		Icon:        "play_arrow",
		Color:       "#4caf50",
		Category:    "flow",
		Execute:     executeStart,
		ConfigSchema: map[string]TaskField{
			"input_schema": {
				Key:         "input_schema",
				Label:       "Input Schema",
				Type:        "json",
				Required:    false,
				Description: "Expected input format",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeEnd,
		Name:        "End",
		Description: "Workflow output",
		Icon:        "stop",
		Color:       "#f44336",
		Category:    "flow",
		Execute:     executeEnd,
		ConfigSchema: map[string]TaskField{
			"output_mapping": {
				Key:         "output_mapping",
				Label:       "Output Mapping",
				Type:        "text",
				Required:    false,
				Description: "Template like {{node_id.key}}",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeLoop,
		Name:        "Loop",
		Description: "Iterate over items or count",
		Icon:        "loop",
		Color:       "#e91e63",
		Category:    "flow",
		Execute:     executeLoop,
		ConfigSchema: map[string]TaskField{
			"mode": {
				Key:      "mode",
				Label:    "Loop Mode",
				Type:     "select",
				Required: true,
				Default:  "count",
				Options:  []string{"count", "items"},
			},
			"count": {
				Key:         "count",
				Label:       "Iteration Count",
				Type:        "number",
				Required:    false,
				Description: "Number of iterations (for count mode)",
			},
			"items": {
				Key:         "items",
				Label:       "Items",
				Type:        "textarea",
				Required:    false,
				Description: "JSON array or comma-separated items (for items mode)",
			},
			"max_iterations": {
				Key:      "max_iterations",
				Label:    "Max Iterations",
				Type:     "number",
				Required: false,
				Default:  10,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeBranch,
		Name:        "Branch",
		Description: "Execute different paths based on condition",
		Icon:        "call_split",
		Color:       "#ff9800",
		Category:    "flow",
		Execute:     executeBranch,
		ConfigSchema: map[string]TaskField{
			"condition": {
				Key:         "condition",
				Label:       "Condition",
				Type:        "text",
				Required:    true,
				Description: "Expression like {{var}} == 'value'",
			},
			"true_node": {
				Key:         "true_node",
				Label:       "True Branch Node ID",
				Type:        "text",
				Required:    false,
				Description: "Node to execute when condition is true",
			},
			"false_node": {
				Key:         "false_node",
				Label:       "False Branch Node ID",
				Type:        "text",
				Required:    false,
				Description: "Node to execute when condition is false",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeParallel,
		Name:        "Parallel",
		Description: "Execute multiple nodes concurrently",
		Icon:        "parallelize",
		Color:       "#673ab7",
		Category:    "flow",
		Execute:     executeParallel,
		ConfigSchema: map[string]TaskField{
			"nodes": {
				Key:         "nodes",
				Label:       "Node IDs",
				Type:        "textarea",
				Required:    true,
				Description: "Comma-separated node IDs to execute in parallel",
			},
			"timeout_ms": {
				Key:      "timeout_ms",
				Label:    "Timeout (ms)",
				Type:     "number",
				Required: false,
				Default:  60000,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeWait,
		Name:        "Wait",
		Description: "Wait for a duration or condition",
		Icon:        "hourglass_empty",
		Color:       "#9e9e9e",
		Category:    "flow",
		Execute:     executeWait,
		ConfigSchema: map[string]TaskField{
			"duration_ms": {
				Key:         "duration_ms",
				Label:       "Duration (ms)",
				Type:        "number",
				Required:    false,
				Description: "Wait duration in milliseconds",
			},
			"condition": {
				Key:         "condition",
				Label:       "Condition",
				Type:        "text",
				Required:    false,
				Description: "Wait until condition is true (checked every 1s)",
			},
			"max_wait_ms": {
				Key:      "max_wait_ms",
				Label:    "Max Wait (ms)",
				Type:     "number",
				Required: false,
				Default:  300000,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeAgent,
		Name:        "Agent",
		Description: "Call an AI agent",
		Icon:        "smart_toy",
		Color:       "#2196f3",
		Category:    "ai",
		Execute:     executeAgent,
		ConfigSchema: map[string]TaskField{
			"agent_id": {
				Key:         "agent_id",
				Label:       "Agent ID",
				Type:        "select",
				Required:    true,
				Description: "Target agent",
			},
			"prompt_template": {
				Key:         "prompt_template",
				Label:       "Prompt Template",
				Type:        "textarea",
				Required:    false,
				Description: "Use {{variable}} for dynamic values",
			},
			"max_iterations": {
				Key:      "max_iterations",
				Label:    "Max Iterations",
				Type:     "number",
				Required: false,
				Default:  16,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeLLM,
		Name:        "LLM",
		Description: "Direct LLM call",
		Icon:        "psychology",
		Color:       "#9c27b0",
		Category:    "ai",
		Execute:     executeLLM,
		ConfigSchema: map[string]TaskField{
			"agent_id": {
				Key:         "agent_id",
				Label:       "Agent ID (for model config)",
				Type:        "select",
				Required:    false,
				Description: "Agent to get model config from",
			},
			"prompt": {
				Key:         "prompt",
				Label:       "Prompt",
				Type:        "textarea",
				Required:    true,
				Description: "LLM prompt with {{variable}} support",
			},
			"system_prompt": {
				Key:      "system_prompt",
				Label:    "System Prompt",
				Type:     "textarea",
				Required: false,
			},
			"temperature": {
				Key:      "temperature",
				Label:    "Temperature",
				Type:     "number",
				Required: false,
				Default:  0.7,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeTool,
		Name:        "Tool",
		Description: "Execute a tool or skill",
		Icon:        "build",
		Color:       "#ff9800",
		Category:    "action",
		Execute:     executeTool,
		ConfigSchema: map[string]TaskField{
			"tool_name": {
				Key:         "tool_name",
				Label:       "Tool Name",
				Type:        "select",
				Required:    true,
				Description: "Tool or skill to execute",
			},
			"tool_input": {
				Key:         "tool_input",
				Label:       "Tool Input",
				Type:        "textarea",
				Required:    false,
				Description: "Input for the tool (supports {{variable}})",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeHTTP,
		Name:        "HTTP Request",
		Description: "Make HTTP requests",
		Icon:        "http",
		Color:       "#00bcd4",
		Category:    "action",
		Execute:     executeHTTP,
		ConfigSchema: map[string]TaskField{
			"method": {
				Key:      "method",
				Label:    "Method",
				Type:     "select",
				Required: true,
				Default:  "GET",
				Options:  []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
			},
			"url": {
				Key:         "url",
				Label:       "URL",
				Type:        "text",
				Required:    true,
				Description: "Supports {{variable}}",
			},
			"headers": {
				Key:      "headers",
				Label:    "Headers",
				Type:     "json",
				Required: false,
			},
			"body": {
				Key:         "body",
				Label:       "Body",
				Type:        "textarea",
				Required:    false,
				Description: "Request body (supports {{variable}})",
			},
			"timeout_ms": {
				Key:      "timeout_ms",
				Label:    "Timeout (ms)",
				Type:     "number",
				Required: false,
				Default:  30000,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeCode,
		Name:        "Code",
		Description: "Execute code (Python/JavaScript)",
		Icon:        "code",
		Color:       "#607d8b",
		Category:    "action",
		Execute:     executeCode,
		ConfigSchema: map[string]TaskField{
			"language": {
				Key:      "language",
				Label:    "Language",
				Type:     "select",
				Required: true,
				Default:  "python",
				Options:  []string{"python", "javascript"},
			},
			"code": {
				Key:         "code",
				Label:       "Code",
				Type:        "textarea",
				Required:    true,
				Description: "Use input['var'] to access variables",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeCondition,
		Name:        "Condition",
		Description: "Branch based on condition",
		Icon:        "git_branch",
		Color:       "#ff5722",
		Category:    "flow",
		Execute:     executeCondition,
		ConfigSchema: map[string]TaskField{
			"condition": {
				Key:         "condition",
				Label:       "Condition",
				Type:        "text",
				Required:    true,
				Description: "Expression like {{var}} == 'value'",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeKnowledge,
		Name:        "Knowledge",
		Description: "Retrieve from knowledge base",
		Icon:        "menu_book",
		Color:       "#8bc34a",
		Category:    "ai",
		Execute:     executeKnowledge,
		ConfigSchema: map[string]TaskField{
			"knowledge_base_id": {
				Key:      "knowledge_base_id",
				Label:    "Knowledge Base ID",
				Type:     "select",
				Required: true,
			},
			"query": {
				Key:         "query",
				Label:       "Query",
				Type:        "text",
				Required:    false,
				Description: "Search query (supports {{variable}})",
			},
			"top_k": {
				Key:      "top_k",
				Label:    "Top K Results",
				Type:     "number",
				Required: false,
				Default:  5,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeTemplate,
		Name:        "Template",
		Description: "Transform data with template",
		Icon:        "description",
		Color:       "#795548",
		Category:    "data",
		Execute:     executeTemplate,
		ConfigSchema: map[string]TaskField{
			"template": {
				Key:         "template",
				Label:       "Template",
				Type:        "textarea",
				Required:    true,
				Description: "Output template with {{variable}}",
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeVariable,
		Name:        "Variable",
		Description: "Set or update variables",
		Icon:        "data_object",
		Color:       "#ffc107",
		Category:    "data",
		Execute:     executeVariable,
		ConfigSchema: map[string]TaskField{
			"assignments": {
				Key:         "assignments",
				Label:       "Assignments",
				Type:        "json",
				Required:    true,
				Description: `{"var_name": "{{source}}"}`,
			},
		},
	})

	RegisterTask(&TaskDefinition{
		Type:        TaskTypeMerge,
		Name:        "Merge",
		Description: "Merge multiple branches",
		Icon:        "merge_type",
		Color:       "#673ab7",
		Category:    "flow",
		Execute:     executeMerge,
		ConfigSchema: map[string]TaskField{
			"merge_mode": {
				Key:         "merge_mode",
				Label:       "Merge Mode",
				Type:        "select",
				Required:    false,
				Default:     "all",
				Options:     []string{"all", "first", "join"},
				Description: "all: collect all, first: take first, join: concatenate",
			},
		},
	})
}

func executeStart(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	// 获取 start 节点定义的输入参数
	inputSchema, _ := input.Config["input_schema"].(map[string]any)

	// 将定义的输入参数设置到 VarContext
	if inputSchema != nil {
		for key, val := range inputSchema {
			input.VarContext.SetInputVariable(key, val)
		}
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":    "start",
			"message": input.UserMessage,
		},
	}, nil
}

func executeEnd(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	output := input.Variables["last_output"]
	if output == nil {
		output = input.UserMessage
	}
	return &TaskOutput{
		Data: map[string]any{
			"type":   "end",
			"output": output,
		},
	}, nil
}

func executeAgent(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	agentID, ok := input.Config["agent_id"]
	if !ok {
		return &TaskOutput{Error: "agent_id not configured"}, nil
	}

	id, ok := agentID.(float64)
	if !ok {
		return &TaskOutput{Error: "invalid agent_id"}, nil
	}

	msg := buildInputMessage(input)

	// Need runtime reference - will be injected
	if agentRuntime == nil {
		return &TaskOutput{Error: "agent runtime not initialized"}, nil
	}

	output, err := agentRuntime.Chat(ctx, int64(id), msg, "", "")
	if err != nil {
		return &TaskOutput{Error: err.Error()}, nil
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":    "agent",
			"content": output,
		},
	}, nil
}

func executeLLM(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	prompt, _ := input.Config["prompt"].(string)
	if prompt == "" {
		prompt = input.UserMessage
	} else {
		prompt = resolveTemplate(prompt, input)
	}

	if agentID, ok := input.Config["agent_id"]; ok {
		if aid, ok := agentID.(float64); ok {
			if agentRuntime == nil {
				return &TaskOutput{Error: "agent runtime not initialized"}, nil
			}
			output, err := agentRuntime.Chat(ctx, int64(aid), prompt, "", "")
			if err != nil {
				return &TaskOutput{Error: err.Error()}, nil
			}
			return &TaskOutput{
				Data: map[string]any{
					"type":    "llm",
					"content": output,
				},
			}, nil
		}
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":    "llm",
			"content": prompt,
		},
	}, nil
}

func executeTool(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	toolName, ok := input.Config["tool_name"]
	if !ok {
		return &TaskOutput{Error: "tool_name not configured"}, nil
	}

	toolInput := buildInputMessage(input)
	if ti, ok := input.Config["tool_input"].(string); ok && ti != "" {
		toolInput = resolveTemplate(ti, input)
	}

	if toolRegistry == nil {
		return &TaskOutput{
			Data: map[string]any{
				"type":   "tool",
				"tool":   toolName,
				"input":  toolInput,
				"result": "tool registry not initialized",
			},
		}, nil
	}

	tool, ok := toolRegistry[toolName.(string)]
	if !ok {
		return &TaskOutput{
			Data: map[string]any{
				"type":   "tool",
				"tool":   toolName,
				"input":  toolInput,
				"result": fmt.Sprintf("tool not found: %v", toolName),
			},
		}, nil
	}

	result, err := tool(ctx, toolInput)
	if err != nil {
		return &TaskOutput{
			Data: map[string]any{
				"type":   "tool",
				"tool":   toolName,
				"input":  toolInput,
				"result": result,
				"error":  err.Error(),
			},
		}, nil
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":   "tool",
			"tool":   toolName,
			"input":  toolInput,
			"result": result,
		},
	}, nil
}

func executeHTTP(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	method, _ := input.Config["method"].(string)
	if method == "" {
		method = "GET"
	}

	url, _ := input.Config["url"].(string)
	if url == "" {
		return &TaskOutput{Error: "url not configured"}, nil
	}
	url = resolveTemplate(url, input)

	body, _ := input.Config["body"].(string)
	if body != "" {
		body = resolveTemplate(body, input)
	}

	timeout := 30000
	if t, ok := input.Config["timeout_ms"].(float64); ok {
		timeout = int(t)
	}

	headers := make(map[string]string)
	if h, ok := input.Config["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = resolveTemplate(s, input)
			}
		}
	}

	result, statusCode, err := doHTTPRequest(ctx, method, url, headers, body, timeout)
	if err != nil {
		return &TaskOutput{
			Data: map[string]any{
				"type":        "http",
				"method":      method,
				"url":         url,
				"status_code": statusCode,
				"error":       err.Error(),
			},
		}, nil
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":        "http",
			"method":      method,
			"url":         url,
			"status_code": statusCode,
			"body":        result,
		},
	}, nil
}

func executeCode(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	language, _ := input.Config["language"].(string)
	if language == "" {
		language = "python"
	}

	code, _ := input.Config["code"].(string)
	if code == "" {
		return &TaskOutput{Error: "code not configured"}, nil
	}

	// 使用 VariableContext 解析模板变量
	resolvedCode := ResolveTemplateForCode(code, input.VarContext)

	// 构建输入变量 (兼容旧代码)
	codeInput := map[string]any{
		"input":        input.UserMessage,
		"variables":    input.Variables,
		"node_outputs": input.NodeOutputs,
	}

	// 优先使用 Docker 沙箱，如果不可用则回退到直接执行
	result, err := ExecuteCode(ctx, language, resolvedCode, codeInput)
	if err != nil {
		return &TaskOutput{
			Data: map[string]any{
				"type":     "code",
				"language": language,
				"error":    err.Error(),
			},
		}, nil
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":     "code",
			"language": language,
			"output":   result,
		},
	}, nil
}

func executeCondition(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	condition, _ := input.Config["condition"].(string)
	if condition == "" {
		return &TaskOutput{Error: "condition not configured"}, nil
	}

	result := evaluateConditionExpr(condition, input)
	return &TaskOutput{
		Data: map[string]any{
			"type":      "condition",
			"condition": condition,
			"result":    result,
			"branch":    map[bool]string{true: "true", false: "false"}[result],
		},
	}, nil
}

func executeKnowledge(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	kbID, ok := input.Config["knowledge_base_id"]
	if !ok {
		return &TaskOutput{Error: "knowledge_base_id not configured"}, nil
	}

	query := input.UserMessage
	if q, ok := input.Config["query"].(string); ok && q != "" {
		query = resolveTemplate(q, input)
	}

	topK := 5
	if k, ok := input.Config["top_k"].(float64); ok {
		topK = int(k)
	}

	// Placeholder - actual implementation depends on knowledge base service
	return &TaskOutput{
		Data: map[string]any{
			"type":              "knowledge",
			"knowledge_base_id": kbID,
			"query":             query,
			"top_k":             topK,
			"results":           []any{},
			"note":              "knowledge base not yet implemented",
		},
	}, nil
}

func executeTemplate(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	tmpl, _ := input.Config["template"].(string)
	if tmpl == "" {
		return &TaskOutput{Error: "template not configured"}, nil
	}

	output := resolveTemplate(tmpl, input)
	return &TaskOutput{
		Data: map[string]any{
			"type":   "template",
			"output": output,
		},
	}, nil
}

func executeVariable(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	assignments, ok := input.Config["assignments"]
	if !ok {
		return &TaskOutput{Error: "assignments not configured"}, nil
	}

	assigned := make(map[string]any)
	if am, ok := assignments.(map[string]any); ok {
		for key, val := range am {
			if s, ok := val.(string); ok {
				assigned[key] = resolveTemplate(s, input)
			} else {
				assigned[key] = val
			}
		}
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":     "variable",
			"assigned": assigned,
		},
	}, nil
}

func executeMerge(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	mode, _ := input.Config["merge_mode"].(string)
	if mode == "" {
		mode = "all"
	}

	var outputs []any
	for _, edge := range input.NodeOutputs {
		outputs = append(outputs, edge)
	}

	var result any
	switch mode {
	case "first":
		if len(outputs) > 0 {
			result = outputs[0]
		}
	case "join":
		var parts []string
		for _, o := range outputs {
			parts = append(parts, fmt.Sprintf("%v", o))
		}
		result = strings.Join(parts, "\n---\n")
	default:
		result = outputs
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":    "merge",
			"mode":    mode,
			"outputs": outputs,
			"result":  result,
		},
	}, nil
}

func buildInputMessage(input *TaskInput) string {
	msg := input.UserMessage
	if len(input.NodeOutputs) > 0 {
		var parts []string
		for nodeID, output := range input.NodeOutputs {
			parts = append(parts, fmt.Sprintf("[%s]: %v", nodeID, output))
		}
		msg = fmt.Sprintf("[Previous outputs]\n%s\n\n[Current input]\n%s", strings.Join(parts, "\n"), msg)
	}
	return msg
}

func resolveTemplate(tmpl string, input *TaskInput) string {
	return resolveTemplateValue(tmpl, input.Variables, input.NodeOutputs)
}

func executeLoop(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	mode, _ := input.Config["mode"].(string)
	if mode == "" {
		mode = "count"
	}

	maxIter := 10
	if mi, ok := input.Config["max_iterations"].(float64); ok {
		maxIter = int(mi)
	}

	var items []any
	var count int

	switch mode {
	case "count":
		if c, ok := input.Config["count"].(float64); ok {
			count = int(c)
		} else {
			count = 5
		}
		items = make([]any, count)
		for i := 0; i < count; i++ {
			items[i] = i
		}
	case "items":
		if it, ok := input.Config["items"].(string); ok && it != "" {
			if strings.HasPrefix(it, "[") {
				if data := []byte(it); len(data) > 0 {
					if arr, err := strconv.ParseComplex(string(data), 0); err == nil {
						_ = arr
					}
				}
			} else {
				parts := strings.Split(it, ",")
				for _, p := range parts {
					items = append(items, strings.TrimSpace(p))
				}
			}
		}
	}

	if len(items) > maxIter {
		items = items[:maxIter]
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":          "loop",
			"mode":          mode,
			"items":         items,
			"count":         len(items),
			"current_index": 0,
		},
	}, nil
}

func executeBranch(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	condition, _ := input.Config["condition"].(string)
	if condition == "" {
		return &TaskOutput{Error: "condition not configured"}, nil
	}

	result := evaluateConditionExpr(condition, input)
	trueNode, _ := input.Config["true_node"].(string)
	falseNode, _ := input.Config["false_node"].(string)

	selectedNode := ""
	if result {
		selectedNode = trueNode
	} else {
		selectedNode = falseNode
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":          "branch",
			"condition":     condition,
			"result":        result,
			"selected_node": selectedNode,
		},
	}, nil
}

func executeParallel(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	nodesStr, _ := input.Config["nodes"].(string)
	if nodesStr == "" {
		return &TaskOutput{Error: "nodes not configured"}, nil
	}

	timeout := 60000
	if tm, ok := input.Config["timeout_ms"].(float64); ok {
		timeout = int(tm)
	}

	nodeIDs := strings.Split(nodesStr, ",")
	for i := range nodeIDs {
		nodeIDs[i] = strings.TrimSpace(nodeIDs[i])
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":       "parallel",
			"node_ids":   nodeIDs,
			"timeout_ms": timeout,
			"note":       "parallel execution requires graph engine support",
		},
	}, nil
}

func executeWait(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	duration := 0
	if d, ok := input.Config["duration_ms"].(float64); ok {
		duration = int(d)
	}

	condition, hasCond := input.Config["condition"].(string)
	maxWait := 300000
	if mw, ok := input.Config["max_wait_ms"].(float64); ok {
		maxWait = int(mw)
	}

	if hasCond && condition != "" {
		result := evaluateConditionExpr(condition, input)
		if result {
			return &TaskOutput{
				Data: map[string]any{
					"type":    "wait",
					"status":  "condition_met",
					"elapsed": 0,
				},
			}, nil
		}

		elapsed := 0
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return &TaskOutput{
					Data: map[string]any{
						"type":   "wait",
						"status": "cancelled",
					},
				}, nil
			case <-ticker.C:
				elapsed += 1000
				if evaluateConditionExpr(condition, input) {
					return &TaskOutput{
						Data: map[string]any{
							"type":    "wait",
							"status":  "condition_met",
							"elapsed": elapsed,
						},
					}, nil
				}
				if elapsed >= maxWait {
					return &TaskOutput{
						Data: map[string]any{
							"type":    "wait",
							"status":  "timeout",
							"elapsed": elapsed,
						},
					}, nil
				}
			}
		}
	}

	if duration > 0 {
		timer := time.NewTimer(time.Duration(duration) * time.Millisecond)
		select {
		case <-ctx.Done():
			return &TaskOutput{
				Data: map[string]any{
					"type":   "wait",
					"status": "cancelled",
				},
			}, nil
		case <-timer.C:
			return &TaskOutput{
				Data: map[string]any{
					"type":    "wait",
					"status":  "completed",
					"elapsed": duration,
				},
			}, nil
		}
	}

	return &TaskOutput{
		Data: map[string]any{
			"type":   "wait",
			"status": "no_condition_or_duration",
		},
	}, nil
}
