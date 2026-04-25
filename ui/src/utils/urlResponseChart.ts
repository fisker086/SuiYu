/**
 * 从 HTTP 工具等返回的观测文本里取出 Body 后的 JSON，并推断能否画折线图。
 * builtin_http_client 返回格式含 "Status: ...\nBody:\n{...}"。
 */

export interface UrlChartPayload {
  labels: string[]
  datasets: { label: string; data: number[] }[]
}

function isPromMatrix (value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return false
  const j0 = value[0] as Record<string, unknown> | undefined
  return !!(j0 && typeof j0 === 'object' && Array.isArray(j0.values) && j0.metric)
}

/** 去掉 HTTP 工具包装，只保留 Body 正文（多为 JSON） */
export function extractHttpBodyFromObservationText (text: string): string {
  const m = /\nBody:\s*\n?/i.exec(text)
  if (m && m.index >= 0) {
    return text.slice(m.index + m[0].length).trim()
  }
  const m2 = /^Body:\s*\n?/im.exec(text)
  if (m2 && m2.index === 0) {
    return text.slice(m2[0].length).trim()
  }
  return text.trim()
}

/** 是否为 Prometheus matrix（与 PromQueryChart 一致） */
export function isPrometheusSeriesPayload (value: unknown): boolean {
  return isPromMatrix(value)
}

/**
 * 从任意 JSON 推断折线图数据（纯数值序列、Chart.js 形、series 形等）。
 * 若像 Prometheus 矩阵则返回 null（交给 PromQueryChart）。
 */
export function inferUrlChartFromJson (value: unknown): UrlChartPayload | null {
  if (value === null || value === undefined) return null
  if (isPromMatrix(value)) return null

  if (Array.isArray(value)) {
    const nums = value.filter((x): x is number => typeof x === 'number' && !Number.isNaN(x))
    if (nums.length >= 2 && nums.length === value.length) {
      return {
        labels: nums.map((_, i) => String(i + 1)),
        datasets: [{ label: 'value', data: nums }]
      }
    }
    const extracted: number[] = []
    for (const row of value) {
      if (row && typeof row === 'object') {
        const o = row as Record<string, unknown>
        const v = o.value ?? o.y ?? o.v ?? o.count ?? o.amount
        if (typeof v === 'number' && !Number.isNaN(v)) extracted.push(v)
      }
    }
    if (extracted.length >= 2 && extracted.length === value.length) {
      return {
        labels: extracted.map((_, i) => String(i + 1)),
        datasets: [{ label: 'value', data: extracted }]
      }
    }
    return null
  }

  if (typeof value !== 'object') return null
  const o = value as Record<string, unknown>

  if (Array.isArray(o.labels) && Array.isArray(o.datasets)) {
    const rawDs = o.datasets as unknown[]
    const datasets: { label: string; data: number[] }[] = []
    for (let i = 0; i < rawDs.length; i++) {
      const dd = rawDs[i] as Record<string, unknown>
      if (!dd || typeof dd !== 'object') continue
      const data = Array.isArray(dd.data) ? dd.data.map(x => Number(x)) : []
      if (data.some(x => Number.isNaN(x))) continue
      if (data.length < 2) continue
      const label = typeof dd.label === 'string' ? dd.label : `series ${i + 1}`
      datasets.push({ label, data })
    }
    if (datasets.length === 0) return null
    const labels = (o.labels as unknown[]).map(x => String(x))
    const n = datasets[0].data.length
    const finalLabels = labels.length === n ? labels : Array.from({ length: n }, (_, i) => String(i + 1))
    return { labels: finalLabels, datasets }
  }

  if (Array.isArray(o.series)) {
    const datasets: { label: string; data: number[] }[] = []
    let maxLen = 0
    for (const s of o.series as unknown[]) {
      if (!s || typeof s !== 'object') continue
      const ss = s as Record<string, unknown>
      const name = String(ss.name ?? ss.label ?? 'series')
      const data = Array.isArray(ss.data) ? ss.data.map(x => Number(x)) : []
      if (data.length < 2 || data.some(x => Number.isNaN(x))) continue
      datasets.push({ label: name, data })
      maxLen = Math.max(maxLen, data.length)
    }
    if (datasets.length === 0) return null
    const labelsFromRoot = Array.isArray(o.labels) ? (o.labels as unknown[]).map(x => String(x)) : []
    const n = datasets[0].data.length
    const labels =
      labelsFromRoot.length === n ? labelsFromRoot : Array.from({ length: n }, (_, i) => String(i + 1))
    return { labels, datasets }
  }

  const arr = o.values ?? o.data
  if (Array.isArray(arr) && arr.length >= 2 && arr.every(x => typeof x === 'number' && !Number.isNaN(x))) {
    const nums = arr as number[]
    const lbl = Array.isArray(o.labels) ? (o.labels as unknown[]).map(x => String(x)) : []
    const labels = lbl.length === nums.length ? lbl : nums.map((_, i) => String(i + 1))
    const title = typeof o.name === 'string' ? o.name : typeof o.title === 'string' ? o.title : 'value'
    return { labels, datasets: [{ label: title, data: nums }] }
  }

  // 嵌套结构：{ result: [...] }、{ data: { points: [1,2,3] } } 等
  const deep = findFirstDenseNumberArray(value, 0)
  if (deep && deep.length >= 2) {
    return {
      labels: deep.map((_, i) => String(i + 1)),
      datasets: [{ label: 'value', data: deep }]
    }
  }

  return null
}

/** 在 JSON 子树中找第一个「全为数字且长度≥2」的数组（跳过 Prometheus 矩阵行） */
function findFirstDenseNumberArray (value: unknown, depth: number): number[] | null {
  if (depth > 6) return null
  if (Array.isArray(value)) {
    if (value.length === 0) return null
    const j0 = value[0]
    if (j0 && typeof j0 === 'object' && !Array.isArray(j0) && 'metric' in (j0 as object) && 'values' in (j0 as object)) {
      return null
    }
    const nums = value.filter((x): x is number => typeof x === 'number' && !Number.isNaN(x))
    if (nums.length >= 2 && nums.length === value.length) return nums
    for (const el of value) {
      const r = findFirstDenseNumberArray(el, depth + 1)
      if (r) return r
    }
  } else if (value && typeof value === 'object') {
    const o = value as Record<string, unknown>
    const preferred = ['data', 'values', 'series', 'items', 'results', 'points', 'metrics', 'result', 'payload', 'rows']
    for (const k of preferred) {
      if (k in o) {
        const r = findFirstDenseNumberArray(o[k], depth + 1)
        if (r) return r
      }
    }
    for (const v of Object.values(o)) {
      const r = findFirstDenseNumberArray(v, depth + 1)
      if (r) return r
    }
  }
  return null
}
