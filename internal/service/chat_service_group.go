package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
)

// initGroupChatHandler wires the in-process group chat handler on ChatService (called from NewChatService).
func initGroupChatHandler(s *ChatService, rt *agent.Runtime) {
	if s == nil || rt == nil {
		return
	}
	caller := agent.NewRuntimeAgentCaller(rt)
	s.groupChatHandler = agent.NewGroupChatHandler(caller)
}

// isGroupChatStreamRequest is true when POST /chat/stream should take the group multiplex path (group_id + @mentions).
func isGroupChatStreamRequest(req *schema.ChatRequest) bool {
	return req != nil && req.GroupID > 0 && len(req.Mentions) > 0
}

// GroupChatStream handles POST /chat/groups/stream: no agent_id/workflow_id; requires group_id.
// Mentions may be empty: user message is recorded; only @mentioned agents are invoked to reply.
func (s *ChatService) GroupChatStream(ctx context.Context, req *schema.ChatRequest, chatUserID string) (io.ReadCloser, error) {
	if req == nil {
		return nil, fmt.Errorf("请求体无效")
	}
	if req.GroupID < 1 {
		return nil, fmt.Errorf("请指定有效的群聊 ID（group_id）")
	}
	if req.WorkflowID != 0 {
		return nil, fmt.Errorf("群聊流式接口不支持 workflow_id")
	}
	if err := validateChatContent(req); err != nil {
		return nil, err
	}
	return s.handleGroupChatStream(ctx, req, chatUserID)
}

// anchorAgentIDForGroup picks an agent_id for anchoring chat_sessions / memory when @mentions are empty (first group member).
func (s *ChatService) anchorAgentIDForGroup(ctx context.Context, groupID int64, mentions []int64) int64 {
	if len(mentions) > 0 && mentions[0] >= 1 {
		return mentions[0]
	}
	g, err := s.GetChatGroup(ctx, groupID)
	if err != nil || g == nil || len(g.Members) == 0 {
		return 0
	}
	return g.Members[0].AgentID
}

// parseGroupMultiplexSSEToAssistantText extracts per-agent assistant text from multiplexed group SSE
// (each line may include agent_id after relayAgentSSEToGroupMux).
func parseGroupMultiplexSSEToAssistantText(raw []byte) map[int64]string {
	builders := make(map[int64]*strings.Builder)
	finalAnswers := make(map[int64]string)

	getBuilder := func(aid int64) *strings.Builder {
		b, ok := builders[aid]
		if !ok {
			b = &strings.Builder{}
			builders[aid] = b
		}
		return b
	}

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
			AgentID   float64 `json:"agent_id"`
			Content   string  `json:"content"`
			Type      string  `json:"type"`
			EventType string  `json:"event_type"`
		}
		if err := json.Unmarshal([]byte(chunk), &p); err != nil {
			continue
		}
		aid := int64(p.AgentID)
		if aid < 1 {
			continue
		}
		acc := getBuilder(aid)
		if p.Type == "final_answer" && strings.TrimSpace(p.Content) != "" {
			finalAnswers[aid] = p.Content
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
	}

	out := make(map[int64]string)
	for aid, b := range builders {
		s := strings.TrimSpace(b.String())
		if s != "" {
			out[aid] = s
		}
	}
	for aid, fa := range finalAnswers {
		if _, ok := out[aid]; !ok && strings.TrimSpace(fa) != "" {
			out[aid] = fa
		}
	}
	return out
}

// groupStreamRecorder buffers multiplexed group SSE and persists user + each assistant turn on Close.
type groupStreamRecorder struct {
	inner      io.ReadCloser
	buf        bytes.Buffer
	once       sync.Once
	svc        *ChatService
	req        *schema.ChatRequest
	chatUserID string
}

func (w *groupStreamRecorder) Read(p []byte) (int, error) {
	n, err := w.inner.Read(p)
	if n > 0 {
		w.buf.Write(p[:n])
	}
	return n, err
}

func (w *groupStreamRecorder) Close() error {
	err := w.inner.Close()
	w.once.Do(func() {
		if w.svc == nil || w.req == nil {
			return
		}
		sid := strings.TrimSpace(w.req.SessionID)
		// Synthetic session ids are not in chat_sessions; skip persistence (no GET /messages).
		if sid == "" || strings.HasPrefix(sid, "group:") {
			return
		}
		raw := w.buf.Bytes()
		byAgent := parseGroupMultiplexSSEToAssistantText(raw)
		userEx := attachmentExtraFromRequest(w.req)
		if anchor := w.svc.anchorAgentIDForGroup(context.Background(), w.req.GroupID, w.req.Mentions); anchor >= 1 {
			w.svc.recordConversationAsync(anchor, w.chatUserID, sid, userTextForMemory(w.req), "", userEx, nil)
		}
		for aid, text := range byAgent {
			if strings.TrimSpace(text) == "" {
				continue
			}
			w.svc.recordConversationAsync(aid, w.chatUserID, sid, "", text, nil, nil)
		}
	})
	return err
}

// relayAgentSSEToGroupMux reads one agent's SSE stream (data: {json}\n\n), injects agent_id into each JSON
// event, and writes to w. Raw byte chunks must not be wrapped as a single "content" string (that broke markdown tokens).
func relayAgentSSEToGroupMux(agentID int64, r io.ReadCloser, w io.Writer, mu *sync.Mutex) {
	defer r.Close()
	var pending []byte
	buf := make([]byte, 4096)
	write := func(p []byte) {
		mu.Lock()
		_, _ = w.Write(p)
		mu.Unlock()
	}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				idx := bytes.Index(pending, []byte("\n\n"))
				if idx < 0 {
					break
				}
				frame := pending[:idx]
				pending = pending[idx+2:]
				forwardGroupMuxSSEFrame(agentID, frame, write)
			}
		}
		if err != nil {
			if len(pending) > 0 {
				forwardGroupMuxSSEFrame(agentID, pending, write)
			}
			break
		}
	}
}

func forwardGroupMuxSSEFrame(agentID int64, frame []byte, write func([]byte)) {
	for _, line := range bytes.Split(frame, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		rest := bytes.TrimSpace(line[5:])
		if len(rest) == 0 {
			continue
		}
		if bytes.Equal(rest, []byte("[DONE]")) {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(rest, &m); err != nil {
			continue
		}
		m["agent_id"] = agentID
		payload, err := json.Marshal(m)
		if err != nil {
			continue
		}
		var out bytes.Buffer
		out.WriteString("data: ")
		out.Write(payload)
		out.WriteString("\n\n")
		write(out.Bytes())
	}
}

// resolveGroupPeerAgentIDs returns distinct agent ids for group peer messaging: all members of the
// chat group plus any @mentioned ids. This allows builtin_group_send_message when the group has
// two or more members even if the user only @mentions one agent for this turn.
func (s *ChatService) resolveGroupPeerAgentIDs(ctx context.Context, groupID int64, mentions []int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	add := func(id int64) {
		if id < 1 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range mentions {
		add(id)
	}
	if s == nil || groupID < 1 {
		return out
	}
	g, err := s.GetChatGroup(ctx, groupID)
	if err != nil || g == nil {
		logger.Debug("group peer ids: get group failed or nil", "group_id", groupID, "err", err)
		return out
	}
	for _, m := range g.Members {
		add(m.AgentID)
	}
	return out
}

// handleGroupChatStream handles group chat: only @mentioned agents are invoked; empty mentions yields user-only turn.
// It calls each mentioned agent in parallel and multiplexes their SSE streams.
func (s *ChatService) handleGroupChatStream(ctx context.Context, req *schema.ChatRequest, chatUserID string) (io.ReadCloser, error) {
	if s.groupChatHandler == nil {
		return nil, fmt.Errorf("group chat handler not initialized")
	}

	// Real chat_sessions row so GET /chat/sessions/:id/messages works after refresh.
	if strings.TrimSpace(req.SessionID) == "" && s.store != nil {
		if anchor := s.anchorAgentIDForGroup(ctx, req.GroupID, req.Mentions); anchor >= 1 {
			sess, err := s.CreateChatSession(ctx, anchor, chatUserID, req.GroupID)
			if err != nil {
				logger.Warn("group chat: create session failed", "err", err)
			} else if sess != nil {
				req.SessionID = sess.SessionID
			}
		}
	}

	// Fallback when session could not be created (e.g. no store): not loadable via messages API.
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("group:%d:user:%s", req.GroupID, chatUserID)
		req.SessionID = sessionID
	}

	peerAgentIDs := s.resolveGroupPeerAgentIDs(ctx, req.GroupID, req.Mentions)

	// Pipe + mutex first so builtin_group_send_message can emit peer/outbound frames to the same writer.
	reader, writer := io.Pipe()
	var writeMu sync.Mutex
	ctx = agent.WithGroupPeerStreamEmitter(ctx, func(agentID int64, payload map[string]any) {
		if payload == nil {
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		payload["agent_id"] = agentID
		b, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(writer, "data: %s\n\n", b)
	})

	streams, err := s.groupChatHandler.HandleGroupMessage(ctx, req.GroupID, req.Mentions, peerAgentIDs, req.Message, sessionID)
	if err != nil {
		_ = writer.CloseWithError(err)
		return nil, fmt.Errorf("group chat handling failed: %w", err)
	}

	go func() {
		defer writer.Close()

		var wg sync.WaitGroup
		for agentID, stream := range streams {
			wg.Add(1)
			go func(agentID int64, stream io.ReadCloser) {
				defer wg.Done()
				relayAgentSSEToGroupMux(agentID, stream, writer, &writeMu)
			}(agentID, stream)
		}

		wg.Wait()
		writeMu.Lock()
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
		writeMu.Unlock()
	}()

	if strings.TrimSpace(req.SessionID) == "" || strings.HasPrefix(strings.TrimSpace(req.SessionID), "group:") {
		return reader, nil
	}
	return &groupStreamRecorder{
		inner:      reader,
		svc:        s,
		req:        req,
		chatUserID: chatUserID,
	}, nil
}

// Chat Group operations (CRUD for chat_groups / members).

func (s *ChatService) CreateChatGroup(ctx context.Context, req *schema.CreateGroupRequest, userID string) (*model.ChatGroup, error) {
	return s.store.CreateChatGroup(ctx, req, userID)
}

func (s *ChatService) GetChatGroup(ctx context.Context, id int64) (*model.ChatGroup, error) {
	return s.store.GetChatGroup(ctx, id)
}

func (s *ChatService) ListChatGroups(ctx context.Context, userID string) ([]*model.ChatGroup, error) {
	return s.store.ListChatGroups(ctx, userID)
}

func (s *ChatService) UpdateChatGroup(ctx context.Context, id int64, req *schema.UpdateGroupRequest) (*model.ChatGroup, error) {
	return s.store.UpdateChatGroup(ctx, id, req)
}

func (s *ChatService) DeleteChatGroup(ctx context.Context, id int64) error {
	return s.store.DeleteChatGroup(ctx, id)
}
