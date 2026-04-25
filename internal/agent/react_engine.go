package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
)

const (
	ExecutionModeDefault     = ""
	ExecutionModeSingleCall  = "single-call"
	ExecutionModeReAct       = "react"
	ExecutionModePlanExecute = "plan-and-execute"
	ExecutionModeAuto        = "auto"
)

// ensureReActToolCallID fills empty tool call IDs from the LLM so client_tool_call and POST /chat/tool_result/stream always have a non-empty call_id.
func ensureReActToolCallID(id, toolName string, iter int) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	safe := strings.ReplaceAll(strings.ReplaceAll(toolName, "/", "_"), " ", "_")
	return fmt.Sprintf("tc_%s_%d_%d", safe, iter, time.Now().UnixNano())
}

// reactTruncateMultiToolCallsToOne keeps Vertex AI (Gemini) compatibility: each assistant turn
// lists N function calls, the conversation must supply N matching tool responses before the next
// model call. Our ReAct loop often pauses after one client tool, so N>1 causes the next
// Generate to fail with INVALID_ARGUMENT ("function response parts ... equal to ... function call parts").
// Until we implement full multi-call batching, execute one tool call per model turn.
func reactTruncateMultiToolCallsToOne(resp *einoschema.Message) {
	if resp == nil || len(resp.ToolCalls) <= 1 {
		return
	}
	dropped := make([]string, 0, len(resp.ToolCalls)-1)
	for i := 1; i < len(resp.ToolCalls); i++ {
		dropped = append(dropped, resp.ToolCalls[i].Function.Name)
	}
	logger.Warn("react: model returned multiple tool calls; using only the first (Vertex/Gemini requires one tool response per call in the same turn)",
		"count", len(resp.ToolCalls),
		"kept_tool", resp.ToolCalls[0].Function.Name,
		"dropped_tools", dropped)
	resp.ToolCalls = resp.ToolCalls[:1]
}

// reflectionQuestionLines match reflectionPromptTemplate — that step is internal; user-facing output
// should be the final report (often after a --- separator) not this English Q&A.
var reflectionQuestionLines = []string{
	"What does this result tell us?",
	"Is the result reliable and complete?",
	"Do we need more information or can we answer now?",
	"What should we do next?",
}

// isReflectionStyleAssistantText detects the English reflection block (not the final inspection report).
func isReflectionStyleAssistantText(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 60 {
		return false
	}
	n := 0
	for _, q := range reflectionQuestionLines {
		if strings.Contains(s, q) {
			n++
		}
	}
	return n >= 2
}

var conclusionKeywords = []string{"结论", "总结", "结果", "overview", "summary", "conclusion"}

func looksLikeConclusion(s string) bool {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return false
	}
	firstLine := strings.TrimSpace(lines[0])
	for _, kw := range conclusionKeywords {
		if strings.Contains(strings.ToLower(firstLine), kw) {
			return true
		}
	}
	return false
}

// extractAfterLastHorizontalRule returns text after the last markdown line that is only "---".
// Models often put English reflection above and the real report (e.g. ## 巡检结论) below.
func extractAfterLastHorizontalRule(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	lastSep := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			lastSep = i
		}
	}
	if lastSep < 0 || lastSep+1 >= len(lines) {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[lastSep+1:], "\n"))
}

// extractAfterFirstHorizontalRule returns text after the first standalone "---" line.
// Used for push notifications: some models add a second "---" before a short footer (e.g. 注：);
// taking the *last* separator would drop the main report and keep only the footer.
func extractAfterFirstHorizontalRule(s string) string {
	_, after, ok := splitOnFirstHorizontalRule(s)
	if !ok {
		return ""
	}
	return strings.TrimSpace(after)
}

// splitOnFirstHorizontalRule splits on the first standalone "---" line (markdown horizontal rule).
func splitOnFirstHorizontalRule(s string) (before, after string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "---" {
			continue
		}
		before = strings.TrimSpace(strings.Join(lines[:i], "\n"))
		if i+1 >= len(lines) {
			return before, "", true
		}
		return before, strings.Join(lines[i+1:], "\n"), true
	}
	return "", "", false
}

// countStandaloneHorizontalRules counts lines that are exactly "---" (markdown HR), ignoring indentation-only edge cases.
func countStandaloneHorizontalRules(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "---" {
			n++
		}
	}
	return n
}

// polishReActUserVisibleText prefers the report below ---; drops reflection-only blocks when possible.
func polishReActUserVisibleText(s string) string {
	s = strings.TrimSpace(s)
	if after := extractAfterLastHorizontalRule(s); after != "" {
		return after
	}
	return s
}

// PolishReActUserVisibleTextForNotify normalizes ReAct text for Lark/DingTalk: prefer content after the *first* "---"
// (reflection vs report). The generic polish uses the *last* "---", which would leave only a trailing "注：" block
// when the model inserts another "---" before the disclaimer.
//
// If the model uses a single "---" *between* two report sections (e.g. ## 巡检结论 ... --- ... ### 修复建议),
// taking "after first" would drop the main section — in that case return the full text.
func PolishReActUserVisibleTextForNotify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	beforeFirst, afterFirst, ok := splitOnFirstHorizontalRule(s)
	if !ok {
		return polishReActUserVisibleText(s)
	}
	trimmed := strings.TrimSpace(afterFirst)
	if trimmed == "" {
		return polishReActUserVisibleText(s)
	}

	before := strings.TrimSpace(beforeFirst)
	// English reflection (or short preamble like "Thought") above first --- → keep body below.
	if before != "" && isReflectionStyleAssistantText(before) {
		return trimmed
	}
	if before != "" && !strings.Contains(before, "##") && utf8.RuneCountInString(before) < 2000 {
		return trimmed
	}

	rules := countStandaloneHorizontalRules(s)
	if rules >= 2 {
		return trimmed
	}

	// Single --- after a section that already has markdown headings: treat as in-report separator, not reflection|report.
	// Only return full text if both sections look like report content (conclusion/summary first, then advice/details).
	// If the second section looks like the main conclusion (contains 结论/总结/结果), it's the reflection|report pattern.
	if rules == 1 && strings.HasPrefix(before, "##") && strings.HasPrefix(trimmed, "##") {
		if looksLikeConclusion(trimmed) {
			return trimmed
		}
		return s
	}

	return trimmed
}

// lastNonToolAssistantContent returns the latest assistant message that is not a tool-call envelope
// (e.g. ReAct "reflection" text). Used when the model returns empty content with no tool calls.
func lastNonToolAssistantContent(msgs []*einoschema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != einoschema.Assistant {
			continue
		}
		if len(m.ToolCalls) > 0 {
			continue
		}
		c := strings.TrimSpace(m.Content)
		if c == "" {
			continue
		}
		if after := extractAfterLastHorizontalRule(c); after != "" {
			return after
		}
		if !isReflectionStyleAssistantText(c) {
			return c
		}
		// Skip English reflection-only turns; older message may be the real final answer.
	}
	return ""
}

// lastToolResultContent returns the latest non-empty tool message (e.g. ReAct final answer only in tool output).
func lastToolResultContent(msgs []*einoschema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != einoschema.Tool {
			continue
		}
		if c := strings.TrimSpace(m.Content); c != "" {
			return c
		}
	}
	return ""
}

const reactSystemSuffix = `

## Execution Instructions
You have access to tools. When you need information or need to perform an action, use the available tools.

Follow this cycle:
1. **Thought**: Analyze what you know, what you need, and plan your approach
2. **Action**: Call a tool if needed
3. **Observation**: Review the tool result (provided by the system)
4. **Reflection**: Critically analyze the tool result — does it answer your question? Is it reliable? What does it tell you? Should you adjust your approach?

After reflection, decide your next step: call another tool, or provide the final answer if you have enough information.

Be thorough in your reflection. If a tool result is incomplete or contradictory, acknowledge it and adjust your strategy.

## Client-Side Tools
Some tools are marked "(Runs on client)" — these execute on the user's local machine, not on this server. When you call them, the system will pause and ask the user to run the command locally. The user will paste the result back, and you will continue from there. Do not worry about how the result is obtained; just analyze it and proceed.`

// reflectionFinalMarker：反思模型在「无需再调工具」时输出该标记；主循环可据此提前结束，避免再跑一轮带工具的 Generate。
const reflectionFinalMarker = "[[FINAL_DONE]]"

const reflectionNeedToolsMarker = "[[NEED_TOOLS]]"

const reflectionPromptTemplate = `Based on the tool result above, reflect:
1. What does this result tell us?
2. Is the result reliable and complete?
3. Do we need more information or can we answer now?
4. What should we do next?

Write your reflection and conclusion in the lines above. On the **last line only**, output exactly one of:
- [[FINAL_DONE]] — you can fully answer the user's original question without more tools (put the complete answer above this line).
- [[NEED_TOOLS]] — you still need more tool calls before answering.

If you output [[FINAL_DONE]], the run ends here without another reasoning round.`

// stripReflectionMarkers removes control tokens from text shown to users / stored in history.
func stripReflectionMarkers(s string) string {
	s = strings.ReplaceAll(s, reflectionFinalMarker, "")
	s = strings.ReplaceAll(s, reflectionNeedToolsMarker, "")
	return strings.TrimSpace(s)
}

// finalAnswerFromReflection returns text before [[FINAL_DONE]] when the marker is present.
func finalAnswerFromReflection(content string) (string, bool) {
	if !strings.Contains(content, reflectionFinalMarker) {
		return "", false
	}
	before, _, found := strings.Cut(content, reflectionFinalMarker)
	if !found {
		return "", false
	}
	answer := strings.TrimSpace(before)
	if answer == "" {
		answer = strings.TrimSpace(strings.ReplaceAll(content, reflectionFinalMarker, ""))
	}
	answer = stripReflectionMarkers(answer)
	answer = polishReActUserVisibleText(answer)
	return answer, true
}

const planSystemSuffix = `

## Execution Instructions
You are a planner. First decide whether the user request needs **multi-step execution with tools** (e.g. browser automation, file/API operations, multi-step research with tools).

**Line 1 must be exactly one of:**
- PLAN_MODE: SIMPLE — use for casual chat, greetings, thanks, short Q&A, or anything answerable in **one** assistant reply **without** calling tools.
- PLAN_MODE: FULL — use when the task needs a numbered plan and may invoke tools step by step.

**Rules:**
- If SIMPLE: output **only** that first line (no extra text, no blank lines with content).
- If FULL: after line 1, output a **numbered** step-by-step plan. Each step should be clear and actionable and mention tools when relevant.`

// shouldBypassPlanExecutePlanner skips the planning Generate for obvious trivial inputs (saves one model round-trip).
func shouldBypassPlanExecutePlanner(userInput string) bool {
	s := strings.TrimSpace(userInput)
	if s == "" {
		return false
	}
	if utf8.RuneCountInString(s) > 36 {
		return false
	}
	for _, suf := range []string{"!", "！", "?", "？", ".", "。", "～", "~", "…"} {
		s = strings.TrimSuffix(s, suf)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	trivial := []string{
		"你好", "您好", "嗨", "hi", "hello", "hey", "哈喽", "在吗", "在么",
		"早上好", "下午好", "晚上好", "早安", "晚安", "中午好",
		"谢谢", "多谢", "感谢", "thanks", "thank you", "thx",
		"ok", "okay", "好的", "嗯", "嗯嗯", "行", "可以", "收到",
		"再见", "拜拜", "bye", "goodbye",
	}
	for _, t := range trivial {
		if lower == strings.ToLower(t) {
			return true
		}
	}
	return false
}

// resolveExecutionModeAuto uses a lightweight LLM call to decide whether the task needs multi-step planning.
// Returns ExecutionModeSingleCall for simple tasks, ExecutionModePlanExecute for complex tasks.
func (r *Runtime) resolveExecutionModeAuto(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	userInput string,
	history []*einoschema.Message,
	tools []tool.BaseTool,
) (string, error) {
	if shouldBypassPlanExecutePlanner(userInput) {
		return ExecutionModeSingleCall, nil
	}

	prompt := `Analyze the user request and determine if it requires a multi-step plan to execute.

User request: ` + userInput + `

Respond with ONLY one word:
- "simple" if the task can be completed with a single response or one tool call
- "plan" if the task requires multiple steps, complex reasoning, or tool orchestration

Response:`

	msgs := []*einoschema.Message{
		{Role: einoschema.System, Content: "You are a task complexity analyzer."},
		{Role: einoschema.User, Content: prompt},
	}

	resp, err := r.chatModel.Generate(ctx, msgs)
	if err != nil {
		logger.Warn("auto mode: llm call failed, defaulting to single-call", "error", err.Error())
		return ExecutionModeSingleCall, nil
	}

	answer := strings.ToLower(strings.TrimSpace(resp.Content))
	if strings.Contains(answer, "plan") {
		logger.Info("auto mode: complexity detected", "user_input", userInput)
		return ExecutionModePlanExecute, nil
	}

	return ExecutionModeSingleCall, nil
}

// parsePlanAndExecuteRouting reads PLAN_MODE on the first non-empty line. If absent, treats the whole body as a legacy full plan (backward compatible).
func parsePlanAndExecuteRouting(content string) (simple bool, planBody string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return false, content
	}
	lines := strings.Split(content, "\n")
	firstIdx := -1
	firstLine := ""
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" {
			firstIdx = i
			firstLine = t
			break
		}
	}
	if firstIdx < 0 {
		return false, content
	}
	idx := strings.Index(firstLine, ":")
	if idx < 0 {
		return false, content
	}
	key := strings.TrimSpace(strings.ToLower(firstLine[:idx]))
	modeVal := strings.TrimSpace(firstLine[idx+1:])
	if key != "plan_mode" {
		return false, content
	}
	rest := strings.TrimSpace(strings.Join(lines[firstIdx+1:], "\n"))
	switch strings.ToUpper(modeVal) {
	case "SIMPLE":
		return true, rest
	case "FULL":
		return false, rest
	default:
		return false, content
	}
}

const planExecuteSystemSuffix = `

## Execution Instructions
You will receive a plan with the results of each step. Synthesize a clear final answer for the user's original request.

Reference outcomes by topic or step where helpful. Do **not** paste the full raw execution trace, line-by-line tool logs, or repeat every tool error string — summarize what succeeded or failed and what the user should do next.`

type ReActResult struct {
	Steps       []ReActStep `json:"steps"`
	FinalAnswer string      `json:"final_answer"`
	TotalCalls  int         `json:"total_calls"`
}

type ReActStep struct {
	Iteration  int    `json:"iteration"`
	Thought    string `json:"thought"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolArgs   string `json:"tool_args,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
	Reflection string `json:"reflection,omitempty"`
	Error      string `json:"error,omitempty"`
}

type PlanResult struct {
	Plan        []PlanStep `json:"plan"`
	Steps       []ExecStep `json:"steps"`
	FinalAnswer string     `json:"final_answer"`
}

type PlanStep struct {
	Number   int    `json:"number"`
	Task     string `json:"task"`
	ToolName string `json:"tool_name,omitempty"`
}

type ExecStep struct {
	PlanStep PlanStep `json:"plan_step"`
	Result   string   `json:"result"`
	Error    string   `json:"error,omitempty"`
}

// PlanTaskItem is emitted in SSE for plan-and-execute so desktop clients can render a Cursor-style checklist.
type PlanTaskItem struct {
	Index int    `json:"index"` // 1-based
	Task  string `json:"task"`
}

type ReActEvent struct {
	Type          string `json:"type"`
	Content       string `json:"content"`
	Step          int    `json:"step,omitempty"`
	Tool          string `json:"tool,omitempty"`
	Arguments     string `json:"arguments,omitempty"`
	CallID        string `json:"call_id,omitempty"`
	Hint          string `json:"hint,omitempty"`
	RiskLevel     string `json:"risk_level,omitempty"`
	ExecutionMode string `json:"execution_mode,omitempty"`
	// >0: desktop must wait for Approvals before local Tauri execution (same rules as server tool HITL).
	ApprovalID int64 `json:"approval_id,omitempty"`
	// Plan-and-execute (desktop task list): plan_tasks once, then plan_step with PlanStepStatus.
	PlanTasks      []PlanTaskItem `json:"plan_tasks,omitempty"`
	PlanStepStatus string         `json:"plan_step_status,omitempty"` // running|done|error
}

type ClientToolCallError struct {
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	ToolArgs string `json:"tool_args"`
}

func (e *ClientToolCallError) Error() string {
	return fmt.Sprintf("client_tool_call:%s", e.ToolName)
}

// ReActResultToReactPayloads builds SSE-shaped maps for agent_memory.extra.react_steps (same shape as stream persistence).
func ReActResultToReactPayloads(result *ReActResult, finalAnswer string) []map[string]any {
	if result == nil {
		if t := strings.TrimSpace(finalAnswer); t != "" {
			return []map[string]any{{"type": "final_answer", "content": t, "step": 1}}
		}
		return nil
	}
	var out []map[string]any
	for _, s := range result.Steps {
		if strings.TrimSpace(s.Thought) != "" {
			out = append(out, map[string]any{"type": "thought", "content": s.Thought, "step": s.Iteration})
		}
		if s.ToolName != "" {
			out = append(out, map[string]any{
				"type": "action", "content": fmt.Sprintf("Calling tool: %s", s.ToolName),
				"tool": s.ToolName, "arguments": s.ToolArgs, "step": s.Iteration,
			})
		}
		if s.Error != "" {
			out = append(out, map[string]any{"type": "error", "content": s.Error, "tool": s.ToolName, "step": s.Iteration})
		} else if strings.TrimSpace(s.ToolResult) != "" {
			out = append(out, map[string]any{"type": "observation", "content": s.ToolResult, "tool": s.ToolName, "step": s.Iteration})
		}
		if strings.TrimSpace(s.Reflection) != "" {
			out = append(out, map[string]any{"type": "reflection", "content": s.Reflection, "step": s.Iteration})
		}
	}
	if fa := strings.TrimSpace(finalAnswer); fa != "" {
		step := 1
		if len(result.Steps) > 0 {
			step = result.Steps[len(result.Steps)-1].Iteration
		}
		out = append(out, map[string]any{"type": "final_answer", "content": fa, "step": step})
	}
	return out
}

func toolsToToolInfos(tools []tool.BaseTool) []*einoschema.ToolInfo {
	infos := make([]*einoschema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos
}

func findInvokableTool(tools []tool.BaseTool, name string) tool.InvokableTool {
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		if info.Name == name {
			if it, ok := t.(tool.InvokableTool); ok {
				return it
			}
		}
	}
	return nil
}

func (r *Runtime) runReActLoop(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	userInput string,
	history []*einoschema.Message,
	tools []tool.BaseTool,
) (string, *ReActResult, error) {
	maxIter := 16
	if agent.RuntimeProfile != nil && agent.RuntimeProfile.MaxIterations > 0 {
		maxIter = agent.RuntimeProfile.MaxIterations
	}

	fullSystemPrompt := systemPrompt + reactSystemSuffix

	messages := []*einoschema.Message{
		{Role: einoschema.System, Content: fullSystemPrompt},
	}
	messages = append(messages, history...)
	messages = append(messages, &einoschema.Message{Role: einoschema.User, Content: userInput})

	result := &ReActResult{}
	toolInfos := toolsToToolInfos(tools)
	ov := skillExecOverrides(agent)

	for iter := 0; iter < maxIter; iter++ {
		resp, err := r.chatModel.Generate(ctx, messages, model.WithTools(toolInfos))
		if err != nil {
			logger.Error("LLM Generate failed", "error", err, "iteration", iter+1)
			return "", nil, fmt.Errorf("LLM call failed at iteration %d: %w", iter+1, err)
		}
		reactTruncateMultiToolCallsToOne(resp)

		if len(resp.ToolCalls) == 0 {
			text := strings.TrimSpace(resp.Content)
			text = polishReActUserVisibleText(text)
			if text == "" || isReflectionStyleAssistantText(text) {
				if alt := lastNonToolAssistantContent(messages); alt != "" {
					text = alt
				}
			}
			if text == "" {
				text = lastToolResultContent(messages)
			}
			if text != "" {
				result.FinalAnswer = text
				return text, result, nil
			}
			return "", result, fmt.Errorf("no response from agent")
		}

		messages = append(messages, &einoschema.Message{
			Role:      einoschema.Assistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for i := range resp.ToolCalls {
			tc := &resp.ToolCalls[i]
			tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, iter)
			step := ReActStep{
				Iteration: iter + 1,
				Thought:   resp.Content,
				ToolName:  tc.Function.Name,
				ToolArgs:  tc.Function.Arguments,
			}

			if isClientTool(tools, tc.Function.Name, ov) {
				msgCopy := make([]*einoschema.Message, len(messages))
				copy(msgCopy, messages)
				r.clientToolMgr.SaveState(&ClientToolCallState{
					CallID:    tc.ID,
					ToolName:  tc.Function.Name,
					ToolArgs:  tc.Function.Arguments,
					Messages:  msgCopy,
					Iter:      iter,
					CreatedAt: time.Now(),
				})
				step.Error = fmt.Sprintf("CLIENT_TOOL_CALL: %s", tc.Function.Name)
				result.Steps = append(result.Steps, step)
				return "", result, &ClientToolCallError{
					CallID:   tc.ID,
					ToolName: tc.Function.Name,
					ToolArgs: tc.Function.Arguments,
				}
			}

			foundTool := findInvokableTool(tools, tc.Function.Name)
			if foundTool == nil {
				step.Error = fmt.Sprintf("tool not found: %s", tc.Function.Name)
				result.Steps = append(result.Steps, step)

				messages = append(messages, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    fmt.Sprintf("Error: tool not found: %s", tc.Function.Name),
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
				continue
			}

			toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
			if err != nil {
				step.Error = err.Error()
				result.Steps = append(result.Steps, step)

				messages = append(messages, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    fmt.Sprintf("Error: %s", err.Error()),
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
				continue
			}

			step.ToolResult = toolResult
			result.TotalCalls++

			messages = append(messages, &einoschema.Message{
				Role:       einoschema.Tool,
				Content:    toolResult,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})

			reflectionMsgs := append([]*einoschema.Message{}, messages...)
			reflectionMsgs = append(reflectionMsgs, &einoschema.Message{
				Role:    einoschema.User,
				Content: reflectionPromptTemplate,
			})

			reflectionResp, err := r.chatModel.Generate(ctx, reflectionMsgs)
			if err == nil && reflectionResp.Content != "" {
				raw := reflectionResp.Content
				step.Reflection = stripReflectionMarkers(raw)
				if len(resp.ToolCalls) == 1 {
					if ans, ok := finalAnswerFromReflection(raw); ok {
						result.Steps = append(result.Steps, step)
						result.FinalAnswer = ans
						return ans, result, nil
					}
				}
				messages = append(messages, &einoschema.Message{
					Role:    einoschema.Assistant,
					Content: stripReflectionMarkers(raw),
				})
			}

			result.Steps = append(result.Steps, step)
		}
	}

	return "", result, fmt.Errorf("ReAct loop exceeded maximum iterations (%d)", maxIter)
}

func (r *Runtime) runReActLoopStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	userInput string,
	history []*einoschema.Message,
	tools []tool.BaseTool,
	clientType string,
	visionParts []VisionPart,
	sessionID string,
	auditUserID string,
	stopCh <-chan struct{},
) (io.ReadCloser, error) {
	maxIter := 16
	if agent.RuntimeProfile != nil && agent.RuntimeProfile.MaxIterations > 0 {
		maxIter = agent.RuntimeProfile.MaxIterations
	}

	fullSystemPrompt := systemPrompt + reactSystemSuffix

	messages := []*einoschema.Message{
		{Role: einoschema.System, Content: fullSystemPrompt},
	}
	messages = append(messages, history...)
	messages = append(messages, buildStreamUserMessage(userInput, visionParts))

	toolInfos := toolsToToolInfos(tools)

	pr, pw := io.Pipe()
	bw := bufio.NewWriterSize(pw, 4096)

	go func() {
		defer func() {
			bw.Flush()
			pw.Close()
		}()
		defer r.flushUsageSession(ctx)

		writeEvent := func(evt ReActEvent) {
			data, _ := json.Marshal(evt)
			fmt.Fprintf(bw, "data: %s\n\n", data)
			bw.Flush()
		}
		ov := skillExecOverrides(agent)

		for iter := 0; iter < maxIter; iter++ {
			select {
			case <-stopCh:
				logger.Info("react stream loop stopped by client", "session_id", sessionID, "iteration", iter)
				writeEvent(ReActEvent{Type: "error", Content: "Stream stopped by user", Step: iter + 1})
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			default:
			}

			resp, err := r.chatModel.Generate(ctx, messages, model.WithTools(toolInfos))
			if err != nil {
				writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("LLM call failed: %v", err), Step: iter + 1})
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}
			reactTruncateMultiToolCallsToOne(resp)

			if len(resp.ToolCalls) == 0 {
				text := strings.TrimSpace(resp.Content)
				text = polishReActUserVisibleText(text)
				if text == "" || isReflectionStyleAssistantText(text) {
					if alt := lastNonToolAssistantContent(messages); alt != "" {
						text = alt
					}
				}
				if text == "" {
					text = lastToolResultContent(messages)
				}
				text = VisibleAssistantOrFallback(text, AssistantEmptyVisibleFallback)
				writeEvent(ReActEvent{Type: "final_answer", Content: text, Step: iter + 1})
				if err := streamStaticTextAsSSE(bw, text); err != nil {
					return
				}
				bw.Flush()
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}

			writeEvent(ReActEvent{Type: "thought", Content: resp.Content, Step: iter + 1})

			messages = append(messages, &einoschema.Message{
				Role:      einoschema.Assistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			for i := range resp.ToolCalls {
				tc := &resp.ToolCalls[i]
				tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, iter)
				writeEvent(ReActEvent{
					Type:      "action",
					Content:   fmt.Sprintf("Calling tool: %s", tc.Function.Name),
					Step:      iter + 1,
					Tool:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})

				if isClientTool(tools, tc.Function.Name, ov) {
					execMode := getToolExecutionModeFromTools(tools, tc.Function.Name, ov)
					if execMode == schema.ExecutionModeClient && clientType != "desktop" {
						writeEvent(ReActEvent{
							Type:    "info",
							Content: fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name),
							Step:    iter + 1,
							Tool:    tc.Function.Name,
						})
						fmt.Fprintf(bw, "data: [DONE]\n\n")
						bw.Flush()
						return
					}
					blocked, apprID, apprErr := r.GateClientToolApproval(agent, sessionID, auditUserID, tc.Function.Name, tc.Function.Arguments)
					if apprErr != nil {
						writeEvent(ReActEvent{Type: "error", Content: apprErr.Error(), Step: iter + 1, Tool: tc.Function.Name})
						fmt.Fprintf(bw, "data: [DONE]\n\n")
						bw.Flush()
						return
					}
					msgCopy := make([]*einoschema.Message, len(messages))
					copy(msgCopy, messages)
					r.clientToolMgr.SaveState(&ClientToolCallState{
						CallID:     tc.ID,
						ToolName:   tc.Function.Name,
						ToolArgs:   tc.Function.Arguments,
						Messages:   msgCopy,
						Iter:       iter,
						CreatedAt:  time.Now(),
						ClientType: clientType,
					})
					evt := ReActEvent{
						Type:          "client_tool_call",
						Content:       fmt.Sprintf("需要在客户端执行: %s", tc.Function.Name),
						Step:          iter + 1,
						Tool:          tc.Function.Name,
						Arguments:     tc.Function.Arguments,
						CallID:        tc.ID,
						Hint:          clientToolHint(tc.Function.Name),
						RiskLevel:     r.resolveToolRiskLevel(tc.Function.Name),
						ExecutionMode: execMode,
					}
					if blocked && apprID > 0 {
						evt.ApprovalID = apprID
					}
					writeEvent(evt)
					fmt.Fprintf(bw, "data: [DONE]\n\n")
					bw.Flush()
					return
				}

				foundTool := findInvokableTool(tools, tc.Function.Name)
				if foundTool == nil {
					errMsg := fmt.Sprintf("tool not found: %s", tc.Function.Name)
					writeEvent(ReActEvent{Type: "error", Content: errMsg, Step: iter + 1, Tool: tc.Function.Name})
					messages = append(messages, &einoschema.Message{
						Role:       einoschema.Tool,
						Content:    "Error: " + errMsg,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					continue
				}

				toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
				if err != nil {
					writeEvent(ReActEvent{Type: "error", Content: err.Error(), Step: iter + 1, Tool: tc.Function.Name})
					messages = append(messages, &einoschema.Message{
						Role:       einoschema.Tool,
						Content:    fmt.Sprintf("Error: %s", err.Error()),
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					continue
				}

				writeEvent(ReActEvent{Type: "observation", Content: toolResult, Step: iter + 1, Tool: tc.Function.Name})

				messages = append(messages, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    toolResult,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})

				reflectionMsgs := append([]*einoschema.Message{}, messages...)
				reflectionMsgs = append(reflectionMsgs, &einoschema.Message{
					Role:    einoschema.User,
					Content: reflectionPromptTemplate,
				})

				reflectionResp, err := r.chatModel.Generate(ctx, reflectionMsgs)
				if err == nil && reflectionResp.Content != "" {
					raw := reflectionResp.Content
					writeEvent(ReActEvent{Type: "reflection", Content: stripReflectionMarkers(raw), Step: iter + 1})
					if len(resp.ToolCalls) == 1 {
						if ans, ok := finalAnswerFromReflection(raw); ok {
							writeEvent(ReActEvent{Type: "final_answer", Content: ans, Step: iter + 1})
							if err := streamStaticTextAsSSE(pw, ans); err != nil {
								return
							}
							fmt.Fprintf(bw, "data: [DONE]\n\n")
							bw.Flush()
							return
						}
					}
					messages = append(messages, &einoschema.Message{
						Role:    einoschema.Assistant,
						Content: stripReflectionMarkers(raw),
					})
				}
			}
		}

		writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("ReAct loop exceeded maximum iterations (%d)", maxIter)})
		fmt.Fprintf(bw, "data: [DONE]\n\n")
		bw.Flush()
	}()

	return pr, nil
}

func (r *Runtime) parsePlan(content string) []PlanStep {
	lines := strings.Split(content, "\n")
	var plan []PlanStep
	num := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = line[2:]
		}

		parsed := false
		for i := 1; i <= len(line) && i <= 3; i++ {
			if i < len(line) && (line[i] == '.' || line[i] == ')') {
				if _, ok := parseNum(line[:i]); ok {
					line = strings.TrimSpace(line[i+1:])
					num++
					plan = append(plan, PlanStep{Number: num, Task: line})
					parsed = true
					break
				}
			}
		}

		if !parsed && line != "" {
			num++
			plan = append(plan, PlanStep{Number: num, Task: line})
		}
	}

	return plan
}

func parseNum(s string) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, n > 0
}

func (p *PlanResult) formatSteps() string {
	var sb strings.Builder
	for i, step := range p.Steps {
		if step.Error != "" {
			sb.WriteString(fmt.Sprintf("Step %d (%s): ERROR - %s\n", i+1, step.PlanStep.Task, step.Error))
		} else {
			sb.WriteString(fmt.Sprintf("Step %d (%s): %s\n", i+1, step.PlanStep.Task, step.Result))
		}
	}
	return sb.String()
}

func formatReActSteps(steps []ReActStep) string {
	data, _ := json.MarshalIndent(steps, "", "  ")
	return string(data)
}

func (r *Runtime) ResumeReActLoop(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	tools []tool.BaseTool,
	callID string,
	result string,
	toolErr string,
	clientType string,
) (string, *ReActResult, error) {
	_, msgs, err := r.clientToolMgr.ResumeState(callID, result, toolErr)
	if err != nil {
		return "", nil, err
	}

	maxIter := 16
	if agent.RuntimeProfile != nil && agent.RuntimeProfile.MaxIterations > 0 {
		maxIter = agent.RuntimeProfile.MaxIterations
	}

	fullSystemPrompt := systemPrompt + reactSystemSuffix

	for i, m := range msgs {
		if m.Role == einoschema.System {
			msgs[i] = &einoschema.Message{Role: einoschema.System, Content: fullSystemPrompt}
			break
		}
	}

	result_ := &ReActResult{}
	toolInfos := toolsToToolInfos(tools)
	ov := skillExecOverrides(agent)

	for iter := 0; iter < maxIter; iter++ {
		resp, err := r.chatModel.Generate(ctx, msgs, model.WithTools(toolInfos))
		if err != nil {
			return "", nil, fmt.Errorf("LLM call failed at resume iteration %d: %w", iter+1, err)
		}
		reactTruncateMultiToolCallsToOne(resp)

		if len(resp.ToolCalls) == 0 {
			text := strings.TrimSpace(resp.Content)
			text = polishReActUserVisibleText(text)
			if text == "" || isReflectionStyleAssistantText(text) {
				if alt := lastNonToolAssistantContent(msgs); alt != "" {
					text = alt
				}
			}
			if text == "" {
				text = lastToolResultContent(msgs)
			}
			if text != "" {
				result_.FinalAnswer = text
				return text, result_, nil
			}
			return "", result_, fmt.Errorf("no response from agent after resume")
		}

		msgs = append(msgs, &einoschema.Message{
			Role:      einoschema.Assistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for i := range resp.ToolCalls {
			tc := &resp.ToolCalls[i]
			tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, iter)
			step := ReActStep{
				Iteration: iter + 1,
				Thought:   resp.Content,
				ToolName:  tc.Function.Name,
				ToolArgs:  tc.Function.Arguments,
			}

			if isClientTool(tools, tc.Function.Name, ov) {
				msgCopy := make([]*einoschema.Message, len(msgs))
				copy(msgCopy, msgs)
				r.clientToolMgr.SaveState(&ClientToolCallState{
					CallID:     tc.ID,
					ToolName:   tc.Function.Name,
					ToolArgs:   tc.Function.Arguments,
					Messages:   msgCopy,
					Iter:       iter,
					CreatedAt:  time.Now(),
					ClientType: streamClientType(clientType),
				})
				step.Error = fmt.Sprintf("CLIENT_TOOL_CALL: %s", tc.Function.Name)
				result_.Steps = append(result_.Steps, step)
				return "", result_, &ClientToolCallError{
					CallID:   tc.ID,
					ToolName: tc.Function.Name,
					ToolArgs: tc.Function.Arguments,
				}
			}

			foundTool := findInvokableTool(tools, tc.Function.Name)
			if foundTool == nil {
				step.Error = fmt.Sprintf("tool not found: %s", tc.Function.Name)
				result_.Steps = append(result_.Steps, step)
				msgs = append(msgs, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    fmt.Sprintf("Error: tool not found: %s", tc.Function.Name),
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
				continue
			}

			toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
			if err != nil {
				step.Error = err.Error()
				result_.Steps = append(result_.Steps, step)
				msgs = append(msgs, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    fmt.Sprintf("Error: %s", err.Error()),
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
				continue
			}

			step.ToolResult = toolResult
			result_.TotalCalls++
			msgs = append(msgs, &einoschema.Message{
				Role:       einoschema.Tool,
				Content:    toolResult,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})

			reflectionMsgs := append([]*einoschema.Message{}, msgs...)
			reflectionMsgs = append(reflectionMsgs, &einoschema.Message{
				Role:    einoschema.User,
				Content: reflectionPromptTemplate,
			})

			reflectionResp, err := r.chatModel.Generate(ctx, reflectionMsgs)
			if err == nil && reflectionResp.Content != "" {
				raw := reflectionResp.Content
				step.Reflection = stripReflectionMarkers(raw)
				if len(resp.ToolCalls) == 1 {
					if ans, ok := finalAnswerFromReflection(raw); ok {
						result_.Steps = append(result_.Steps, step)
						result_.FinalAnswer = ans
						return ans, result_, nil
					}
				}
				msgs = append(msgs, &einoschema.Message{
					Role:    einoschema.Assistant,
					Content: stripReflectionMarkers(raw),
				})
			}

			result_.Steps = append(result_.Steps, step)
		}
	}

	return "", result_, fmt.Errorf("ReAct resume loop exceeded maximum iterations (%d)", maxIter)
}

func (r *Runtime) ResumeReActLoopStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	tools []tool.BaseTool,
	callID string,
	result string,
	toolErr string,
	sessionID string,
	auditUserID string,
	clientType string,
) (io.ReadCloser, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)

	_, msgs, err := r.clientToolMgr.ResumeState(callID, result, toolErr)
	if err != nil {
		return nil, err
	}

	maxIter := 16
	if agent.RuntimeProfile != nil && agent.RuntimeProfile.MaxIterations > 0 {
		maxIter = agent.RuntimeProfile.MaxIterations
	}

	fullSystemPrompt := systemPrompt + reactSystemSuffix

	for i, m := range msgs {
		if m.Role == einoschema.System {
			msgs[i] = &einoschema.Message{Role: einoschema.System, Content: fullSystemPrompt}
			break
		}
	}

	toolInfos := toolsToToolInfos(tools)

	pr, pw := io.Pipe()
	bw := bufio.NewWriterSize(pw, 4096)

	go func() {
		defer func() {
			bw.Flush()
			pw.Close()
		}()
		defer r.flushUsageSession(ctx)

		writeEvent := func(evt ReActEvent) {
			data, _ := json.Marshal(evt)
			fmt.Fprintf(bw, "data: %s\n\n", data)
			bw.Flush()
		}
		ov := skillExecOverrides(agent)

		for iter := 0; iter < maxIter; iter++ {
			resp, err := r.chatModel.Generate(ctx, msgs, model.WithTools(toolInfos))
			if err != nil {
				writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("LLM call failed: %v", err), Step: iter + 1})
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}
			reactTruncateMultiToolCallsToOne(resp)

			if len(resp.ToolCalls) == 0 {
				text := strings.TrimSpace(resp.Content)
				text = polishReActUserVisibleText(text)
				if text == "" || isReflectionStyleAssistantText(text) {
					if alt := lastNonToolAssistantContent(msgs); alt != "" {
						text = alt
					}
				}
				if text == "" {
					text = lastToolResultContent(msgs)
				}
				text = VisibleAssistantOrFallback(text, AssistantEmptyVisibleFallback)
				writeEvent(ReActEvent{Type: "final_answer", Content: text, Step: iter + 1})
				if err := streamStaticTextAsSSE(pw, text); err != nil {
					fmt.Fprintf(bw, "data: [DONE]\n\n")
					bw.Flush()
					return
				}
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}

			writeEvent(ReActEvent{Type: "thought", Content: resp.Content, Step: iter + 1})

			msgs = append(msgs, &einoschema.Message{
				Role:      einoschema.Assistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			for i := range resp.ToolCalls {
				tc := &resp.ToolCalls[i]
				tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, iter)
				writeEvent(ReActEvent{
					Type:      "action",
					Content:   fmt.Sprintf("Calling tool: %s", tc.Function.Name),
					Step:      iter + 1,
					Tool:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})

				if isClientTool(tools, tc.Function.Name, ov) {
					blocked, apprID, apprErr := r.GateClientToolApproval(agent, sessionID, auditUserID, tc.Function.Name, tc.Function.Arguments)
					if apprErr != nil {
						writeEvent(ReActEvent{Type: "error", Content: apprErr.Error(), Step: iter + 1, Tool: tc.Function.Name})
						fmt.Fprintf(bw, "data: [DONE]\n\n")
						bw.Flush()
						return
					}
					msgCopy := make([]*einoschema.Message, len(msgs))
					copy(msgCopy, msgs)
					r.clientToolMgr.SaveState(&ClientToolCallState{
						CallID:     tc.ID,
						ToolName:   tc.Function.Name,
						ToolArgs:   tc.Function.Arguments,
						Messages:   msgCopy,
						Iter:       iter,
						CreatedAt:  time.Now(),
						ClientType: streamClientType(clientType),
					})
					execMode := getToolExecutionModeFromTools(tools, tc.Function.Name, ov)
					evt := ReActEvent{
						Type:          "client_tool_call",
						Content:       fmt.Sprintf("需要在客户端执行: %s", tc.Function.Name),
						Step:          iter + 1,
						Tool:          tc.Function.Name,
						Arguments:     tc.Function.Arguments,
						CallID:        tc.ID,
						Hint:          clientToolHint(tc.Function.Name),
						RiskLevel:     r.resolveToolRiskLevel(tc.Function.Name),
						ExecutionMode: execMode,
					}
					if blocked && apprID > 0 {
						evt.ApprovalID = apprID
					}
					writeEvent(evt)
					fmt.Fprintf(bw, "data: [DONE]\n\n")
					bw.Flush()
					return
				}

				foundTool := findInvokableTool(tools, tc.Function.Name)
				if foundTool == nil {
					errMsg := fmt.Sprintf("tool not found: %s", tc.Function.Name)
					writeEvent(ReActEvent{Type: "error", Content: errMsg, Step: iter + 1, Tool: tc.Function.Name})
					msgs = append(msgs, &einoschema.Message{
						Role:       einoschema.Tool,
						Content:    "Error: " + errMsg,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					continue
				}

				toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
				if err != nil {
					writeEvent(ReActEvent{Type: "error", Content: err.Error(), Step: iter + 1, Tool: tc.Function.Name})
					msgs = append(msgs, &einoschema.Message{
						Role:       einoschema.Tool,
						Content:    fmt.Sprintf("Error: %s", err.Error()),
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
					continue
				}

				writeEvent(ReActEvent{Type: "observation", Content: toolResult, Step: iter + 1, Tool: tc.Function.Name})

				msgs = append(msgs, &einoschema.Message{
					Role:       einoschema.Tool,
					Content:    toolResult,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})

				reflectionMsgs := append([]*einoschema.Message{}, msgs...)
				reflectionMsgs = append(reflectionMsgs, &einoschema.Message{
					Role:    einoschema.User,
					Content: reflectionPromptTemplate,
				})

				reflectionResp, err := r.chatModel.Generate(ctx, reflectionMsgs)
				if err == nil && reflectionResp.Content != "" {
					raw := reflectionResp.Content
					writeEvent(ReActEvent{Type: "reflection", Content: stripReflectionMarkers(raw), Step: iter + 1})
					if len(resp.ToolCalls) == 1 {
						if ans, ok := finalAnswerFromReflection(raw); ok {
							writeEvent(ReActEvent{Type: "final_answer", Content: ans, Step: iter + 1})
							if err := streamStaticTextAsSSE(pw, ans); err != nil {
								return
							}
							fmt.Fprintf(bw, "data: [DONE]\n\n")
							bw.Flush()
							return
						}
					}
					msgs = append(msgs, &einoschema.Message{
						Role:    einoschema.Assistant,
						Content: stripReflectionMarkers(raw),
					})
				}
			}
		}

		writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("ReAct resume stream loop exceeded maximum iterations (%d)", maxIter)})
		fmt.Fprintf(bw, "data: [DONE]\n\n")
		bw.Flush()
	}()

	return pr, nil
}
