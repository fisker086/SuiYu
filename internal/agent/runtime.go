package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/approval"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/mcp"
	agentmodel "github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/skills"
	storepkg "github.com/fisk086/sya/internal/storage"
)

type Runtime struct {
	mu         sync.RWMutex
	agents     map[int64]*schema.AgentWithRuntime
	byName     map[string]*schema.AgentWithRuntime
	byCategory map[string][]*schema.AgentWithRuntime

	chatModel        model.ToolCallingChatModel
	mcpClient        *mcp.Client
	skillLoader      *skills.Loader
	skillRegistry    *skills.Registry
	store            any
	auditLogger      *AuditLogger
	clientToolMgr    *ClientToolManager
	approvalProvider approval.ExternalApprovalProvider
	approvalNotifier approval.ApprovalNotifier

	stopStreams map[string]chan struct{}

	usageSink TokenUsageSink

	// defaultChatModelName matches server OPENAI_MODEL / ARK_MODEL when agent has no per-agent llm_model.
	defaultChatModelName string

	// groupCoordinator provides group capability discovery and coordination.
	groupCoordinator *GroupCoordinator
}

func NewRuntime() *Runtime {
	return &Runtime{
		agents:      make(map[int64]*schema.AgentWithRuntime),
		byName:      make(map[string]*schema.AgentWithRuntime),
		byCategory:  make(map[string][]*schema.AgentWithRuntime),
		stopStreams: make(map[string]chan struct{}),
	}
}

func NewRuntimeWithSkill(chatModel model.ToolCallingChatModel, mcpClient *mcp.Client, skillLoader *skills.Loader, skillRegistry *skills.Registry, store any) *Runtime {
	r := NewRuntime()
	r.chatModel = chatModel
	r.mcpClient = mcpClient
	r.skillLoader = skillLoader
	r.skillRegistry = skillRegistry
	r.store = store
	r.clientToolMgr = NewClientToolManager()
	if as, ok := store.(AuditStore); ok {
		r.auditLogger = NewAuditLogger(as)
	}
	return r
}

func (r *Runtime) AuditLogger() *AuditLogger {
	return r.auditLogger
}

func (r *Runtime) GetStatePlanMode(callID string) (bool, error) {
	return r.clientToolMgr.GetStatePlanMode(callID)
}

func (r *Runtime) resolveToolRiskLevel(toolName string) string {
	if store, ok := r.store.(storepkg.Storage); ok {
		if skills, err := store.ListSkills(); err == nil {
			skillKey := normalizeToolToSkillKey(toolName)
			for _, sk := range skills {
				if sk == nil || sk.Key != skillKey {
					continue
				}
				if s := strings.TrimSpace(sk.RiskLevel); s != "" {
					return strings.ToLower(s)
				}
				break
			}
		}
	}
	return getToolRiskLevel(toolName)
}

func (r *Runtime) RegisterAgent(agent *schema.AgentWithRuntime) {
	if agent == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.ID] = agent
	r.byName[agent.Name] = agent
	r.byCategory[agent.Category] = append(r.byCategory[agent.Category], agent)
}

func (r *Runtime) UnregisterAgent(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[id]
	if !ok {
		return
	}
	delete(r.agents, id)
	delete(r.byName, agent.Name)
}

func (r *Runtime) GetAgent(id int64) (*schema.AgentWithRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

// agentWithSkillOverrides returns a shallow copy of the registered agent with SkillExecutionOverrides filled from DB
// (skills.execution_mode wins over SKILL/code defaults when both differ). UI changes apply on the next request without restart.
func (r *Runtime) agentWithSkillOverrides(src *schema.AgentWithRuntime) *schema.AgentWithRuntime {
	if src == nil {
		return nil
	}
	out := *src
	if st, ok := r.store.(storepkg.Storage); ok {
		out.SkillExecutionOverrides = SkillExecutionOverridesFromStore(st, &out)
	}
	return &out
}

func (r *Runtime) GetAgentByName(name string) (*schema.AgentWithRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.byName[name]
	return agent, ok
}

func (r *Runtime) SetGroupCoordinator(coordinator *GroupCoordinator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groupCoordinator = coordinator
}

func (r *Runtime) GetGroupCoordinator() *GroupCoordinator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.groupCoordinator
}

func (r *Runtime) ListAgents() []*schema.AgentWithRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*schema.AgentWithRuntime, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (r *Runtime) ListAgentsByCategory(category string) []*schema.AgentWithRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byCategory[category]
}

func (r *Runtime) RegisterStreamStop(sessionID string) chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan struct{})
	r.stopStreams[sessionID] = ch
	return ch
}

func (r *Runtime) StopStream(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.stopStreams[sessionID]; ok {
		close(ch)
		delete(r.stopStreams, sessionID)
	}
}

func (r *Runtime) ChatWithMemoryContext(ctx context.Context, agentID int64, message, imageBase64, imageMime string, memContext string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, error) {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}

	historyWithMemory := historyMsgs
	if memContext != "" {
		// 将记忆作为 user 消息注入，而非拼接到 system prompt
		// 防止用户注入的恶意指令获得 system 权限
		historyWithMemory = append([]*einoschema.Message{
			{Role: einoschema.User, Content: "[记忆上下文]\n" + memContext},
		}, historyMsgs...)
	}

	resp, _, err := r.runAgent(ctx, agent, systemPrompt, message, historyWithMemory, sessionID, auditUserID)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// ChatWithMemoryContextSchedule is like ChatWithMemoryContext but returns ReAct step payloads for agent_memory.extra.react_steps
// (scheduled runs: chat UI can show thinking cards). Callers should send only the reply string to external clients (e.g. Lark).
func (r *Runtime) ChatWithMemoryContextSchedule(ctx context.Context, agentID int64, message, imageBase64, imageMime string, memContext string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, []map[string]any, error) {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}
	historyWithMemory := historyMsgs
	if memContext != "" {
		historyWithMemory = append([]*einoschema.Message{
			{Role: einoschema.User, Content: "[记忆上下文]\n" + memContext},
		}, historyMsgs...)
	}
	resp, reactRes, err := r.runAgent(ctx, agent, systemPrompt, message, historyWithMemory, sessionID, auditUserID)
	if err != nil {
		return "", nil, err
	}
	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}
	if execMode != ExecutionModeReAct {
		return resp, nil, nil
	}
	payloads := ReActResultToReactPayloads(reactRes, resp)
	return resp, payloads, nil
}

func (r *Runtime) ChatWithUserProfile(ctx context.Context, agentID int64, message, imageBase64, imageMime, userProfile string, sessionID, auditUserID string, historyMsgs []*einoschema.Message) (string, error) {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return "", fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}

	if userProfile != "" {
		systemPrompt = userProfile + "\n\n" + systemPrompt
	}

	resp, _, err := r.runAgent(ctx, agent, systemPrompt, message, historyMsgs, sessionID, auditUserID)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (r *Runtime) Chat(ctx context.Context, agentID int64, message, sessionID, auditUserID string) (string, error) {
	return r.ChatWithMemoryContext(ctx, agentID, message, "", "", "", sessionID, auditUserID, nil)
}

// openChatStream is the single entry for POST /chat/stream after system prompt and tools are resolved.
// visionParts are passed on every path that supports multimodal (ReAct, ADK tool loop, plain stream).
//
// client_type adaptation (see also controller.NormalizeClientTypeFromUserAgent):
//   - desktop + default mode + client-execution tools → ReAct stream (SSE client_tool_call for Tauri).
//   - web (+ same agent) → ADK tool loop when tools exist; client-only tools get an info event (existing behavior inside runAgentCoreStream).
//   - ReAct / PlanExecute modes unchanged.
func (r *Runtime) openChatStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	message string,
	historyMsgs []*einoschema.Message,
	chatTools []tool.BaseTool,
	clientType string,
	visionParts []VisionPart,
	sessionID, auditUserID string,
	stopCh <-chan struct{},
) (io.ReadCloser, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)

	ct := streamClientType(clientType)
	route := resolveChatStreamRoute(agent, ct, chatTools)
	logChatStreamRoute(agent, ct, chatTools, route)

	switch route {
	case "react_stream", "react_stream_desktop_client_tools":
		return r.runReActLoopStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, ct, visionParts, sessionID, auditUserID, stopCh)
	case "plan_execute_stream":
		return r.runPlanAndExecuteStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, ct, sessionID, auditUserID)
	case "adk_tool_loop":
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			defer r.flushUsageSession(ctx)
			if err := r.runAgentCoreStream(ctx, pw, agent, systemPrompt, message, historyMsgs, chatTools, ct, visionParts); err != nil {
				// Log the real error: UserVisibleStreamFailure(err) shown to the client is intentionally generic.
				logger.Warn("chat stream adk", "phase", "run_failed", "agent_id", agent.ID, "err", err)
				// Emit a normal SSE completion so clients see a message and history is not [user,user] without assistant.
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
					return
				}
				return
			}
			if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
				_ = pw.CloseWithError(err)
			}
		}()
		return pr, nil
	}

	instruction := systemPrompt
	if instruction == "" {
		instruction = "You are a helpful AI assistant."
	}

	msgs := buildStreamChatMessages(instruction, historyMsgs, message, visionParts)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer r.flushUsageSession(ctx)
		logger.Info("chat stream plain", "phase", "start", "history_msgs", len(historyMsgs))
		sr, err := r.chatModel.Stream(ctx, msgs)
		if err != nil {
			logger.Warn("chat stream plain", "phase", "stream_open", "err", err)
			_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
			if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
				_ = pw.CloseWithError(werr)
			}
			return
		}
		defer sr.Close()
		chunkIdx := 0
		for {
			select {
			case <-stopCh:
				logger.Info("chat stream plain", "phase", "stopped", "chunks", chunkIdx)
				_ = pw.CloseWithError(io.EOF)
				return
			default:
			}
			chunk, err := sr.Recv()
			if errors.Is(err, io.EOF) {
				logger.Info("chat stream plain", "phase", "eof", "chunks", chunkIdx)
				break
			}
			if err != nil {
				logger.Warn("chat stream plain", "phase", "recv", "err", err, "chunks", chunkIdx)
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
				}
				return
			}
			if chunk == nil {
				continue
			}
			chunkIdx++
			if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
				}
				return
			}
			out := assistantChunkText(chunk)
			logger.Debug("chat stream plain", "phase", "chunk", "n", chunkIdx, "out_len", len(out), "tool_calls", len(chunk.ToolCalls))
			if err := streamStaticTextAsSSE(pw, out); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func (r *Runtime) ChatStreamWithUserProfile(ctx context.Context, agentID int64, message string, visionParts []VisionPart, userProfile string, historyMsgs []*einoschema.Message, stopCh <-chan struct{}, sessionID, auditUserID, clientType string) (io.ReadCloser, error) {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}
	if userProfile != "" {
		systemPrompt = userProfile + "\n\n" + systemPrompt
	}

	chatTools, err := r.allToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	chatTools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, chatTools)

	return r.openChatStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, clientType, visionParts, sessionID, auditUserID, stopCh)
}

func (r *Runtime) ChatStreamWithMemoryContext(ctx context.Context, agentID int64, message string, visionParts []VisionPart, memContext string, historyMsgs []*einoschema.Message, stopCh <-chan struct{}, sessionID, auditUserID, clientType string, groupPeers *GroupPeerContext) (io.ReadCloser, error) {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}
	if memContext != "" {
		systemPrompt = memContext + "\n\n" + systemPrompt
	}
	if groupPeers != nil && len(groupPeers.PeerAgentIDs) >= 2 {
		if hint := r.buildGroupPeerSystemHint(groupPeers); hint != "" {
			systemPrompt = hint + "\n\n" + systemPrompt
		}
		// Add group capability hint for multi-agent coordination
		if r.groupCoordinator != nil {
			if capHint := r.groupCoordinator.BuildGroupCapabilityHint(ctx, groupPeers.GroupID, groupPeers.CallerAgentID); capHint != "" {
				systemPrompt = systemPrompt + "\n\n" + capHint
			}
		}
		// Trailing reminder: models often weight the end of the system prompt.
		systemPrompt += "\n\n[Group peer reminder] To message another agent in this group, call tool `" + toolGroupSendMessage + "` with fields target_agent_id and message. Do not claim you only have email/DingTalk/HTTP; this tool is the supported path."
	}

	chatTools, err := r.allToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	if groupPeers != nil && len(groupPeers.PeerAgentIDs) >= 2 {
		if t := r.newGroupPeerMessageTool(groupPeers, sessionID, auditUserID, clientType); t != nil {
			chatTools = append(chatTools, t)
		}
	}
	chatTools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, chatTools)

	return r.openChatStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, clientType, visionParts, sessionID, auditUserID, stopCh)
}

// VisionPart is one user image for multimodal streaming chat.
type VisionPart struct {
	Base64 string
	Mime   string
}

func buildStreamChatMessages(instruction string, history []*einoschema.Message, userText string, visionParts []VisionPart) []*einoschema.Message {
	var msgs []*einoschema.Message
	if strings.TrimSpace(instruction) != "" {
		msgs = append(msgs, &einoschema.Message{
			Role:    einoschema.System,
			Content: instruction,
		})
	}
	if len(history) > 0 {
		msgs = append(msgs, history...)
	}
	msgs = append(msgs, buildStreamUserMessage(userText, visionParts))
	return msgs
}

func buildStreamUserMessage(userText string, visionParts []VisionPart) *einoschema.Message {
	if len(visionParts) == 0 {
		return &einoschema.Message{
			Role:    einoschema.User,
			Content: userText,
		}
	}
	var contentParts []einoschema.MessageInputPart
	if t := strings.TrimSpace(userText); t != "" {
		contentParts = append(contentParts, einoschema.MessageInputPart{Type: einoschema.ChatMessagePartTypeText, Text: userText})
	}
	for _, vp := range visionParts {
		b64 := strings.TrimSpace(vp.Base64)
		if b64 == "" {
			continue
		}
		mime := strings.TrimSpace(vp.Mime)
		if mime == "" {
			mime = "image/png"
		}
		b64Copy := b64
		contentParts = append(contentParts, einoschema.MessageInputPart{
			Type: einoschema.ChatMessagePartTypeImageURL,
			Image: &einoschema.MessageInputImage{
				MessagePartCommon: einoschema.MessagePartCommon{
					Base64Data: &b64Copy,
					MIMEType:   mime,
				},
			},
		})
	}
	return &einoschema.Message{
		Role:                  einoschema.User,
		UserInputMultiContent: contentParts,
	}
}

// streamChatModelTokensToSSE streams assistant text in the same SSE JSON shape as plain LLM chat.
// On success returns nil (caller should write data: [DONE]). On open/recv failure may write UserVisibleStreamFailure.
func (r *Runtime) streamChatModelTokensToSSE(ctx context.Context, w io.Writer, msgs []*einoschema.Message) error {
	sr, err := r.chatModel.Stream(ctx, msgs)
	if err != nil {
		_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
		return err
	}
	defer sr.Close()
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
			return err
		}
		if chunk == nil {
			continue
		}
		if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
			return fmt.Errorf("streaming with tool calls is not supported in this runtime")
		}
		out := assistantChunkText(chunk)
		if err := streamStaticTextAsSSE(w, out); err != nil {
			return err
		}
	}
}

type sseWriterFlusher interface {
	Flush() error
}

// streamChatModelTokensToSSEAccumulate streams plan/draft text as plain {"content":"..."} SSE chunks
// (one event per model chunk, low latency) and returns the full concatenated assistant text for parsing.
func (r *Runtime) streamChatModelTokensToSSEAccumulate(ctx context.Context, w io.Writer, msgs []*einoschema.Message) (string, error) {
	sr, err := r.chatModel.Stream(ctx, msgs)
	if err != nil {
		_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
		return "", err
	}
	defer sr.Close()
	var acc strings.Builder
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return acc.String(), nil
		}
		if err != nil {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
			return acc.String(), err
		}
		if chunk == nil {
			continue
		}
		if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
			return acc.String(), fmt.Errorf("streaming with tool calls is not supported in this runtime")
		}
		out := assistantChunkText(chunk)
		if out == "" {
			continue
		}
		acc.WriteString(out)
		if err := writeSSEJSON(w, out); err != nil {
			return acc.String(), err
		}
		if f, ok := w.(sseWriterFlusher); ok {
			_ = f.Flush()
		}
	}
}

func (r *Runtime) ResolveCapabilities(agentID int64) []schema.Capability {
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil
	}
	return agent.Capabilities
}

func (r *Runtime) runAgent(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, sessionID, auditUserID string) (string, *ReActResult, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)
	defer r.flushUsageSession(ctx)

	tools, err := r.allToolsForAgent(agent)
	if err != nil {
		return "", nil, err
	}
	tools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, tools)

	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}

	if execMode == ExecutionModeAuto {
		mode, err := r.resolveExecutionModeAuto(ctx, agent, userInput, history, tools)
		if err != nil {
			return "", nil, err
		}
		execMode = mode
		logger.Info("auto execution mode resolved", "agent_id", agent.ID, "resolved_mode", execMode, "user_input", userInput)
	}

	switch execMode {
	case ExecutionModeReAct:
		return r.runReActLoop(ctx, agent, systemPrompt, userInput, history, tools)
	case ExecutionModePlanExecute:
		s, _, err := r.runPlanAndExecute(ctx, agent, systemPrompt, userInput, history, tools, sessionID, auditUserID)
		return s, nil, err
	default:
		s, err := r.runAgentCore(ctx, agent, systemPrompt, userInput, history, tools)
		return s, nil, err
	}
}

func (r *Runtime) WrapToolsWithAudit(agent *schema.AgentWithRuntime, sessionID, auditUserID string, tools []tool.BaseTool) []tool.BaseTool {
	return r.wrapToolsWithAudit(agent, sessionID, auditUserID, tools)
}

func (r *Runtime) wrapToolsWithAudit(agent *schema.AgentWithRuntime, sessionID, auditUserID string, tools []tool.BaseTool) []tool.BaseTool {
	if r.auditLogger == nil {
		return tools
	}

	approvalMode := ""
	if agent.RuntimeProfile != nil {
		approvalMode = agent.RuntimeProfile.ApprovalMode
	}
	if approvalMode == "" {
		approvalMode = "auto"
	}

	wrapped := make([]tool.BaseTool, 0, len(tools))
	for _, t := range tools {
		if inv, ok := t.(tool.InvokableTool); ok {
			info, err := inv.Info(context.Background())
			if err != nil {
				wrapped = append(wrapped, t)
				continue
			}
			riskLevel := r.resolveToolRiskLevel(info.Name)
			wrapped = append(wrapped, &auditHITLWrapper{
				inner:           inv,
				toolName:        info.Name,
				riskLevel:       riskLevel,
				agentID:         agent.ID,
				sessionID:       sessionID,
				userID:          auditUserID,
				approvalMode:    approvalMode,
				auditLogger:     r.auditLogger,
				approvalChecker: r.buildApprovalChecker(agent, sessionID, auditUserID),
			})
		} else {
			wrapped = append(wrapped, t)
		}
	}
	return wrapped
}

func (r *Runtime) buildApprovalChecker(agent *schema.AgentWithRuntime, sessionID, auditUserID string) func(agentID int64, sessionID, toolName, riskLevel, input string) (bool, int64, error) {
	return func(agentID int64, sessionID, toolName, riskLevel, input string) (bool, int64, error) {
		approvalMode := ""
		approvalType := "internal"
		approvers := []string{}
		approvalTimeout := 30 // 默认30分钟

		if agent.RuntimeProfile != nil {
			approvalMode = agent.RuntimeProfile.ApprovalMode
			approvalType = agent.RuntimeProfile.ApprovalType
			approvers = agent.RuntimeProfile.Approvers
			if agent.RuntimeProfile.ApprovalTimeout > 0 {
				approvalTimeout = agent.RuntimeProfile.ApprovalTimeout
			}
		}
		if approvalMode == "" {
			approvalMode = "auto"
		}
		if approvalMode == "auto" {
			return true, 0, nil
		}

		if r.auditLogger == nil {
			return true, 0, nil
		}

		store, ok := r.auditLogger.store.(interface {
			CreateApprovalRequest(req *agentmodel.ApprovalRequest) (*agentmodel.ApprovalRequest, error)
		})
		if !ok {
			return true, 0, nil
		}

		var expiresAtPtr *time.Time
		if approvalTimeout > 0 {
			t := time.Now().Add(time.Duration(approvalTimeout) * time.Minute)
			expiresAtPtr = &t
		}

		req := &agentmodel.ApprovalRequest{
			AgentID:      agentID,
			SessionID:    sessionID,
			UserID:       auditUserID,
			ToolName:     toolName,
			RiskLevel:    riskLevel,
			Input:        input,
			Status:       "pending",
			CreatedAt:    time.Now(),
			ApprovalType: approvalType,
			ExpiresAt:    expiresAtPtr,
		}

		if approvalType != "internal" && len(approvers) > 0 {
			provider := r.approvalProvider
			if provider != nil {
				externalID, err := provider.SubmitApproval(context.Background(), &approval.ExternalApprovalRequest{
					AgentID:   agentID,
					SessionID: sessionID,
					UserID:    auditUserID,
					ToolName:  toolName,
					RiskLevel: riskLevel,
					Input:     input,
					Approvers: approvers,
					Title:     fmt.Sprintf("审批请求: %s (%s)", toolName, riskLevel),
					Timeout:   time.Duration(approvalTimeout) * time.Minute,
				})
				if err != nil {
					return false, 0, fmt.Errorf("failed to submit external approval: %w", err)
				}
				req.ExternalID = externalID
			}
		}

		createdReq, err := store.CreateApprovalRequest(req)
		if err != nil {
			return false, 0, fmt.Errorf("failed to create approval request: %w", err)
		}

		approvalID := int64(0)
		if createdReq != nil && createdReq.ID > 0 {
			approvalID = createdReq.ID
			r.notifyApprovers(createdReq, approvers)
		}

		return false, approvalID, nil
	}
}

func (r *Runtime) notifyApprovers(req *agentmodel.ApprovalRequest, approvers []string) {
	if len(approvers) == 0 {
		return
	}

	notifier := r.approvalNotifier
	if notifier == nil {
		return
	}

	go func() {
		ctx := context.Background()
		if err := notifier.NotifyApprovalRequest(ctx, req, approvers); err != nil {
			log.Printf("failed to notify approvers: %v", err)
		}
	}()
}

// GateClientToolApproval mirrors auditHITLWrapper.needsApproval + buildApprovalChecker for ReAct client tools,
// which bypass InvokableRun (and thus the audit wrapper). When (true, id, nil), the client must wait for
// Approvals API before running local Tauri handlers.
func (r *Runtime) GateClientToolApproval(agent *schema.AgentWithRuntime, sessionID, auditUserID, toolName, toolArgs string) (blockedPending bool, approvalID int64, err error) {
	riskLevel := r.resolveToolRiskLevel(toolName)
	approvalMode := "auto"
	if agent.RuntimeProfile != nil && strings.TrimSpace(agent.RuntimeProfile.ApprovalMode) != "" {
		approvalMode = strings.TrimSpace(agent.RuntimeProfile.ApprovalMode)
	}
	needApproval := false
	switch approvalMode {
	case "all":
		needApproval = true
	case "high_and_above":
		needApproval = riskLevel == "high" || riskLevel == "critical"
	default:
		needApproval = false
	}
	if !needApproval {
		return false, 0, nil
	}
	checker := r.buildApprovalChecker(agent, sessionID, auditUserID)
	approved, aid, err := checker(agent.ID, sessionID, toolName, riskLevel, toolArgs)
	if err != nil {
		return false, 0, err
	}
	if approved {
		return false, 0, nil
	}
	return true, aid, nil
}

// runAgentCoreIterator runs the same ADK ChatModelAgent loop as runAgentCore and returns the event iterator.
func (r *Runtime) runAgentCoreIterator(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool, visionParts []VisionPart) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	instruction := systemPrompt
	if instruction == "" {
		instruction = "You are a helpful AI assistant."
	}
	if len(chatTools) > 0 {
		if hints := r.mcpUsageHintsFromAgent(agent); hints != "" {
			instruction += "\n\n" + hints
		}
		if hints := r.skillUsageHintsFromAgent(agent); hints != "" {
			instruction += "\n\n" + hints
		}
	}

	msgs := history
	if msgs == nil {
		msgs = make([]*einoschema.Message, 0)
	}
	msgs = append(msgs, buildStreamUserMessage(userInput, visionParts))

	toolsCfg := adk.ToolsConfig{}
	if len(chatTools) > 0 {
		toolsCfg = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: chatTools},
		}
	}

	// eino ADK requires non-empty Description; DB agents may omit description.
	adkDesc := strings.TrimSpace(agent.Desc)
	if adkDesc == "" {
		adkDesc = strings.TrimSpace(agent.Name)
	}
	if adkDesc == "" {
		adkDesc = "Assistant"
	}

	agentImpl, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          agent.Name,
		Description:   adkDesc,
		Instruction:   instruction,
		Model:         r.chatModel,
		ToolsConfig:   toolsCfg,
		MaxIterations: 32,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agentImpl,
		EnableStreaming: true,
	})

	return runner.Run(ctx, msgs), nil
}

// emitToolCallsFromMessage writes tool_call SSE lines for each tool call on msg (same shape as Web / desktop clients).
func emitToolCallsFromMessage(
	w io.Writer,
	msg *einoschema.Message,
	chatTools []tool.BaseTool,
	agent *schema.AgentWithRuntime,
	clientType string,
	ts func() string,
) error {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return nil
	}
	for _, tc := range msg.ToolCalls {
		name := tc.Function.Name
		if name == "" {
			name = "tool"
		}
		execMode := getToolExecutionModeFromTools(chatTools, name, skillExecOverrides(agent))
		logger.Info("emit tool call", "tool_name", name, "exec_mode", execMode, "client_type", clientType)
		if execMode == schema.ExecutionModeClient && clientType != "desktop" {
			logger.Warn("tool not supported on web", "tool_name", name, "exec_mode", execMode, "client_type", clientType)
			if err := writeSSEJSONEvent(w, map[string]any{
				"event_type": "info",
				"content":    fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", name),
				"timestamp":  ts(),
			}); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				return err
			}
			return errClientToolUnsupportedWeb
		}
		var input map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		if input == nil {
			input = map[string]any{}
		}
		if err := writeSSEJSONEvent(w, map[string]any{
			"event_type": "tool_call",
			"tool_name":  name,
			"name":       name,
			"input":      input,
			"timestamp":  ts(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// errClientToolUnsupportedWeb is returned when we already wrote [DONE] after the info event.
var errClientToolUnsupportedWeb = errors.New("client tool unsupported on web")

// runAgentCoreStream runs the ADK tool loop, emits SSE for tool_call/tool_result, and streams assistant tokens.
// When ADK EnableStreaming is on, MessageStream must be consumed with Recv — calling GetMessage first drains the stream
// into one string (no typing effect). See cloudwego/eino adk.MessageVariant.GetMessage.
func (r *Runtime) runAgentCoreStream(ctx context.Context, w io.Writer, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool, clientType string, visionParts []VisionPart) error {
	logger.Info("chat stream adk", "phase", "start", "agent_id", agent.ID, "tools", len(chatTools), "history", len(history), "vision", len(visionParts), "client_type", clientType)
	iter, err := r.runAgentCoreIterator(ctx, agent, systemPrompt, userInput, history, chatTools, visionParts)
	if err != nil {
		logger.Warn("chat stream adk", "phase", "iterator_open", "agent_id", agent.ID, "err", err)
		return err
	}
	var lastFinal string
	ts := func() string { return time.Now().Format(time.RFC3339Nano) }
	events := 0
	for {
		event, ok := iter.Next()
		if !ok {
			logger.Info("chat stream adk", "phase", "iterator_exhausted", "agent_id", agent.ID, "events", events, "last_final_len", len(strings.TrimSpace(lastFinal)))
			break
		}
		events++
		if event.Err != nil {
			logger.Warn("chat stream adk", "phase", "iterator", "agent_id", agent.ID, "event_n", events, "err", event.Err)
			return event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput

		// Streaming: do not call GetMessage() first — it drains MessageStream into one string.
		if mv.IsStreaming && mv.MessageStream != nil {
			mv.MessageStream.SetAutomaticClose()
			var streamBuf strings.Builder
			chunks := 0
			for {
				chunk, rerr := mv.MessageStream.Recv()
				if errors.Is(rerr, io.EOF) {
					logger.Info("chat stream adk", "phase", "message_stream_eof", "chunks", chunks, "acc_bytes", streamBuf.Len())
					break
				}
				if rerr != nil {
					logger.Warn("chat stream adk", "phase", "message_stream_recv", "agent_id", agent.ID, "err", rerr)
					return rerr
				}
				if chunk == nil {
					continue
				}
				chunks++
				logger.Debug("chat stream adk", "phase", "stream_recv", "n", chunks,
					"content_len", len(chunk.Content), "reasoning_len", len(chunk.ReasoningContent), "tool_calls", len(chunk.ToolCalls))
				if err := emitToolCallsFromMessage(w, chunk, chatTools, agent, clientType, ts); err != nil {
					if errors.Is(err, errClientToolUnsupportedWeb) {
						return nil
					}
					return err
				}
				out := assistantChunkText(chunk)
				if out != "" {
					streamBuf.WriteString(out)
					if err := streamStaticTextAsSSE(w, out); err != nil {
						return err
					}
				}
			}
			mv.MessageStream.Close()
			if streamBuf.Len() > 0 {
				lastFinal = streamBuf.String()
			}
			continue
		}

		// 非流式事件：IsStreaming=false 或 MessageStream=nil（常见如 Tool 结果、或 ADK/模型未对该步开流式）。
		// GetMessage 等价于拿「已生成完」的整段 Message；此处无法再按 token Recv。
		// 下文仍用 streamStaticTextAsSSE 把正文拆成多条 SSE，减轻前端一次到齐的观感（生成侧仍是一次性）。
		msg, err := mv.GetMessage()
		if err != nil {
			logger.Warn("chat stream adk", "phase", "get_message", "agent_id", agent.ID, "err", err)
			return err
		}
		if err := emitToolCallsFromMessage(w, msg, chatTools, agent, clientType, ts); err != nil {
			if errors.Is(err, errClientToolUnsupportedWeb) {
				return nil
			}
			return err
		}
		role := msg.Role
		if role == "" {
			role = mv.Role
		}
		logger.Debug("chat stream adk", "phase", "get_message", "role", role, "content_len", len(msg.Content), "tool_calls", len(msg.ToolCalls))
		if role == einoschema.Tool {
			toolName := msg.ToolName
			if toolName == "" {
				toolName = mv.ToolName
			}
			if toolName == "" {
				toolName = "tool"
			}
			resultData := map[string]any{
				"event_type": "tool_result",
				"tool_name":  toolName,
				"result":     msg.Content,
				"timestamp":  ts(),
			}
			if err := writeSSEJSONEvent(w, resultData); err != nil {
				return err
			}
		}
		if msg.Content != "" && len(msg.ToolCalls) == 0 {
			lastFinal = msg.Content
			if err := streamStaticTextAsSSE(w, msg.Content); err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(lastFinal) == "" {
		logger.Warn("chat stream adk", "phase", "finish", "agent_id", agent.ID, "reason", "empty_last_final", "events", events)
		return fmt.Errorf("no response from agent")
	}
	logger.Info("chat stream adk", "phase", "finish", "ok", true, "agent_id", agent.ID, "last_len", len(lastFinal))
	return nil
}

func (r *Runtime) runAgentCore(ctx context.Context, agent *schema.AgentWithRuntime, systemPrompt, userInput string, history []*einoschema.Message, chatTools []tool.BaseTool) (string, error) {
	iter, err := r.runAgentCoreIterator(ctx, agent, systemPrompt, userInput, history, chatTools, nil)
	if err != nil {
		return "", err
	}
	var lastFinal string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				return "", err
			}
			// Do not return on the first non-empty reply: the model may emit text alongside
			// tool_calls, or intermediate narration before further tools. Only treat as the
			// user-visible final turn when there are no pending tool calls.
			if msg.Content != "" && len(msg.ToolCalls) == 0 {
				lastFinal = msg.Content
			}
		}
	}

	if strings.TrimSpace(lastFinal) != "" {
		return lastFinal, nil
	}
	return "", fmt.Errorf("no response from agent")
}

func buildSystemPrompt(profile *schema.RuntimeProfile) string {
	if profile == nil {
		return ""
	}

	var parts []string

	if profile.Role != "" {
		parts = append(parts, "Role: "+profile.Role)
	}
	if profile.Goal != "" {
		parts = append(parts, "Goal: "+profile.Goal)
	}
	if profile.Backstory != "" {
		parts = append(parts, "Backstory: "+profile.Backstory)
	}

	return strings.Join(parts, "\n")
}

func truncateRunesForPlanDetail(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}

func summarizeToolResultForPlanDetail(result string) string {
	s := strings.TrimSpace(result)
	if s == "" {
		return ""
	}
	if idx := strings.Index(s, "\n--- extracted text for model ---"); idx >= 0 {
		s = s[:idx]
	}
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, 4)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kept = append(kept, line)
		if len(kept) >= 4 {
			break
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return truncateRunesForPlanDetail(strings.Join(kept, " | "), 320)
}

func (r *Runtime) ResumePlanExecuteStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	tools []tool.BaseTool,
	callID string,
	result string,
	toolErr string,
	sessionID string,
	auditUserID string,
	clientType string,
) (io.ReadCloser, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)

	state, msgs, err := r.clientToolMgr.ResumeState(callID, result, toolErr)
	if err != nil {
		return nil, err
	}

	if !state.PlanMode {
		return nil, fmt.Errorf("invalid state: not a plan-execute mode call")
	}

	logger.Info("plan-execute: resume stream",
		"call_id", logger.CallIDForLog(callID),
		"plan_index", state.PlanIndex,
		"plan_steps", len(state.PlanSteps),
		"msgs_len", len(msgs),
	)

	plan := state.PlanSteps
	planText := state.PlanText
	userInput := state.UserInput
	currentIdx := state.PlanIndex
	ov := skillExecOverrides(agent)
	toolInfos := toolsToToolInfos(tools)

	pr, pw := io.Pipe()
	bw := bufio.NewWriterSize(pw, 4096)

	go func() {
		defer func() {
			bw.Flush()
			pw.Close()
		}()
		defer r.flushUsageSession(ctx)

		writeEvent := func(evt ReActEvent) {
			data, _ := json.Marshal(evt)
			fmt.Fprintf(bw, "data: %s\n\n", data)
			bw.Flush()
		}

		writeEvent(ReActEvent{Type: "observation", Content: fmt.Sprintf("计划已生成:\n%s", planText), Step: 0})

		planItems := make([]PlanTaskItem, 0, len(plan))
		for i, s := range plan {
			planItems = append(planItems, PlanTaskItem{Index: i + 1, Task: s.Task})
		}
		writeEvent(ReActEvent{
			Type:      "plan_tasks",
			Content:   fmt.Sprintf("%d steps", len(plan)),
			Step:      len(plan),
			PlanTasks: planItems,
		})

		// Replay completed steps so the client checklist keeps checkmarks after resume (otherwise only
		// `plan_tasks` + current `running` arrive and earlier steps look pending).
		for j := 0; j < currentIdx && j < len(plan); j++ {
			writeEvent(ReActEvent{
				Type:           "plan_step",
				Content:        plan[j].Task,
				Step:           j + 1,
				PlanStepStatus: "done",
			})
		}

		planExec := &PlanResult{Plan: plan}
		for j := 0; j < currentIdx && j < len(plan); j++ {
			planExec.Steps = append(planExec.Steps, ExecStep{
				PlanStep: plan[j],
				Result:   "[Completed earlier in session; see prior observations.]",
			})
		}

		for i := currentIdx; i < len(plan); i++ {
			step := plan[i]

			writeEvent(ReActEvent{
				Type:           "plan_step",
				Content:        step.Task,
				Step:           i + 1,
				PlanStepStatus: "running",
			})
			writeEvent(ReActEvent{Type: "thought", Content: fmt.Sprintf("执行步骤 %d/%d: %s", i+1, len(plan), step.Task), Step: i + 1})
			if i == currentIdx {
				if toolErr != "" {
					writeEvent(ReActEvent{
						Type:           "plan_step",
						Content:        step.Task,
						Step:           i + 1,
						PlanStepStatus: "error",
					})
					writeEvent(ReActEvent{
						Type:    "error",
						Content: truncateRunesForPlanDetail(toolErr, 320),
						Step:    i + 1,
						Tool:    state.ToolName,
					})
				} else if preview := summarizeToolResultForPlanDetail(result); preview != "" {
					writeEvent(ReActEvent{
						Type:    "observation",
						Content: preview,
						Step:    i + 1,
						Tool:    state.ToolName,
					})
				}
			}

			stepMessages := []*einoschema.Message{
				{Role: einoschema.System, Content: systemPrompt},
				{Role: einoschema.User, Content: userInput},
				{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s", planText)},
			}

			for j := 0; j < i; j++ {
				stepMessages = append(stepMessages, &einoschema.Message{
					Role:    einoschema.Assistant,
					Content: fmt.Sprintf("Step %d completed: %s", j+1, plan[j].Task),
				})
			}

			if i == currentIdx {
				stepMessages = append(stepMessages, msgs...)
			} else {
				currentTask := fmt.Sprintf("Execute step %d of %d: %s", i+1, len(plan), step.Task)
				stepMessages = append(stepMessages, &einoschema.Message{
					Role:    einoschema.User,
					Content: currentTask,
				})
			}

			resp, err := r.chatModel.Generate(ctx, stepMessages, model.WithTools(toolInfos))
			execStep := ExecStep{PlanStep: step}
			if err != nil {
				logger.Error("runtime: plan step Generate failed", "error", err, "step", i+1)
				execStep.Error = err.Error()
				planExec.Steps = append(planExec.Steps, execStep)
				writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("step %d failed: %v", i+1, err), Step: i + 1})
				writeEvent(ReActEvent{
					Type:           "plan_step",
					Content:        err.Error(),
					Step:           i + 1,
					PlanStepStatus: "error",
				})
				writeEvent(ReActEvent{
					Type:    "final_answer",
					Content: fmt.Sprintf("步骤 %d 执行失败: %v", i+1, err),
					Step:    i + 1,
				})
				if err := streamStaticTextAsSSE(pw, fmt.Sprintf("步骤 %d 执行失败: %v", i+1, err)); err != nil {
					return
				}
				fmt.Fprintf(pw, "data: [DONE]\n\n")
				return
			}

			stepResult := ""
			if i == currentIdx && toolErr == "" && strings.TrimSpace(result) != "" {
				stepResult = result
			}
			if len(resp.ToolCalls) > 0 {
				for _, tc := range resp.ToolCalls {
					writeEvent(ReActEvent{Type: "action", Content: fmt.Sprintf("调用工具: %s", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name, Arguments: tc.Function.Arguments})

					if isClientTool(tools, tc.Function.Name, ov) {
						execMode := getToolExecutionModeFromTools(tools, tc.Function.Name, ov)
						if execMode == schema.ExecutionModeClient && clientType != "desktop" {
							writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name})
							writeEvent(ReActEvent{Type: "final_answer", Content: fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name), Step: i + 1})
							if err := streamStaticTextAsSSE(pw, fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name)); err != nil {
								return
							}
							fmt.Fprintf(pw, "data: [DONE]\n\n")
							return
						}
						blocked, apprID, apprErr := r.GateClientToolApproval(agent, sessionID, auditUserID, tc.Function.Name, tc.Function.Arguments)
						if apprErr != nil {
							writeEvent(ReActEvent{Type: "error", Content: apprErr.Error(), Step: i + 1, Tool: tc.Function.Name})
							writeEvent(ReActEvent{Type: "final_answer", Content: fmt.Sprintf("需要审批: %s", apprErr.Error()), Step: i + 1})
							if err := streamStaticTextAsSSE(pw, fmt.Sprintf("需要审批: %s", apprErr.Error())); err != nil {
								return
							}
							fmt.Fprintf(pw, "data: [DONE]\n\n")
							return
						}
						tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, i)
						msgsForSave := append(append([]*einoschema.Message(nil), stepMessages...), &einoschema.Message{
							Role:      einoschema.Assistant,
							Content:   resp.Content,
							ToolCalls: resp.ToolCalls,
						})
						msgCopy := make([]*einoschema.Message, len(msgsForSave))
						copy(msgCopy, msgsForSave)
						r.clientToolMgr.SaveState(&ClientToolCallState{
							CallID:       tc.ID,
							ToolName:     tc.Function.Name,
							ToolArgs:     tc.Function.Arguments,
							Messages:     msgCopy,
							Iter:         i,
							CreatedAt:    time.Now(),
							ClientType:   clientType,
							PlanMode:     true,
							PlanText:     planText,
							PlanIndex:    i,
							PlanSteps:    plan,
							UserInput:    userInput,
							SystemPrompt: systemPrompt,
						})
						execModeVal := getToolExecutionModeFromTools(tools, tc.Function.Name, ov)
						evt := ReActEvent{
							Type:          "client_tool_call",
							Content:       fmt.Sprintf("需要在客户端执行: %s", tc.Function.Name),
							Step:          i + 1,
							Tool:          tc.Function.Name,
							Arguments:     tc.Function.Arguments,
							CallID:        tc.ID,
							Hint:          clientToolHint(tc.Function.Name),
							RiskLevel:     r.resolveToolRiskLevel(tc.Function.Name),
							ExecutionMode: execModeVal,
						}
						if blocked && apprID > 0 {
							evt.ApprovalID = apprID
						}
						writeEvent(evt)
						fmt.Fprintf(pw, "data: [DONE]\n\n")
						return
					}

					foundTool := findInvokableTool(tools, tc.Function.Name)
					if foundTool == nil {
						writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("tool not found: %s", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name})
						execStep.Error = fmt.Sprintf("tool not found: %s", tc.Function.Name)
						continue
					}

					toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
					if err != nil {
						writeEvent(ReActEvent{Type: "error", Content: err.Error(), Step: i + 1, Tool: tc.Function.Name})
						execStep.Error = fmt.Sprintf("tool %s: %v", tc.Function.Name, err)
					} else {
						writeEvent(ReActEvent{Type: "observation", Content: toolResult, Step: i + 1, Tool: tc.Function.Name})
						stepResult += toolResult + "\n"
					}
				}
			}

			if i == currentIdx && toolErr != "" {
				execStep.Error = toolErr
			}
			if execStep.Error == "" && stepResult == "" {
				stepResult = resp.Content
			}
			if execStep.Error == "" {
				execStep.Result = stepResult
			}
			planExec.Steps = append(planExec.Steps, execStep)

			writeEvent(ReActEvent{Type: "observation", Content: fmt.Sprintf("步骤 %d 完成", i+1), Step: i + 1})
			writeEvent(ReActEvent{
				Type:           "plan_step",
				Content:        step.Task,
				Step:           i + 1,
				PlanStepStatus: "done",
			})
		}

		writeEvent(ReActEvent{Type: "thought", Content: "正在综合所有步骤结果...", Step: len(plan) + 1})

		synthPrompt := systemPrompt + planExecuteSystemSuffix
		synthMessages := []*einoschema.Message{
			{Role: einoschema.System, Content: synthPrompt},
			{Role: einoschema.User, Content: userInput},
			{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s\n\nExecution Results:\n%s", planText, planExec.formatSteps())},
		}

		finalResp, err := r.chatModel.Generate(ctx, synthMessages)
		if err != nil {
			logger.Error("runtime: plan synthesis Generate failed", "error", err)
			writeEvent(ReActEvent{Type: "thought", Content: "综合步骤结果失败（步骤已全部完成）。"})
			writeEvent(ReActEvent{
				Type:    "final_answer",
				Content: "任务执行完成，所有步骤已成功执行。",
				Step:    len(plan) + 1,
			})
			if err := streamStaticTextAsSSE(pw, "任务执行完成，所有步骤已成功执行。"); err != nil {
				return
			}
			fmt.Fprintf(pw, "data: [DONE]\n\n")
			return
		}

		finalText := VisibleAssistantOrFallback(finalResp.Content, PlanExecuteSynthesisEmptyFallback)
		writeEvent(ReActEvent{Type: "final_answer", Content: finalText, Step: len(plan) + 1})
		if err := streamStaticTextAsSSE(pw, finalText); err != nil {
			return
		}
		fmt.Fprintf(pw, "data: [DONE]\n\n")
	}()

	return pr, nil
}
