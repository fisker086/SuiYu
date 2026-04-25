package agent

import (
	"strings"
	"time"

	agentmodel "github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	einoschema "github.com/cloudwego/eino/schema"
)

// usageAccumulator sums token usage across multiple Generate calls in one chat stream.
type usageAccumulator struct {
	prompt, completion, total int64
}

func (a *usageAccumulator) addMessage(msg *einoschema.Message) {
	if a == nil || msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return
	}
	u := msg.ResponseMeta.Usage
	a.prompt += int64(u.PromptTokens)
	a.completion += int64(u.CompletionTokens)
	if u.TotalTokens > 0 {
		a.total += int64(u.TotalTokens)
	}
}

// TokenUsageSink records aggregated token usage per chat completion (implements *service.TokenUsageService).
type TokenUsageSink interface {
	RecordUsageAsync(*agentmodel.TokenUsage)
}

// SetTokenUsageSink wires DB persistence for token stats (optional; nil = no-op).
func (r *Runtime) SetTokenUsageSink(sink TokenUsageSink) {
	r.usageSink = sink
}

// SetDefaultChatModelName sets the process-wide LLM model id for usage rows when RuntimeProfile.LlmModel is empty.
func (r *Runtime) SetDefaultChatModelName(name string) {
	r.defaultChatModelName = strings.TrimSpace(name)
}

func (r *Runtime) flushTokenUsage(agent *schema.AgentWithRuntime, auditUserID string, acc *usageAccumulator) {
	if r.usageSink == nil || acc == nil || agent == nil {
		return
	}
	if acc.prompt == 0 && acc.completion == 0 {
		return
	}
	total := acc.total
	if total == 0 {
		total = acc.prompt + acc.completion
	}
	modelName := strings.TrimSpace(r.defaultChatModelName)
	if agent.RuntimeProfile != nil && strings.TrimSpace(agent.RuntimeProfile.LlmModel) != "" {
		modelName = strings.TrimSpace(agent.RuntimeProfile.LlmModel)
	}
	if modelName == "" {
		modelName = "unknown"
	}
	uid := strings.TrimSpace(auditUserID)
	if uid == "" {
		uid = "0"
	}
	u := &agentmodel.TokenUsage{
		UserID:       uid,
		UserName:     "",
		AgentID:      agent.ID,
		AgentName:    agent.Name,
		Model:        modelName,
		PromptTokens: acc.prompt,
		Completion:   acc.completion,
		TotalTokens:  total,
		Cost:         0,
		Date:         time.Now().UTC().Format("2006-01-02"),
		CreatedAt:    time.Now(),
	}
	r.usageSink.RecordUsageAsync(u)
}
