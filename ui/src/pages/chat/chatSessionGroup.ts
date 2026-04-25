import type { ChatGroup, ChatSession } from 'src/api/types'
import { bucketSessionByDay } from './chatRouteHelpers'

export function groupSessionsByDay (sessions: ChatSession[]) {
  const today: ChatSession[] = []
  const yesterday: ChatSession[] = []
  const earlier: ChatSession[] = []
  for (const s of sessions) {
    const b = bucketSessionByDay(s.updated_at)
    if (b === 'today') today.push(s)
    else if (b === 'yesterday') yesterday.push(s)
    else earlier.push(s)
  }
  const sortDesc = (a: ChatSession, b: ChatSession) =>
    new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  today.sort(sortDesc)
  yesterday.sort(sortDesc)
  earlier.sort(sortDesc)
  return { today, yesterday, earlier }
}

export type GroupedByDay<T> = { today: T[]; yesterday: T[]; earlier: T[] }

export function sessionBlocksFromGrouped (
  g: GroupedByDay<ChatSession>,
  t: (key: string) => string
): { key: string; label: string; items: ChatSession[] }[] {
  if (g.today.length === 0 && g.yesterday.length === 0 && g.earlier.length > 0) {
    return [{ key: 'history', label: t('chatSessionHistoryFlat'), items: g.earlier }]
  }
  return [
    { key: 'today', label: t('chatSessionGroupToday'), items: g.today },
    { key: 'yesterday', label: t('chatSessionGroupYesterday'), items: g.yesterday },
    { key: 'earlier', label: t('chatSessionGroupEarlier'), items: g.earlier }
  ].filter(b => b.items.length > 0)
}

export function groupChatGroupsByDay (groups: ChatGroup[]): GroupedByDay<ChatGroup> {
  const withKey = groups.map(g => {
    const at =
      g.updated_at != null && String(g.updated_at).trim() !== ''
        ? g.updated_at
        : g.created_at
    return { g, at }
  })
  withKey.sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
  const today: ChatGroup[] = []
  const yesterday: ChatGroup[] = []
  const earlier: ChatGroup[] = []
  for (const { g, at } of withKey) {
    const b = bucketSessionByDay(at)
    if (b === 'today') today.push(g)
    else if (b === 'yesterday') yesterday.push(g)
    else earlier.push(g)
  }
  return { today, yesterday, earlier }
}

export function groupRailBlocksFromGrouped (
  g: GroupedByDay<ChatGroup>,
  t: (key: string) => string
): { key: string; label: string; items: ChatGroup[] }[] {
  if (g.today.length === 0 && g.yesterday.length === 0 && g.earlier.length > 0) {
    return [{ key: 'history', label: t('chatSessionHistoryFlat'), items: g.earlier }]
  }
  return [
    { key: 'today', label: t('chatSessionGroupToday'), items: g.today },
    { key: 'yesterday', label: t('chatSessionGroupYesterday'), items: g.yesterday },
    { key: 'earlier', label: t('chatSessionGroupEarlier'), items: g.earlier }
  ].filter(b => b.items.length > 0)
}
