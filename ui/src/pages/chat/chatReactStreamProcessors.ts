import { toolResultPayloadToContentString } from 'src/utils/reactStepsHydrate'
import { riskLevelLabelZh } from 'src/utils/toolRisk'
import type { ThoughtStepVM } from './chatReactStepsSync'

export type StreamProcessorsContext = {
  pushThoughtStep: (step: ThoughtStepVM) => void
  resolveToolRiskForStream: (toolName: string, payloadRisk: unknown) => string
  /** ADK：记录最近一次 tool_result 全文，供主气泡去掉模型重复粘贴的整段工具输出 */
  setLastServerToolResult?: (text: string) => void
  /** Plan-and-execute: merge `plan_tasks` / `plan_step` into one checklist row (same idea as desktop PlanExecuteTaskPanel). */
  upsertPlanExecute?: (payload: Record<string, unknown>) => void
  /** Attach `error` / `observation` / `thought` / `action` with `step` to that plan row (ordered sub-lines). */
  mergePlanExecuteReActEvent?: (payload: Record<string, unknown>) => boolean
}

export function processReActEvent (payload: Record<string, unknown>, ctx: StreamProcessorsContext): void {
  const { pushThoughtStep, resolveToolRiskForStream, upsertPlanExecute, mergePlanExecuteReActEvent } = ctx
  const reactType = payload.type as string | undefined
  if (!reactType) return

  if (reactType === 'plan_tasks' || reactType === 'plan_step') {
    upsertPlanExecute?.(payload)
    return
  }

  if (
    reactType === 'error' ||
    reactType === 'observation' ||
    reactType === 'thought' ||
    reactType === 'action'
  ) {
    if (mergePlanExecuteReActEvent?.(payload)) return
  }

  /** ReAct 暂停并交给浏览器/桌面客户端执行；SSE 可能整包到达，与是否流式分片无关，只要每行 `data: {...}` 可解析即可 */
  if (reactType === 'client_tool_call') {
    const toolName = (payload.tool as string) || ''
    const risk = resolveToolRiskForStream(toolName, payload.risk_level)
    const hint = typeof payload.hint === 'string' ? payload.hint : ''
    const argsStr = payload.arguments as string | undefined
    let clientInput: Record<string, unknown> | undefined
    if (argsStr != null && String(argsStr).trim() !== '') {
      try {
        const parsed = JSON.parse(String(argsStr)) as unknown
        if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
          clientInput = parsed as Record<string, unknown>
        } else {
          clientInput = { value: parsed }
        }
      } catch {
        clientInput = { raw: String(argsStr) }
      }
    }
    const content =
      (payload.content as string) ||
      (toolName ? `需在客户端执行: ${toolName}（风险 ${riskLevelLabelZh(risk)}）` : '需在客户端执行工具')
    pushThoughtStep({
      type: 'action',
      data: {
        kind: 'client_tool_call',
        name: toolName,
        tool: toolName,
        content,
        hint,
        call_id: payload.call_id,
        risk_level: risk,
        reactType,
        ...(clientInput != null ? { input: clientInput } : {}),
        label: `客户端工具 · 风险 ${riskLevelLabelZh(risk)}${toolName ? ` · ${toolName}` : ''}`
      },
      timestamp: new Date().toISOString()
    })
    return
  }

  const step = payload.step as number | undefined
  const tool = payload.tool as string | undefined
  const content = (payload.content as string) || ''
  const argsStr = payload.arguments as string | undefined
  let input: Record<string, unknown> | undefined
  if (argsStr != null && String(argsStr).trim() !== '') {
    try {
      const parsed = JSON.parse(String(argsStr)) as unknown
      if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
        input = parsed as Record<string, unknown>
      } else {
        input = { value: parsed }
      }
    } catch {
      input = { raw: String(argsStr) }
    }
  }

  const typeMap: Record<string, string> = {
    thought: 'thought',
    action: 'action',
    observation: 'observation',
    reflection: 'reflection',
    final_answer: 'final',
    error: 'error'
  }

  const stepType = typeMap[reactType] || 'thought'

  const labels: Record<string, string> = {
    thought: '思考',
    action: tool ? `调用工具: ${tool}` : '行动',
    observation: tool ? `工具返回: ${tool}` : '观察',
    reflection: '反思',
    final_answer: '最终回答',
    error: '错误'
  }

  pushThoughtStep({
    type: stepType,
    data: {
      content,
      step,
      tool,
      name: tool,
      ...(input != null ? { input } : {}),
      reactType,
      label: labels[reactType] || reactType
    },
    timestamp: new Date().toISOString()
  })
}

export function processStreamEvent (payload: Record<string, unknown>, ctx: StreamProcessorsContext): void {
  const { pushThoughtStep, resolveToolRiskForStream, setLastServerToolResult } = ctx
  const eventType = payload?.event_type as string
  if (!eventType) return

  if (eventType === 'approval_required') {
    pushThoughtStep({
      type: 'action',
      data: {
        name: 'HITL',
        kind: 'hitl',
        phase: 'requested',
        input: {
          approval_id: payload.approval_id || null,
          requested_action: (payload.approval_payload as Record<string, unknown>)?.requested_action || null,
          title: (payload.approval_payload as Record<string, unknown>)?.title || null,
          field_count: Array.isArray((payload.approval_payload as Record<string, unknown>)?.fields)
            ? ((payload.approval_payload as Record<string, unknown>).fields as unknown[]).length
            : 0
        },
        content: '触发人工确认，等待用户选择。'
      },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
    return
  }

  if (eventType === 'agent_routed') {
    const agent = (payload.agent as string) || (payload.node as string) || 'unknown_agent'
    pushThoughtStep({
      type: 'thought',
      data: { content: `已路由到 ${agent}`, agent, source: 'agent_routed' },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
    return
  }

  if (eventType === 'node_start') {
    const node = payload.node as string
    if (!node || node === 'Supervisor') return
    if (node.endsWith('_agent')) return
    pushThoughtStep({
      type: 'thought',
      data: { content: `进入节点 ${node}`, node, source: 'node_start' },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
    return
  }

  if (eventType === 'node_complete') {
    const node = payload.node as string
    if (!node || node === 'Supervisor') return
    if (node.endsWith('_agent')) return
    pushThoughtStep({
      type: 'observation',
      data: { content: `完成节点 ${node}`, node },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
    return
  }

  if (eventType === 'tool_call') {
    const toolName = (payload.tool_name as string) || (payload.name as string) || 'unknown'
    const risk = resolveToolRiskForStream(toolName, payload.risk_level)
    pushThoughtStep({
      type: 'action',
      data: {
        name: toolName,
        input: (payload.input as Record<string, unknown>) || {},
        content: (payload.content as string) || '',
        risk_level: risk,
        kind: 'server_tool_call'
      },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
    return
  }

  if (eventType === 'tool_result') {
    const toolName = (payload.tool_name as string) || 'tool'
    const resultContent = toolResultPayloadToContentString(payload as Record<string, unknown>)
    if (resultContent !== '') {
      setLastServerToolResult?.(resultContent)
    }
    pushThoughtStep({
      type: 'observation',
      data: { name: toolName, content: resultContent },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
  }

  if (eventType === 'info') {
    const content = payload.content as string || ''
    pushThoughtStep({
      type: 'observation',
      data: {
        content,
        kind: 'info'
      },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    })
  }
}
