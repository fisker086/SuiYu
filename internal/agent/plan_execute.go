package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
)

func (r *Runtime) runPlanAndExecute(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	userInput string,
	history []*einoschema.Message,
	tools []tool.BaseTool,
	sessionID, auditUserID string,
) (string, *ReActResult, error) {
	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}

	if execMode == ExecutionModeAuto {
		mode, err := r.resolveExecutionModeAuto(ctx, agent, userInput, history, tools)
		if err != nil {
			return "", nil, err
		}
		execMode = mode
		logger.Info("auto mode resolved", "agent_id", agent.ID, "resolved_mode", execMode, "user_input", userInput)
	}

	toolInfos := toolsToToolInfos(tools)
	ov := skillExecOverrides(agent)

	if execMode == ExecutionModeSingleCall || shouldBypassPlanExecutePlanner(userInput) {
		var msgs []*einoschema.Message
		if strings.TrimSpace(systemPrompt) != "" {
			msgs = append(msgs, &einoschema.Message{Role: einoschema.System, Content: systemPrompt})
		}
		msgs = append(msgs, history...)
		msgs = append(msgs, &einoschema.Message{Role: einoschema.User, Content: userInput})
		resp, err := r.chatModel.Generate(ctx, msgs)
		if err != nil {
			logger.Error("plan-execute: single-call Generate failed", "error", err)
			return "", nil, err
		}
		return resp.Content, nil, nil
	}

	planPrompt := systemPrompt + planSystemSuffix
	planMessages := []*einoschema.Message{
		{Role: einoschema.System, Content: planPrompt},
	}
	planMessages = append(planMessages, history...)
	planMessages = append(planMessages, &einoschema.Message{
		Role:    einoschema.User,
		Content: fmt.Sprintf("Create a step-by-step plan for this request:\n\n%s", userInput),
	})

	planResp, err := r.chatModel.Generate(ctx, planMessages)
	if err != nil {
		logger.Error("plan-execute: planning Generate failed", "error", err)
		return "", nil, fmt.Errorf("planning failed: %w", err)
	}

	simple, planBody := parsePlanAndExecuteRouting(planResp.Content)
	if simple {
		logger.Info("plan-execute sync: routing SIMPLE", "agent_id", agent.ID)
		var msgs []*einoschema.Message
		if strings.TrimSpace(systemPrompt) != "" {
			msgs = append(msgs, &einoschema.Message{Role: einoschema.System, Content: systemPrompt})
		}
		msgs = append(msgs, history...)
		msgs = append(msgs, &einoschema.Message{Role: einoschema.User, Content: userInput})
		resp, err := r.chatModel.Generate(ctx, msgs)
		if err != nil {
			return "", nil, err
		}
		return resp.Content, nil, nil
	}

	planText := planBody
	if strings.TrimSpace(planText) == "" {
		planText = planResp.Content
	}
	plan := r.parsePlan(planBody)
	if len(plan) == 0 {
		plan = r.parsePlan(planResp.Content)
	}
	if len(plan) == 0 {
		return "", nil, fmt.Errorf("failed to parse plan from LLM response")
	}

	result := &PlanResult{Plan: plan}

	for i, step := range plan {
		stepMessages := []*einoschema.Message{
			{Role: einoschema.System, Content: systemPrompt},
			{Role: einoschema.User, Content: userInput},
			{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s", planText)},
		}

		for j := 0; j < i; j++ {
			if result.Steps[j].Error == "" {
				stepMessages = append(stepMessages, &einoschema.Message{
					Role:    einoschema.Assistant,
					Content: fmt.Sprintf("Step %d completed: %s\nResult: %s", j+1, result.Steps[j].PlanStep.Task, result.Steps[j].Result),
				})
			}
		}

		currentTask := fmt.Sprintf("Execute step %d of %d: %s", i+1, len(plan), step.Task)
		stepMessages = append(stepMessages, &einoschema.Message{
			Role:    einoschema.User,
			Content: currentTask,
		})

		resp, err := r.chatModel.Generate(ctx, stepMessages, model.WithTools(toolInfos))
		execStep := ExecStep{PlanStep: step}

		if err != nil {
			logger.Error("plan-execute: step Generate failed", "error", err, "step", i+1)
			execStep.Error = err.Error()
			result.Steps = append(result.Steps, execStep)
			continue
		}

		if len(resp.ToolCalls) > 0 {
			for _, tc := range resp.ToolCalls {
				if isClientTool(tools, tc.Function.Name, ov) {
					tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, i)
					return "", nil, &ClientToolCallError{
						CallID:   tc.ID,
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					}
				}

				foundTool := findInvokableTool(tools, tc.Function.Name)
				if foundTool == nil {
					execStep.Error = fmt.Sprintf("tool not found: %s", tc.Function.Name)
					continue
				}

				toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
				if err != nil {
					execStep.Error = fmt.Sprintf("tool %s: %v", tc.Function.Name, err)
				} else {
					execStep.Result += toolResult + "\n"
				}
			}
		}

		if execStep.Result == "" {
			execStep.Result = resp.Content
		}

		result.Steps = append(result.Steps, execStep)
	}

	synthPrompt := systemPrompt + planExecuteSystemSuffix
	synthMessages := []*einoschema.Message{
		{Role: einoschema.System, Content: synthPrompt},
		{Role: einoschema.User, Content: userInput},
		{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s\n\nExecution Results:\n%s", planText, result.formatSteps())},
	}

	finalResp, err := r.chatModel.Generate(ctx, synthMessages)
	if err != nil {
		logger.Error("plan-execute: synthesis Generate failed", "error", err)
		return "", nil, fmt.Errorf("synthesis failed: %w", err)
	}

	result.FinalAnswer = finalResp.Content
	return finalResp.Content, nil, nil
}

func (r *Runtime) runPlanAndExecuteStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	userInput string,
	history []*einoschema.Message,
	tools []tool.BaseTool,
	clientType, sessionID, auditUserID string,
) (io.ReadCloser, error) {
	execMode := ExecutionModeDefault
	if agent.RuntimeProfile != nil {
		execMode = agent.RuntimeProfile.ExecutionMode
	}

	if execMode == ExecutionModeAuto {
		mode, err := r.resolveExecutionModeAuto(ctx, agent, userInput, history, tools)
		if err != nil {
			return nil, err
		}
		execMode = mode
		logger.Info("auto mode resolved (stream)", "agent_id", agent.ID, "resolved_mode", execMode, "user_input", userInput)
	}

	toolInfos := toolsToToolInfos(tools)
	ov := skillExecOverrides(agent)

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

		if shouldBypassPlanExecutePlanner(userInput) || execMode == ExecutionModeSingleCall {
			writeEvent(ReActEvent{Type: "thought", Content: "正在直接回复…", Step: 0})
			msgs := buildStreamChatMessages(systemPrompt, history, userInput, nil)
			if err := r.streamChatModelTokensToSSE(ctx, bw, msgs); err != nil {
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}
			bw.Flush()
			fmt.Fprintf(bw, "data: [DONE]\n\n")
			bw.Flush()
			return
		}

		planPrompt := systemPrompt + planSystemSuffix
		planMessages := []*einoschema.Message{
			{Role: einoschema.System, Content: planPrompt},
		}
		planMessages = append(planMessages, history...)
		planMessages = append(planMessages, &einoschema.Message{
			Role:    einoschema.User,
			Content: fmt.Sprintf("Create a step-by-step plan for this request:\n\n%s", userInput),
		})

		writeEvent(ReActEvent{Type: "thought", Content: "正在生成执行计划...", Step: 0})

		planRaw, err := r.streamChatModelTokensToSSEAccumulate(ctx, bw, planMessages)
		if err != nil {
			writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("planning failed: %v", err)})
			return
		}

		simple, planBody := parsePlanAndExecuteRouting(planRaw)
		if simple {
			logger.Info("plan-execute stream: routing SIMPLE", "agent_id", agent.ID)
			writeEvent(ReActEvent{Type: "thought", Content: "当前请求无需多步执行正在直接回复…", Step: 0})
			msgs := buildStreamChatMessages(systemPrompt, history, userInput, nil)
			if err := r.streamChatModelTokensToSSE(ctx, bw, msgs); err != nil {
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}
			fmt.Fprintf(bw, "data: [DONE]\n\n")
			bw.Flush()
			return
		}

		planText := planBody
		if strings.TrimSpace(planText) == "" {
			planText = planRaw
		}
		plan := r.parsePlan(planBody)
		if len(plan) == 0 {
			plan = r.parsePlan(planRaw)
		}
		if len(plan) == 0 {
			writeEvent(ReActEvent{Type: "error", Content: "failed to parse plan"})
			return
		}

		writeEvent(ReActEvent{
			Type:    "observation",
			Content: fmt.Sprintf("已解析为 %d 步执行计划", len(plan)),
			Step:    0,
		})

		planItems := make([]PlanTaskItem, 0, len(plan))
		for i, s := range plan {
			planItems = append(planItems, PlanTaskItem{Index: i + 1, Task: s.Task})
		}
		writeEvent(ReActEvent{
			Type:      "plan_tasks",
			Content:   fmt.Sprintf("%d steps", len(plan)),
			Step:      len(plan),
			PlanTasks: planItems,
		})

		planExec := &PlanResult{Plan: plan}

		for i, step := range plan {
			writeEvent(ReActEvent{
				Type:           "plan_step",
				Content:        step.Task,
				Step:           i + 1,
				PlanStepStatus: "running",
			})
			writeEvent(ReActEvent{Type: "thought", Content: fmt.Sprintf("执行步骤 %d/%d: %s", i+1, len(plan), step.Task), Step: i + 1})

			stepMessages := []*einoschema.Message{
				{Role: einoschema.System, Content: systemPrompt},
				{Role: einoschema.User, Content: userInput},
				{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s", planText)},
			}

			currentTask := fmt.Sprintf("Execute step %d of %d: %s", i+1, len(plan), step.Task)
			stepMessages = append(stepMessages, &einoschema.Message{
				Role:    einoschema.User,
				Content: currentTask,
			})

			resp, err := r.chatModel.Generate(ctx, stepMessages, model.WithTools(toolInfos))
			execStep := ExecStep{PlanStep: step}
			if err != nil {
				logger.Error("plan-execute: async step Generate failed", "error", err, "step", i+1)
				execStep.Error = err.Error()
				planExec.Steps = append(planExec.Steps, execStep)
				writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("step %d failed: %v", i+1, err), Step: i + 1})
				writeEvent(ReActEvent{
					Type:           "plan_step",
					Content:        err.Error(),
					Step:           i + 1,
					PlanStepStatus: "error",
				})
				writeEvent(ReActEvent{
					Type:    "final_answer",
					Content: fmt.Sprintf("步骤 %d 执行失败: %v", i+1, err),
					Step:    i + 1,
				})
				if err := streamStaticTextAsSSE(pw, fmt.Sprintf("步骤 %d 执行失败: %v", i+1, err)); err != nil {
					return
				}
				fmt.Fprintf(bw, "data: [DONE]\n\n")
				bw.Flush()
				return
			}

			writeEvent(ReActEvent{Type: "thought", Content: resp.Content, Step: i + 1})

			if len(resp.ToolCalls) > 0 {
				for _, tc := range resp.ToolCalls {
					writeEvent(ReActEvent{Type: "action", Content: fmt.Sprintf("调用工具: %s", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name, Arguments: tc.Function.Arguments})

					if isClientTool(tools, tc.Function.Name, ov) {
						execMode := getToolExecutionModeFromTools(tools, tc.Function.Name, ov)
						if execMode == schema.ExecutionModeClient && clientType != "desktop" {
							writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name})
							writeEvent(ReActEvent{Type: "final_answer", Content: fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name), Step: i + 1})
							if err := streamStaticTextAsSSE(pw, fmt.Sprintf("工具 %s 暂时不支持 Web 端，请在桌面客户端上使用", tc.Function.Name)); err != nil {
								return
							}
							fmt.Fprintf(bw, "data: [DONE]\n\n")
							bw.Flush()
							return
						}
						blocked, apprID, apprErr := r.GateClientToolApproval(agent, sessionID, auditUserID, tc.Function.Name, tc.Function.Arguments)
						if apprErr != nil {
							writeEvent(ReActEvent{Type: "error", Content: apprErr.Error(), Step: i + 1, Tool: tc.Function.Name})
							writeEvent(ReActEvent{Type: "final_answer", Content: fmt.Sprintf("需要审批: %s", apprErr.Error()), Step: i + 1})
							if err := streamStaticTextAsSSE(pw, fmt.Sprintf("需要审批: %s", apprErr.Error())); err != nil {
								return
							}
							fmt.Fprintf(bw, "data: [DONE]\n\n")
							bw.Flush()
							return
						}
						tc.ID = ensureReActToolCallID(tc.ID, tc.Function.Name, i)
						// Must include the assistant turn with ToolCalls before the tool result (same as ReAct
						// SaveState). Otherwise ResumeState appends Tool to a history with no matching
						// tool_calls — the model returns empty / unusable on resume (desktop shows generic failure).
						msgsForSave := append(append([]*einoschema.Message(nil), stepMessages...), &einoschema.Message{
							Role:      einoschema.Assistant,
							Content:   resp.Content,
							ToolCalls: resp.ToolCalls,
						})
						msgCopy := make([]*einoschema.Message, len(msgsForSave))
						copy(msgCopy, msgsForSave)
						r.clientToolMgr.SaveState(&ClientToolCallState{
							CallID:       tc.ID,
							ToolName:     tc.Function.Name,
							ToolArgs:     tc.Function.Arguments,
							Messages:     msgCopy,
							Iter:         i,
							CreatedAt:    time.Now(),
							ClientType:   clientType,
							PlanMode:     true,
							PlanText:     planText,
							PlanIndex:    i,
							PlanSteps:    plan,
							UserInput:    userInput,
							SystemPrompt: systemPrompt,
						})
						evt := ReActEvent{
							Type:          "client_tool_call",
							Content:       fmt.Sprintf("需要在客户端执行: %s", tc.Function.Name),
							Step:          i + 1,
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
						writeEvent(ReActEvent{Type: "error", Content: fmt.Sprintf("tool not found: %s", tc.Function.Name), Step: i + 1, Tool: tc.Function.Name})
						execStep.Error = fmt.Sprintf("tool not found: %s", tc.Function.Name)
						continue
					}

					toolResult, err := foundTool.InvokableRun(ctx, tc.Function.Arguments)
					if err != nil {
						writeEvent(ReActEvent{Type: "error", Content: err.Error(), Step: i + 1, Tool: tc.Function.Name})
						execStep.Error = fmt.Sprintf("tool %s: %v", tc.Function.Name, err)
					} else {
						writeEvent(ReActEvent{Type: "observation", Content: toolResult, Step: i + 1, Tool: tc.Function.Name})
						execStep.Result += toolResult + "\n"
					}
				}
			}

			if execStep.Result == "" && execStep.Error == "" {
				execStep.Result = resp.Content
			}

			planExec.Steps = append(planExec.Steps, execStep)

			writeEvent(ReActEvent{Type: "observation", Content: fmt.Sprintf("步骤 %d 完成", i+1), Step: i + 1})
			writeEvent(ReActEvent{
				Type:           "plan_step",
				Content:        step.Task,
				Step:           i + 1,
				PlanStepStatus: "done",
			})
		}

		writeEvent(ReActEvent{Type: "thought", Content: "正在综合所有步骤结果...", Step: len(plan) + 1})

		synthPrompt := systemPrompt + planExecuteSystemSuffix
		synthMessages := []*einoschema.Message{
			{Role: einoschema.System, Content: synthPrompt},
			{Role: einoschema.User, Content: userInput},
			{Role: einoschema.Assistant, Content: fmt.Sprintf("Plan:\n%s\n\nExecution Results:\n%s", planText, planExec.formatSteps())},
		}

		finalResp, err := r.chatModel.Generate(ctx, synthMessages)
		if err != nil {
			logger.Error("plan-execute: stream synthesis Generate failed", "error", err)
			writeEvent(ReActEvent{Type: "thought", Content: "综合步骤结果失败（步骤已全部完成）。"})
			writeEvent(ReActEvent{
				Type:    "final_answer",
				Content: "任务执行完成，所有步骤已成功执行。",
				Step:    len(plan) + 1,
			})
			if err := streamStaticTextAsSSE(pw, "任务执行完成，所有步骤已成功执行。"); err != nil {
				return
			}
			fmt.Fprintf(bw, "data: [DONE]\n\n")
			bw.Flush()
			return
		}

		finalText := VisibleAssistantOrFallback(finalResp.Content, PlanExecuteSynthesisEmptyFallback)
		writeEvent(ReActEvent{Type: "final_answer", Content: finalText, Step: len(plan) + 1})
		if err := streamStaticTextAsSSE(pw, finalText); err != nil {
			return
		}
		fmt.Fprintf(bw, "data: [DONE]\n\n")
		bw.Flush()
	}()

	return pr, nil
}
