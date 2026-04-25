import type { Agent } from 'src/api/types'
import type { ThoughtStepVM } from './chatReactStepsSync'

/** sendStream 内原 parseStreamEvents 所操作的本地气泡行（字段为实际读写子集） */
export type ChatParseStreamLocalTurn = {
  role: string
  content: string
  displayedContent: string
  createdAt?: string
  agentId?: number
}

/**
 * 从 accumulated SSE 文本解析并更新侧栏/群聊气泡；返回主对话区应展示的纯文本缓冲。
 * 逻辑与 useChatPage.sendStream 内联实现一致，仅通过 ctx 注入依赖。
 */
export type ParseStreamEventsCtx = {
  isGroupChat: boolean
  groupAssistantBlockStartIdx: number
  pendingAgentIds: number[]
  agents: { value: Agent[] }
  localTurns: { value: ChatParseStreamLocalTurn[] }
  assistantEntries: Map<number, number>
  processStreamEvent: (payload: Record<string, unknown>) => void
  processReActEvent: (payload: Record<string, unknown>) => void
  thoughtSteps: { value: ThoughtStepVM[] }
  currentStreamModelName: { value: string | null }
  appendThoughtText: (content: string, source?: string) => void
  scrollChatToBottom: () => void
  ensureAssistantRow: (aid: number) => number
  /** 与 processStreamEvent(tool_result) 同步：从主气泡去掉与工具返回完全重复的头部（模型常整段粘贴） */
  lastServerToolResultText?: { value: string }
}

export function createParseStreamEvents (ctx: ParseStreamEventsCtx): (responseText: string) => string {
  const {
    isGroupChat,
    groupAssistantBlockStartIdx,
    pendingAgentIds,
    agents,
    localTurns,
    assistantEntries,
    processStreamEvent,
    processReActEvent,
    thoughtSteps,
    currentStreamModelName,
    appendThoughtText,
    scrollChatToBottom,
    ensureAssistantRow,
    lastServerToolResultText
  } = ctx

  return (responseText: string): string => {
    let contentBuffer = ''
    let inThinkBlock = false
    let fallbackFinalAnswer = ''
    /** 群聊：每次 onprogress 会传入完整 accumulated，若对 row 做 += 会重复；改为扫全量后一次性写入 Map 再赋给气泡 */
    const groupAcc = isGroupChat ? new Map<number, string>() : null
    /** 按 SSE 流中首次出现的顺序排列助手气泡（Map 插入顺序在无 @ 预建气泡时可能与时间线不一致） */
    const groupStreamOrder: number[] = []
    const groupStreamOrderSeen = new Set<number>()
    const bumpGroupAcc = (aid: number, delta: string): void => {
      if (!groupAcc) return
      groupAcc.set(aid, (groupAcc.get(aid) ?? '') + delta)
      if (!groupStreamOrderSeen.has(aid)) {
        groupStreamOrderSeen.add(aid)
        groupStreamOrder.push(aid)
      }
    }

    /** 单次 chunk 内可含多行 `data:`；`\r\n` 结尾的 JSON 需去掉 `\r` 再 parse */
    const lines = responseText.split('\n')
    for (const line of lines) {
      if (!line.startsWith('data:')) continue
      const payloadText = line.slice(5).trimStart().replace(/\r$/, '')
      if (!payloadText || payloadText === '[DONE]') continue

      const tryJson = (): boolean => {
        const t = payloadText.trimStart()
        if (t[0] !== '{' && t[0] !== '[') return false
        try {
          const payload = JSON.parse(payloadText) as Record<string, unknown>
          // 群内 A2A：发送方气泡内展示「已向谁发送」
          if (payload.type === 'group_peer_outbound') {
            const aid = payload.agent_id as number | undefined
            if (aid != null && Number.isFinite(Number(aid))) {
              const toId = payload.to_agent_id as number | undefined
              const toRaw = payload.to_agent_name as string | undefined
              const toName =
                (toRaw && String(toRaw).trim()) ||
                (toId != null ? agents.value.find((a) => a.id === toId)?.name : undefined) ||
                (toId != null ? `Agent ${toId}` : '同伴')
              const msg = String(payload.message ?? '')
              const lineOut = `\n\n📤 已向 ${toName} 发送：${msg}`
              bumpGroupAcc(Number(aid), lineOut)
            }
            return true
          }

          const eventType = payload.event_type as string | undefined
          if (eventType) {
            processStreamEvent(payload)
          }

          const reactType = payload.type as string | undefined
          if (reactType && !eventType) {
            processReActEvent(payload)
            if (reactType === 'final_answer') {
              const fc = payload.content as string | undefined
              if (fc != null && fc !== '') {
                fallbackFinalAnswer = fc
              }
            }
          }

          const content = payload.content as string | undefined
          const thinkContent = payload.think as string | undefined
          const modelName = payload.model_name as string | undefined
          const agentId = payload.agent_id as number | undefined

          // Group chat: 累加到 groupAcc，函数末尾一次性写入气泡（避免多次 applyParsed 重复追加）
          if (agentId !== undefined) {
            if (groupAcc && content && !eventType && !reactType) {
              const aid = Number(agentId)
              bumpGroupAcc(aid, content)
            } else if (!groupAcc) {
              const idx = ensureAssistantRow(Number(agentId))
              const row = localTurns.value[idx]
              if (row && content && !eventType && !reactType) {
                row.content = (row.content || '') + content
                row.displayedContent = row.content
                if (!row.createdAt && row.content.trim() !== '') {
                  row.createdAt = new Date().toISOString()
                }
              }
              scrollChatToBottom()
            }
            return true
          }

          if (modelName) {
            currentStreamModelName.value = modelName
          }

          if (thinkContent) {
            if (inThinkBlock) {
              const lastStep = thoughtSteps.value[thoughtSteps.value.length - 1]
              if (lastStep && lastStep.type === 'thought') {
                lastStep.data.content = `${lastStep.data.content || ''}${thinkContent}`
              }
            } else {
              appendThoughtText(thinkContent, 'think_stream')
            }
            inThinkBlock = false
          }

          // Web 端不支持某客户端工具时，服务端发 event_type=info（ADK）或 type=info（ReAct）。
          // 原先仅写入侧栏 thoughtSteps，主气泡因 !eventType && !reactType 条件被跳过而一直空白。
          if (
            content != null &&
            typeof content === 'string' &&
            content !== '' &&
            (eventType === 'info' || reactType === 'info')
          ) {
            contentBuffer += content
          }

          // Plain {"content":"..."} token chunks (e.g. streamStaticTextAsSSE): only fill the assistant
          // bubble. Do not mirror into thoughtSteps — ReAct already emits thought/action/observation/
          // reflection/final_answer; duplicating here looked like "思考过程" in the sidebar and doubled text.
          if (
            content != null &&
            typeof content === 'string' &&
            content !== '' &&
            !eventType &&
            !reactType
          ) {
            contentBuffer += content
          }
          return true
        } catch {
          return false
        }
      }

      if (!tryJson()) {
        const head = payloadText.trimStart()[0]
        if (head === '{' || head === '[') {
          continue
        }
        contentBuffer += payloadText
      }
    }
    const stripDuplicateToolEcho = (buf: string): string => {
      const strip = lastServerToolResultText?.value
      if (!strip || strip.length === 0 || buf.length === 0) return buf
      if (buf.startsWith(strip)) {
        return buf.slice(strip.length).replace(/^[\s\n\r]+/, '')
      }
      return buf
    }

    if (groupAcc && groupAcc.size > 0 && groupAssistantBlockStartIdx >= 0) {
      const streamSeen = new Set(groupStreamOrder)
      const prependPending: number[] = []
      for (const aid of pendingAgentIds) {
        if (!streamSeen.has(aid)) prependPending.push(aid)
      }
      const merged: number[] = [...prependPending, ...groupStreamOrder]
      const mergedSeen = new Set(merged)
      for (const aid of groupAcc.keys()) {
        if (!mergedSeen.has(aid)) {
          merged.push(aid)
          mergedSeen.add(aid)
        }
      }
      const prevCreatedAt = new Map<number, string | undefined>()
      for (let i = groupAssistantBlockStartIdx; i < localTurns.value.length; i++) {
        const r = localTurns.value[i]
        if (r.role === 'assistant' && r.agentId != null) {
          prevCreatedAt.set(r.agentId, r.createdAt)
        }
      }
      const newRows = merged.map(aid => {
        const raw = (groupAcc.get(aid) ?? '').replace(/^\n+/, '')
        const row: ChatParseStreamLocalTurn = {
          role: 'assistant',
          content: raw,
          displayedContent: raw,
          agentId: aid
        }
        if (raw.trim() !== '') {
          row.createdAt = prevCreatedAt.get(aid) ?? new Date().toISOString()
        }
        return row
      })
      localTurns.value.splice(
        groupAssistantBlockStartIdx,
        localTurns.value.length - groupAssistantBlockStartIdx,
        ...newRows
      )
      assistantEntries.clear()
      merged.forEach((aid, i) => assistantEntries.set(aid, groupAssistantBlockStartIdx + i))
      scrollChatToBottom()
      return ''
    }
    if (contentBuffer !== '') return stripDuplicateToolEcho(contentBuffer)
    // ReAct 先发 {"type":"final_answer","content":"全文"}，再发 {"content":"分片"}。onprogress 时 isFinal 为 false，
    // 若尚未收到分片，必须用 final_answer.content 作为正文，否则主界面会一直空白。
    if (fallbackFinalAnswer !== '') return stripDuplicateToolEcho(fallbackFinalAnswer)
    // Do not use reflection as the main bubble body: reflection is internal reasoning (侧栏即可).
    // Scheduled runs store react_steps in extra; 正文仍应是 final_answer / 分片，而非反思文本。
    return ''
  }
}
