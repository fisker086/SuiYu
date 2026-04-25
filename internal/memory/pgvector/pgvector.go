// Package pgvector implements memory.Provider using OpenAI-compatible embeddings + pgvector storage.
package pgvector

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/sya/internal/embedding"
	"github.com/fisk086/sya/internal/memory"
	"github.com/fisk086/sya/internal/storage"
)

// Options tune retrieval; safe zero value uses defaults.
type Options struct {
	RetrieveTopK int // default 5
}

type Provider struct {
	embed  *embedding.Service
	store  storage.Storage
	opt    Options
}

// New returns a Provider backed by embedding API + storage.SearchMemory / StoreMemory.
// embed and store must be non-nil.
func New(embed *embedding.Service, store storage.Storage, opt Options) *Provider {
	if opt.RetrieveTopK <= 0 {
		opt.RetrieveTopK = 5
	}
	return &Provider{embed: embed, store: store, opt: opt}
}

// Retrieve embeds UserText, runs vector search, formats snippets for the system prompt.
func (p *Provider) Retrieve(ctx context.Context, q memory.Query) (string, error) {
	if q.SessionID == "" || strings.TrimSpace(q.UserText) == "" {
		return "", nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = p.opt.RetrieveTopK
	}
	vec, err := p.embed.Embed(ctx, q.UserText)
	if err != nil {
		return "", fmt.Errorf("memory retrieve embed: %w", err)
	}
	rows, err := p.store.SearchMemory(ctx, q.AgentID, vec, limit)
	if err != nil {
		return "", fmt.Errorf("memory search: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("The following lines are retrieved from prior conversation (semantic similarity); use only if relevant.\n")
	for _, m := range rows {
		line := strings.TrimSpace(m.Content)
		if line == "" {
			continue
		}
		if len(line) > 500 {
			line = line[:500] + "…"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", m.Role, line)
	}
	return strings.TrimSpace(b.String()), nil
}

// Record embeds content and appends to agent_memory.
func (p *Provider) Record(ctx context.Context, t memory.Turn) error {
	if t.SessionID == "" || strings.TrimSpace(t.Content) == "" {
		return nil
	}
	vec, err := p.embed.Embed(ctx, t.Content)
	if err != nil {
		return fmt.Errorf("memory record embed: %w", err)
	}
	return p.store.StoreMemory(ctx, t.AgentID, t.UserID, t.SessionID, t.Role, t.Content, vec, t.Extra)
}
