<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('auditLogs') }}</div>
      <q-space />
      <q-btn flat icon="refresh" round dense @click="load" :loading="loading" />
    </div>

    <div class="row q-col-gutter-sm q-mb-md">
      <div class="col-12 col-sm-3">
        <q-select v-model="filterTool" :options="toolOptions" outlined dense :label="t('filterByTool')" emit-value map-options clearable @update:model-value="load" />
      </div>
      <div class="col-12 col-sm-3">
        <q-select v-model="filterRisk" :options="riskOptions" outlined dense :label="t('filterByRisk')" emit-value map-options clearable @update:model-value="load" />
      </div>
      <div class="col-12 col-sm-3">
        <q-select v-model="filterStatus" :options="statusOptions" outlined dense :label="t('filterByStatus')" emit-value map-options clearable @update:model-value="load" />
      </div>
      <div class="col-12 col-sm-3">
        <q-input v-model="filterSessionId" outlined dense :label="t('filterBySession')" clearable @clear="load" @keydown.enter="load" />
      </div>
    </div>

    <q-table flat bordered :rows="rows" :columns="columns" row-key="id" :loading="loading" :pagination="pagination" @request="onRequest" no-data-label="暂无数据">
      <template #body-cell-username="props">
        <q-td :props="props">
          <span class="text-body2">{{ props.row.username || '—' }}</span>
        </q-td>
      </template>
      <template #body-cell-risk_level="props">
        <q-td :props="props">
          <q-badge :color="getRiskColor(props.row.risk_level)" :label="getRiskLabel(props.row.risk_level)" />
        </q-td>
      </template>
      <template #body-cell-status="props">
        <q-td :props="props">
          <q-badge :color="props.row.status === 'success' ? 'positive' : 'negative'" :label="props.row.status" />
        </q-td>
      </template>
      <template #body-cell-input="props">
        <q-td :props="props">
          <div class="text-truncate" style="max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
            {{ props.row.input || '-' }}
          </div>
        </q-td>
      </template>
      <template #body-cell-created_at="props">
        <q-td :props="props">
          <span class="text-body2">{{ formatShortTime(props.row.created_at) }}</span>
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="primary" :label="t('detail')" @click="openDetail(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="detailOpen">
      <q-card style="min-width: 600px; max-width: 90vw;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ t('auditLogDetail') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none" style="max-height: 70vh; overflow-y: auto;">
          <div class="row q-col-gutter-md">
            <div class="col-6">
              <div class="text-caption text-grey">Tool</div>
              <div class="text-body1">{{ detailRow?.tool_name }}</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">Risk Level</div>
              <q-badge :color="getRiskColor(detailRow?.risk_level)" :label="getRiskLabel(detailRow?.risk_level)" />
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">Status</div>
              <div class="text-body1">{{ detailRow?.status }}</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">Duration</div>
              <div class="text-body1">{{ detailRow?.duration_ms }}ms</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">{{ t('auditColAgent') }}</div>
              <div class="text-body1">{{ agentDisplayPrimary }}</div>
              <div v-if="agentDisplayHint" class="text-caption text-grey q-mt-xs">{{ agentDisplayHint }}</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">Session ID</div>
              <div class="text-body1">{{ detailRow?.session_id || '-' }}</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">{{ t('auditColUsername') }}</div>
              <div class="text-body1">{{ detailRow?.username || '—' }}</div>
            </div>
            <div class="col-6">
              <div class="text-caption text-grey">{{ t('auditColIp') }}</div>
              <div class="text-body1">{{ detailRow?.ip_address || '—' }}</div>
            </div>
            <div class="col-12">
              <div class="text-caption text-grey">Action</div>
              <div class="text-body1">{{ detailRow?.action }}</div>
            </div>
            <div class="col-12">
              <div class="text-caption text-grey">Input</div>
              <q-input v-model="detailRow!.input" outlined dense readonly type="textarea" rows="3" class="font-mono" />
            </div>
            <div class="col-12">
              <div class="text-caption text-grey">Output</div>
              <q-input v-model="detailRow!.output" outlined dense readonly type="textarea" rows="5" class="font-mono" />
            </div>
            <div class="col-12" v-if="detailRow?.error">
              <div class="text-caption text-negative">Error</div>
              <q-input v-model="detailRow!.error" outlined dense readonly type="textarea" rows="3" class="font-mono" color="negative" />
            </div>
            <div class="col-12">
              <div class="text-caption text-grey">Time</div>
              <div class="text-body2">{{ formatShortTime(detailRow?.created_at) }}</div>
            </div>
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('close')" v-close-popup />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { date } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

defineOptions({ name: 'AuditLogsPage' })

const { t } = useI18n()

const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ page: 1, rowsPerPage: 10 })

const filterTool = ref('')
const filterRisk = ref('')
const filterStatus = ref('')
const filterSessionId = ref('')

const toolOptions = [
  { label: '全部', value: '' }
]
const riskOptions = [
  { label: 'Low', value: 'low' },
  { label: 'Medium', value: 'medium' },
  { label: 'High', value: 'high' },
  { label: 'Critical', value: 'critical' }
]
const statusOptions = [
  { label: 'Success', value: 'success' },
  { label: 'Failed', value: 'failed' },
  { label: 'Blocked', value: 'blocked' },
  { label: 'Pending', value: 'pending' }
]

const columns = [
  { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
  { name: 'username', label: t('auditColUsername'), field: 'username', align: 'left' as const },
  { name: 'tool_name', label: 'Tool', field: 'tool_name', align: 'left' as const },
  { name: 'action', label: 'Action', field: 'action', align: 'left' as const },
  { name: 'risk_level', label: 'Risk', field: 'risk_level', align: 'center' as const },
  { name: 'status', label: 'Status', field: 'status', align: 'center' as const },
  { name: 'input', label: 'Input', field: 'input', align: 'left' as const },
  { name: 'duration_ms', label: 'Duration', field: 'duration_ms', align: 'center' as const },
  { name: 'created_at', label: 'Time', field: 'created_at', align: 'left' as const },
  { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
]

const detailOpen = ref(false)
const detailRow = ref<any>(null)

const agentDisplayPrimary = computed(() => {
  const row = detailRow.value
  if (row == null) return '—'
  const name = typeof row.agent_name === 'string' ? row.agent_name.trim() : ''
  if (name !== '') return name
  const aid = row.agent_id
  if (aid != null && Number(aid) > 0) return String(aid)
  return '—'
})

const agentDisplayHint = computed(() => {
  const row = detailRow.value
  if (row == null) return ''
  const name = typeof row.agent_name === 'string' ? row.agent_name.trim() : ''
  const aid = row.agent_id
  if (name === '' || aid == null || !(Number(aid) > 0)) return ''
  return t('auditAgentIdHint', { id: aid })
})

async function load () {
  loading.value = true
  try {
    const params: Record<string, string | number> = {
      page: pagination.page,
      page_size: pagination.rowsPerPage
    }
    if (filterTool.value) params.tool_name = filterTool.value
    if (filterRisk.value) params.risk_level = filterRisk.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterSessionId.value) params.session_id = filterSessionId.value

    const { data } = await api.get<APIResponse<any>>('/audit', { params })
    const result = data.data
    rows.value = result?.logs ?? []
    total.value = result?.total ?? 0
  } catch {
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onRequest (props: { pagination: { page: number; rowsPerPage: number } }) {
  pagination.page = props.pagination.page
  pagination.rowsPerPage = props.pagination.rowsPerPage
  load()
}

function openDetail (row: any) {
  detailRow.value = row
  detailOpen.value = true
}

function getRiskColor (level: string) {
  switch (level) {
    case 'low': return 'green'
    case 'medium': return 'orange'
    case 'high': return 'deep-orange'
    case 'critical': return 'red'
    default: return 'grey'
  }
}

function getRiskLabel (level: string) {
  switch (level) {
    case 'low': return 'Low'
    case 'medium': return 'Medium'
    case 'high': return 'High'
    case 'critical': return 'Critical'
    default: return level
  }
}

function formatShortTime (v: string | undefined) {
  if (v == null || v === '') return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return date.formatDate(d, 'YYYY-MM-DD HH:mm')
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.font-mono {
  font-family: monospace;
}
</style>
