package agent

import (
	"strings"
	"testing"
)

func TestExtractAfterLastHorizontalRule(t *testing.T) {
	reflection := `What does this result tell us?
The breakdown shows alerts.

---

## 巡检结论
整体健康度: 有风险`
	got := extractAfterLastHorizontalRule(reflection)
	if got == "" || !strings.Contains(got, "巡检") || !strings.Contains(got, "整体健康度") {
		t.Fatalf("expected Chinese section after ---, got %q", got)
	}
}

func TestExtractAfterFirstHorizontalRuleKeepsReportBeforeSecondSeparator(t *testing.T) {
	// Model adds a second --- before 注; last-separator polish would keep only the disclaimer.
	s := `English reflection
---

## 巡检结论
实例数: 3
活跃告警: 0

---

注：本巡检为只读。`
	gotFirst := extractAfterFirstHorizontalRule(s)
	if !strings.Contains(gotFirst, "巡检结论") || !strings.Contains(gotFirst, "实例数") {
		t.Fatalf("expected full report after first ---, got %q", gotFirst)
	}
	gotLast := extractAfterLastHorizontalRule(s)
	if strings.Contains(gotLast, "巡检结论") || !strings.Contains(gotLast, "注：") {
		t.Fatalf("last-separator should drop main report; got %q", gotLast)
	}
}

func TestPolishReActUserVisibleTextForNotifyUsesFirstSeparator(t *testing.T) {
	s := `Thought
---

## 结论
数据A

---

注：说明`
	out := PolishReActUserVisibleTextForNotify(s)
	if !strings.Contains(out, "结论") || !strings.Contains(out, "数据A") {
		t.Fatalf("expected report + footer, got %q", out)
	}
}

func TestPolishReActUserVisibleTextForNotifyKeepsReportWhenSingleSeparatorBetweenSections(t *testing.T) {
	// One "---" between main conclusion and "### 修复建议" — must not keep only the subsection after first ---.
	s := `## 巡检结论
集群 A 正常。

---

### 修复建议
- 项 1`
	out := PolishReActUserVisibleTextForNotify(s)
	if !strings.Contains(out, "巡检结论") || !strings.Contains(out, "集群 A") || !strings.Contains(out, "修复建议") {
		t.Fatalf("expected full report including main section, got %q", out)
	}
}

func TestIsReflectionStyleAssistantText(t *testing.T) {
	ref := `What does this result tell us?
It shows 57 alerts.
Is the result reliable and complete?
Yes.
Do we need more information or can we answer now?
We can answer.
What should we do next?
Synthesize.`
	if !isReflectionStyleAssistantText(ref) {
		t.Fatal("expected reflection block detected")
	}
	if isReflectionStyleAssistantText("## 巡检结论\n仅中文结论，无英文模板问题") {
		t.Fatal("Chinese-only report should not be flagged")
	}
}

func TestPolishReActUserVisibleTextForNotifyDropsThinkingWithHeading(t *testing.T) {
	s := `## 分析过程
正在检查服务状态...

---

## 巡检结论
✅ 所有服务运行正常`
	out := PolishReActUserVisibleTextForNotify(s)
	if !strings.Contains(out, "巡检结论") {
		t.Fatalf("expected conclusion, got %q", out)
	}
	if strings.Contains(out, "分析过程") {
		t.Fatalf("expected no thinking, got %q", out)
	}
}
