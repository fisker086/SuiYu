// Package agent: group peer tool (builtin_group_send_message) is the in-process A2A surface —
// agents use it to message peers in the same group without HTTP. Remote HTTP A2A (separate
// processes/hosts) should be a different tool once A2AAdapter / CallAgentRemote is wired.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolGroupSendMessage = "builtin_group_send_message"

const (
	groupPeerMaxReplyBytes = 4 << 20 // 4 MiB raw SSE cap
	groupPeerInvokeTimeout  = 3 * time.Minute
)

// buildGroupPeerSystemHint adds instructions when internal peer messaging is available.
func (r *Runtime) buildGroupPeerSystemHint(gp *GroupPeerContext) string {
	if gp == nil || len(gp.PeerAgentIDs) < 2 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Group chat — internal A2A (required)\n")
	b.WriteString("In-process agent-to-agent messaging is exposed as the single tool `")
	b.WriteString(toolGroupSendMessage)
	b.WriteString("` (this **is** internal A2A for this session; no separate “A2A HTTP” tool is needed here). To send text to another agent in this group (including nicknames like 「一号员工」), you **must** call it with `target_agent_id` and `message`. **Do not** say you lack enterprise email, DingTalk, WeCom, or internal APIs; **do not** ask for webhooks or DSN unless the user explicitly asks for external systems.\n")
	b.WriteString("You cannot message yourself. Valid `target_agent_id` values:\n")
	for _, id := range gp.PeerAgentIDs {
		if id == gp.CallerAgentID {
			continue
		}
		line := fmt.Sprintf("- agent id %d", id)
		if a, ok := r.GetAgent(id); ok && a != nil && strings.TrimSpace(a.Name) != "" {
			line = fmt.Sprintf("- %s (id %d)", a.Name, id)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (r *Runtime) newGroupPeerMessageTool(gp *GroupPeerContext, sessionID, auditUserID, clientType string) tool.BaseTool {
	if gp == nil {
		return nil
	}
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolGroupSendMessage,
			Desc: "Internal A2A (in-process): send a message to another agent in this group by numeric id and receive their reply. This is the supported agent-to-agent channel for group chat (not corporate IM). Use when the user wants you to greet or message a peer agent.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"target_agent_id": {Type: einoschema.Integer, Desc: "Peer agent id from the list in the system prompt (same group)", Required: true},
				"message":         {Type: einoschema.String, Desc: "Message to send (e.g. 你好)", Required: true},
			}),
		},
		func(ctx context.Context, in map[string]any) (string, error) {
			return r.execGroupPeerMessage(ctx, gp, sessionID, auditUserID, clientType, in)
		},
	)
}

func (r *Runtime) execGroupPeerMessage(ctx context.Context, gp *GroupPeerContext, sessionID, auditUserID, clientType string, in map[string]any) (string, error) {
	if gp == nil {
		return "", fmt.Errorf("group peer context missing")
	}
	targetID, ok := int64FromToolArg(in, "target_agent_id", "targetAgentId", "agent_id", "to_agent_id")
	if !ok || targetID < 1 {
		return "", fmt.Errorf("invalid or missing target_agent_id")
	}
	if targetID == gp.CallerAgentID {
		return "", fmt.Errorf("cannot message yourself")
	}
	allowed := false
	for _, id := range gp.PeerAgentIDs {
		if id == targetID {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("target_agent_id %d is not in this group session", targetID)
	}
	msg := strArgFromMap(in, "message", "text", "content", "body")
	if strings.TrimSpace(msg) == "" {
		return "", fmt.Errorf("message is empty")
	}
	if len(msg) > 32000 {
		msg = msg[:32000] + "\n\n[truncated]"
	}

	toName := fmt.Sprintf("%d", targetID)
	if a, ok := r.GetAgent(targetID); ok && a != nil && strings.TrimSpace(a.Name) != "" {
		toName = a.Name
	}

	fromName := fmt.Sprintf("agent %d", gp.CallerAgentID)
	if a, ok := r.GetAgent(gp.CallerAgentID); ok && a != nil && strings.TrimSpace(a.Name) != "" {
		fromName = a.Name
	}
	wrapped := fmt.Sprintf("[Peer message from %s]\n%s", fromName, msg)

	if emit := GroupPeerStreamEmitterFromContext(ctx); emit != nil {
		emit(gp.CallerAgentID, map[string]any{
			"type":          "group_peer_outbound",
			"to_agent_id":   targetID,
			"to_agent_name": toName,
			"message":       msg,
		})
	}

	subCtx, cancel := context.WithTimeout(ctx, groupPeerInvokeTimeout)
	defer cancel()

	rc, err := r.ChatStreamWithMemoryContext(subCtx, targetID, wrapped, nil, "", nil, nil, sessionID, auditUserID, clientType, nil)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, groupPeerMaxReplyBytes))
	if err != nil {
		return "", err
	}
	out := sseStreamBytesToAssistantText(raw)
	out = strings.TrimSpace(out)
	if out == "" {
		return "(peer returned empty reply)", nil
	}

	if emit := GroupPeerStreamEmitterFromContext(ctx); emit != nil {
		emitPeerContentChunks(emit, targetID, out)
	}

	return fmt.Sprintf("已向 %s 发送；对方回复已单独展示在其气泡中，请勿在正文中重复粘贴全文。", toName), nil
}

func emitPeerContentChunks(emit GroupPeerStreamEmitter, agentID int64, text string) {
	if emit == nil {
		return
	}
	const chunk = 16
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunk {
		j := i + chunk
		if j > len(runes) {
			j = len(runes)
		}
		emit(agentID, map[string]any{"content": string(runes[i:j])})
	}
}

func int64FromToolArg(in map[string]any, keys ...string) (int64, bool) {
	for _, k := range keys {
		v, ok := in[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return int64(x), true
		case float32:
			return int64(x), true
		case int64:
			return x, true
		case int:
			return int64(x), true
		case int32:
			return int64(x), true
		case json.Number:
			i, err := x.Int64()
			if err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

func strArgFromMap(in map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := in[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case string:
			return x
		default:
			b, err := json.Marshal(x)
			if err == nil {
				return string(b)
			}
		}
	}
	return ""
}

// sseStreamBytesToAssistantText extracts user-visible assistant text from an SSE body (same rules as chat_service.parseSSEAssistantPayload).
func sseStreamBytesToAssistantText(raw []byte) string {
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
		acc.WriteString(chunk)
	}
	out := acc.String()
	if strings.TrimSpace(out) != "" {
		return out
	}
	return finalAnswer
}
