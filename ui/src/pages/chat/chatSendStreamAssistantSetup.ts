import type { Ref } from 'vue'
import type { Agent } from 'src/api/types'
import { extractMentions } from './chatMentions'

/**
 * 流式开始前在 localTurns 中预置助手占位行，并构造群聊 agent_id → 行下标映射。
 * 与原先 sendStream 内联逻辑一致。
 */
export type AssistantStreamSetup = {
  assistantIdx: number
  assistantEntries: Map<number, number>
  /** 群聊本回合首条助手行下标，供 parseStreamEvents 整段 splice */
  groupAssistantBlockStartIdx: number
  pendingAgentIds: number[]
  ensureAssistantRow: (aid: number) => number
}

export function createAssistantStreamSetup<T extends {
  role: string
  content: string
  displayedContent: string
  agentId?: number
}> (
  isGroupChat: boolean,
  text: string,
  agentsList: Agent[],
  localTurns: Ref<T[]>
): AssistantStreamSetup {
  const mentions = isGroupChat ? extractMentions(text, agentsList) : []
  const pendingAgentIds = isGroupChat && mentions.length > 0 ? [...mentions] : []
  const assistantEntries = new Map<number, number>()
  const groupAssistantBlockStartIdx = isGroupChat ? localTurns.value.length : -1

  let assistantIdx: number
  if (isGroupChat && pendingAgentIds.length > 0) {
    for (const aid of pendingAgentIds) {
      localTurns.value.push({
        role: 'assistant',
        content: '',
        displayedContent: '',
        agentId: aid
      } as T)
      assistantEntries.set(aid, localTurns.value.length - 1)
    }
    assistantIdx = localTurns.value.length - 1
  } else if (!isGroupChat) {
    assistantIdx = localTurns.value.length
    localTurns.value.push({
      role: 'assistant',
      content: '',
      displayedContent: ''
    } as T)
  } else {
    assistantIdx = -1
  }

  const ensureAssistantRow = (aid: number): number => {
    let idx = assistantEntries.get(aid)
    if (idx !== undefined) return idx
    localTurns.value.push({
      role: 'assistant',
      content: '',
      displayedContent: '',
      agentId: aid
    } as T)
    idx = localTurns.value.length - 1
    assistantEntries.set(aid, idx)
    return idx
  }

  return {
    assistantIdx,
    assistantEntries,
    groupAssistantBlockStartIdx,
    pendingAgentIds,
    ensureAssistantRow
  }
}
