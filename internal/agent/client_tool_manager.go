package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

const ClientToolCallTimeout = 5 * time.Minute

// countAssistantToolTurns counts assistant messages that carry tool_calls (for log diagnostics).
func countAssistantToolTurns(msgs []*einoschema.Message) int {
	n := 0
	for _, m := range msgs {
		if m != nil && m.Role == einoschema.Assistant && len(m.ToolCalls) > 0 {
			n++
		}
	}
	return n
}

type ClientToolCallState struct {
	CallID     string
	ToolName   string
	ToolArgs   string
	Messages   []*einoschema.Message
	Iter       int
	CreatedAt  time.Time
	ClientType string

	// Plan-and-execute specific fields
	PlanMode     bool
	PlanText     string
	PlanIndex    int
	PlanSteps    []PlanStep
	UserInput    string
	SystemPrompt string
}

type ClientToolManager struct {
	mu     sync.RWMutex
	states map[string]*ClientToolCallState
}

func NewClientToolManager() *ClientToolManager {
	return &ClientToolManager{
		states: make(map[string]*ClientToolCallState),
	}
}

func (m *ClientToolManager) SaveState(state *ClientToolCallState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.CallID] = state
	logger.Info("client_tool: state saved",
		"call_id", logger.CallIDForLog(state.CallID),
		"tool", state.ToolName,
		"client_type", state.ClientType,
		"plan_mode", state.PlanMode,
		"plan_index", state.PlanIndex,
		"iter", state.Iter,
		"msgs_len", len(state.Messages),
		"assistant_tool_turns", countAssistantToolTurns(state.Messages),
	)
}

func (m *ClientToolManager) GetState(callID string) (*ClientToolCallState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.states[callID]
	return s, ok
}

func (m *ClientToolManager) GetStatePlanMode(callID string) (bool, error) {
	m.mu.RLock()
	s, ok := m.states[callID]
	m.mu.RUnlock()
	if !ok {
		logger.Warn("client_tool: plan_mode lookup failed (no saved state)", "call_id", logger.CallIDForLog(callID))
		return false, fmt.Errorf("tool call state not found: %s", callID)
	}
	return s.PlanMode, nil
}

func (m *ClientToolManager) DeleteState(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, callID)
}

func (m *ClientToolManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.states {
		if now.Sub(s.CreatedAt) > ClientToolCallTimeout {
			delete(m.states, id)
		}
	}
}

func (m *ClientToolManager) ResumeState(callID string, result string, toolErr string) (*ClientToolCallState, []*einoschema.Message, error) {
	m.mu.Lock()
	s, ok := m.states[callID]
	if !ok {
		m.mu.Unlock()
		logger.Warn("client_tool: resume failed (no saved state)", "call_id", logger.CallIDForLog(callID))
		return nil, nil, fmt.Errorf("tool call state not found: %s", callID)
	}
	delete(m.states, callID)
	m.mu.Unlock()

	msgsBefore := len(s.Messages)
	assistToolBefore := countAssistantToolTurns(s.Messages)

	msgs := s.Messages
	if toolErr != "" {
		msgs = append(msgs, &einoschema.Message{
			Role:       einoschema.Tool,
			Content:    fmt.Sprintf("Error: %s", toolErr),
			ToolCallID: callID,
			Name:       s.ToolName,
		})
	} else {
		msgs = append(msgs, &einoschema.Message{
			Role:       einoschema.Tool,
			Content:    result,
			ToolCallID: callID,
			Name:       s.ToolName,
		})
	}

	logger.Info("client_tool: state resumed",
		"call_id", logger.CallIDForLog(callID),
		"tool", s.ToolName,
		"plan_mode", s.PlanMode,
		"plan_index", s.PlanIndex,
		"iter", s.Iter,
		"msgs_before", msgsBefore,
		"msgs_after", len(msgs),
		"assistant_tool_turns_before", assistToolBefore,
		"assistant_tool_turns_after", countAssistantToolTurns(msgs),
		"has_tool_err", toolErr != "",
		"result_len", len(result),
	)

	return s, msgs, nil
}

func skillExecOverrides(agent *schema.AgentWithRuntime) map[string]string {
	if agent == nil || len(agent.SkillExecutionOverrides) == 0 {
		return nil
	}
	return agent.SkillExecutionOverrides
}

func lookupSkillExecutionOverride(overrides map[string]string, toolName string) (string, bool) {
	if overrides == nil || toolName == "" {
		return "", false
	}
	if m, ok := overrides[toolName]; ok {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == schema.ExecutionModeClient || m == schema.ExecutionModeServer {
			return m, true
		}
	}
	if strings.HasPrefix(toolName, "builtin_") && !strings.HasPrefix(toolName, "builtin_skill.") {
		sk := "builtin_skill." + strings.TrimPrefix(toolName, "builtin_")
		if m, ok := overrides[sk]; ok {
			m = strings.TrimSpace(strings.ToLower(m))
			if m == schema.ExecutionModeClient || m == schema.ExecutionModeServer {
				return m, true
			}
		}
	}
	return "", false
}

func isClientTool(tools []tool.BaseTool, name string, overrides map[string]string) bool {
	return getToolExecutionModeFromTools(tools, name, overrides) == schema.ExecutionModeClient
}

func clientToolHint(toolName string) string {
	hints := map[string]string{
		"builtin_docker_operator": "请在本地终端执行 docker 命令，将结果返回给我",
		"builtin_git_operator":    "请在本地执行 git 命令，将结果返回给我",
		"builtin_office_doc":      "请在本地解析文档内容，将结果返回给我",
		"builtin_browser":         "桌面端 builtin_browser 会启动可见 Chrome（CDP）并在当前页操作，将页面文本/操作结果返回给模型；不要说是无头或后台 HTTP。若用户关掉了窗口，可再次调用以重新启动。服务端拉页请用 builtin_http_client。",
	}
	if h, ok := hints[toolName]; ok {
		return h
	}
	return fmt.Sprintf("请在本地执行 %s，将结果返回给我", toolName)
}

// GetToolExecutionMode returns client vs server for a tool name (aligned with skill_service.defaultSkillExecutionMode).
// Used only as the last fallback when neither DB overrides nor tool Extra supply execution_mode; see getToolExecutionModeFromTools.
func GetToolExecutionMode(toolName string) string {
	var skillKey string
	if strings.HasPrefix(toolName, "builtin_skill.") {
		skillKey = toolName
	} else if strings.HasPrefix(toolName, "builtin_") {
		skillKey = "builtin_skill." + strings.TrimPrefix(toolName, "builtin_")
	} else {
		return schema.ExecutionModeServer
	}
	// builtin_browser is client-only (browser_client skill); tool Extra sets execution_mode=client.
	switch skillKey {
	case "builtin_skill.docker_operator", "builtin_skill.git_operator", "builtin_skill.file_parser",
		"builtin_skill.system_monitor", "builtin_skill.cron_manager", "builtin_skill.network_tools",
		"builtin_skill.cert_checker", "builtin_skill.nginx_diagnose", "builtin_skill.dns_lookup",
		"builtin_skill.datetime", "builtin_skill.regex", "builtin_skill.json_parser",
		"builtin_skill.csv_analyzer", "builtin_skill.log_analyzer", "builtin_skill.image_analyzer",
		"builtin_skill.terraform_plan", "builtin_skill.redis_tool", "builtin_skill.browser_client":
		return schema.ExecutionModeClient
	default:
		return schema.ExecutionModeServer
	}
}

// HasAnyClientExecutionTool is true if any registered tool is configured for client execution (DB first, then Extra, then builtin default).
// Used to route desktop /chat/stream to ReAct so Tauri can receive client_tool_call instead of ADK server-only tool_call.
func HasAnyClientExecutionTool(tools []tool.BaseTool, overrides map[string]string) bool {
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		if getToolExecutionModeFromTools(tools, info.Name, overrides) == schema.ExecutionModeClient {
			return true
		}
	}
	return false
}

// getToolExecutionModeFromTools resolves client|server with fixed precedence:
//  1. DB (skills.execution_mode for skills bound to the agent, via overrides)
//  2. tool Info Extra from SKILL.md / generated code (when DB has no row or no mode for that binding)
//  3. built-in default by skill key (GetToolExecutionMode)
func getToolExecutionModeFromTools(tools []tool.BaseTool, name string, overrides map[string]string) string {
	if m, ok := lookupSkillExecutionOverride(overrides, name); ok {
		return m
	}
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		if info.Name != name {
			continue
		}
		if mode, ok := info.Extra["execution_mode"]; ok {
			if s, ok := mode.(string); ok {
				s = strings.TrimSpace(strings.ToLower(s))
				if s == schema.ExecutionModeClient || s == schema.ExecutionModeServer {
					return s
				}
			}
		}
		break
	}
	return GetToolExecutionMode(name)
}

// ResolveToolExecutionMode returns effective client|server using the same precedence as getToolExecutionModeFromTools.
func ResolveToolExecutionMode(agent *schema.AgentWithRuntime, tools []tool.BaseTool, toolName string) string {
	return getToolExecutionModeFromTools(tools, toolName, skillExecOverrides(agent))
}

// SkillExecutionOverridesFromStore builds tool-name → client|server from the skills table for skills bound to the agent
// (runtime skill_ids: builtin_skill.* keys and/or numeric skill id strings).
// When skills.execution_mode is set in DB, it overrides SKILL.md / code Extra for that skill; empty DB mode falls back to Extra then defaults.
func SkillExecutionOverridesFromStore(store storage.Storage, ag *schema.AgentWithRuntime) map[string]string {
	if store == nil || ag == nil {
		return nil
	}
	ids := skillIDsForAgent(ag)
	if len(ids) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		s := strings.TrimSpace(id)
		if s != "" {
			selected[s] = struct{}{}
		}
	}
	list, err := store.ListSkills()
	if err != nil {
		return nil
	}
	out := make(map[string]string)
	for _, sk := range list {
		if sk == nil {
			continue
		}
		key := strings.TrimSpace(sk.Key)
		if key == "" {
			continue
		}
		idStr := strconv.FormatInt(sk.ID, 10)
		_, byKey := selected[key]
		_, byID := selected[idStr]
		if !byKey && !byID {
			continue
		}
		mode := strings.TrimSpace(strings.ToLower(sk.ExecutionMode))
		if mode != schema.ExecutionModeClient && mode != schema.ExecutionModeServer {
			continue
		}
		out[key] = mode
		if strings.HasPrefix(key, "builtin_skill.") {
			short := "builtin_" + strings.TrimPrefix(key, "builtin_skill.")
			out[short] = mode
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
