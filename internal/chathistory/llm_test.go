package chathistory

import (
	"testing"

	"github.com/fisk086/sya/internal/schema"
)

func TestIncludeInLLMContext_skipsEmptyAssistant(t *testing.T) {
	if IncludeInLLMContext(schema.ChatHistoryMessage{Role: "assistant", Content: "  "}) {
		t.Fatal("empty assistant should be excluded from LLM context")
	}
	if !IncludeInLLMContext(schema.ChatHistoryMessage{Role: "assistant", Content: "ok"}) {
		t.Fatal("non-empty assistant should be included")
	}
}

func TestIncludeInLLMContext_keepsUserWithAttachmentsOnly(t *testing.T) {
	if !IncludeInLLMContext(schema.ChatHistoryMessage{Role: "user", Content: "", ImageURLs: []string{"https://x/y.png"}}) {
		t.Fatal("user with image only should be included")
	}
}

func TestToEinoMessages_dropsEmptyAssistant(t *testing.T) {
	msgs := []schema.ChatHistoryMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: ""},
		{Role: "assistant", Content: "reply"},
	}
	out := ToEinoMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("want 2 messages, got %d", len(out))
	}
}

func TestNormalizeEmptyAssistantForAPI(t *testing.T) {
	msgs := []schema.ChatHistoryMessage{
		{Role: "assistant", Content: "", ReactSteps: []map[string]any{{"type": "thought"}}},
	}
	NormalizeEmptyAssistantForAPI(msgs)
	if msgs[0].Content != EmptyAssistantWithStepsPlaceholder {
		t.Fatalf("got %q", msgs[0].Content)
	}
}
