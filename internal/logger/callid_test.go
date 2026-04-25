package logger

import (
	"strings"
	"testing"
)

func TestCallIDForLog(t *testing.T) {
	if got := CallIDForLog(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	short := "call_abc_1"
	if got := CallIDForLog(short); got != short {
		t.Fatalf("short: got %q want %q", got, short)
	}
	long := "x" + strings.Repeat("a", 200)
	got := CallIDForLog(long)
	if !strings.Contains(got, "…(len=") {
		t.Fatalf("expected len suffix: %q", got)
	}
	if strings.Contains(got, strings.Repeat("a", 100)) {
		t.Fatalf("expected truncation: %q", got)
	}
}
