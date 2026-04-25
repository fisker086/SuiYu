import type { ChatReactStep } from 'src/api/types'

/** 与 ChatReactInline 一致：可展示的 ReAct 步骤（排除 final、流式占位等） */
export function filterVisibleReactSteps (steps: ChatReactStep[]): ChatReactStep[] {
  return steps.filter(s => {
    if (s.type === 'final') return false
    if (s.type === 'thought' && s.data?.source === 'stream_tokens') return false
    return ['thought', 'action', 'observation', 'reflection', 'error'].includes(s.type)
  })
}
