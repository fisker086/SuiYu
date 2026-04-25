import type { Ref } from 'vue'
import type { ChatGroup, ChatReactStep } from 'src/api/types'
import type { ThoughtStepVM } from './chatReactStepsSync'
import { cloneThoughtStepsSnapshot } from './chatReactStepsSync'

/** sendStream 中 localTurns 行在生命周期内读写的字段 */
export type ChatSendStreamLocalTurnRow = {
  role: string
  content: string
  displayedContent: string
  duration?: number
  createdAt?: string
  agentId?: number
  reactSteps?: ChatReactStep[]
}

export function isStreamFetchAbortError (e: unknown, signal: AbortSignal): boolean {
  if (signal.aborted) return true
  if (e instanceof DOMException && e.name === 'AbortError') return true
  if (e instanceof Error && e.name === 'AbortError') return true
  return false
}

export type CreateSendStreamLifecycleDeps = {
  isGroupChat: boolean
  assistantIdx: number
  pendingAgentIds: number[]
  assistantEntries: Map<number, number>
  localTurns: Ref<ChatSendStreamLocalTurnRow[]>
  thoughtStatus: Ref<'running' | 'completed'>
  thoughtSteps: Ref<ThoughtStepVM[]>
  sessionId: Ref<string | null>
  currentGroup: Ref<ChatGroup | null>
  lastTurnDurationMs: Ref<number | null>
  parseStreamEvents: (responseText: string) => string
  scrollChatToBottom: () => void
  t: (key: string) => string
  setStoredGroupSessionId: (groupId: number, sid: string) => void
  setStreamAbortController: (ac: AbortController | null) => void
}

export function createSendStreamLifecycle (deps: CreateSendStreamLifecycleDeps) {
  const {
    isGroupChat,
    assistantIdx,
    pendingAgentIds,
    assistantEntries,
    localTurns,
    thoughtStatus,
    thoughtSteps,
    sessionId,
    currentGroup,
    lastTurnDurationMs,
    parseStreamEvents,
    scrollChatToBottom,
    t,
    setStoredGroupSessionId,
    setStreamAbortController
  } = deps

  const accumulated = { value: '' }

  const stampAssistantTimeOnce = (): void => {
    if (isGroupChat) {
      for (const aid of assistantEntries.keys()) {
        const idx = assistantEntries.get(aid)
        if (idx === undefined) continue
        const row = localTurns.value[idx]
        if (row && !row.createdAt && (row.content || '').trim() !== '') {
          row.createdAt = new Date().toISOString()
        }
      }
      return
    }
    const row = localTurns.value[assistantIdx]
    if (!row || row.createdAt) return
    row.createdAt = new Date().toISOString()
  }

  /** 流式期间正文写入 content；界面用 displayedContent 由 rAF 逐字追赶，避免网络一次到齐时整块跳出。 */
  let typewriterRaf: number | null = null
  const stopTypewriter = (): void => {
    if (typewriterRaf != null) {
      cancelAnimationFrame(typewriterRaf)
      typewriterRaf = null
    }
  }
  const tickTypewriter = (): void => {
    typewriterRaf = null
    const row = localTurns.value[assistantIdx]
    if (!row || row.role !== 'assistant') return
    const target = row.content ?? ''
    const shown = row.displayedContent ?? ''
    if (shown.length >= target.length) return
    const behind = target.length - shown.length
    const streamOpen = thoughtStatus.value === 'running'
    let step: number
    if (streamOpen) {
      step = behind > 2400 ? 2 : 1
    } else {
      step =
        behind > 500 ? 16 : behind > 200 ? 8 : behind > 60 ? 4 : behind > 15 ? 2 : 1
    }
    row.displayedContent = target.slice(0, shown.length + step)
    scrollChatToBottom()
    typewriterRaf = requestAnimationFrame(tickTypewriter)
  }
  const scheduleTypewriter = (): void => {
    if (typewriterRaf != null) return
    typewriterRaf = requestAnimationFrame(tickTypewriter)
  }

  const applyParsed = (isFinal: boolean): void => {
    const parsedContent = parseStreamEvents(accumulated.value)

    if (isGroupChat) {
      stampAssistantTimeOnce()
      if (!isFinal) {
        scrollChatToBottom()
      }
      return
    }

    const row = localTurns.value[assistantIdx]
    if (!row) return
    row.content = parsedContent
    const shownLen = row.displayedContent?.length ?? 0
    if (isFinal) {
      if (shownLen >= parsedContent.length) {
        stopTypewriter()
        row.displayedContent = parsedContent
      } else {
        scheduleTypewriter()
      }
    } else {
      scheduleTypewriter()
    }
    row.reactSteps = cloneThoughtStepsSnapshot(thoughtSteps.value)
    if (parsedContent.trim() !== '') {
      stampAssistantTimeOnce()
    }
    if (!isFinal) {
      scrollChatToBottom()
    }
  }

  const finalizeStreamAbort = (): void => {
    thoughtStatus.value = 'completed'
    applyParsed(true)
    const suffix = '\n\n' + t('chatReplyStoppedSuffix')
    const stoppedLabel = t('chatReplyStoppedSuffix')
    if (isGroupChat) {
      for (const aid of assistantEntries.keys()) {
        const idx = assistantEntries.get(aid)
        if (idx === undefined) continue
        const row = localTurns.value[idx]
        if (!row) continue
        const cur = (row.content || '').trim()
        if (cur === '') {
          row.content = stoppedLabel
        } else {
          row.content = (row.content || '') + suffix
        }
        row.displayedContent = row.content
      }
    } else {
      const row = localTurns.value[assistantIdx]
      if (row) {
        const cur = (row.content || '').trim()
        if (cur === '') {
          row.content = stoppedLabel
        } else {
          row.content = (row.content || '') + suffix
        }
        row.displayedContent = row.content
      }
    }
    stampAssistantTimeOnce()
    scrollChatToBottom()
  }

  async function runStreamFetch (args: {
    url: string
    headers: Record<string, string>
    body: Record<string, unknown>
    ac: AbortController
  }): Promise<void> {
    const { url, headers, body, ac } = args
    setStreamAbortController(ac)

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: ac.signal
      })

      if (!response.ok) {
        thoughtStatus.value = 'completed'
        let msg = '请求失败'
        try {
          const j = (await response.json()) as { message?: string }
          if (j.message) msg = j.message
        } catch {
          /* body may not be JSON */
        }
        throw new Error(msg)
      }

      const sessionIdHeader = response.headers.get('X-Session-ID')
      if (sessionIdHeader) {
        sessionId.value = sessionIdHeader
        if (isGroupChat && currentGroup.value) {
          setStoredGroupSessionId(currentGroup.value.id, sessionIdHeader)
        }
      }

      const reader = response.body?.getReader()
      if (!reader) {
        accumulated.value = await response.text()
        thoughtStatus.value = 'completed'
        applyParsed(true)
      } else {
        const decoder = new TextDecoder()
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          accumulated.value += decoder.decode(value, { stream: true })
          applyParsed(false)
        }
        accumulated.value += decoder.decode()
        thoughtStatus.value = 'completed'
        applyParsed(true)
      }

      const durationHeader = response.headers.get('X-Duration-MS')
      if (durationHeader) {
        const duration = parseInt(durationHeader, 10)
        if (!Number.isNaN(duration)) {
          if (isGroupChat && pendingAgentIds.length > 0) {
            const lastAid = pendingAgentIds[pendingAgentIds.length - 1]
            const idx = assistantEntries.get(lastAid)
            const row = idx !== undefined ? localTurns.value[idx] : undefined
            if (row) {
              row.duration = duration
              lastTurnDurationMs.value = duration
            }
          } else if (assistantIdx >= 0) {
            const row = localTurns.value[assistantIdx]
            if (row) {
              row.duration = duration
              lastTurnDurationMs.value = duration
            }
          }
        }
      }
      stampAssistantTimeOnce()
    } catch (e: unknown) {
      thoughtStatus.value = 'completed'
      if (isStreamFetchAbortError(e, ac.signal)) {
        finalizeStreamAbort()
        return
      }
      throw e
    } finally {
      setStreamAbortController(null)
    }
  }

  return { runStreamFetch }
}
