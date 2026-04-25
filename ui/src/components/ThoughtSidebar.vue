<script setup lang="ts">
import { ref, computed, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { riskLevelLabelZh } from 'src/utils/toolRisk'

interface ThoughtStep {
  type: string
  data: Record<string, unknown>
  meta?: Record<string, unknown>
  timestamp?: string
}

const props = defineProps<{
  steps: ThoughtStep[]
  isOpen: boolean
  status: 'running' | 'completed'
  /** 最近一次流式完成耗时（毫秒），来自 X-Duration-MS；展示时换算为秒 */
  durationMs?: number | null
}>()

const { t } = useI18n()

/** 毫秒 → 秒文案：小于 100s 保留一位小数，整数去掉 .0 */
function formatDurationSeconds (ms: number): string {
  const sec = ms / 1000
  if (sec < 100) {
    const x = Math.round(sec * 10) / 10
    return Number.isInteger(x) ? String(x) : x.toFixed(1)
  }
  return String(Math.round(sec))
}

defineEmits<{
  toggle: []
}>()

/** QScrollArea 组件实例（template ref 不是 DOM，无 scrollTo） */
type QScrollAreaExposed = {
  setScrollPosition: (axis: 'vertical' | 'horizontal', offset: number, duration?: number) => void
  setScrollPercentage: (axis: 'vertical' | 'horizontal', offset: number, duration?: number) => void
}
const thoughtContainer = ref<QScrollAreaExposed | null>(null)
/** 使用 Set 存展开的下标；每次变更须赋新 Set，否则 Vue 无法追踪 .add/.delete */
const expandedSteps = ref<Set<number>>(new Set())

const thinkingSteps = computed(() => {
  return props.steps.filter(step =>
    ['thought', 'action', 'observation', 'reflection', 'final', 'error', 'plan'].includes(step.type)
  )
})

/** 无工具/计划步骤时仍可能有总耗时头，但不展示「耗时」——与空侧栏文案一致 */
const hasToolCalls = computed(() => props.steps.some(s => s.type === 'action' || s.type === 'plan'))

const toggleStep = (index: number) => {
  const next = new Set(expandedSteps.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  expandedSteps.value = next
}

const isStepExpanded = (index: number) => expandedSteps.value.has(index)

const expandAll = () => {
  expandedSteps.value = new Set(thinkingSteps.value.map((_, i) => i))
}

const collapseAll = () => {
  expandedSteps.value = new Set()
}

const scrollToTop = () => {
  nextTick(() => {
    thoughtContainer.value?.setScrollPosition('vertical', 0)
  })
}

let isSessionSwitching = false

watch(() => props.steps, (newSteps, oldSteps) => {
  if (newSteps !== oldSteps) {
    isSessionSwitching = true
    scrollToTop()
    collapseAll()
    nextTick(() => {
      isSessionSwitching = false
    })
  }
})

watch(() => thinkingSteps.value.length, (newLength, oldLength) => {
  if (!isSessionSwitching && newLength > oldLength) {
    const last = thinkingSteps.value[newLength - 1]
    if (last?.type === 'plan' && last.data?.kind === 'plan_execute') {
      const tasks = (last.data.tasks as { task: string; status: string }[]) || []
      if (tasks.length > 0 && tasks.length <= 8) {
        const next = new Set(expandedSteps.value)
        next.add(newLength - 1)
        expandedSteps.value = next
      }
    }
    nextTick(() => {
      thoughtContainer.value?.setScrollPercentage('vertical', 1, 250)
    })
  }
})

const getStepIcon = (type: string) => {
  switch (type) {
    case 'thought': return 'psychology'
    case 'action': return 'memory'
    case 'observation': return 'visibility'
    case 'reflection': return 'auto_fix_high'
    case 'final': return 'check_circle'
    case 'error': return 'error'
    case 'plan': return 'task_alt'
    default: return 'circle'
  }
}

const getStepColor = (type: string) => {
  switch (type) {
    case 'thought': return 'text-purple'
    case 'action': return 'text-blue'
    case 'observation': return 'text-green'
    case 'reflection': return 'text-orange'
    case 'final': return 'text-positive'
    case 'error': return 'text-negative'
    case 'plan': return 'text-indigo'
    default: return 'text-grey'
  }
}

const getObservationBody = (step: ThoughtStep): string => {
  const c = step.data?.content
  if (typeof c === 'string') return c
  if (c != null) return String(c)
  return ''
}

/** ADK 用 name，ReAct 用 tool；侧栏统一取其一 */
const toolOrName = (step: ThoughtStep): string =>
  String(step.data?.name ?? step.data?.tool ?? 'tool')

const hasActionInput = (step: ThoughtStep): boolean => {
  const inp = step.data?.input
  if (inp == null) return false
  if (typeof inp === 'object' && !Array.isArray(inp)) {
    return Object.keys(inp as object).length > 0
  }
  return true
}

const getStepTitle = (step: ThoughtStep) => {
  if (step.data?.label) return step.data.label as string
  if (step.type === 'thought' && step.data?.source === 'stream_tokens') {
    return '模型输出'
  }
  switch (step.type) {
    case 'thought': return '思考过程'
    case 'action':
      if (step.data?.kind === 'hitl') return '人工确认 (HITL)'
      if (step.data?.kind === 'server_tool_call' && typeof step.data?.risk_level === 'string') {
        return `执行工具: ${toolOrName(step)}（风险 ${riskLevelLabelZh(step.data.risk_level as string)}）`
      }
      return `执行工具: ${toolOrName(step)}`
    case 'observation':
      if (step.data?.name && typeof step.data.name === 'string') {
        return `工具返回: ${step.data.name}`
      }
      if (step.data?.tool && typeof step.data.tool === 'string') {
        return `工具返回: ${step.data.tool}`
      }
      return '观测结果'
    case 'reflection': return '自我反思'
    case 'final': return '最终回答'
    case 'error': return '错误'
    case 'plan':
      if (step.data?.kind === 'plan_execute') return t('chatPlanExecuteTitle')
      return '计划'
    default: return '步骤'
  }
}

type PlanTaskRowVM = {
  index: number
  task: string
  status: 'pending' | 'running' | 'done' | 'error'
  details?: { text: string; tone: 'error' | 'muted' }[]
}

function planExecuteTasks (step: ThoughtStep): PlanTaskRowVM[] {
  const raw = step.data?.tasks
  if (Array.isArray(raw)) return raw as PlanTaskRowVM[]
  return []
}

function planExecuteSummaryLine (step: ThoughtStep): string {
  const tasks = planExecuteTasks(step)
  let done = 0
  let running = 0
  let err = 0
  let pending = 0
  for (const r of tasks) {
    if (r.status === 'done') done++
    else if (r.status === 'running') running++
    else if (r.status === 'error') err++
    else pending++
  }
  return t('chatPlanExecuteSummary', { total: tasks.length, done, running, err, pending })
}

function planRowStatusIcon (status: string): string {
  if (status === 'done') return 'check_circle'
  if (status === 'error') return 'error_outline'
  if (status === 'running') return 'progress_activity'
  return 'radio_button_unchecked'
}

function planRowStatusColor (status: string): string {
  if (status === 'done') return 'positive'
  if (status === 'error') return 'negative'
  if (status === 'running') return 'primary'
  return 'grey-5'
}

function timelineBorderForStep (step: ThoughtStep): string {
  switch (step.type) {
    case 'thought': return '#b39ddb'
    case 'action': return '#90caf9'
    case 'observation': return '#a5d6a7'
    case 'reflection': return '#ffcc80'
    case 'final': return '#81c784'
    case 'plan': return '#5c6bc0'
    case 'error': return '#ef9a9a'
    default: return '#bdbdbd'
  }
}

const getActionBadgeText = (step: ThoughtStep) => {
  if (step.data?.kind === 'hitl') {
    switch (step.data?.phase) {
      case 'requested': return 'HITL_REQUEST'
      case 'approved': return 'HITL_APPROVED'
      case 'rejected': return 'HITL_REJECTED'
      default: return 'HITL'
    }
  }
  if (step.data?.kind === 'client_tool_call') {
    const r = step.data?.risk_level
    const nm = toolOrName(step)
    return typeof r === 'string' ? `CLIENT · ${String(r).toUpperCase()} · ${nm}` : `CLIENT · ${nm}`
  }
  if (step.data?.kind === 'server_tool_call' && typeof step.data?.risk_level === 'string') {
    return `RISK ${String(step.data.risk_level).toUpperCase()} · ${toolOrName(step)}`
  }
  return `CALL: ${toolOrName(step)}`
}

const badgeClassForRisk = (risk: string): string => {
  const r = (risk || 'medium').toLowerCase()
  if (r === 'low') return 'text-green-8 bg-green-1'
  if (r === 'medium') return 'text-amber-8 bg-amber-1'
  if (r === 'high') return 'text-deep-orange-9 bg-orange-1'
  if (r === 'critical') return 'text-red-8 bg-red-1'
  return 'text-blue-6 bg-blue-1'
}

const getActionBadgeClass = (step: ThoughtStep) => {
  if (step.data?.kind === 'hitl') {
    switch (step.data?.phase) {
      case 'requested': return 'text-amber-8 bg-amber-1'
      case 'approved': return 'text-green-8 bg-green-1'
      case 'rejected': return 'text-red-8 bg-red-1'
      default: return 'text-grey-6 bg-grey-2'
    }
  }
  if (step.data?.kind === 'client_tool_call' && typeof step.data?.risk_level === 'string') {
    return badgeClassForRisk(step.data.risk_level as string)
  }
  if (step.data?.kind === 'server_tool_call' && typeof step.data?.risk_level === 'string') {
    return badgeClassForRisk(step.data.risk_level as string)
  }
  return 'text-blue-6 bg-blue-1'
}

const getActionContentClass = (step: ThoughtStep) => {
  if (step.data?.kind !== 'hitl') {
    return 'text-grey-7'
  }
  switch (step.data?.phase) {
    case 'requested': return 'text-amber-8'
    case 'approved': return 'text-green-8'
    case 'rejected': return 'text-red-8'
    default: return 'text-grey-7'
  }
}

/** ReAct 可能下发空的 final_answer；正文已在「反思」中 —— 展示时回退到前序 reflection */
const getFinalAnswerBody = (step: ThoughtStep, index: number): string => {
  const c = step.data?.content
  if (typeof c === 'string' && c.trim() !== '') return c
  if (step.type !== 'final') return ''
  const list = thinkingSteps.value
  for (let i = index - 1; i >= 0; i--) {
    if (list[i]?.type === 'reflection') {
      const rc = list[i].data?.content
      if (typeof rc === 'string' && rc.trim() !== '') return rc
    }
  }
  return ''
}

const getProcessedContent = (step: ThoughtStep) => {
  const content = step.data?.content || step.data?.thought || ''
  if (typeof content !== 'string') return ''

  const isLastThinkingStep = thinkingSteps.value[thinkingSteps.value.length - 1] === step
  const isStreaming = props.status === 'running' && isLastThinkingStep

  if (isStreaming) {
    const lowerContent = content.toLowerCase()
    if (lowerContent.includes('<think>')) {
      const index = lowerContent.lastIndexOf('<think>')
      return content.substring(index + 7).trim().replace(/<\/?[a-zA-Z]*>?$/gi, '')
    }
    return content.replace(/<\/?[a-zA-Z]*>?$/gi, '').trim()
  } else {
    return content
      .replace(/<think>[\s\S]*?<\/think>/gi, '')
      .replace(/^[\s\S]*?<\/think>/gi, '')
      .replace(/<think>[\s\S]*$/gi, '')
      .replace(/\n{3,}/g, '\n\n')
      .trim()
  }
}
</script>

<template>
  <div
    class="thought-sidebar full-height column"
    :style="{ width: isOpen ? '350px' : '0px', minWidth: isOpen ? '350px' : '0px' }"
    :class="{ 'overflow-hidden': !isOpen }"
  >
    <div v-if="isOpen" class="flex column full-height overflow-hidden bg-grey-1">
      <!-- Header -->
      <div class="q-pa-md bg-white border-bottom q-pb-sm">
        <div class="row items-center justify-between">
          <div class="column">
            <div class="row items-center q-gutter-x-sm">
              <q-icon name="psychology" color="primary" size="xs" />
              <span class="text-subtitle2 text-weight-bold">思考过程 (Thinking)</span>
            </div>
            <div
              v-if="durationMs != null && durationMs > 0 && hasToolCalls"
              class="text-caption text-grey-6 q-mt-xs q-pl-lg"
            >
              {{ t('chatDuration', { s: formatDurationSeconds(durationMs) }) }}
            </div>
          </div>
          <div class="row items-center q-gutter-sm">
            <q-btn
              flat
              dense
              size="sm"
              :label="expandedSteps.size === thinkingSteps.length ? '折叠' : '展开'"
              color="primary"
              @click="expandedSteps.size === thinkingSteps.length ? collapseAll() : expandAll()"
            />
            <q-chip dense size="sm" color="grey-3" text-color="grey-7">
              {{ thinkingSteps.length }}
            </q-chip>
          </div>
        </div>
      </div>

      <!-- Content -->
      <q-scroll-area ref="thoughtContainer" class="col q-pa-md">
        <div v-if="thinkingSteps.length === 0" class="full-height column items-center justify-center text-center q-pa-xl">
          <template v-if="status === 'running'">
            <q-spinner-dots color="primary" size="lg" />
            <p class="text-caption text-grey-6 q-mt-md">正在生成回答…</p>
          </template>
          <template v-else>
            <q-icon name="info_outline" size="40px" color="grey-4" />
            <p class="text-caption text-grey-5 q-mt-md">本回复无工具调用与推理步骤</p>
          </template>
        </div>

        <div class="relative" style="padding-left: 14px;">
          <!-- Timeline Line -->
          <div class="absolute" style="left: 6px; top: 8px; bottom: 8px; width: 1px; background: #e0e0e0;" />

          <div
            v-for="(step, index) in thinkingSteps"
            :key="index"
            class="q-mb-md relative"
          >
            <!-- Timeline Dot -->
            <div
              class="absolute rounded-borders z-index-1"
              :style="{
                left: '-10.5px',
                top: '6px',
                width: '7px',
                height: '7px',
                background: 'white',
                border: `2px solid ${timelineBorderForStep(step)}`,
                boxShadow: isStepExpanded(index) ? '0 0 3px rgba(0,0,0,0.1)' : 'none'
              }"
            />

            <q-card
              flat
              bordered
              class="cursor-pointer hover:bg-grey-2"
              @click="toggleStep(index)"
            >
              <q-card-section class="q-pa-sm">
                <div class="row items-center justify-between q-mb-xs">
                  <div class="row items-center q-gutter-x-xs overflow-hidden">
                    <q-icon :name="getStepIcon(step.type)" :class="getStepColor(step.type)" size="xs" />
                    <span class="text-caption text-grey-6 text-uppercase ellipsis">{{ getStepTitle(step) }}</span>
                  </div>
                  <div class="row items-center q-gutter-xs flex-no-wrap">
                    <span v-if="step.timestamp" class="text-caption text-grey-5">{{ new Date(step.timestamp).toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) }}</span>
                    <q-icon
                      name="expand_more"
                      :class="['text-grey-4', isStepExpanded(index) ? 'rotate-180' : '']"
                      :style="{ transition: 'transform 0.3s' }"
                      size="xs"
                    />
                  </div>
                </div>

                <div v-if="step.type === 'thought'" :class="['text-body2 text-grey-7', isStepExpanded(index) ? '' : 'text-truncate-2']">
                  <template v-if="getProcessedContent(step)">
                    {{ getProcessedContent(step) }}
                  </template>
                  <div v-else-if="props.status !== 'running'" class="row items-center q-gutter-xs text-grey-5 italic text-caption">
                    <q-icon name="check_circle" size="xs" class="opacity-50" />
                    <span>思考过程已收起</span>
                  </div>
                </div>

                <div
                  v-else-if="step.type === 'plan' && step.data?.kind === 'plan_execute'"
                  class="column q-gutter-xs"
                >
                  <div class="text-caption text-grey-6">
                    {{ planExecuteSummaryLine(step) }}
                  </div>
                  <q-slide-transition>
                    <div
                      v-show="isStepExpanded(index)"
                      class="plan-task-scroll q-pl-xs"
                      @click.stop
                    >
                      <div
                        v-for="(row, ri) in planExecuteTasks(step)"
                        :key="ri"
                        class="column q-py-xs"
                      >
                        <div class="row items-start no-wrap">
                          <div class="q-mt-xs" style="flex-shrink: 0; width: 18px;">
                            <q-spinner
                              v-if="row.status === 'running'"
                              color="primary"
                              size="16px"
                            />
                            <q-icon
                              v-else
                              :name="planRowStatusIcon(row.status)"
                              :color="planRowStatusColor(row.status)"
                              size="16px"
                            />
                          </div>
                          <div
                            class="text-body2 col"
                            style="word-break: break-word; min-width: 0;"
                            :class="row.status === 'done' ? 'text-grey-6' : 'text-grey-8'"
                          >
                            <span class="text-weight-bold text-grey-7 q-mr-xs">{{ row.index }}.</span>
                            {{ row.task }}
                          </div>
                        </div>
                        <ul
                          v-if="row.details?.length"
                          class="q-ma-none q-pl-lg q-mt-xs text-caption"
                          style="list-style: disc;"
                        >
                          <li
                            v-for="(d, di) in row.details"
                            :key="di"
                            class="q-py-xs"
                            style="word-break: break-word;"
                            :class="d.tone === 'error' ? 'text-negative' : 'text-grey-6'"
                          >
                            {{ d.text }}
                          </li>
                        </ul>
                      </div>
                    </div>
                  </q-slide-transition>
                </div>

                <div v-else-if="step.type === 'action'" class="column q-gutter-xs">
                  <div class="row items-center q-gutter-xs flex-wrap">
                    <q-chip
                      dense
                      size="sm"
                      :class="['text-caption', 'q-pa-xs', getActionBadgeClass(step)]"
                    >
                      {{ getActionBadgeText(step) }}
                    </q-chip>
                    <q-chip
                      v-if="step.meta?.modelName"
                      dense
                      size="sm"
                      class="text-caption bg-grey-2 text-grey-7"
                    >
                      LLM: {{ step.meta.modelName }}
                    </q-chip>
                  </div>
                  <div
                    v-if="step.data?.content"
                    :class="['text-caption', getActionContentClass(step)]"
                  >
                    {{ step.data.content }}
                  </div>
                  <div
                    v-if="isStepExpanded(index)"
                    class="q-mt-sm bg-grey-2 q-pa-xs rounded-borders"
                  >
                    <pre
                      v-if="hasActionInput(step)"
                      class="text-caption text-grey-8 q-ma-none"
                      style="white-space: pre-wrap; word-break: break-all;"
                    >{{ JSON.stringify(step.data.input, null, 2) }}</pre>
                    <div v-else class="text-caption text-grey-5">
                      （无结构化参数；若为新会话，请重新请求以加载工具参数）
                    </div>
                  </div>
                </div>

                <div v-else-if="step.type === 'observation'" class="column q-gutter-xs">
                  <div class="row items-center text-caption text-green-6 text-weight-bold">
                    <q-icon name="storage" size="xs" class="q-mr-xs" />
                    TOOL_RESULT
                  </div>
                  <div
                    v-if="getObservationBody(step)"
                    :class="[
                      'text-caption text-grey-7',
                      isStepExpanded(index) ? 'observation-body-expanded' : 'text-truncate-2'
                    ]"
                    style="white-space: pre-wrap; word-break: break-word;"
                  >
                    {{ getObservationBody(step) }}
                  </div>
                  <div v-else-if="props.status !== 'running'" class="text-caption text-grey-5 italic">
                    （无返回正文）
                  </div>
                </div>

                <div v-else-if="step.type === 'reflection'" class="column q-gutter-xs">
                  <div class="row items-center text-caption text-orange-6 text-weight-bold">
                    <q-icon name="auto_fix_high" size="xs" class="q-mr-xs" />
                    REFLECTION
                  </div>
                  <div
                    v-if="step.data?.content"
                    :class="['text-caption text-grey-7', isStepExpanded(index) ? '' : 'text-truncate-2']"
                    :style="isStepExpanded(index) ? { whiteSpace: 'pre-wrap', wordBreak: 'break-word' } : {}"
                  >
                    {{ step.data.content }}
                  </div>
                </div>

                <div v-else-if="step.type === 'final'" class="column q-gutter-xs">
                  <div class="row items-center text-caption text-positive text-weight-bold">
                    <q-icon name="check_circle" size="xs" class="q-mr-xs" />
                    FINAL_ANSWER
                  </div>
                  <div
                    v-if="getFinalAnswerBody(step, index)"
                    :class="['text-caption text-grey-7', isStepExpanded(index) ? '' : 'text-truncate-2']"
                    :style="isStepExpanded(index) ? { whiteSpace: 'pre-wrap', wordBreak: 'break-word' } : {}"
                  >
                    {{ getFinalAnswerBody(step, index) }}
                  </div>
                </div>

                <div v-else-if="step.type === 'error'" class="column q-gutter-xs">
                  <div class="row items-center text-caption text-negative text-weight-bold">
                    <q-icon name="error" size="xs" class="q-mr-xs" />
                    ERROR
                  </div>
                  <div v-if="step.data?.content" class="text-caption text-negative">
                    {{ step.data.content }}
                  </div>
                </div>
              </q-card-section>
            </q-card>
          </div>

          <!-- Processing Indicator -->
          <div v-if="status === 'running'" class="q-pb-sm relative">
            <div
              class="absolute rounded-borders z-index-1"
              style="left: -10.5px; top: 6px; width: 7px; height: 7px; background: white; border: 2px solid #90caf9;"
            />
            <div class="row items-center q-pa-xs bg-blue-1 rounded-borders border-blue-2">
              <div class="row items-center q-gutter-xs">
                <q-spinner-dots color="blue" size="xs" />
                <span class="text-caption text-blue text-weight-bold">AI 正在深度分析中...</span>
              </div>
            </div>
          </div>
        </div>
      </q-scroll-area>
    </div>
  </div>
</template>

<style scoped>
.thought-sidebar {
  flex-shrink: 0;
  transition: width 0.3s ease;
  border-left: 1px solid #e0e0e0;
}

.thought-toggle-btn {
  position: absolute;
  left: -20px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 20;
}

.thought-toggle-btn-left {
  position: absolute;
  right: -40px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 20;
}

.text-truncate-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.observation-body-expanded {
  max-height: 45vh;
  overflow-y: auto;
}

.plan-task-scroll {
  max-height: 320px;
  overflow-y: auto;
}

.rotate-180 {
  transform: rotate(180deg);
}

.z-index-1 {
  z-index: 1;
}

.overflow-hidden {
  overflow: hidden !important;
}

.bg-tech-grid {
  background-image:
    radial-gradient(circle at 2px 2px, rgba(0, 122, 255, 0.02) 1px, transparent 0);
  background-size: 16px 16px;
}

:deep(.q-scrollarea__container) {
  overflow: auto;
}

:deep(.q-scrollarea__content) {
  overflow: visible;
}
</style>
