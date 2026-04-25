package service

import (
	"testing"

	"github.com/fisk086/sya/internal/schema"
)

func TestDiffRuntimeProfilePersistence_executionMode(t *testing.T) {
	want := &schema.RuntimeProfile{ExecutionMode: "react"}
	got := &schema.RuntimeProfile{ExecutionMode: ""}
	d := diffRuntimeProfilePersistence(want, got)
	if len(d) != 1 || d[0] != "execution_mode" {
		t.Fatalf("expected [execution_mode], got %v", d)
	}
}

func TestDiffRuntimeProfilePersistence_ok(t *testing.T) {
	p := &schema.RuntimeProfile{
		SourceAgent:   "general_chat_agent",
		Archetype:     "general_chat",
		ExecutionMode: "react",
		MaxIterations: 16,
	}
	if d := diffRuntimeProfilePersistence(p, p); len(d) != 0 {
		t.Fatalf("expected no diff, got %v", d)
	}
}
