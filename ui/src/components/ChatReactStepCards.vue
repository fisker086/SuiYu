<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChatReactStep } from 'src/api/types'
import type { UrlChartPayload } from 'src/utils/urlResponseChart'
import PromQueryChart, { type PromSeries } from './chat/PromQueryChart.vue'
import UrlJsonChart from './chat/UrlJsonChart.vue'

export interface StepWithParsed {
  step: ChatReactStep
  idx: number
  parsed: {
    prom: PromSeries[] | null
    grafanaList: unknown[] | null
    grafanaQueries: unknown[] | null
    urlChart: UrlChartPayload | null
    rawText: string
  }
}

defineProps<{
  item: StepWithParsed
  rawVisible: boolean
}>()

const emit = defineEmits<{
  toggleRaw: []
}>()

const { t } = useI18n()

function stepTitle (step: ChatReactStep): string {
  if (typeof step.data?.label === 'string' && step.data.label) return step.data.label
  switch (step.type) {
    case 'thought':
      return '思考'
    case 'action':
      return step.data?.kind === 'hitl' ? '人工确认' : `工具: ${String(step.data?.name ?? step.data?.tool ?? 'tool')}`
    case 'observation':
      return step.data?.name ? `观测: ${String(step.data.name)}` : '观测结果'
    case 'reflection':
      return '反思'
    case 'error':
      return '错误'
    default:
      return step.type
  }
}

function thoughtBody (step: ChatReactStep): string {
  const c = step.data?.content ?? step.data?.thought
  return typeof c === 'string' ? c : ''
}

function grafanaDashRows (list: unknown[] | null): { title?: string; uid?: string; tags?: string[] }[] {
  if (!list?.length) return []
  return list as { title?: string; uid?: string; tags?: string[] }[]
}

function grafanaQueryRows (list: unknown[] | null): { panel_title?: string; promql?: string }[] {
  if (!list?.length) return []
  return list as { panel_title?: string; promql?: string }[]
}
</script>

<template>
  <div class="chat-react-step-cards">
    <!-- 思考 -->
    <q-card
      v-if="item.step.type === 'thought'"
      flat
      bordered
      class="chat-react-card chat-react-card--thought"
    >
      <q-card-section class="q-py-sm q-px-md">
        <div class="row items-center no-wrap q-mb-xs">
          <q-icon name="psychology" size="xs" color="deep-purple" class="q-mr-xs" />
          <span class="text-caption text-weight-bold text-deep-purple">{{ stepTitle(item.step) }}</span>
        </div>
        <div class="text-body2 text-text2 chat-react-pre-wrap">{{ thoughtBody(item.step) }}</div>
      </q-card-section>
    </q-card>

    <!-- 工具调用 -->
    <q-card
      v-else-if="item.step.type === 'action'"
      flat
      bordered
      class="chat-react-card chat-react-card--action"
    >
      <q-card-section class="q-py-sm q-px-md">
        <div class="row items-center no-wrap q-mb-xs">
          <q-icon name="memory" size="xs" color="primary" class="q-mr-xs" />
          <span class="text-caption text-weight-bold text-primary">{{ stepTitle(item.step) }}</span>
        </div>
        <pre
          v-if="item.step.data?.input != null && typeof item.step.data.input === 'object'"
          class="chat-react-json text-caption q-ma-none"
        >{{ JSON.stringify(item.step.data.input, null, 2) }}</pre>
        <div v-else-if="typeof item.step.data?.content === 'string' && item.step.data.content" class="text-caption text-text3">
          {{ item.step.data.content }}
        </div>
      </q-card-section>
    </q-card>

    <!-- 观测 -->
    <q-card
      v-else-if="item.step.type === 'observation'"
      flat
      bordered
      class="chat-react-card chat-react-card--obs"
    >
      <q-card-section class="q-py-sm q-px-md">
        <div class="row items-center justify-between no-wrap q-mb-sm">
          <div class="row items-center no-wrap">
            <q-icon name="show_chart" size="xs" color="secondary" class="q-mr-xs" />
            <span class="text-caption text-weight-bold text-secondary">{{ stepTitle(item.step) }}</span>
          </div>
          <q-btn
            flat
            dense
            no-caps
            size="sm"
            color="secondary"
            :label="rawVisible ? t('chatReactHideRaw') : t('chatReactShowRaw')"
            @click="emit('toggleRaw')"
          />
        </div>

        <template v-if="item.parsed.prom?.length">
          <div class="chat-react-chart-box bg-white rounded-borders q-pa-sm">
            <PromQueryChart :series="item.parsed.prom" />
          </div>
          <pre
            v-if="rawVisible"
            class="chat-react-json text-caption q-mt-sm q-ma-none"
          >{{ item.parsed.rawText || JSON.stringify(item.step.data?.content, null, 2) }}</pre>
        </template>

        <template v-else-if="item.parsed.urlChart">
          <div class="text-caption text-text3 q-mb-xs">{{ t('chatReactUrlResponseChartHint') }}</div>
          <div class="chat-react-chart-box bg-white rounded-borders q-pa-sm">
            <UrlJsonChart :payload="item.parsed.urlChart" />
          </div>
          <pre
            v-if="rawVisible"
            class="chat-react-json text-caption q-mt-sm q-ma-none"
          >{{ item.parsed.rawText }}</pre>
        </template>

        <template v-else-if="item.parsed.grafanaList?.length">
          <q-markup-table flat dense bordered separator="cell" class="chat-react-table">
            <thead>
              <tr>
                <th class="text-left">面板名称</th>
                <th class="text-left">UID</th>
                <th class="text-left">标签</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, ri) in grafanaDashRows(item.parsed.grafanaList)"
                :key="ri"
              >
                <td>{{ row.title }}</td>
                <td class="text-caption text-grey-7">{{ row.uid }}</td>
                <td>
                  <q-badge
                    v-for="tg in row.tags || []"
                    :key="tg"
                    outline
                    color="grey-7"
                    class="q-mr-xs"
                  >
                    {{ tg }}
                  </q-badge>
                </td>
              </tr>
            </tbody>
          </q-markup-table>
          <pre
            v-if="rawVisible"
            class="chat-react-json text-caption q-mt-sm q-ma-none"
          >{{ item.parsed.rawText }}</pre>
        </template>

        <template v-else-if="item.parsed.grafanaQueries?.length">
          <q-markup-table flat dense bordered separator="cell" class="chat-react-table">
            <thead>
              <tr>
                <th class="text-left" style="width: 28%">Panel</th>
                <th class="text-left">PromQL</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, ri) in grafanaQueryRows(item.parsed.grafanaQueries)"
                :key="ri"
              >
                <td class="text-body2" style="vertical-align: top">{{ row.panel_title }}</td>
                <td>
                  <code class="chat-react-code">{{ row.promql }}</code>
                </td>
              </tr>
            </tbody>
          </q-markup-table>
          <pre
            v-if="rawVisible"
            class="chat-react-json text-caption q-mt-sm q-ma-none"
          >{{ item.parsed.rawText }}</pre>
        </template>

        <template v-else>
          <pre
            v-if="rawVisible"
            class="chat-react-json text-caption q-ma-none"
          >{{ item.parsed.rawText }}</pre>
          <div
            v-else
            class="text-caption text-grey-7"
          >
            {{ t('chatReactRawCollapsedHint') }}
          </div>
        </template>
      </q-card-section>
    </q-card>

    <!-- 反思 -->
    <q-card
      v-else-if="item.step.type === 'reflection'"
      flat
      bordered
      class="chat-react-card chat-react-card--reflect"
    >
      <q-card-section class="q-py-sm q-px-md">
        <div class="row items-center no-wrap q-mb-xs">
          <q-icon name="auto_fix_high" size="xs" color="orange-9" class="q-mr-xs" />
          <span class="text-caption text-weight-bold text-orange-9">{{ stepTitle(item.step) }}</span>
        </div>
        <div class="text-body2 text-text2 chat-react-pre-wrap">{{ thoughtBody(item.step) }}</div>
      </q-card-section>
    </q-card>

    <!-- 错误 -->
    <q-card
      v-else-if="item.step.type === 'error'"
      flat
      bordered
      class="chat-react-card chat-react-card--error"
    >
      <q-card-section class="q-py-sm q-px-md">
        <div class="row items-center no-wrap q-mb-xs">
          <q-icon name="error" size="xs" color="negative" class="q-mr-xs" />
          <span class="text-caption text-weight-bold text-negative">{{ stepTitle(item.step) }}</span>
        </div>
        <div class="text-body2 text-negative chat-react-pre-wrap">{{ thoughtBody(item.step) }}</div>
      </q-card-section>
    </q-card>
  </div>
</template>

<style scoped>
.chat-react-pre-wrap {
  white-space: pre-wrap;
  word-break: break-word;
}
.chat-react-card {
  border-radius: 12px;
  max-width: 100%;
}
.chat-react-card--thought {
  background: rgba(103, 58, 183, 0.06);
  border-color: rgba(103, 58, 183, 0.2);
}
.chat-react-card--action {
  background: rgba(25, 118, 210, 0.05);
  border-color: rgba(25, 118, 210, 0.2);
}
.chat-react-card--obs {
  background: rgba(0, 137, 123, 0.06);
  border-color: rgba(0, 137, 123, 0.2);
}
.chat-react-card--reflect {
  background: rgba(230, 81, 0, 0.06);
  border-color: rgba(230, 81, 0, 0.2);
}
.chat-react-card--error {
  background: rgba(198, 40, 40, 0.06);
  border-color: rgba(198, 40, 40, 0.2);
}
.chat-react-json {
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, monospace;
  background: rgba(0, 0, 0, 0.04);
  padding: 8px;
  border-radius: 8px;
}
.chat-react-code {
  display: block;
  font-size: 11px;
  padding: 8px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
}
.chat-react-chart-box {
  border: 1px solid rgba(0, 0, 0, 0.08);
}
.chat-react-table {
  font-size: 12px;
}
</style>
