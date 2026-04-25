<script setup lang="ts">
import { computed, ref, watch } from 'vue'

interface PlanDetailLine {
  text: string
  tone: 'error' | 'muted'
}

interface PlanTaskRow {
  index: number
  task: string
  status: 'pending' | 'running' | 'done' | 'error'
  details?: PlanDetailLine[]
}

const props = defineProps<{
  tasks: PlanTaskRow[]
  title?: string
}>()

const emit = defineEmits<{
  toggle: []
}>()

const LONG_PLAN_THRESHOLD = 8
/** 与 taskmate-desktop PlanExecuteTaskPanel 一致：步骤数较多时每步可折叠 */
const STEP_ACCORDION_THRESHOLD = 5

/** 长计划默认折叠；短计划默认展开。用户点击标题可切换（与 tasks 数量无关）。 */
const collapsed = ref(false)

watch(
  () => props.tasks,
  (tasks) => {
    collapsed.value = tasks.length > LONG_PLAN_THRESHOLD
  },
  { immediate: true, deep: true }
)

const expanded = computed(() => !collapsed.value)

function onHeaderClick (): void {
  collapsed.value = !collapsed.value
  emit('toggle')
}

const useStepAccordion = computed(() => props.tasks.length >= STEP_ACCORDION_THRESHOLD)

/** 手风琴模式下当前展开的步骤下标 [0..n)；-1 表示全部收起（与桌面端一致：默认先展开第 1 步） */
const openStepIdx = ref(0)
const prevRunningIdx = ref<number | null>(null)

function syncOpenStepToRunning (): void {
  if (!useStepAccordion.value) return
  const tasks = props.tasks
  const runningIdx = tasks.findIndex((r) => r.status === 'running')
  if (runningIdx >= 0 && runningIdx !== prevRunningIdx.value) {
    openStepIdx.value = runningIdx
    prevRunningIdx.value = runningIdx
  }
  if (runningIdx < 0) prevRunningIdx.value = null
}

watch(
  () => props.tasks,
  () => {
    syncOpenStepToRunning()
  },
  { deep: true, immediate: true }
)

function onStepModelUpdate (i: number, v: boolean): void {
  if (v) openStepIdx.value = i
  else if (openStepIdx.value === i) openStepIdx.value = -1
}

const stats = computed(() => {
  let done = 0
  let running = 0
  let err = 0
  let pending = 0
  for (const r of props.tasks) {
    if (r.status === 'done') done += 1
    else if (r.status === 'running') running += 1
    else if (r.status === 'error') err += 1
    else pending += 1
  }
  return { done, running, err, pending, total: props.tasks.length }
})

function statusIcon (status: string): string {
  if (status === 'done') return 'check_circle'
  if (status === 'error') return 'error_outline'
  if (status === 'running') return 'progress_activity'
  return 'radio_button_unchecked'
}

function statusColor (status: string): string {
  if (status === 'done') return 'positive'
  if (status === 'error') return 'negative'
  if (status === 'running') return 'primary'
  return 'grey-5'
}
</script>

<template>
  <div v-if="tasks.length === 0" />
  <div v-else class="plan-execute-panel">
    <div class="plan-execute-header" @click="onHeaderClick">
      <div class="plan-execute-header-content">
        <div class="text-caption text-weight-medium text-grey-7">
          {{ title || '执行计划' }}
        </div>
        <div class="text-caption text-grey-6 q-mt-xs">
          {{ stats.total }} 步骤 · {{ stats.done }} 完成 · {{ stats.running }} 执行中 · {{ stats.err }} 错误 · {{ stats.pending }} 待执行
        </div>
      </div>
      <q-icon
        :name="expanded ? 'expand_more' : 'chevron_right'"
        class="plan-execute-toggle"
        color="grey-6"
      />
    </div>
    <q-slide-transition>
      <div v-show="expanded" class="plan-execute-list q-mt-sm">
        <!-- 步骤较多：每步 q-expansion-item，与桌面端手风琴一致 -->
        <template v-if="useStepAccordion">
          <q-expansion-item
            v-for="(row, i) in tasks"
            :key="row.index"
            group="plan-execute-steps"
            dense
            class="plan-step-expansion"
            :model-value="openStepIdx === i"
            @update:model-value="onStepModelUpdate(i, $event)"
          >
            <template #header="{ expanded: stepExpanded }">
              <q-item-section avatar top class="plan-task-icon-wrap">
                <q-spinner
                  v-if="row.status === 'running'"
                  color="primary"
                  size="18px"
                />
                <q-icon
                  v-else
                  :name="statusIcon(row.status)"
                  :color="statusColor(row.status)"
                  size="18px"
                />
              </q-item-section>
              <q-item-section>
                <div
                  class="text-body2 plan-task-text"
                  :class="[
                    row.status === 'done' ? 'text-grey-6' : 'text-grey-8',
                    !stepExpanded ? 'plan-task-title-clamp' : ''
                  ]"
                >
                  <span class="text-weight-medium text-grey-7 q-mr-xs">{{ row.index }}.</span>
                  {{ row.task }}
                </div>
              </q-item-section>
            </template>
            <div v-if="openStepIdx === i && row.details?.length" class="plan-expansion-details">
              <ul class="plan-task-details plan-task-details--accordion">
                <li
                  v-for="(d, di) in row.details"
                  :key="di"
                  class="plan-task-detail-item"
                  :class="d.tone === 'error' ? 'text-negative' : 'text-grey-6'"
                >
                  {{ d.text }}
                </li>
              </ul>
            </div>
          </q-expansion-item>
        </template>
        <!-- 步骤较少：平铺（与原先一致） -->
        <template v-else>
          <div
            v-for="row in tasks"
            :key="row.index"
            class="plan-task-row"
          >
            <div class="row items-start no-wrap">
              <div class="q-mt-xs plan-task-icon">
                <q-spinner
                  v-if="row.status === 'running'"
                  color="primary"
                  size="18px"
                />
                <q-icon
                  v-else
                  :name="statusIcon(row.status)"
                  :color="statusColor(row.status)"
                  size="18px"
                />
              </div>
              <div
                class="col plan-task-text"
                :class="row.status === 'done' ? 'text-grey-6' : 'text-grey-8'"
              >
                <span class="text-weight-medium text-grey-7 q-mr-xs">{{ row.index }}.</span>
                {{ row.task }}
              </div>
            </div>
            <ul
              v-if="row.details?.length"
              class="plan-task-details"
            >
              <li
                v-for="(d, di) in row.details"
                :key="di"
                class="plan-task-detail-item"
                :class="d.tone === 'error' ? 'text-negative' : 'text-grey-6'"
              >
                {{ d.text }}
              </li>
            </ul>
          </div>
        </template>
      </div>
    </q-slide-transition>
  </div>
</template>

<style scoped>
.plan-execute-panel {
  width: 100%;
  margin-bottom: 12px;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(0, 0, 0, 0.02);
}

.body--dark .plan-execute-panel {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
}

.plan-execute-header {
  display: flex;
  align-items: flex-start;
  cursor: pointer;
  user-select: none;
  padding: 4px;
  border-radius: 4px;
}

.plan-execute-header:hover {
  background: rgba(0, 0, 0, 0.04);
}

.body--dark .plan-execute-header:hover {
  background: rgba(255, 255, 255, 0.04);
}

.plan-execute-header-content {
  flex: 1;
  min-width: 0;
}

.plan-execute-toggle {
  flex-shrink: 0;
  margin-left: 8px;
}

.plan-execute-list {
  max-height: 420px;
  overflow-y: auto;
  padding-right: 4px;
}

.plan-step-expansion {
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-radius: 6px;
  margin-bottom: 6px;
  overflow: hidden;
}

.body--dark .plan-step-expansion {
  border-color: rgba(255, 255, 255, 0.08);
}

.plan-step-expansion:last-child {
  margin-bottom: 0;
}

.plan-task-icon-wrap {
  min-width: 28px;
  justify-content: flex-start;
}

.plan-task-title-clamp {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.plan-expansion-details {
  padding: 0 12px 10px 12px;
}

.plan-task-details--accordion {
  margin-top: 0;
  padding-left: 8px;
  margin-left: 0;
}

.plan-task-row {
  padding: 8px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.04);
}

.plan-task-row:last-child {
  border-bottom: none;
}

.plan-task-icon {
  flex-shrink: 0;
  width: 24px;
  margin-right: 4px;
}

.plan-task-text {
  word-break: break-word;
  min-width: 0;
}

.plan-task-details {
  margin: 4px 0 0 0;
  padding-left: 28px;
  list-style: disc;
  width: 100%;
  box-sizing: border-box;
}

.plan-task-detail-item {
  padding: 2px 0;
  font-size: 0.75rem;
  line-height: 1.45;
  word-break: break-word;
}
</style>
