package skills

import "testing"

func TestNormalizeLokiTime(t *testing.T) {
	if got := normalizeLokiTime("1704067200"); got != "1704067200000000000" {
		t.Fatalf("unix seconds: got %q", got)
	}
	if got := normalizeLokiTime("1704067200000000000"); got != "1704067200000000000" {
		t.Fatalf("nanoseconds passthrough: got %q", got)
	}
	if got := normalizeLokiTime("2024-01-01T00:00:00Z"); got != "1704067200000000000" {
		t.Fatalf("RFC3339: got %q want 1704067200000000000", got)
	}
}
