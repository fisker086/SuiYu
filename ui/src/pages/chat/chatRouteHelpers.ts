import type { Agent } from 'src/api/types'

export const LAST_AGENT_PUBLIC_ID_KEY = 'lastAgentPublicId'

export function normAgentKey (s: string): string {
  return s.trim().toLowerCase()
}

/** Resolve agent from route param: prefer `public_id`, accept legacy numeric `id` and caller may replace URL. */
export function findAgentFromRouteParam (raw: string, list: Agent[]): Agent | null {
  const t = (raw || '').trim()
  if (!t || list.length === 0) return null
  const byPub = list.find(a => (a.public_id || '').trim() !== '' && normAgentKey(a.public_id) === normAgentKey(t))
  if (byPub) return byPub
  if (/^\d+$/.test(t)) {
    const n = parseInt(t, 10)
    if (!Number.isNaN(n) && n >= 1) {
      const byId = list.find(a => a.id === n)
      if (byId) return byId
    }
  }
  return null
}

export function startOfLocalDay (d: Date): number {
  const x = new Date(d)
  x.setHours(0, 0, 0, 0)
  return x.getTime()
}

export function bucketSessionByDay (updatedAtISO: string): 'today' | 'yesterday' | 'earlier' {
  const t = new Date(updatedAtISO).getTime()
  const now = new Date()
  const todayStart = startOfLocalDay(now)
  const yest = new Date(now)
  yest.setDate(yest.getDate() - 1)
  const yesterdayStart = startOfLocalDay(yest)
  if (t >= todayStart) return 'today'
  if (t >= yesterdayStart) return 'yesterday'
  return 'earlier'
}
