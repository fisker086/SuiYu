/**
 * 工具风险展示：优先 SSE 的 `risk_level`，否则用 `GET /skills` 构建的查找表（与库内/服务端 enrich 一致）。
 * 无数据时回退 `medium`，避免再维护一份与后端重复的硬编码表。
 */

/**
 * 由 `GET /skills` 返回列表构建 `toolName -> risk`（含 `builtin_skill.*` 与 `builtin_*` 别名）。
 */
export function buildSkillRiskLookup (
  skills: Array<{ key: string; risk_level?: string }>
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const s of skills) {
    const key = (s.key || '').trim()
    if (!key) continue
    let r = (s.risk_level || '').trim().toLowerCase()
    if (!r) r = 'medium'
    out[key] = r
    if (key.startsWith('builtin_skill.')) {
      const suffix = key.slice('builtin_skill.'.length)
      const clientTool = `builtin_${suffix}`
      if (out[clientTool] === undefined) out[clientTool] = r
    }
  }
  return out
}

/**
 * @param lookup — 来自 `buildSkillRiskLookup`；未加载成功时可传 `undefined`，仅回退 `medium`
 */
export function resolveClientToolRiskLevel (
  toolName: string,
  payloadRisk: unknown,
  lookup?: Record<string, string> | null
): string {
  if (typeof payloadRisk === 'string' && payloadRisk.trim() !== '') {
    return payloadRisk.trim().toLowerCase()
  }
  const t = (toolName || '').trim()
  if (t === '') return 'medium'
  if (lookup && lookup[t]) return lookup[t]
  if (lookup && t.startsWith('builtin_') && !t.startsWith('builtin_skill.')) {
    const sk = `builtin_skill.${t.slice('builtin_'.length)}`
    if (lookup[sk]) return lookup[sk]
  }
  return 'medium'
}

export function riskLevelLabelZh (level: string): string {
  const r = (level || 'medium').toLowerCase()
  const m: Record<string, string> = {
    low: '低',
    medium: '中',
    high: '高',
    critical: '严重'
  }
  return m[r] || r
}
