package service

import (
	"strings"
	"testing"
)

func TestParseSSEReactStepPayloads_ReActAndADK(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"thought","content":"think","step":1}`,
		`data: {"content":"visible"}`,
		`data: {"event_type":"tool_call","tool_name":"x","input":{}}`,
		`data: {"type":"final_answer","content":"done"}`,
	}, "\n")
	got := parseSSEReactStepPayloads([]byte(raw))
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(got), got)
	}
	if got[0]["type"] != "thought" {
		t.Fatalf("got[0]=%v", got[0])
	}
	if got[1]["event_type"] != "tool_call" {
		t.Fatalf("got[1]=%v", got[1])
	}
	if got[2]["type"] != "final_answer" {
		t.Fatalf("got[2]=%v", got[2])
	}
}

func TestParseSSEReactStepPayloads_SkipsBareContent(t *testing.T) {
	raw := `data: {"content":"only"}` + "\n"
	got := parseSSEReactStepPayloads([]byte(raw))
	if len(got) != 0 {
		t.Fatalf("want empty, got %#v", got)
	}
}
