package agent

import "testing"

func TestParsePlanAndExecuteRouting(t *testing.T) {
	t.Parallel()
	simple, body := parsePlanAndExecuteRouting("PLAN_MODE: SIMPLE")
	if !simple || body != "" {
		t.Fatalf("SIMPLE: got simple=%v body=%q", simple, body)
	}
	simple, body = parsePlanAndExecuteRouting("plan_mode: SIMPLE")
	if !simple || body != "" {
		t.Fatalf("case simple key: got simple=%v body=%q", simple, body)
	}
	simple, body = parsePlanAndExecuteRouting("PLAN_MODE: FULL\n1. a\n2. b")
	if simple || body != "1. a\n2. b" {
		t.Fatalf("FULL: got simple=%v body=%q", simple, body)
	}
	simple, body = parsePlanAndExecuteRouting("1. only legacy plan\n2. second")
	if simple || body != "1. only legacy plan\n2. second" {
		t.Fatalf("legacy: got simple=%v body=%q", simple, body)
	}
}

func TestShouldBypassPlanExecutePlanner(t *testing.T) {
	t.Parallel()
	if !shouldBypassPlanExecutePlanner("你好") {
		t.Fatal("expected 你好 to bypass")
	}
	if !shouldBypassPlanExecutePlanner("谢谢！") {
		t.Fatal("expected 谢谢！ to bypass")
	}
	if shouldBypassPlanExecutePlanner("请帮我打开浏览器访问百度并搜索天气") {
		t.Fatal("long task should not bypass")
	}
}
