package service

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fisk086/sya/internal/agent"
)

func TestParseSSEAssistantPayload_ReActSkipsInternalContent(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"action","content":"Calling tool: builtin_fetch_url","tool":"builtin_fetch_url"}`,
		`data: {"type":"observation","content":"<html>noise</html>","tool":"builtin_fetch_url"}`,
		`data: {"type":"reflection","content":"long reflection"}`,
		`data: {"content":"hello"}`,
		`data: {"content":" world"}`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	if got != "hello world" {
		t.Fatalf("got %q want %q", got, "hello world")
	}
}

func TestParseSSEAssistantPayload_ADKSkipsToolEvents(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"event_type":"tool_call","tool_name":"x","name":"x","input":{}}`,
		`data: {"event_type":"tool_result","tool_name":"x","result":"big"}`,
		`data: {"content":"done"}`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	if got != "done" {
		t.Fatalf("got %q want %q", got, "done")
	}
}

func TestParseSSEAssistantPayload_ReActFinalAnswerOnly(t *testing.T) {
	// ReAct often emits only final_answer JSON without separate {"content":"..."} token lines.
	raw := `data: {"type":"final_answer","content":"Full reply for history"}` + "\n"
	got := parseSSEAssistantPayload([]byte(raw))
	if got != "Full reply for history" {
		t.Fatalf("got %q want %q", got, "Full reply for history")
	}
}

func TestParseSSEAssistantPayload_PrefersFinalAnswerOverSyntheticAcc(t *testing.T) {
	// Early generic token + later final_answer: acc must not win over real reply.
	raw := strings.Join([]string{
		`data: {"content":"` + agent.StreamFailureMessageGeneric + `"}`,
		`data: {"type":"final_answer","content":"Actual assistant reply after recovery."}`,
		`data: [DONE]`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	want := "Actual assistant reply after recovery."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseSSEAssistantPayload_ReActThoughtNotInAssistantBody(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"thought","content":"I will check docker."}`,
		`data: {"type":"action","content":"Calling tool: x"}`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	if got != "" {
		t.Fatalf("thought must not persist into assistant message body; got %q want empty", got)
	}
}

func TestParseSSEAssistantPayload_ReActThoughtThenFinalAnswerUsesFinalOnly(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"thought","content":"internal chain-of-thought"}`,
		`data: {"type":"final_answer","content":"User-visible reply only."}`,
		`data: [DONE]`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	want := "User-visible reply only."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseSSEAssistantPayload_PlanExecuteSkipsThoughtNoise(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"thought","content":"正在生成执行计划..."}`,
		`data: {"type":"thought","content":"执行步骤 1/15: open browser"}`,
		`data: {"content":"最终"}`,
		`data: {"content":"答案"}`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	if got != "最终答案" {
		t.Fatalf("got %q want %q", got, "最终答案")
	}
}

func TestParseSSEAssistantPayload_ReActInfoPersisted(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"info","content":"工具 x 暂时不支持 Web 端，请在桌面客户端上使用","step":1,"tool":"x"}`,
		`data: [DONE]`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	want := "工具 x 暂时不支持 Web 端，请在桌面客户端上使用"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseSSEAssistantPayload_ReActErrorPersisted(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"error","content":"LLM call failed: upstream timeout","step":1}`,
		`data: [DONE]`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	want := "LLM call failed: upstream timeout"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFinalizeAssistantForPersistence_cleanDone(t *testing.T) {
	raw := []byte(`data: {"content":"ok"}` + "\n\n" + `data: [DONE]` + "\n\n")
	got, up := finalizeAssistantForPersistence(raw, "ok", io.EOF)
	if got != "ok" || !up {
		t.Fatalf("got %q updateProfile=%v", got, up)
	}
}

func TestFinalizeAssistantForPersistence_syntheticNoProfile(t *testing.T) {
	raw := []byte(`data: {"content":"抱歉，本次回复未能生成。请稍后重试。"}` + "\n\n" + `data: [DONE]` + "\n\n")
	got, up := finalizeAssistantForPersistence(raw, agent.StreamFailureMessageGeneric, io.EOF)
	if got != agent.StreamFailureMessageGeneric || up {
		t.Fatalf("got %q updateProfile=%v", got, up)
	}
}

func TestFinalizeAssistantForPersistence_clientToolHandoffNoGenericRow(t *testing.T) {
	// ReAct: empty thought + action + client_tool_call + [DONE]; parseSSEAssistantPayload yields "".
	raw := strings.Join([]string{
		`data: {"type":"thought","content":""}`,
		`data: {"type":"action","content":"Calling tool: builtin_browser","tool":"builtin_browser"}`,
		`data: {"type":"client_tool_call","call_id":"cid1","tool_name":"builtin_browser","content":"需要在客户端执行"}`,
		`data: [DONE]`,
	}, "\n\n")
	got, up := finalizeAssistantForPersistence([]byte(raw), "", io.EOF)
	if got != "" || up {
		t.Fatalf("got %q updateProfile=%v want empty assistant (user row only until tool_result/stream)", got, up)
	}
}

func TestFinalizeAssistantForPersistence_brokenRead(t *testing.T) {
	raw := []byte(`data: {"content":"partial"}` + "\n\n")
	got, up := finalizeAssistantForPersistence(raw, "partial", errors.New("reset"))
	if !strings.Contains(got, "未完整结束") || up {
		t.Fatalf("got %q updateProfile=%v", got, up)
	}
}

func TestParseSSEAssistantPayload_ADKInfoPersisted(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"event_type":"info","content":"请在桌面客户端使用","timestamp":"2026-01-01T00:00:00Z"}`,
		`data: [DONE]`,
	}, "\n\n")
	got := parseSSEAssistantPayload([]byte(raw))
	want := "请在桌面客户端使用"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
