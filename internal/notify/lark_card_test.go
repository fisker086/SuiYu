package notify

import (
	"strings"
	"testing"
)

func TestLarkCardTitleAndMarkdown(t *testing.T) {
	title, md := larkCardTitleAndMarkdown(nil, "hello")
	if title != "通知" || md != "hello" {
		t.Fatalf("single line: got %q / %q", title, md)
	}

	title, md = larkCardTitleAndMarkdown(nil, "[定时任务] job1\n状态: ok\n耗时: 1ms")
	if title != "[定时任务] job1" {
		t.Fatalf("title: %q", title)
	}
	if md != "状态: ok\n耗时: 1ms" {
		t.Fatalf("body: %q", md)
	}

	title, md = larkCardTitleAndMarkdown(map[string]string{"card_title": "自定义"}, "body here")
	if title != "自定义" || md != "body here" {
		t.Fatalf("override: %q / %q", title, md)
	}
}

func TestWrapForLarkMarkdownFence(t *testing.T) {
	s := WrapForLarkMarkdownFence("err: ***\n`x`\n```inner```")
	if !strings.HasPrefix(s, "```text\n") || !strings.HasSuffix(s, "\n```") {
		t.Fatalf("unexpected fence: %q", s)
	}
	if strings.Contains(s, "```inner") {
		t.Fatalf("inner triple-backtick should be escaped: %q", s)
	}
}

func TestBuildLarkInteractiveCardMap(t *testing.T) {
	m := buildLarkInteractiveCardMap("标题", "**bold**")
	if m["schema"] != "2.0" {
		t.Fatalf("schema: want 2.0, got %v", m["schema"])
	}
	if m["header"] == nil {
		t.Fatal("missing header")
	}
	body, _ := m["body"].(map[string]any)
	if body == nil {
		t.Fatal("missing body")
	}
	els, _ := body["elements"].([]any)
	if len(els) != 1 {
		t.Fatalf("body.elements: want 1 element, got %d", len(els))
	}
}
