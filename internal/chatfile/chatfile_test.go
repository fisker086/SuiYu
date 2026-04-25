package chatfile

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChatAttachedHTTPHint(t *testing.T) {
	t.Parallel()
	if ChatAttachedHTTPHint("plain", nil) != "" {
		t.Fatal("expected empty without marker or file URLs")
	}
	if got := ChatAttachedHTTPHint("x"+AttachedContentMarker+"y", nil); got != chatAttachedHTTPHintText {
		t.Fatalf("got %q want hint constant", got)
	}
	if got := ChatAttachedHTTPHint("hi", []string{"https://example.com/a"}); got != chatAttachedHTTPHintText {
		t.Fatalf("got %q want hint constant", got)
	}
	if ChatAttachedHTTPHint("hi", []string{"", "  "}) != "" {
		t.Fatal("expected empty when file_urls are blank")
	}
}

func TestPrependMemContext(t *testing.T) {
	t.Parallel()
	if got := PrependMemContext("m", nil, "mem"); got != "mem" {
		t.Fatalf("got %q", got)
	}
	want := chatAttachedHTTPHintText + "\n\nmem"
	if got := PrependMemContext("m"+AttachedContentMarker, nil, "mem"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripAttachedContentForStorage(t *testing.T) {
	t.Parallel()
	const user = "hello"
	const injected = AttachedContentMarker + "--- File: a.txt ---\nbody"
	combined := user + injected
	if got := StripAttachedContentForStorage(combined); got != user {
		t.Fatalf("got %q want %q", got, user)
	}
	if got := StripAttachedContentForStorage("no marker"); got != "no marker" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	s := strings.Repeat("a", 100)
	out := TruncateRunes(s, 20)
	if !strings.Contains(out, "truncated") {
		t.Fatal("expected truncation notice")
	}
	if utf8.RuneCountInString(out) <= 20 {
		t.Fatalf("expected output longer than maxRunes due to notice, got %d", utf8.RuneCountInString(out))
	}
}

func TestExtractTextTxt(t *testing.T) {
	t.Parallel()
	out, err := ExtractText(".txt", []byte("  hello 世界 \n"))
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello 世界" {
		t.Fatalf("got %q", out)
	}
}
