package storage

import "testing"

func TestSessionTitleFromFirstMessage_stripsImagePrefix(t *testing.T) {
	got := SessionTitleFromFirstMessage("[图片×1] 帮我看看图片中的文字是什么")
	want := "帮我看看图片中的文字是什么"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSessionTitleFromFirstMessage_imageOnly(t *testing.T) {
	got := SessionTitleFromFirstMessage("[图片×2]")
	if got != "" {
		t.Fatalf("expected empty after strip, got %q", got)
	}
}

func TestSessionTitleFromFirstMessage_stripsFilePrefix(t *testing.T) {
	got := SessionTitleFromFirstMessage("[文件×1] 摘要标题")
	want := "摘要标题"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSessionTitleFromFirstMessage_fileOnly(t *testing.T) {
	got := SessionTitleFromFirstMessage("[文件×1]")
	if got != "" {
		t.Fatalf("expected empty after strip, got %q", got)
	}
}
