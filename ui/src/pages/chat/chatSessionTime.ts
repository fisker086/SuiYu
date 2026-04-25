import type { ChatGroup, ChatSession } from 'src/api/types'

export function formatSessionTime (iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

export function dayKeyLocal (iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`
}

/** 与上一条不在同一天时展示日期分隔（首条也会展示） */
export function chatDateDividerText (prevIso: string | undefined, curIso: string | undefined): string {
  if (!curIso) return ''
  const curK = dayKeyLocal(curIso)
  if (!curK) return ''
  if (prevIso) {
    const prevK = dayKeyLocal(prevIso)
    if (prevK === curK) return ''
  }
  try {
    const d = new Date(curIso)
    return d.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric', weekday: 'short' })
  } catch {
    return ''
  }
}

/**
 * 气泡上方：本地时间的时:分:秒（不含年月日；跨天由日期分隔条展示）。
 * `iso` 须为带时区的 RFC3339（后端 TIMESTAMPTZ 序列化）。
 */
export function formatChatMessageTime (iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function chatMessageTimeLabel (m: { createdAt?: string; created_at?: string }): string {
  return formatChatMessageTime(m.createdAt ?? m.created_at)
}

export function messageDateDividerAt (
  messages: { createdAt?: string; created_at?: string }[] | undefined,
  idx: number
): string {
  if (!messages || !messages[idx]) return ''
  const cur = messages[idx].createdAt ?? messages[idx].created_at
  const prev = idx > 0 ? messages[idx - 1].createdAt ?? messages[idx - 1].created_at : undefined
  return chatDateDividerText(prev, cur)
}

export function sessionTitle (s: ChatSession): string {
  const raw = (s.title || '').trim()
  if (raw) return raw
  const short = s.session_id.length > 10 ? `${s.session_id.slice(0, 8)}…` : s.session_id
  return short
}

export function groupCaptionTime (g: ChatGroup): string {
  const raw =
    g.updated_at != null && String(g.updated_at).trim() !== '' ? g.updated_at : g.created_at
  return formatSessionTime(raw)
}
