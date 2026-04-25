import type { ChatReactStep } from 'src/api/types'

export type ThoughtStepVM = {
  type: string
  data: Record<string, unknown>
  meta?: Record<string, unknown>
  timestamp?: string
}

/** 与 cloneThoughtStepsSnapshot 相反：把持久化/接口中的 ChatReactStep 填回侧栏（定时任务等非流式场景无 SSE，仅靠 GET /messages） */
export function chatReactStepsToThoughtSteps (steps: ChatReactStep[]): ThoughtStepVM[] {
  return steps.map(s => ({
    type: s.type,
    data: JSON.parse(JSON.stringify(s.data)) as Record<string, unknown>,
    meta: s.meta != null ? (JSON.parse(JSON.stringify(s.meta)) as Record<string, unknown>) : undefined,
    timestamp: s.timestamp
  }))
}

/** 多条 plan_execute 时仅保留最后一条（与 getCurrentPlanTasks / merge 逻辑一致），避免主气泡与侧栏重复卡片 */
function dedupePlanExecuteThoughtSteps (steps: ThoughtStepVM[]): ThoughtStepVM[] {
  const planIdx: number[] = []
  for (let i = 0; i < steps.length; i++) {
    const s = steps[i]
    if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
      planIdx.push(i)
    }
  }
  if (planIdx.length <= 1) return steps
  const keep = planIdx[planIdx.length - 1]
  return steps.filter((_, i) => !(planIdx.includes(i) && i !== keep))
}

export function cloneThoughtStepsSnapshot (steps: ThoughtStepVM[]): ChatReactStep[] {
  const deduped = dedupePlanExecuteThoughtSteps(steps)
  return deduped.map(s => ({
    type: s.type,
    data: JSON.parse(JSON.stringify(s.data)) as Record<string, unknown>,
    meta: s.meta != null ? (JSON.parse(JSON.stringify(s.meta)) as Record<string, unknown>) : undefined,
    timestamp: s.timestamp
  }))
}
