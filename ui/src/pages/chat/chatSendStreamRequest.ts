import type { Agent, ChatGroup } from 'src/api/types'
import { extractMentions } from './chatMentions'

/**
 * 流式接口路径（单聊 `/chat/stream`，群聊 `/chat/groups/stream`）。
 */
export function buildChatStreamUrl (baseURL: string, isGroupChat: boolean): string {
  const b = baseURL.replace(/\/$/, '')
  return isGroupChat ? `${b}/chat/groups/stream` : `${b}/chat/stream`
}

export type ChatStreamHttpRequestArgs = {
  text: string
  sessionId: string | null
  isGroupChat: boolean
  currentGroup: ChatGroup | null
  agentId: number | null
  imageUrlsForHistory?: string[]
  fileUrls?: string[]
  agents: Agent[]
  token: string | null
}

/**
 * POST 体与头：与原先 sendStream 内联拼装一致（含群聊 group_id / mentions、单聊 agent_id、附件 URL）。
 */
export function buildChatStreamHttpRequest (args: ChatStreamHttpRequestArgs): {
  body: Record<string, unknown>
  headers: Record<string, string>
} {
  const body: Record<string, unknown> = {
    message: args.text,
    session_id: args.sessionId ?? undefined,
    client_type: 'web'
  }

  if (args.isGroupChat && args.currentGroup) {
    body.group_id = args.currentGroup.id
    const m = extractMentions(args.text, args.agents)
    if (m.length > 0) {
      body.mentions = m
    }
  } else if (args.agentId != null && args.agentId > 0) {
    body.agent_id = args.agentId
  }

  if (args.imageUrlsForHistory != null && args.imageUrlsForHistory.length > 0) {
    body.image_urls = args.imageUrlsForHistory
  }
  if (args.fileUrls != null && args.fileUrls.length > 0) {
    body.file_urls = args.fileUrls
  }

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(args.token ? { Authorization: `Bearer ${args.token}` } : {})
  }

  return { body, headers }
}
