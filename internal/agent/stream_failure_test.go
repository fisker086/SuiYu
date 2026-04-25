package agent

import (
	"errors"
	"testing"
)

func TestUserVisibleStreamFailure_403(t *testing.T) {
	got := UserVisibleStreamFailure(errors.New(`LLM upstream returned HTML instead of JSON (HTTP 403)`))
	if got != StreamFailureMessageUpstreamBlocked {
		t.Fatalf("got %q", got)
	}
}

func TestIsSyntheticStreamFailureAssistant(t *testing.T) {
	if !IsSyntheticStreamFailureAssistant(StreamFailureMessageGeneric) {
		t.Fatal("expected generic to be synthetic")
	}
	if IsSyntheticStreamFailureAssistant("Hello, this is a normal reply.") {
		t.Fatal("normal reply should not be synthetic")
	}
	if !IsSyntheticStreamFailureAssistant("partial" + StreamMessageTruncatedSuffix) {
		t.Fatal("truncation suffix should be synthetic")
	}
}
