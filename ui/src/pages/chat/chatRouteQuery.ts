/** 旧书签：?session= 会重定向到路径；与单聊一致为 `/chat/<uuid>` */
export const ROUTE_SESSION_Q = 'session'
/** 旧书签兼容：`?group=<id>` */
export const ROUTE_GROUP_Q = 'group'

export function normalizeRouteQuery (q: string | string[] | undefined | null): string {
  if (q == null) return ''
  const s = Array.isArray(q) ? q[0] : q
  return String(s ?? '').trim()
}

/** 路径段是否为标准 UUID（与智能体 public_id 同形；路由解析时先匹配智能体再 GET 会话） */
export function isLikelySessionUUID (s: string): boolean {
  return /^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$/i.test(s.trim())
}
