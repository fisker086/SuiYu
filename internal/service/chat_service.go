package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/chathistory"
	"github.com/fisk086/sya/internal/chatfile"
	"github.com/fisk086/sya/internal/embedding"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/memory"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

type ChatService struct {
	runtime   *agent.Runtime
	store     storage.Storage
	memory    memory.Provider
	embed     *embedding.Service
	workflows *WorkflowService
	vectorDim int // embedding dimension for agent_memory (zero-vector fallback when memory provider is nil)
	wg        sync.WaitGroup

	// groupChatHandler: wired in chat_service_group.go (initGroupChatHandler); used for multiplex group SSE.
	groupChatHandler *agent.GroupChatHandler
}

// NewChatService wires chat; mem may be nil (no long-term memory). Use pgvector.New via factory in main when configured.
// vectorDim must match DB embedding column size (e.g. EMBEDDING_DIMENSION); used to persist chat turns without an embedding API via zero vectors.
func NewChatService(runtime *agent.Runtime, store storage.Storage, mem memory.Provider, embed *embedding.Service, workflows *WorkflowService, vectorDim int) *ChatService {
	if vectorDim <= 0 {
		vectorDim = 1536
	}
	svc := &ChatService{runtime: runtime, store: store, memory: mem, embed: embed, workflows: workflows, vectorDim: vectorDim}
	initGroupChatHandler(svc, runtime)
	return svc
}

func (s *ChatService) GetAgent(ctx context.Context, agentID int64) (*schema.Agent, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not available")
	}
	agent, err := s.store.GetAgent(agentID)
	if err != nil {
		return nil, err
	}
	return &schema.Agent{
		ID:        agent.ID,
		Name:      agent.Name,
		Desc:      agent.Desc,
		Category:  agent.Category,
		IsBuiltin: agent.IsBuiltin,
		IsActive:  agent.IsActive,
		CreatedAt: agent.CreatedAt,
		UpdatedAt: agent.UpdatedAt,
	}, nil
}

func validateChatTarget(req *schema.ChatRequest) error {
	if req.WorkflowID > 0 && req.AgentID > 0 {
		return fmt.Errorf("specify either agent_id or workflow_id, not both")
	}
	if req.WorkflowID == 0 && req.AgentID < 1 {
		return fmt.Errorf("agent_id (>=1) or workflow_id is required")
	}
	return nil
}

const maxChatImageBase64Len = 15 * 1024 * 1024 // ~11 MiB raw image after decode
const maxChatImageParts = 12

func visionPartsFromRequest(req *schema.ChatRequest) []agent.VisionPart {
	var out []agent.VisionPart
	for _, p := range req.ImageParts {
		b := strings.TrimSpace(p.Base64)
		if b == "" {
			continue
		}
		m := strings.TrimSpace(p.Mime)
		if m == "" {
			m = "image/png"
		}
		out = append(out, agent.VisionPart{Base64: b, Mime: m})
	}
	if len(out) == 0 {
		b := strings.TrimSpace(req.ImageBase64)
		if b != "" {
			m := strings.TrimSpace(req.ImageMime)
			if m == "" {
				m = "image/png"
			}
			out = append(out, agent.VisionPart{Base64: b, Mime: m})
		}
	}
	return out
}

func validateChatContent(req *schema.ChatRequest) error {
	msg := strings.TrimSpace(req.Message)
	parts := visionPartsFromRequest(req)
	if msg == "" && len(parts) == 0 {
		return fmt.Errorf("message or image_base64 is required")
	}
	if len(parts) > maxChatImageParts {
		return fmt.Errorf("too many images (max %d)", maxChatImageParts)
	}
	for _, p := range parts {
		if len(p.Base64) > maxChatImageBase64Len {
			return fmt.Errorf("image_base64 too large")
		}
		mime := strings.TrimSpace(p.Mime)
		if mime == "" {
			return fmt.Errorf("image_mime is required when image_base64 is set")
		}
		if !strings.HasPrefix(strings.ToLower(mime), "image/") {
			return fmt.Errorf("image_mime must be an image/* type")
		}
	}
	return nil
}

func userTextForMemory(req *schema.ChatRequest) string {
	msg := strings.TrimSpace(req.Message)
	msg = chatfile.StripAttachedContentForStorage(msg)
	parts := visionPartsFromRequest(req)
	nFiles := 0
	for _, u := range req.FileURLs {
		if strings.TrimSpace(u) != "" {
			nFiles++
		}
	}
	if len(parts) == 0 && nFiles == 0 {
		return msg
	}
	// 仅对图片保留「[图片×N]」前缀；附件数量由 extra 中的 file_urls 表示，不再写入「文件×N」以免气泡/记忆重复啰嗦。
	var tags []string
	if len(parts) > 0 {
		tags = append(tags, fmt.Sprintf("图片×%d", len(parts)))
	}
	tag := strings.Join(tags, " ")
	if msg != "" {
		if tag != "" {
			return fmt.Sprintf("[%s] %s", tag, msg)
		}
		return msg
	}
	if tag != "" {
		return fmt.Sprintf("[%s]", tag)
	}
	return ""
}

func (s *ChatService) retrieveUserProfile(ctx context.Context, userID string, agentID int64, query string) string {
	if s.store == nil || userID == "" || agentID <= 0 {
		return ""
	}
	profile, err := s.store.GetUserProfile(ctx, userID, agentID)
	if err != nil || profile == nil || profile.Profile == nil {
		return ""
	}
	profileJSON, err := json.Marshal(profile.Profile)
	if err != nil || len(profileJSON) <= 2 {
		return ""
	}
	return fmt.Sprintf("User Profile: %s", profileJSON)
}

// coerceLastConversations normalizes last_conversation from user profile storage.
// After JSON decode (or some drivers), slices are []any of strings; in-memory updates use []string.
func coerceLastConversations(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		out := make([]string, len(s))
		copy(out, s)
		return out
	case []any:
		out := make([]string, 0, len(s))
		for _, x := range s {
			switch t := x.(type) {
			case string:
				out = append(out, t)
			default:
				if x != nil {
					out = append(out, fmt.Sprint(x))
				}
			}
		}
		return out
	default:
		return nil
	}
}

func (s *ChatService) updateUserProfileFromConversation(ctx context.Context, userID string, agentID int64, userMsg, assistantMsg string) {
	if s.store == nil || s.embed == nil || userID == "" || agentID <= 0 {
		return
	}
	userMsg = strings.TrimSpace(userMsg)
	assistantMsg = strings.TrimSpace(assistantMsg)
	if userMsg == "" || assistantMsg == "" {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		combined := fmt.Sprintf("User: %s\nAssistant: %s", userMsg, assistantMsg)
		vec, err := s.embed.Embed(ctx, combined)
		if err != nil {
			logger.Error("failed to embed user profile", "err", err)
			return
		}

		existing, err := s.store.GetUserProfile(ctx, userID, agentID)
		var newProfile map[string]any
		if err == nil && existing != nil && existing.Profile != nil {
			newProfile = existing.Profile
		} else {
			newProfile = make(map[string]any)
		}

		conversations := coerceLastConversations(newProfile["last_conversation"])
		if conversations == nil {
			conversations = []string{}
		}
		conversations = append([]string{combined}, conversations...)
		if len(conversations) > 10 {
			conversations = conversations[:10]
		}
		newProfile["last_conversation"] = conversations

		if err := s.store.UpsertUserProfile(ctx, userID, agentID, newProfile, vec); err != nil {
			logger.Error("failed to upsert user profile", "err", err)
		}
	}()
}

func (s *ChatService) retrieveMemory(ctx context.Context, agentID int64, sessionID, userText string) string {
	if s.memory == nil || sessionID == "" {
		return ""
	}
	txt, err := s.memory.Retrieve(ctx, memory.Query{
		AgentID:   agentID,
		SessionID: sessionID,
		UserText:  userText,
	})
	if err != nil || strings.TrimSpace(txt) == "" {
		return ""
	}
	return txt
}

func (s *ChatService) isMemoryEnabled(ctx context.Context, agentID int64) bool {
	if s.store == nil || agentID <= 0 {
		return false
	}
	agent, err := s.store.GetAgent(agentID)
	if err != nil || agent.RuntimeProfile == nil {
		return false
	}
	return agent.RuntimeProfile.MemoryEnabled
}

func (s *ChatService) isStreamEnabled(ctx context.Context, agentID int64) bool {
	if s.store == nil || agentID <= 0 {
		return true
	}
	agent, err := s.store.GetAgent(agentID)
	if err != nil || agent.RuntimeProfile == nil {
		return true
	}
	return agent.RuntimeProfile.StreamEnabled
}

// retrieveHistory fetches previous conversation messages for the session.
func (s *ChatService) retrieveHistory(ctx context.Context, sessionID string, limit int) []*einoschema.Message {
	if s.store == nil || sessionID == "" {
		return nil
	}
	msgs, err := s.store.ListRecentSessionMessages(ctx, sessionID, limit)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	return chathistory.ToEinoMessages(msgs)
}

// attachmentExtraFromRequest builds agent_memory.extra (image_urls / file_urls) for history API.
func attachmentExtraFromRequest(req *schema.ChatRequest) map[string]any {
	if req == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var imgs []string
	for _, u := range req.ImageURLs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		imgs = append(imgs, u)
	}
	if u := strings.TrimSpace(req.ImageURL); u != "" {
		if _, ok := seen[u]; !ok {
			imgs = append(imgs, u)
		}
	}
	files := append([]string(nil), req.FileURLs...)
	if len(imgs) == 0 && len(files) == 0 {
		return nil
	}
	ex := map[string]any{}
	if len(imgs) > 0 {
		ex["image_urls"] = imgs
	}
	if len(files) > 0 {
		ex["file_urls"] = files
	}
	return ex
}

// recordConversationAsync persists user + assistant turns to agent_memory for GET /chat/sessions/:id/messages.
// Session transcript is always stored (zero embeddings) so the chat UI can reload history after refresh.
// When memory_enabled is true and memory.Provider is set, uses Record() with embeddings for semantic recall.
// assistantExtra is stored in agent_memory.extra for the assistant row (e.g. react_steps from SSE).
func (s *ChatService) recordConversationAsync(agentID int64, userID, sessionID, userMsg, assistantMsg string, userExtra map[string]any, assistantExtra map[string]any) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	userMsg = strings.TrimSpace(userMsg)
	assistantMsg = strings.TrimSpace(assistantMsg)
	if userMsg == "" && assistantMsg == "" {
		return
	}

	memoryEnabled := s.isMemoryEnabled(context.Background(), agentID)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if memoryEnabled && s.memory != nil {
			if userMsg != "" {
				if err := s.memory.Record(ctx, memory.Turn{AgentID: agentID, UserID: userID, SessionID: sessionID, Role: "user", Content: userMsg, Extra: userExtra}); err != nil {
					logger.Error("failed to record user memory", "agent_id", agentID, "session_id", sessionID, "err", err)
				}
			}
			if assistantMsg != "" {
				if err := s.memory.Record(ctx, memory.Turn{AgentID: agentID, UserID: userID, SessionID: sessionID, Role: "assistant", Content: assistantMsg, Extra: assistantExtra}); err != nil {
					logger.Error("failed to record assistant memory", "agent_id", agentID, "session_id", sessionID, "err", err)
				}
			}
			return
		}

		if s.store == nil {
			return
		}
		zero := make([]float32, s.vectorDim)
		if userMsg != "" {
			if err := s.store.StoreMemory(ctx, agentID, userID, sessionID, "user", userMsg, zero, userExtra); err != nil {
				logger.Error("failed to store user turn", "agent_id", agentID, "session_id", sessionID, "err", err)
			}
		}
		if assistantMsg != "" {
			if err := s.store.StoreMemory(ctx, agentID, userID, sessionID, "assistant", assistantMsg, zero, assistantExtra); err != nil {
				logger.Error("failed to store assistant turn", "agent_id", agentID, "session_id", sessionID, "err", err)
			}
		}
	}()
}

func (s *ChatService) Chat(ctx context.Context, req *schema.ChatRequest, chatUserID string) (*schema.ChatResponse, error) {
	if err := validateChatTarget(req); err != nil {
		return nil, err
	}
	if err := validateChatContent(req); err != nil {
		return nil, err
	}

	if req.WorkflowID > 0 {
		if s.workflows == nil {
			return nil, fmt.Errorf("workflow service not configured")
		}
		wf, err := s.workflows.GetForChat(ctx, req.WorkflowID)
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(strings.TrimSpace(wf.Kind)) {
		case schema.WorkflowKindSequential:
			return s.chatSequential(ctx, req, wf, chatUserID)
		default:
			return nil, fmt.Errorf("workflow kind %q is not executable yet (supported: sequential)", wf.Kind)
		}
	}

	memoryEnabled := s.isMemoryEnabled(ctx, req.AgentID)
	var historyMsgs []*einoschema.Message
	if memoryEnabled {
		historyMsgs = s.retrieveHistory(ctx, req.SessionID, 20)
	}

	start := time.Now()
	userProfile := chatfile.ChatAttachedHTTPHint(req.Message, req.FileURLs)
	resp, err := s.runtime.ChatWithUserProfile(ctx, req.AgentID, req.Message, req.ImageBase64, req.ImageMime, userProfile, req.SessionID, chatUserID, historyMsgs)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	s.recordConversationAsync(req.AgentID, chatUserID, req.SessionID, userTextForMemory(req), resp, attachmentExtraFromRequest(req), nil)
	s.updateUserProfileFromConversation(ctx, chatUserID, req.AgentID, userTextForMemory(req), resp)

	return &schema.ChatResponse{
		Message:    resp,
		AgentID:    req.AgentID,
		SessionID:  req.SessionID,
		DurationMS: elapsed,
	}, nil
}

func (s *ChatService) chatSequential(ctx context.Context, req *schema.ChatRequest, wf *schema.AgentWorkflow, chatUserID string) (*schema.ChatResponse, error) {
	if len(wf.StepAgentIDs) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}
	memAgentID := wf.StepAgentIDs[0]
	memSupp := chatfile.PrependMemContext(req.Message, req.FileURLs, s.retrieveMemory(ctx, memAgentID, req.SessionID, userTextForMemory(req)))

	memoryEnabled := s.isMemoryEnabled(ctx, memAgentID)
	var historyMsgs []*einoschema.Message
	if memoryEnabled {
		historyMsgs = s.retrieveHistory(ctx, req.SessionID, 20)
	}

	start := time.Now()
	input := req.Message
	attachHint := chatfile.ChatAttachedHTTPHint(req.Message, req.FileURLs)
	var last string
	var lastAgentID int64
	for i, aid := range wf.StepAgentIDs {
		var out string
		var err error
		if i == 0 {
			out, err = s.runtime.ChatWithMemoryContext(ctx, aid, input, req.ImageBase64, req.ImageMime, memSupp, req.SessionID, chatUserID, historyMsgs)
		} else {
			out, err = s.runtime.ChatWithUserProfile(ctx, aid, input, "", "", attachHint, req.SessionID, chatUserID, nil)
		}
		if err != nil {
			return nil, err
		}
		last = out
		lastAgentID = aid
		if i < len(wf.StepAgentIDs)-1 {
			input = fmt.Sprintf("[Previous step output]\n%s\n\n[User message]\n%s", strings.TrimSpace(out), req.Message)
		}
	}
	elapsed := time.Since(start).Milliseconds()

	s.recordConversationAsync(memAgentID, chatUserID, req.SessionID, userTextForMemory(req), last, attachmentExtraFromRequest(req), nil)

	return &schema.ChatResponse{
		Message:    last,
		AgentID:    lastAgentID,
		WorkflowID: wf.ID,
		SessionID:  req.SessionID,
		DurationMS: elapsed,
	}, nil
}

func (s *ChatService) ChatStream(ctx context.Context, req *schema.ChatRequest, chatUserID string) (io.ReadCloser, error) {
	if req.WorkflowID > 0 {
		return nil, fmt.Errorf("workflow chat does not support POST /chat/stream yet; use POST /chat")
	}
	if err := validateChatTarget(req); err != nil {
		return nil, err
	}
	if err := validateChatContent(req); err != nil {
		return nil, err
	}

	if isGroupChatStreamRequest(req) {
		return s.handleGroupChatStream(ctx, req, chatUserID)
	}

	streamEnabled := s.isStreamEnabled(ctx, req.AgentID)

	if !streamEnabled {
		resp, err := s.Chat(ctx, req, chatUserID)
		if err != nil {
			return nil, err
		}
		reader, writer := io.Pipe()
		go func() {
			_, _ = writer.Write([]byte("data: {\"content\": " + strconv.Quote(resp.Message) + "}\n\n"))
			writer.Close()
		}()
		return reader, nil
	}

	memoryEnabled := s.isMemoryEnabled(ctx, req.AgentID)
	var historyMsgs []*einoschema.Message
	if memoryEnabled {
		historyMsgs = s.retrieveHistory(ctx, req.SessionID, 20)
	}

	var stopCh <-chan struct{}
	if req.SessionID != "" {
		stopCh = s.runtime.RegisterStreamStop(req.SessionID)
	}

	parts := visionPartsFromRequest(req)
	userProfile := chatfile.ChatAttachedHTTPHint(req.Message, req.FileURLs)
	r, err := s.runtime.ChatStreamWithUserProfile(ctx, req.AgentID, req.Message, parts, userProfile, historyMsgs, stopCh, req.SessionID, chatUserID, req.ClientType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return r, nil
	}
	// ReAct / Plan / Auto / ADK 等：持久化带 type 或 event_type 的 SSE 行（parseSSEReactStepPayloads），供侧栏重放。
	// 裸 {"content":"..."} 分片不会被写入 react_steps。
	persistReactSteps := false
	if ag, ok := s.runtime.GetAgent(req.AgentID); ok {
		nTools := 0
		if tools, err := s.allToolsForAgent(ag); err == nil {
			nTools = len(tools)
		}
		persistReactSteps = shouldPersistReactStepsForAgent(ag, nTools)
	}
	return &sseStreamRecorder{
		r:                 r,
		persistReactSteps: persistReactSteps,
		onClose: func(assistant string, reactSteps []map[string]any, updateProfile bool) {
			var assistantExtra map[string]any
			if persistReactSteps && len(reactSteps) > 0 {
				assistantExtra = map[string]any{"react_steps": reactSteps}
			}
			s.recordConversationAsync(req.AgentID, chatUserID, req.SessionID, userTextForMemory(req), assistant, attachmentExtraFromRequest(req), assistantExtra)
			if updateProfile {
				s.updateUserProfileFromConversation(ctx, chatUserID, req.AgentID, userTextForMemory(req), assistant)
			}
		},
	}, nil
}

// sseStreamRecorder buffers SSE bytes from ChatStream and records the full assistant text on Close (same persistence as non-stream).
type sseStreamRecorder struct {
	r                 io.ReadCloser
	buf               bytes.Buffer
	once              sync.Once
	persistReactSteps bool
	readMu            sync.Mutex
	readErr           error
	onClose           func(assistant string, reactSteps []map[string]any, updateProfile bool)
}

func (w *sseStreamRecorder) Read(p []byte) (int, error) {
	n, err := w.r.Read(p)
	if n > 0 {
		w.buf.Write(p[:n])
	}
	if err != nil {
		w.readMu.Lock()
		w.readErr = err
		w.readMu.Unlock()
	}
	return n, err
}

func (w *sseStreamRecorder) Close() error {
	err := w.r.Close()
	w.once.Do(func() {
		if w.onClose != nil {
			raw := w.buf.Bytes()
			assistant := parseSSEAssistantPayload(raw)
			var steps []map[string]any
			if w.persistReactSteps {
				steps = parseSSEReactStepPayloads(raw)
			}
			w.readMu.Lock()
			re := w.readErr
			w.readMu.Unlock()
			assistant, updateProfile := finalizeAssistantForPersistence(raw, assistant, re)
			w.onClose(assistant, steps, updateProfile)
		}
	})
	return err
}

// sseIndicatesClientToolHandoff is true when ReAct paused for a client-side tool (desktop/Tauri).
// The visible parse is often empty (empty `thought`, skipped action/client_tool frames) while [DONE] is still sent;
// we must not persist StreamFailureMessageGeneric here — the real assistant line is recorded after POST /chat/tool_result/stream.
func sseIndicatesClientToolHandoff(raw []byte) bool {
	return bytes.Contains(raw, []byte("client_tool_call"))
}

// finalizeAssistantForPersistence ensures we never persist a user turn without an assistant line when memory is on:
// missing [DONE], broken reads, or empty parsed text get a safe placeholder; synthetic failure lines skip profile embedding.
func finalizeAssistantForPersistence(raw []byte, parsed string, readErr error) (assistant string, updateProfile bool) {
	hasDONE := bytes.Contains(raw, []byte("data: [DONE]")) || bytes.Contains(raw, []byte("data:[DONE]"))
	trimmed := strings.TrimSpace(parsed)

	if readErr != nil && !errors.Is(readErr, io.EOF) {
		if trimmed == "" {
			return agent.StreamFailureMessageGeneric, false
		}
		return trimmed + agent.StreamMessageTruncatedSuffix, false
	}

	if !hasDONE {
		if trimmed == "" {
			return agent.StreamFailureMessageGeneric, false
		}
		return trimmed + agent.StreamMessageTruncatedSuffix, false
	}

	// Clean completion with [DONE]
	if trimmed == "" {
		if sseIndicatesClientToolHandoff(raw) {
			return "", false
		}
		return agent.StreamFailureMessageGeneric, false
	}
	if agent.IsSyntheticStreamFailureAssistant(trimmed) {
		return parsed, false
	}
	return parsed, true
}

// parseSSEAssistantPayload extracts the user-visible assistant text from the raw SSE body for persistence.
// The stream mixes: (1) token chunks {"content":"..."} only; (2) ReAct envelopes {"type":"thought|action|...", "content":...};
// (3) {"type":"final_answer","content":"..."} — final answer when no plain token stream follows; (4) ADK metadata
// {"event_type":"tool_call|tool_result", ...}. ReAct "thought" is **not** persisted here — it lives in agent_memory.extra.react_steps
// for the thought sidebar; other typed ReAct lines (action, observation, …) and ADK event_type are skipped.
// ReAct {"type":"info"} and ADK {"event_type":"info"} (Web 端不支持某客户端工具) are persisted as assistant-visible text.
// ReAct {"type":"error"} (e.g. LLM call failed after tool_result/stream resume) is persisted so the thread is not empty.
func parseSSEAssistantPayload(raw []byte) string {
	var acc strings.Builder
	var finalAnswer string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		chunk := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(chunk) == "[DONE]" {
			continue
		}
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		var p struct {
			Content   string `json:"content"`
			Type      string `json:"type"`
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal([]byte(chunk), &p); err == nil {
			if p.Type == "final_answer" && strings.TrimSpace(p.Content) != "" {
				finalAnswer = p.Content
				continue
			}
			if p.Type == "thought" {
				continue
			}
			if p.Type == "info" && strings.TrimSpace(p.Content) != "" {
				if acc.Len() > 0 {
					acc.WriteString("\n")
				}
				acc.WriteString(p.Content)
				continue
			}
			// ReAct resume after client tool: LLM failure is only {"type":"error","content":"..."} + [DONE].
			// Without this, assistant text is empty and the user sees no reply after refresh.
			if p.Type == "error" && strings.TrimSpace(p.Content) != "" {
				if acc.Len() > 0 {
					acc.WriteString("\n")
				}
				acc.WriteString(p.Content)
				continue
			}
			if p.EventType == "info" && strings.TrimSpace(p.Content) != "" {
				if acc.Len() > 0 {
					acc.WriteString("\n")
				}
				acc.WriteString(p.Content)
				continue
			}
			if p.Type != "" {
				continue
			}
			if p.EventType != "" {
				continue
			}
			acc.WriteString(p.Content)
			continue
		}
		// Legacy: plain-text tokens (pre-JSON SSE)
		acc.WriteString(chunk)
	}
	out := acc.String()
	trimOut := strings.TrimSpace(out)
	trimFinal := strings.TrimSpace(finalAnswer)
	if trimOut != "" {
		// If the only accumulated visible text is our generic failure placeholder but a later
		// final_answer carried the real reply, prefer final_answer for persistence (avoids DB row
		// stuck on the apology line while the client had shown the full answer).
		if agent.IsSyntheticStreamFailureAssistant(trimOut) && trimFinal != "" &&
			!agent.IsSyntheticStreamFailureAssistant(trimFinal) {
			return finalAnswer
		}
		return out
	}
	return finalAnswer
}

func (s *ChatService) GetCapabilities(agentID int64) []schema.Capability {
	return s.runtime.ResolveCapabilities(agentID)
}

func (s *ChatService) CreateChatSession(ctx context.Context, agentID int64, userID string, groupID int64) (*schema.ChatSession, error) {
	return s.store.CreateChatSession(ctx, agentID, userID, groupID)
}

func (s *ChatService) ListChatSessions(ctx context.Context, agentID int64, userID string, limit, offset int) ([]schema.ChatSession, error) {
	list, err := s.store.ListChatSessions(ctx, agentID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []schema.ChatSession{}, nil
	}
	return list, nil
}

func (s *ChatService) ListRecentSessionMessages(ctx context.Context, sessionID string, limit int) ([]schema.ChatHistoryMessage, error) {
	msgs, err := s.store.ListRecentSessionMessages(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		if msgs[i].ImageURLs == nil {
			msgs[i].ImageURLs = []string{}
		}
		if msgs[i].FileURLs == nil {
			msgs[i].FileURLs = []string{}
		}
	}
	chathistory.NormalizeEmptyAssistantForAPI(msgs)
	return msgs, nil
}

func (s *ChatService) ListSessionMessagesPage(ctx context.Context, sessionID string, offset, limit int) ([]schema.ChatHistoryMessage, error) {
	msgs, err := s.store.ListSessionMessagesPage(ctx, sessionID, offset, limit)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		if msgs[i].ImageURLs == nil {
			msgs[i].ImageURLs = []string{}
		}
		if msgs[i].FileURLs == nil {
			msgs[i].FileURLs = []string{}
		}
	}
	chathistory.NormalizeEmptyAssistantForAPI(msgs)
	return msgs, nil
}

func (s *ChatService) GetChatSession(ctx context.Context, sessionID string) (*schema.ChatSession, error) {
	return s.store.GetChatSession(ctx, sessionID)
}

// UpdateChatSessionTitle sets the display title; userID must match the session owner.
func (s *ChatService) UpdateChatSessionTitle(ctx context.Context, sessionID, userID, title string) error {
	sess, err := s.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != userID {
		return storage.ErrSessionForbidden
	}
	return s.store.UpdateChatSessionTitle(ctx, sessionID, userID, title)
}

// DeleteChatSession removes a session and its messages; userID must match the session owner.
func (s *ChatService) DeleteChatSession(ctx context.Context, sessionID string, userID string) error {
	sess, err := s.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != userID {
		return storage.ErrSessionForbidden
	}
	s.StopStream(sessionID)
	return s.store.DeleteChatSession(ctx, sessionID)
}

func (s *ChatService) StopStream(sessionID string) {
	if s.runtime != nil {
		s.runtime.StopStream(sessionID)
	}
}

func (s *ChatService) GetStats(ctx context.Context, userID string, isAdmin bool) (map[string]int64, error) {
	return s.store.GetChatStats(ctx, userID, isAdmin)
}

func (s *ChatService) GetRecentChats(ctx context.Context, userID string, limit int) ([]map[string]any, error) {
	return s.store.GetRecentChats(ctx, userID, limit)
}

func (s *ChatService) GetChatActivity(ctx context.Context, userID string, days int) ([]map[string]any, error) {
	return s.store.GetChatActivity(ctx, userID, days)
}

func (s *ChatService) SubmitToolResult(ctx context.Context, req *schema.SubmitToolResultRequest, chatUserID string) (*schema.ChatResponseWithToolCall, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not available")
	}

	ag, err := s.resolveAgentForSession(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("resolve agent for session: %w", err)
	}

	var systemPrompt string
	if ag.RuntimeProfile != nil {
		systemPrompt = ag.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPromptForService(ag.RuntimeProfile)
		}
	}

	tools, err := s.allToolsForAgent(ag)
	if err != nil {
		return nil, err
	}
	tools = s.runtime.WrapToolsWithAudit(ag, req.SessionID, chatUserID, tools)

	logger.Info("chat: submit_tool_result",
		"phase", "sync",
		"agent_id", ag.ID,
		"session_id", req.SessionID,
		"call_id", logger.CallIDForLog(req.CallID),
		"has_tool_err", req.Error != "",
		"result_len", len(req.Result),
	)
	if req.Error == "" && len(req.Result) == 0 {
		logger.Warn("chat: submit_tool_result empty result with no error — desktop/web client sent no tool output; assistant may show generic failure or retry until a non-empty result",
			"phase", "sync",
			"session_id", req.SessionID,
			"call_id", logger.CallIDForLog(req.CallID),
		)
	}

	start := time.Now()
	ct := agent.StreamClientType(req.ClientType)
	resp, _, err := s.runtime.ResumeReActLoop(ctx, ag, systemPrompt, tools, req.CallID, req.Result, req.Error, ct)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		var clientErr *agent.ClientToolCallError
		if errors.As(err, &clientErr) {
			hint := clientToolHintForService(clientErr.ToolName)
			call, _ := buildClientToolCallForService(ag, tools, clientErr.CallID, clientErr.ToolName, clientErr.ToolArgs, hint)
			return &schema.ChatResponseWithToolCall{
				SessionID:      req.SessionID,
				AgentID:        ag.ID,
				DurationMS:     elapsed,
				ClientToolCall: call,
			}, nil
		}
		return nil, err
	}

	// Do not insert a fake user row; the real user message was already stored with POST /chat/stream.
	s.recordConversationAsync(ag.ID, chatUserID, req.SessionID, "", resp, nil, nil)

	return &schema.ChatResponseWithToolCall{
		Message:    resp,
		SessionID:  req.SessionID,
		AgentID:    ag.ID,
		DurationMS: elapsed,
	}, nil
}

func (s *ChatService) SubmitToolResultStream(ctx context.Context, req *schema.SubmitToolResultRequest, chatUserID string) (io.ReadCloser, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store not available")
	}

	ag, err := s.resolveAgentForSession(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("resolve agent for session: %w", err)
	}

	var systemPrompt string
	if ag.RuntimeProfile != nil {
		systemPrompt = ag.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPromptForService(ag.RuntimeProfile)
		}
	}

	tools, err := s.allToolsForAgent(ag)
	if err != nil {
		return nil, err
	}
	tools = s.runtime.WrapToolsWithAudit(ag, req.SessionID, chatUserID, tools)

	logger.Info("chat: submit_tool_result",
		"phase", "stream",
		"agent_id", ag.ID,
		"session_id", req.SessionID,
		"call_id", logger.CallIDForLog(req.CallID),
		"has_tool_err", req.Error != "",
		"result_len", len(req.Result),
	)
	if req.Error == "" && len(req.Result) == 0 {
		logger.Warn("chat: submit_tool_result empty result with no error — desktop/web client sent no tool output; assistant may show generic failure or retry until a non-empty result",
			"phase", "stream",
			"session_id", req.SessionID,
			"call_id", logger.CallIDForLog(req.CallID),
		)
	}

	streamEnabled := s.isStreamEnabled(ctx, ag.ID)
	if !streamEnabled {
		resp, err := s.SubmitToolResult(ctx, req, chatUserID)
		if err != nil {
			return nil, err
		}
		reader, writer := io.Pipe()
		go func() {
			if resp.ClientToolCall != nil {
				data, _ := json.Marshal(map[string]any{
					"type":           "client_tool_call",
					"call_id":        resp.ClientToolCall.CallID,
					"tool_name":      resp.ClientToolCall.ToolName,
					"params":         resp.ClientToolCall.Params,
					"hint":           resp.ClientToolCall.Hint,
					"risk_level":     resp.ClientToolCall.RiskLevel,
					"execution_mode": resp.ClientToolCall.ExecutionMode,
				})
				fmt.Fprintf(writer, "data: %s\n\n", data)
			} else {
				_ = streamStaticTextAsSSEFromService(writer, resp.Message)
			}
			fmt.Fprintf(writer, "data: [DONE]\n\n")
			writer.Close()
		}()
		return reader, nil
	}

	isPlanMode, err := s.runtime.GetStatePlanMode(req.CallID)
	if err != nil {
		return nil, err
	}

	ct := agent.StreamClientType(req.ClientType)

	logger.Info("chat: submit_tool_result stream branch",
		"agent_id", ag.ID,
		"call_id", logger.CallIDForLog(req.CallID),
		"plan_execute", isPlanMode,
		"client_type", ct,
	)

	var r io.ReadCloser
	if isPlanMode {
		r, err = s.runtime.ResumePlanExecuteStream(ctx, ag, systemPrompt, tools, req.CallID, req.Result, req.Error, req.SessionID, chatUserID, ct)
	} else {
		r, err = s.runtime.ResumeReActLoopStream(ctx, ag, systemPrompt, tools, req.CallID, req.Result, req.Error, req.SessionID, chatUserID, ct)
	}
	if err != nil {
		return nil, err
	}
	persistReactSteps := shouldPersistReactStepsForAgent(ag, len(tools))
	return &sseStreamRecorder{
		r:                 r,
		persistReactSteps: persistReactSteps,
		onClose: func(assistant string, reactSteps []map[string]any, _ bool) {
			var assistantExtra map[string]any
			if persistReactSteps && len(reactSteps) > 0 {
				assistantExtra = map[string]any{"react_steps": reactSteps}
			}
			// Assistant-only turn after client tool; user text was persisted on the initial stream.
			s.recordConversationAsync(ag.ID, chatUserID, req.SessionID, "", assistant, nil, assistantExtra)
		},
	}, nil
}

func (s *ChatService) resolveAgentForSession(ctx context.Context, sessionID string) (*schema.AgentWithRuntime, error) {
	sess, err := s.store.GetChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	agentModel, err := s.store.GetAgent(sess.AgentID)
	if err != nil {
		return nil, err
	}
	out := &schema.AgentWithRuntime{
		Agent: schema.Agent{
			ID:        agentModel.ID,
			Name:      agentModel.Name,
			Desc:      agentModel.Desc,
			Category:  agentModel.Category,
			IsBuiltin: agentModel.IsBuiltin,
			IsActive:  agentModel.IsActive,
		},
		RuntimeProfile: agentModel.RuntimeProfile,
	}
	if s.store != nil {
		out.SkillExecutionOverrides = agent.SkillExecutionOverridesFromStore(s.store, out)
	}
	return out, nil
}

func (s *ChatService) allToolsForAgent(agent *schema.AgentWithRuntime) ([]tool.BaseTool, error) {
	if s.runtime == nil {
		return nil, fmt.Errorf("runtime not configured")
	}
	return s.runtime.AllToolsForAgent(agent)
}

func buildSystemPromptForService(profile *schema.RuntimeProfile) string {
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

func clientToolHintForService(toolName string) string {
	hints := map[string]string{
		"builtin_docker_operator": "请在本地终端执行 docker 命令，将结果返回给我",
		"builtin_git_operator":    "请在本地执行 git 命令，将结果返回给我",
		"builtin_file_parser":     "请在本地解析文件内容，将结果返回给我",
	}
	if h, ok := hints[toolName]; ok {
		return h
	}
	return fmt.Sprintf("请在本地执行 %s，将结果返回给我", toolName)
}

func buildClientToolCallForService(ag *schema.AgentWithRuntime, tools []tool.BaseTool, callID, toolName, toolArgs, hint string) (*schema.ClientToolCall, error) {
	var params map[string]any
	if toolArgs != "" {
		_ = json.Unmarshal([]byte(toolArgs), &params)
	}
	return &schema.ClientToolCall{
		CallID:        callID,
		ToolName:      toolName,
		Params:        params,
		Hint:          hint,
		RiskLevel:     agent.GetToolRiskLevel(toolName),
		ExecutionMode: agent.ResolveToolExecutionMode(ag, tools, toolName),
	}, nil
}

func streamStaticTextAsSSEFromService(w io.Writer, text string) error {
	const chunk = 10
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunk {
		j := i + chunk
		if j > len(runes) {
			j = len(runes)
		}
		payload, _ := json.Marshal(map[string]string{"content": string(runes[i:j])})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
	}
	return nil
}

// shouldPersistReactStepsForAgent mirrors which streams store SSE JSON lines (type / event_type) in agent_memory.extra.react_steps.
func shouldPersistReactStepsForAgent(ag *schema.AgentWithRuntime, nTools int) bool {
	if ag == nil || ag.RuntimeProfile == nil {
		return false
	}
	em := strings.ToLower(strings.TrimSpace(ag.RuntimeProfile.ExecutionMode))
	if em == strings.ToLower(agent.ExecutionModeReAct) ||
		em == strings.ToLower(agent.ExecutionModePlanExecute) ||
		em == strings.ToLower(agent.ExecutionModeAuto) {
		return true
	}
	// Web 默认 ADK 工具环：含 builtin_http_client 等，SSE 含 event_type=tool_call|tool_result，需持久化才能在刷新后侧栏重放。
	if nTools > 0 {
		return true
	}
	return false
}
