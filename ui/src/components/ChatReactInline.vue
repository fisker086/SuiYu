<script setup lang="ts">
/**
 * layout=timeline 时由外层 q-timeline 包裹，用于时间线展示。
 */
import { computed, ref } from 'vue'
import type { ChatReactStep } from 'src/api/types'
import { filterVisibleReactSteps } from 'src/utils/reactStepsFilter'
import {
  extractHttpBodyFromObservationText,
  inferUrlChartFromJson,
  type UrlChartPayload,
  isPrometheusSeriesPayload
} from 'src/utils/urlResponseChart'
import { prometheusQueryAPIResultToSeries } from 'src/utils/prometheusPayload'
import type { PromSeries } from './chat/PromQueryChart.vue'
import ChatReactStepCards from './ChatReactStepCards.vue'

export type { ChatReactStep }

const props = withDefaults(
  defineProps<{
    steps: ChatReactStep[]
    /** default：卡片堆叠；timeline：仅输出 q-timeline-entry，须放在 q-timeline 内 */
    layout?: 'default' | 'timeline'
  }>(),
  { layout: 'default' }
)

const showRawByIdx = ref<Record<number, boolean>>({})

function toggleRaw (idx: number): void {
  showRawByIdx.value = { ...showRawByIdx.value, [idx]: !showRawByIdx.value[idx] }
}

function parseObservationStructured (step: ChatReactStep): {
  prom: PromSeries[] | null
  grafanaList: unknown[] | null
  grafanaQueries: unknown[] | null
  urlChart: UrlChartPayload | null
  rawText: string
} {
  const c = step.data?.content
  const empty = { prom: null, grafanaList: null, grafanaQueries: null, urlChart: null, rawText: '' }

  type ParsedObs = ReturnType<typeof parseObservationStructured>

  const tryJsonPayload = (parsed: unknown, rawForDisplay: string): ParsedObs | null => {
    if (parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const api = prometheusQueryAPIResultToSeries(parsed)
      if (api?.length) {
        return { ...empty, prom: api, rawText: rawForDisplay }
      }
    }
    if (parsed === null) return null
    if (Array.isArray(parsed) && parsed.length > 0) {
      const j0 = parsed[0] as Record<string, unknown>
      if (j0 && Array.isArray(j0.values) && j0.metric) {
        return { ...empty, prom: parsed as PromSeries[], rawText: rawForDisplay }
      }
      if (j0?.uid && j0?.title) {
        return { ...empty, grafanaList: parsed, rawText: rawForDisplay }
      }
      if (j0?.panel_title && j0?.promql) {
        return { ...empty, grafanaQueries: parsed, rawText: rawForDisplay }
      }
    }
    if (!isPrometheusSeriesPayload(parsed)) {
      const urlChart = inferUrlChartFromJson(parsed)
      if (urlChart) {
        return { ...empty, urlChart, rawText: rawForDisplay }
      }
    }
    return null
  }

  if (Array.isArray(c)) {
    if (c.length > 0 && c[0] && typeof c[0] === 'object' && Array.isArray((c[0] as PromSeries).values)) {
      return { ...empty, prom: c as PromSeries[], rawText: '' }
    }
    const row0 = c[0] as Record<string, unknown> | undefined
    if (row0?.uid && row0?.title) {
      return { ...empty, grafanaList: c, rawText: '' }
    }
    if (row0?.panel_title && row0?.promql) {
      return { ...empty, grafanaQueries: c, rawText: '' }
    }
  }
  if (typeof c !== 'string') {
    if (c != null && typeof c === 'object' && !Array.isArray(c)) {
      const hit = tryJsonPayload(c, JSON.stringify(c, null, 2))
      if (hit) return hit
      return { ...empty, rawText: JSON.stringify(c, null, 2) }
    }
    return { ...empty, rawText: c != null ? JSON.stringify(c, null, 2) : '' }
  }

  try {
    const j = JSON.parse(c) as unknown
    const hit = tryJsonPayload(j, c)
    if (hit) return hit
  } catch {
    /* fall through */
  }

  const body = extractHttpBodyFromObservationText(c)
  if (body !== c.trim()) {
    try {
      const j2 = JSON.parse(body) as unknown
      const hit2 = tryJsonPayload(j2, c)
      if (hit2) return hit2
    } catch {
      /* plain */
    }
  }

  return { ...empty, rawText: c }
}

const visibleSteps = computed(() => filterVisibleReactSteps(props.steps))

const stepsWithParsed = computed(() =>
  visibleSteps.value.map((step, idx) => ({
    step,
    idx,
    parsed: parseObservationStructured(step)
  }))
)

function stepIcon (step: ChatReactStep): string {
  switch (step.type) {
    case 'thought':
      return 'psychology'
    case 'action':
      return 'memory'
    case 'observation':
      return 'show_chart'
    case 'reflection':
      return 'auto_fix_high'
    case 'error':
      return 'error'
    default:
      return 'circle'
  }
}

function stepColor (step: ChatReactStep): string {
  switch (step.type) {
    case 'thought':
      return 'deep-purple'
    case 'action':
      return 'primary'
    case 'observation':
      return 'secondary'
    case 'reflection':
      return 'orange-9'
    case 'error':
      return 'negative'
    default:
      return 'grey-7'
  }
}

function timelineSubtitle (step: ChatReactStep): string | undefined {
  const ts = step.timestamp
  if (typeof ts !== 'string' || !ts.trim()) return undefined
  try {
    const d = new Date(ts)
    if (Number.isNaN(d.getTime())) return undefined
    return d.toLocaleString()
  } catch {
    return undefined
  }
}
</script>

<template>
  <div v-if="layout === 'default'" class="chat-react-inline column q-gutter-y-sm">
    <div
      v-for="item in stepsWithParsed"
      :key="item.idx + '-' + item.step.type + '-' + (item.step.timestamp || '')"
      class="chat-react-block"
    >
      <ChatReactStepCards
        :item="item"
        :raw-visible="!!showRawByIdx[item.idx]"
        @toggle-raw="() => toggleRaw(item.idx)"
      />
    </div>
  </div>
  <template v-else>
    <q-timeline-entry
      v-for="item in stepsWithParsed"
      :key="item.idx + '-' + item.step.type + '-' + (item.step.timestamp || '')"
      :icon="stepIcon(item.step)"
      :color="stepColor(item.step)"
      :subtitle="timelineSubtitle(item.step)"
    >
      <ChatReactStepCards
        :item="item"
        :raw-visible="!!showRawByIdx[item.idx]"
        @toggle-raw="() => toggleRaw(item.idx)"
      />
    </q-timeline-entry>
  </template>
</template>

<style scoped>
.chat-react-inline {
  max-width: 100%;
}
.chat-react-block {
  max-width: 100%;
}
</style>
