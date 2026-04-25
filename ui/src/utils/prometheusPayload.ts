import type { PromSeries } from 'src/components/chat/PromQueryChart.vue'

/**
 * Prometheus HTTP API 响应转为 PromSeries[]，供 PromQueryChart 使用。
 * - query / query_range: data.resultType + data.result（vector | matrix | scalar | string）
 * - 若缺少 resultType：根据 result[0] 是否含 value / values 推断 vector 或 matrix
 * - Alertmanager 风格: data.alerts[]（labels / annotations / state / activeAt / value）
 */
const prometheusChartMaxSeries = 200

function isVectorSampleRow (row: unknown): boolean {
  if (!row || typeof row !== 'object' || Array.isArray(row)) return false
  const r = row as Record<string, unknown>
  const val = r.value
  return Array.isArray(val) && val.length >= 2
}

function isMatrixSampleRow (row: unknown): boolean {
  if (!row || typeof row !== 'object' || Array.isArray(row)) return false
  const r = row as Record<string, unknown>
  const values = r.values
  return Array.isArray(values) && values.length > 0
}

function metricRecordToStringMap (m: Record<string, unknown>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(m)) {
    if (v != null) out[k] = String(v)
  }
  return out
}

function prometheusAlertsToPromSeries (alerts: unknown[]): PromSeries[] {
  const out: PromSeries[] = []
  for (let i = 0; i < alerts.length && out.length < prometheusChartMaxSeries; i++) {
    const raw = alerts[i]
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) continue
    const a = raw as Record<string, unknown>
    const metric: Record<string, string> = {}
    const labels = a.labels
    if (labels && typeof labels === 'object' && !Array.isArray(labels)) {
      Object.assign(metric, metricRecordToStringMap(labels as Record<string, unknown>))
    }
    if (Object.keys(metric).length === 0) {
      metric._index = String(i)
    }
    const valRaw = a.value
    let num: number
    if (typeof valRaw === 'number' && !Number.isNaN(valRaw)) {
      num = valRaw
    } else if (typeof valRaw === 'string' && valRaw.trim() !== '') {
      num = parseFloat(valRaw.trim())
    } else {
      continue
    }
    if (Number.isNaN(num)) continue
    let ts = Date.now() / 1000
    if (typeof a.activeAt === 'string' && a.activeAt.trim() !== '') {
      const ms = Date.parse(a.activeAt)
      if (!Number.isNaN(ms)) ts = ms / 1000
    }
    out.push({ metric, values: [[ts, String(num)]] })
  }
  return out
}

export function prometheusQueryAPIResultToSeries (parsed: unknown): PromSeries[] | null {
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
  const root = parsed as Record<string, unknown>
  const data = root.data
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const d = data as Record<string, unknown>

  if (Array.isArray(d.alerts) && d.alerts.length > 0) {
    const series = prometheusAlertsToPromSeries(d.alerts)
    return series.length > 0 ? series : null
  }

  const rtRaw = d.resultType
  const rt = typeof rtRaw === 'string' ? rtRaw.toLowerCase() : ''
  const result = d.result

  const out: PromSeries[] = []

  // 即时查询 scalar/string：data.result 为 [ts, "value"]，不是 series 数组
  if ((rt === 'scalar' || rt === 'string') && Array.isArray(result) && result.length >= 2) {
    const ts = Number(result[0])
    const s = String(result[1])
    if (!Number.isNaN(ts)) {
      const name = rt === 'string' ? 'string' : 'scalar'
      out.push({ metric: { __name__: name }, values: [[ts, s]] })
    }
    return out.length > 0 ? out : null
  }

  if (!Array.isArray(result) || result.length === 0) return null

  let rtEff = rt
  if (rtEff !== 'vector' && rtEff !== 'matrix') {
    const first = result[0]
    if (isMatrixSampleRow(first)) rtEff = 'matrix'
    else if (isVectorSampleRow(first)) rtEff = 'vector'
    else return null
  }

  if (rtEff === 'vector') {
    for (const row of result) {
      if (out.length >= prometheusChartMaxSeries) break
      if (!row || typeof row !== 'object') continue
      const r = row as Record<string, unknown>
      const metricRaw = r.metric && typeof r.metric === 'object' && !Array.isArray(r.metric)
        ? (r.metric as Record<string, unknown>)
        : {}
      const metric = metricRecordToStringMap(metricRaw)
      const val = r.value
      if (Array.isArray(val) && val.length >= 2) {
        const ts = Number(val[0])
        const s = String(val[1])
        if (!Number.isNaN(ts)) {
          out.push({ metric, values: [[ts, s]] })
        }
      }
    }
  } else {
    for (const row of result) {
      if (out.length >= prometheusChartMaxSeries) break
      if (!row || typeof row !== 'object') continue
      const r = row as Record<string, unknown>
      const metricRaw = r.metric && typeof r.metric === 'object' && !Array.isArray(r.metric)
        ? (r.metric as Record<string, unknown>)
        : {}
      const metric = metricRecordToStringMap(metricRaw)
      const values = r.values
      if (!Array.isArray(values) || values.length === 0) continue
      const tuples: [number, string][] = []
      for (const pair of values) {
        if (Array.isArray(pair) && pair.length >= 2) {
          tuples.push([Number(pair[0]), String(pair[1])])
        }
      }
      if (tuples.length > 0) out.push({ metric, values: tuples })
    }
  }

  return out.length > 0 ? out : null
}
