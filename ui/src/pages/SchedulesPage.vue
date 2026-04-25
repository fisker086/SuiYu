<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('schedules') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createSchedule')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense :loading="loading" @click="load" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-table flat bordered :rows="rows" :columns="columns" row-key="id" :loading="loading" :no-data-label="t('noData')">
      <template #body-cell-channel_name="props">
        <q-td :props="props">
          {{ props.row.channel_name || '—' }}
        </q-td>
      </template>
      <template #body-cell-schedule_kind="props">
        <q-td :props="props">
          <q-badge :color="getScheduleKindColor(props.row.schedule_kind)" :label="getScheduleKindLabel(props.row.schedule_kind)" />
        </q-td>
      </template>
      <template #body-cell-enabled="props">
        <q-td :props="props">
          <q-toggle v-model="props.row.enabled" dense @update:model-value="toggleEnabled(props.row)" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="primary" :label="t('trigger')" @click="triggerSchedule(props.row)" :loading="triggering === props.row.id" />
          <q-btn dense flat color="secondary" :label="t('edit')" @click="openDialog(props.row)" />
          <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="dialogOpen">
      <q-card style="min-width: 480px; max-width: min(92vw, 520px);">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ isEdit ? t('editSchedule') : t('createSchedule') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none" style="max-height: 70vh; overflow-y: auto;">
          <div class="row q-col-gutter-md">
            <div class="col-12">
              <q-input v-model="form.name" :label="t('scheduleName')" outlined dense :rules="[v => !!v]" />
            </div>
            <div class="col-12">
              <q-input v-model="form.description" :label="t('description')" outlined dense />
            </div>
            <div class="col-12">
              <q-select
                v-model="form.channel_id"
                :label="t('scheduleNotifyChannel')"
                :options="channelOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
                clearable
                :hint="t('scheduleNotifyChannelHint')"
              />
            </div>
            <div class="col-12">
              <q-select
                v-model="targetType"
                :label="t('scheduleTargetType')"
                :options="targetOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
              />
            </div>
            <div class="col-12" v-if="targetType === 'agent'">
              <q-select
                v-model="form.agent_id"
                :label="t('targetAgent')"
                :options="agentOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
                clearable
                :placeholder="t('selectAgentPlaceholder')"
                :hint="agentOptions.length === 0 ? t('scheduleNoAgentHint') : undefined"
                :rules="[v => (v != null && Number(v) > 0) || t('selectAgentPlaceholder')]"
              />
            </div>
            <div class="col-12" v-if="targetType === 'workflow'">
              <q-select
                v-model="form.workflow_id"
                :label="t('targetWorkflow')"
                :options="workflowOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
                clearable
                :placeholder="t('selectWorkflowPlaceholder')"
                :hint="workflowOptions.length === 0 ? t('scheduleNoWorkflowHint') : undefined"
                :rules="[v => (v != null && Number(v) > 0) || t('selectWorkflowPlaceholder')]"
              />
            </div>
            <div class="col-12" v-if="targetType === 'workflow'">
              <q-input
                v-model="form.prompt"
                :label="t('scheduleWorkflowPrompt')"
                outlined
                dense
                type="textarea"
                rows="3"
                :hint="t('scheduleWorkflowPromptHint')"
              />
            </div>
            <div class="col-12" v-if="targetType === 'code'">
              <q-select
                v-model="form.code_language"
                :label="t('scheduleCodeLanguage')"
                :options="codeLanguageOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
              />
            </div>
            <div class="col-12" v-if="targetType === 'code'">
              <q-input
                v-model="form.prompt"
                :label="t('scheduleCodeEditor')"
                outlined
                dense
                type="textarea"
                rows="14"
                class="monospace"
                :hint="t('scheduleCodeHint')"
              />
            </div>
            <div class="col-12">
              <q-select
                v-model="form.schedule_kind"
                :label="t('scheduleType')"
                :options="scheduleKindOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
                :rules="[v => !!v]"
              />
            </div>
            <div class="col-12" v-if="form.schedule_kind === 'cron'">
              <q-input v-model="form.cron_expr" :label="t('cronExpressionLabel')" outlined dense placeholder="0 7 * * *" />
            </div>
            <div class="col-12" v-if="form.schedule_kind === 'at'">
              <q-input v-model="form.at" :label="t('atTimeLabel')" outlined dense placeholder="2026-01-01T09:00:00Z" />
            </div>
            <div class="col-12" v-if="form.schedule_kind === 'every'">
              <q-input v-model.number="form.every_ms" :label="t('everyMsLabel')" outlined dense type="number" />
            </div>
            <div class="col-12" v-if="targetType === 'agent'">
              <q-select
                v-model="form.session_target"
                :label="t('sessionTarget')"
                :options="sessionTargetOptions"
                outlined
                dense
                emit-value
                map-options
                option-value="value"
                option-label="label"
              />
            </div>
            <div class="col-12" v-if="targetType === 'agent'">
              <q-input v-model="form.prompt" :label="t('prompt')" outlined dense type="textarea" rows="4" :rules="[v => !!v]" />
            </div>
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn v-close-popup flat :label="t('cancel')" />
          <q-btn color="primary" :label="t('save')" :loading="saving" unelevated @click="save" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { Schedule, Agent, APIResponse, CreateScheduleRequest, NotifyChannel, UpdateScheduleRequest, WorkflowDefinition } from 'src/api/types'

defineOptions({ name: 'SchedulesPage' })

const { t } = useI18n()
const $q = useQuasar()

const loading = ref(false)
const saving = ref(false)
const triggering = ref(0)
const errorMsg = ref('')
const rows = ref<Schedule[]>([])
const agents = ref<Agent[]>([])
const agentOptions = ref<{ label: string; value: number }[]>([])
const workflows = ref<WorkflowDefinition[]>([])
const workflowOptions = ref<{ label: string; value: number }[]>([])
const channelOptions = ref<{ label: string; value: number }[]>([])

const codeLangLabel: Record<string, string> = {
  python: 'Python',
  shell: 'Shell',
  javascript: 'JavaScript'
}

/** 列表「执行目标」列：代码类只显示语言，避免长脚本撑破表格 */
function scheduleTargetCell (row: Schedule): string {
  if (row.code_language) {
    const lang = codeLangLabel[row.code_language] ?? row.code_language
    return t('scheduleCodeTargetBrief', { lang })
  }
  return row.workflow_name || row.agent_name || '—'
}

const columns = computed(() => [
  { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
  { name: 'name', label: t('name'), field: 'name', align: 'left' as const },
  {
    name: 'target_name',
    label: t('scheduleTargetCol'),
    field: (row: Schedule) => scheduleTargetCell(row),
    align: 'left' as const
  },
  { name: 'channel_name', label: t('notifyChannelShort'), field: 'channel_name', align: 'left' as const },
  { name: 'schedule_kind', label: t('scheduleKindShort'), field: 'schedule_kind', align: 'center' as const },
  { name: 'cron_expr', label: t('scheduleExpr'), field: 'cron_expr', align: 'left' as const },
  { name: 'enabled', label: t('isActive'), field: 'enabled', align: 'center' as const },
  { name: 'actions', label: t('actions'), field: 'actions', align: 'center' as const }
])

const dialogOpen = ref(false)
const isEdit = ref(false)
const editId = ref(0)
const targetType = ref<'agent' | 'workflow' | 'code'>('agent')
const form = reactive({
  name: '',
  description: '',
  /** null = 未选择；避免用 0 导致下拉显示成数字 */
  agent_id: null as number | null,
  workflow_id: null as number | null,
  code_language: 'python' as 'python' | 'shell' | 'javascript',
  schedule_kind: 'cron',
  cron_expr: '0 7 * * *',
  at: '',
  every_ms: 3600000,
  session_target: 'main',
  prompt: '',
  channel_id: null as number | null
})

const targetOptions = computed(() => [
  { label: t('targetAgent'), value: 'agent' },
  { label: t('targetWorkflow'), value: 'workflow' },
  { label: t('scheduleTargetCode'), value: 'code' }
])

const codeLanguageOptions = computed(() => [
  { label: 'Python', value: 'python' },
  { label: 'Shell', value: 'shell' },
  { label: 'JavaScript', value: 'javascript' }
])

const scheduleKindOptions = computed(() => [
  { label: t('scheduleKindCronOpt'), value: 'cron' },
  { label: t('scheduleKindAtOpt'), value: 'at' },
  { label: t('scheduleKindEveryOpt'), value: 'every' }
])

const sessionTargetOptions = computed(() => [
  { label: t('sessionTargetMainOpt'), value: 'main' },
  { label: t('sessionTargetIsolatedOpt'), value: 'isolated' }
])

async function loadAgents () {
  try {
    const { data } = await api.get<APIResponse<Agent[]>>('/agents/for-schedule')
    agents.value = (data.data ?? []) as Agent[]
    agentOptions.value = agents.value.map(a => ({ label: a.name, value: a.id }))
  } catch {
    agents.value = []
    agentOptions.value = []
  }
}

async function loadWorkflows () {
  try {
    const { data } = await api.get<APIResponse<WorkflowDefinition[]>>('/workflows/graph')
    workflows.value = (data.data ?? []) as WorkflowDefinition[]
    workflowOptions.value = workflows.value
      .filter(w => w.is_active)
      .map(w => ({ label: w.name, value: w.id }))
  } catch {
    workflows.value = []
    workflowOptions.value = []
  }
}

async function loadChannels () {
  try {
    const { data } = await api.get<APIResponse<NotifyChannel[]>>('/channels')
    const list = (data.data ?? []) as NotifyChannel[]
    channelOptions.value = list
      .filter(c => c.is_active)
      .map(c => ({ label: `${c.name} (${c.kind})`, value: c.id }))
  } catch {
    channelOptions.value = []
  }
}

async function load () {
  loading.value = true
  errorMsg.value = ''
  try {
    const { data } = await api.get<APIResponse<Schedule[]>>('/schedules')
    const schedules = (data.data ?? []) as Schedule[]
    for (const sch of schedules) {
      if (sch.agent_id) {
        const agent = agents.value.find(a => a.id === sch.agent_id)
        sch.agent_name = agent?.name ?? ''
      }
      if (sch.workflow_id) {
        const wf = workflows.value.find(w => w.id === sch.workflow_id)
        sch.workflow_name = wf?.name ?? ''
      }
    }
    rows.value = schedules
  } catch (e: unknown) {
    rows.value = []
    const err = e as { response?: { data?: { message?: string } } }
    errorMsg.value = err.response?.data?.message ?? t('loadFailed')
  } finally {
    loading.value = false
  }
}

function openDialog (row?: Schedule) {
  if (row) {
    isEdit.value = true
    editId.value = row.id
    form.name = row.name
    form.description = row.description || ''
    if (row.code_language) {
      targetType.value = 'code'
      form.code_language = row.code_language as 'python' | 'shell' | 'javascript'
      form.agent_id = null
      form.workflow_id = null
    } else if (row.workflow_id) {
      targetType.value = 'workflow'
      form.workflow_id = row.workflow_id
      form.agent_id = null
    } else {
      targetType.value = 'agent'
      form.agent_id = row.agent_id ?? null
      form.workflow_id = null
    }
    form.schedule_kind = row.schedule_kind
    form.cron_expr = row.cron_expr || ''
    form.at = row.at || ''
    form.every_ms = row.every_ms || 3600000
    form.session_target = row.session_target
    form.prompt = row.prompt
    form.channel_id = row.channel_id != null && row.channel_id > 0 ? row.channel_id : null
  } else {
    isEdit.value = false
    editId.value = 0
    const firstAgent = agentOptions.value[0]?.value ?? null
    targetType.value = 'agent'
    Object.assign(form, {
      name: '',
      description: '',
      agent_id: firstAgent,
      workflow_id: null,
      code_language: 'python',
      schedule_kind: 'cron',
      cron_expr: '0 7 * * *',
      at: '',
      every_ms: 3600000,
      session_target: 'main',
      prompt: '',
      channel_id: null
    })
  }
  dialogOpen.value = true
}

async function save () {
  if (!form.name?.trim()) {
    $q.notify({ type: 'negative', message: t('scheduleNameRequired') })
    return
  }
  if (targetType.value === 'agent') {
    if (form.agent_id == null || form.agent_id < 1) {
      $q.notify({ type: 'negative', message: t('selectAgentPlaceholder') })
      return
    }
    if (!form.prompt?.trim()) {
      $q.notify({ type: 'negative', message: t('schedulePromptRequired') })
      return
    }
  } else if (targetType.value === 'workflow') {
    if (form.workflow_id == null || form.workflow_id < 1) {
      $q.notify({ type: 'negative', message: t('selectWorkflowPlaceholder') })
      return
    }
  } else if (targetType.value === 'code') {
    if (!form.prompt?.trim()) {
      $q.notify({ type: 'negative', message: t('scheduleCodeRequired') })
      return
    }
  }

  const baseCommon = {
    name: form.name.trim(),
    description: form.description.trim(),
    schedule_kind: form.schedule_kind as 'at' | 'every' | 'cron',
    cron_expr: form.cron_expr,
    at: form.at,
    every_ms: form.every_ms,
    wake_mode: 'now' as const,
    session_target: 'main' as const,
    enabled: true,
    ...(form.channel_id != null && form.channel_id > 0 ? { channel_id: form.channel_id } : {})
  }

  saving.value = true
  try {
    let req: CreateScheduleRequest
    if (targetType.value === 'workflow') {
      req = {
        ...baseCommon,
        workflow_id: form.workflow_id as number,
        prompt: form.prompt.trim()
      }
    } else if (targetType.value === 'code') {
      req = {
        ...baseCommon,
        code_language: form.code_language,
        prompt: form.prompt.trim()
      }
    } else {
      req = {
        ...baseCommon,
        agent_id: form.agent_id as number,
        session_target: form.session_target,
        prompt: form.prompt.trim()
      }
    }

    if (isEdit.value) {
      const body: UpdateScheduleRequest = {
        name: form.name.trim(),
        description: form.description.trim(),
        schedule_kind: form.schedule_kind,
        cron_expr: form.cron_expr,
        at: form.at,
        every_ms: form.every_ms,
        enabled: true,
        prompt: form.prompt.trim()
      }
      if (targetType.value === 'workflow') {
        body.workflow_id = form.workflow_id as number
      } else if (targetType.value === 'code') {
        body.code_language = form.code_language
      } else {
        body.agent_id = form.agent_id as number
        body.session_target = form.session_target
      }
      if (form.channel_id != null && form.channel_id > 0) {
        body.channel_id = form.channel_id
      }
      await api.put(`/schedules/${editId.value}`, body)
      $q.notify({ type: 'positive', message: t('saveOk') })
    } else {
      await api.post('/schedules', req)
      $q.notify({ type: 'positive', message: t('createOk') })
    }
    dialogOpen.value = false
    await load()
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
  } finally {
    saving.value = false
  }
}

function confirmDelete (row: Schedule) {
  $q.dialog({
    title: t('confirmDelete'),
    message: `${row.name} (ID: ${row.id})`,
    cancel: { label: t('cancel'), flat: true },
    ok: { label: t('delete'), color: 'negative' }
  }).onOk(async () => {
    try {
      await api.delete(`/schedules/${row.id}`)
      $q.notify({ type: 'positive', message: t('deleteOk') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
    }
  })
}

async function toggleEnabled (row: Schedule) {
  try {
    await api.put(`/schedules/${row.id}`, { enabled: row.enabled })
    $q.notify({ type: 'positive', message: row.enabled ? t('toggleEnabledOn') : t('toggleDisabledOff') })
  } catch (e: unknown) {
    row.enabled = !row.enabled
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('updateFailed') })
  }
}

async function triggerSchedule (row: Schedule) {
  triggering.value = row.id
  try {
    await api.post(`/schedules/${row.id}/trigger`)
    $q.notify({ type: 'positive', message: t('scheduleTriggered') })
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('scheduleTriggerFailed') })
  } finally {
    triggering.value = 0
  }
}

function getScheduleKindColor (kind: string) {
  switch (kind) {
    case 'cron': return 'blue'
    case 'at': return 'green'
    case 'every': return 'orange'
    default: return 'grey'
  }
}

function getScheduleKindLabel (kind: string) {
  switch (kind) {
    case 'cron': return t('scheduleKindCronOpt')
    case 'at': return t('scheduleKindAt')
    case 'every': return t('scheduleKindEvery')
    default: return kind
  }
}

onMounted(async () => {
  await loadAgents()
  await loadWorkflows()
  await loadChannels()
  await load()
})
</script>
