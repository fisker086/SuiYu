// Package memory defines pluggable long-term / session-scoped recall for chat.
// Implementations may use PostgreSQL + pgvector (default), mem0, remote APIs, etc.
package memory

import "context"

// Query is passed to Retrieve for context augmentation before the LLM call.
type Query struct {
	AgentID   int64
	SessionID string
	UserID    string // optional; for future RBAC / mem0 user scoping
	UserText  string // current user message (for embedding / keyword retrieval)
	Limit     int    // max items; 0 means implementation default
}

// Turn is one stored message after the model responds (or user message persistence).
type Turn struct {
	AgentID   int64
	SessionID string
	UserID    string
	Role      string // "user" | "assistant"
	Content   string
	// Extra optional JSON-serializable metadata (e.g. image_urls, file_urls for chat history).
	Extra map[string]any
}

// Provider abstracts how relevant history is retrieved and how turns are persisted.
// ChatService depends on this interface only — swap implementations without changing HTTP handlers.
type Provider interface {
	// Retrieve returns text to inject into the system side of the chat (empty if none / disabled).
	Retrieve(ctx context.Context, q Query) (contextText string, err error)
	// Record persists a conversational turn (typically called after assistant reply; may run async from caller).
	Record(ctx context.Context, t Turn) error
}
