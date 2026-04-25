package agent

import (
	"context"
)

// GroupPeerContext is attached to the request context for group-chat turns so the
// runtime can expose internal peer messaging (builtin_group_send_message).
type GroupPeerContext struct {
	GroupID       int64
	PeerAgentIDs  []int64
	CallerAgentID int64
}

// GroupPeerStreamEmitter writes extra multiplexed SSE JSON frames (with agent_id) to the group
// chat response, e.g. peer outbound notice + peer reply as separate bubbles.
type GroupPeerStreamEmitter func(agentID int64, payload map[string]any)

type groupPeerEmitKey struct{}

// WithGroupPeerStreamEmitter attaches an emitter to ctx (used by builtin_group_send_message).
func WithGroupPeerStreamEmitter(ctx context.Context, emit GroupPeerStreamEmitter) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, groupPeerEmitKey{}, emit)
}

// GroupPeerStreamEmitterFromContext returns the emitter or nil.
func GroupPeerStreamEmitterFromContext(ctx context.Context) GroupPeerStreamEmitter {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(groupPeerEmitKey{}).(GroupPeerStreamEmitter)
	return fn
}

type groupPeerCtxKey struct{}

// WithGroupPeerContext returns ctx that carries group peer metadata for one agent turn.
func WithGroupPeerContext(ctx context.Context, gp *GroupPeerContext) context.Context {
	if gp == nil {
		return ctx
	}
	return context.WithValue(ctx, groupPeerCtxKey{}, gp)
}

// GroupPeerFromContext returns peer metadata or nil.
func GroupPeerFromContext(ctx context.Context) *GroupPeerContext {
	if ctx == nil {
		return nil
	}
	gp, _ := ctx.Value(groupPeerCtxKey{}).(*GroupPeerContext)
	return gp
}
