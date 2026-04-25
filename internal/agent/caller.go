package agent

import (
	"context"
	"io"

	"github.com/fisk086/sya/internal/chatfile"
)

// AgentCaller defines an interface for calling agents.
// It supports both internal (in-process) and remote (A2A HTTP) calls.
// The remote call can be implemented later with an A2A HTTP adapter.
type AgentCaller interface {
	// CallAgent calls an agent and returns a streaming response.
	// This is the primary method used by the chat service for group messaging.
	CallAgent(ctx context.Context, agentID int64, message string, sessionID string) (io.ReadCloser, error)
}

// AgentCallerFunc is a function adapter for AgentCaller.
type AgentCallerFunc func(ctx context.Context, agentID int64, message string, sessionID string) (io.ReadCloser, error)

func (f AgentCallerFunc) CallAgent(ctx context.Context, agentID int64, message string, sessionID string) (io.ReadCloser, error) {
	return f(ctx, agentID, message, sessionID)
}

// RuntimeAgentCaller wraps Runtime to implement AgentCaller.
// This is the internal implementation that calls agents in the same process.
type RuntimeAgentCaller struct {
	runtime *Runtime
}

// NewRuntimeAgentCaller creates a new RuntimeAgentCaller.
func NewRuntimeAgentCaller(runtime *Runtime) *RuntimeAgentCaller {
	return &RuntimeAgentCaller{runtime: runtime}
}

// CallAgent calls an agent via the runtime (internal call).
// This implementation directly invokes the agent without HTTP overhead.
func (c *RuntimeAgentCaller) CallAgent(ctx context.Context, agentID int64, message string, sessionID string) (io.ReadCloser, error) {
	gp := GroupPeerFromContext(ctx)
	mem := chatfile.ChatAttachedHTTPHint(message, nil)
	return c.runtime.ChatStreamWithMemoryContext(ctx, agentID, message, nil, mem, nil, nil, sessionID, "", "", gp)
}

// A2AAdapter is an optional interface for remote agent calls.
// Implement this interface to enable A2A protocol support for agents running in separate processes/servers.
type A2AAdapter interface {
	// CallAgentRemote calls a remote agent via A2A HTTP protocol.
	// The endpoint should be the A2A agent's URL (e.g., "http://localhost:8080/a2a/agents/1").
	CallAgentRemote(ctx context.Context, endpoint string, message string) (io.ReadCloser, error)
}

// GroupChatHandler handles group chat message routing.
// It parses @mentions and coordinates parallel agent calls.
type GroupChatHandler struct {
	agentCaller AgentCaller
	a2aAdapter  A2AAdapter // Optional: only needed for remote agents
}

// NewGroupChatHandler creates a new GroupChatHandler.
func NewGroupChatHandler(agentCaller AgentCaller) *GroupChatHandler {
	return &GroupChatHandler{
		agentCaller: agentCaller,
	}
}

// SetA2AAdapter sets the optional A2A adapter for remote agent calls.
func (h *GroupChatHandler) SetA2AAdapter(adapter A2AAdapter) {
	h.a2aAdapter = adapter
}

// HandleGroupMessage processes a group chat message.
// invokeAgentIDs are agents to run this turn (from @mentions). peerAgentIDs is the roster used for
// builtin_group_send_message (typically all group members ∪ mentions); when len(peerAgentIDs) >= 2,
// each invoked agent receives GroupPeerContext even if only one agent is @mentioned.
func (h *GroupChatHandler) HandleGroupMessage(ctx context.Context, groupID int64, invokeAgentIDs []int64, peerAgentIDs []int64, message string, sessionID string) (map[int64]io.ReadCloser, error) {
	results := make(map[int64]io.ReadCloser)

	for _, agentID := range invokeAgentIDs {
		callCtx := ctx
		if groupID > 0 && len(peerAgentIDs) >= 2 {
			gp := &GroupPeerContext{
				GroupID:       groupID,
				PeerAgentIDs:  append([]int64(nil), peerAgentIDs...),
				CallerAgentID: agentID,
			}
			callCtx = WithGroupPeerContext(ctx, gp)
		}
		resp, err := h.agentCaller.CallAgent(callCtx, agentID, message, sessionID)
		if err != nil {
			continue
		}
		results[agentID] = resp
	}

	return results, nil
}
