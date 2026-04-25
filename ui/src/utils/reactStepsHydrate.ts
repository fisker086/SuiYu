import type { ChatReactStep } from 'src/api/types'
import {
  applyPlanStepStatusToTasks,
  initTasksFromPlanTasksPayload,
  mergePlanDetailFromReActPayload,
  type PlanTaskRowWeb
} from 'src/utils/planExecuteMerge'

/** tool_result / 部分 ADK 事件里 result 可能是 JSON 对象，统一成可解析的字符串 */
export function toolResultPayloadToContentString (payload: Record<string, unknown>): string {
  const raw = payload.result ?? payload.content
  if (typeof raw === 'string') return raw
  if (raw != null && typeof raw === 'object') return JSON.stringify(raw)
  if (raw != null) return String(raw)
  return ''
}

/**
 * Converts persisted SSE payloads (from agent_memory.extra.react_steps) into the same
 * ChatReactStep shape as live streaming (processReActEvent / processStreamEvent).
 */
export function hydrateReactStepsFromServer (raw: unknown): ChatReactStep[] {
  if (!Array.isArray(raw) || raw.length === 0) return []
  const payloads = raw.filter((x): x is Record<string, unknown> => x !== null && typeof x === 'object')

  const planIdx = payloads.findIndex(
    p => p.type === 'plan_tasks' && Array.isArray(p.plan_tasks) && (p.plan_tasks as unknown[]).length > 0
  )
  if (planIdx < 0) {
    const out: ChatReactStep[] = []
    for (const p of payloads) {
      const step = serverPayloadToReactStep(p)
      if (step) out.push(step)
    }
    return out
  }

  const before: ChatReactStep[] = []
  for (let i = 0; i < planIdx; i++) {
    const step = serverPayloadToReactStep(payloads[i])
    if (step) before.push(step)
  }

  let tasks: PlanTaskRowWeb[] = initTasksFromPlanTasksPayload(payloads[planIdx])
  const after: ChatReactStep[] = []
  for (let i = planIdx + 1; i < payloads.length; i++) {
    const p = payloads[i]
    const t = p.type as string | undefined
    if (t === 'plan_step') {
      tasks = applyPlanStepStatusToTasks(tasks, p)
      continue
    }
    if (t === 'plan_tasks') {
      continue
    }
    if (t === 'error' || t === 'observation' || t === 'thought' || t === 'action') {
      const merged = mergePlanDetailFromReActPayload(tasks, p)
      if (merged) {
        tasks = merged
        continue
      }
      const step = serverPayloadToReactStep(p)
      if (step) after.push(step)
      continue
    }
    const step = serverPayloadToReactStep(p)
    if (step) after.push(step)
  }

  const planCard: ChatReactStep = {
    type: 'plan',
    data: { kind: 'plan_execute', tasks: tasks.map((row) => ({ ...row })) },
    timestamp: new Date().toISOString()
  }
  return [...before, planCard, ...after]
}

function serverPayloadToReactStep (payload: Record<string, unknown>): ChatReactStep | null {
  if (payload.event_type != null && String(payload.event_type).trim() !== '') {
    return streamPayloadToStep(payload)
  }
  if (payload.type != null && String(payload.type).trim() !== '') {
    return reactPayloadToStep(payload)
  }
  return null
}

function reactPayloadToStep (payload: Record<string, unknown>): ChatReactStep | null {
  const reactType = payload.type as string | undefined
  if (!reactType) return null

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

  return {
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
  }
}

function streamPayloadToStep (payload: Record<string, unknown>): ChatReactStep | null {
  const eventType = payload?.event_type as string
  if (!eventType) return null

  if (eventType === 'approval_required') {
    return {
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
    }
  }

  if (eventType === 'agent_routed') {
    const agent = (payload.agent as string) || (payload.node as string) || 'unknown_agent'
    return {
      type: 'thought',
      data: { content: `已路由到 ${agent}`, agent, source: 'agent_routed' },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    }
  }

  if (eventType === 'node_start') {
    const node = payload.node as string
    if (!node || node === 'Supervisor') return null
    if (node.endsWith('_agent')) return null
    return {
      type: 'thought',
      data: { content: `进入节点 ${node}`, node, source: 'node_start' },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    }
  }

  if (eventType === 'node_complete') {
    const node = payload.node as string
    if (!node || node === 'Supervisor') return null
    if (node.endsWith('_agent')) return null
    return {
      type: 'observation',
      data: { content: `完成节点 ${node}`, node },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    }
  }

  if (eventType === 'tool_call') {
    const toolName = (payload.tool_name as string) || (payload.name as string) || 'unknown'
    return {
      type: 'action',
      data: {
        name: toolName,
        input: (payload.input as Record<string, unknown>) || {},
        content: (payload.content as string) || ''
      },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    }
  }

  if (eventType === 'tool_result') {
    const toolName = (payload.tool_name as string) || 'tool'
    const resultContent = toolResultPayloadToContentString(payload)
    return {
      type: 'observation',
      data: { name: toolName, content: resultContent },
      meta: { modelName: (payload as Record<string, unknown>).model_name || '' },
      timestamp: (payload as Record<string, unknown>).timestamp as string
    }
  }

  return null
}
