package agent

import (
	"testing"

	einoschema "github.com/cloudwego/eino/schema"
)

func TestEnsureOpenAICompatibleMessageContent_EmptySystemGetsSpace(t *testing.T) {
	msgs := []*einoschema.Message{
		{Role: einoschema.System, Content: ""},
		{Role: einoschema.User, Content: "hi"},
	}
	ensureOpenAICompatibleMessageContent(msgs)
	if msgs[0].Content != " " {
		t.Fatalf("system content: got %q want single space", msgs[0].Content)
	}
	if msgs[1].Content != "hi" {
		t.Fatalf("user unchanged")
	}
}

func TestEnsureOpenAICompatibleMessageContent_MultimodalUnchanged(t *testing.T) {
	msgs := []*einoschema.Message{{
		Role: einoschema.User,
		UserInputMultiContent: []einoschema.MessageInputPart{
			{Type: einoschema.ChatMessagePartTypeText, Text: "x"},
		},
	}}
	ensureOpenAICompatibleMessageContent(msgs)
	if msgs[0].Content != "" {
		t.Fatalf("expected empty Content for multimodal user")
	}
}
