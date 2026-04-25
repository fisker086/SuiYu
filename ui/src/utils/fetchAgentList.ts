import { api } from 'boot/axios'
import type { Agent } from 'src/api/types'

function parseAgentList (body: unknown): Agent[] {
  if (body == null || typeof body !== 'object') return []
  const d = body as { data?: unknown }
  return Array.isArray(d.data) ? (d.data as Agent[]) : []
}

/** 供权限表单等使用：优先 /agents/all（全量），失败则 /agents（受 RBAC 过滤） */
export async function fetchAgentList (): Promise<Agent[]> {
  try {
    const res = await api.get('/agents/all')
    let list = parseAgentList(res.data)
    if (list.length === 0) {
      const res2 = await api.get('/agents')
      list = parseAgentList(res2.data)
    }
    return list
  } catch {
    try {
      const res = await api.get('/agents')
      return parseAgentList(res.data)
    } catch {
      return []
    }
  }
}
