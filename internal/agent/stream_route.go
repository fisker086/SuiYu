package agent

import (
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
)

// streamClientType normalizes JSON client_type for routing. Only "desktop" selects desktop-specific
// behavior; everything else is treated as web.
func streamClientType(clientType string) string {
	if strings.EqualFold(strings.TrimSpace(clientType), "desktop") {
		return "desktop"
	}
	return "web"
}

// StreamClientType is exported for chat service (e.g. POST /chat/tool_result/stream resume) to match openChatStream.
func StreamClientType(clientType string) string {
	return streamClientType(clientType)
}

// resolveChatStreamRoute mirrors openChatStream dispatch; single source of truth for logging + switch.
func resolveChatStreamRoute(agent *schema.AgentWithRuntime, ct string, chatTools []tool.BaseTool) string {
	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}
	n := len(chatTools)

	// auto mode goes through react stream; complexity is resolved at runtime via resolveExecutionModeAuto
	if execMode == ExecutionModeAuto && n > 0 {
		return "plan_execute_stream"
	}

	switch {
	case execMode == ExecutionModeReAct && n > 0:
		return "react_stream"
	// Plan-and-execute must use runPlanAndExecuteStream even when the agent has zero tools: otherwise we
	// fall through to plain_llm_stream, which errors on tool-call chunks (UserVisibleStreamFailure generic line)
	// and never emits plan_tasks / plan_step for the desktop checklist.
	case execMode == ExecutionModePlanExecute:
		return "plan_execute_stream"
	case execMode != ExecutionModeReAct && execMode != ExecutionModePlanExecute && ct == "desktop" && n > 0 && HasAnyClientExecutionTool(chatTools, skillExecOverrides(agent)):
		return "react_stream_desktop_client_tools"
	case n > 0:
		return "adk_tool_loop"
	default:
		return "plain_llm_stream"
	}
}

func logChatStreamRoute(agent *schema.AgentWithRuntime, ct string, chatTools []tool.BaseTool, route string) {
	logger.Info("chat stream route",
		"agent_id", agent.ID,
		"exec_mode", executionModeOf(agent),
		"client_type", ct,
		"tools", len(chatTools),
		"route", route,
	)
}

func executionModeOf(agent *schema.AgentWithRuntime) string {
	if agent.RuntimeProfile != nil {
		return agent.RuntimeProfile.ExecutionMode
	}
	return ExecutionModeDefault
}
