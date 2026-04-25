<template>
  <q-page padding class="approvals-page">
    <div class="q-mb-md">
      <div class="row items-start no-wrap">
        <div class="col">
          <div class="text-h5 text-weight-medium text-text2">{{ t('approvals') }}</div>
          <div class="text-caption text-grey-7 q-mt-xs">{{ t('approvalsSubtitle') }}</div>
        </div>
        <q-btn
          flat
          round
          dense
          color="primary"
          icon="refresh"
          :loading="loading"
          @click="load"
        >
          <q-tooltip>{{ t('refresh') }}</q-tooltip>
        </q-btn>
      </div>
    </div>

    <q-banner dense rounded class="bg-grey-2 text-grey-9 q-mb-md">
      <template #avatar>
        <q-icon name="verified_user" color="primary" size="sm" />
      </template>
      {{ t('approvalsBannerHint') }}
    </q-banner>

    <q-card flat bordered class="radius-sm overflow-hidden">
      <q-tabs
        v-model="activeTab"
        dense
        align="justify"
        class="approvals-tabs bg-grey-2 text-grey-8"
        active-color="primary"
        indicator-color="primary"
        narrow-indicator
        no-caps
        inline-label
        @update:model-value="onTabChange"
      >
        <q-tab name="pending" icon="schedule" :label="t('approvalTabPending')" />
        <q-tab name="approved" icon="check_circle" :label="t('approvalTabApproved')" />
        <q-tab name="rejected" icon="cancel" :label="t('approvalTabRejected')" />
        <q-tab name="all" icon="view_list" :label="t('approvalTabAll')" />
      </q-tabs>

      <q-separator />

      <q-table
        flat
        dense
        bordered
        separator="horizontal"
        row-key="id"
        class="approvals-table"
        table-header-class="bg-grey-2 text-grey-8 text-weight-medium"
        table-class="bg-white"
        :rows="rows"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        :rows-per-page-options="[10, 25, 50, 100]"
        @request="onRequest"
      >
        <template #no-data>
          <div class="full-width column flex-center q-gutter-sm q-pa-xl text-grey-6">
            <q-icon name="inbox" size="48px" color="grey-4" />
            <span class="text-body2">{{ emptyLabel }}</span>
          </div>
        </template>

        <template #body-cell-risk_level="props">
          <q-td :props="props">
            <q-badge :color="getRiskColor(props.row.risk_level)" :label="props.row.risk_level" />
          </q-td>
        </template>
        <template #body-cell-status="props">
          <q-td :props="props">
            <q-badge :color="getStatusColor(props.row.status)" :label="props.row.status" />
          </q-td>
        </template>
        <template #body-cell-input="props">
          <q-td :props="props">
            <div class="ellipsis-2-lines text-body2" style="max-width: 280px;">
              {{ props.row.input || '—' }}
            </div>
          </q-td>
        </template>
        <template #body-cell-actions="props">
          <q-td :props="props">
            <div class="row q-gutter-xs no-wrap justify-end items-center approval-row-actions">
              <template v-if="props.row.status === 'pending'">
                <q-btn
                  unelevated
                  no-caps
                  dense
                  size="sm"
                  color="positive"
                  padding="xs sm"
                  class="approval-inline-btn"
                  :label="t('approve')"
                  @click="openApproveDialog(props.row)"
                />
                <q-btn
                  unelevated
                  no-caps
                  dense
                  size="sm"
                  outline
                  color="negative"
                  padding="xs sm"
                  class="approval-inline-btn"
                  :label="t('reject')"
                  @click="openRejectDialog(props.row)"
                />
              </template>
              <q-btn
                v-else
                flat
                no-caps
                dense
                size="sm"
                color="primary"
                :label="t('detail')"
                @click="openDetail(props.row)"
              />
            </div>
          </q-td>
        </template>
      </q-table>
    </q-card>

    <!-- Approve -->
    <q-dialog v-model="approveOpen">
      <q-card flat bordered class="radius-sm" style="min-width: 420px; max-width: 92vw">
        <q-toolbar class="bg-grey-2 text-dark">
          <q-icon name="task_alt" color="positive" size="sm" class="q-mr-sm" />
          <q-toolbar-title class="text-subtitle1 text-weight-medium">
            {{ t('approveRequest') }}
          </q-toolbar-title>
          <q-btn flat round dense icon="close" v-close-popup />
        </q-toolbar>
        <q-separator />
        <q-card-section class="q-pt-md">
          <div class="row q-col-gutter-sm q-mb-md">
            <div class="col-12 col-sm-6">
              <div class="text-caption text-grey-7">{{ t('approvalColTool') }}</div>
              <div class="text-body2 text-weight-medium">{{ targetRow?.tool_name }}</div>
            </div>
            <div class="col-12 col-sm-6">
              <div class="text-caption text-grey-7">{{ t('approvalColRisk') }}</div>
              <q-badge :color="getRiskColor(targetRow?.risk_level)" :label="targetRow?.risk_level" />
            </div>
          </div>
          <div class="text-caption text-grey-7 q-mb-xs">{{ t('approvalColInput') }}</div>
          <q-input
            :model-value="targetRow?.input"
            type="textarea"
            outlined
            dense
            readonly
            autogrow
            class="font-mono text-body2"
            input-class="text-body2"
          />
          <q-input
            v-model="approveComment"
            class="q-mt-md"
            outlined
            dense
            :label="t('comment')"
            type="textarea"
            rows="2"
          />
        </q-card-section>
        <q-separator />
        <q-card-actions align="right" class="q-px-md q-pb-md">
          <q-btn flat no-caps :label="t('cancel')" v-close-popup />
          <q-btn unelevated no-caps color="positive" :label="t('approve')" icon="check" @click="doApprove" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Reject -->
    <q-dialog v-model="rejectOpen">
      <q-card flat bordered class="radius-sm" style="min-width: 420px; max-width: 92vw">
        <q-toolbar class="bg-grey-2 text-dark">
          <q-icon name="block" color="negative" size="sm" class="q-mr-sm" />
          <q-toolbar-title class="text-subtitle1 text-weight-medium">
            {{ t('rejectRequest') }}
          </q-toolbar-title>
          <q-btn flat round dense icon="close" v-close-popup />
        </q-toolbar>
        <q-separator />
        <q-card-section class="q-pt-md">
          <div class="row q-col-gutter-sm q-mb-md">
            <div class="col-12 col-sm-6">
              <div class="text-caption text-grey-7">{{ t('approvalColTool') }}</div>
              <div class="text-body2 text-weight-medium">{{ targetRow?.tool_name }}</div>
            </div>
            <div class="col-12 col-sm-6">
              <div class="text-caption text-grey-7">{{ t('approvalColRisk') }}</div>
              <q-badge :color="getRiskColor(targetRow?.risk_level)" :label="targetRow?.risk_level" />
            </div>
          </div>
          <q-input
            v-model="rejectComment"
            outlined
            dense
            :label="t('rejectReason')"
            type="textarea"
            rows="3"
          />
        </q-card-section>
        <q-separator />
        <q-card-actions align="right" class="q-px-md q-pb-md">
          <q-btn flat no-caps :label="t('cancel')" v-close-popup />
          <q-btn unelevated no-caps color="negative" :label="t('reject')" icon="close" @click="doReject" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Detail -->
    <q-dialog v-model="detailOpen">
      <q-card flat bordered class="radius-sm" style="min-width: 520px; max-width: 92vw">
        <q-toolbar class="bg-grey-2 text-dark">
          <q-icon name="info" color="primary" size="sm" class="q-mr-sm" />
          <q-toolbar-title class="text-subtitle1 text-weight-medium">
            {{ t('approvalDetail') }}
          </q-toolbar-title>
          <q-btn flat round dense icon="close" v-close-popup />
        </q-toolbar>
        <q-separator />
        <q-card-section class="q-pt-sm">
          <q-list bordered separator class="rounded-borders">
            <q-item>
              <q-item-section>
                <q-item-label caption>{{ t('approvalColTool') }}</q-item-label>
                <q-item-label>{{ detailRow?.tool_name }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section>
                <q-item-label caption>{{ t('approvalColRisk') }}</q-item-label>
                <q-item-label>
                  <q-badge :color="getRiskColor(detailRow?.risk_level)" :label="detailRow?.risk_level" />
                </q-item-label>
              </q-item-section>
              <q-item-section side top>
                <q-item-label caption>{{ t('approvalColStatus') }}</q-item-label>
                <q-item-label>
                  <q-badge :color="getStatusColor(detailRow?.status)" :label="detailRow?.status" />
                </q-item-label>
              </q-item-section>
            </q-item>
            <q-item>
              <q-item-section>
                <q-item-label caption>Agent ID</q-item-label>
                <q-item-label>{{ detailRow?.agent_id }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item align="top">
              <q-item-section>
                <q-item-label caption>{{ t('approvalColInput') }}</q-item-label>
                <q-item-label class="text-body2" style="white-space: pre-wrap; word-break: break-word;">
                  {{ detailRow?.input }}
                </q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="detailRow?.comment">
              <q-item-section>
                <q-item-label caption>{{ t('comment') }}</q-item-label>
                <q-item-label>{{ detailRow.comment }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="detailRow?.approver_id">
              <q-item-section>
                <q-item-label caption>Approver</q-item-label>
                <q-item-label>{{ detailRow.approver_id }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item v-if="detailRow?.approved_at">
              <q-item-section>
                <q-item-label caption>Time</q-item-label>
                <q-item-label>{{ formatApprovalTime(detailRow.approved_at) }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
        <q-separator />
        <q-card-actions align="right" class="q-px-md q-pb-md">
          <q-btn flat no-caps color="primary" :label="t('close')" v-close-popup />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

defineOptions({ name: 'ApprovalsPage' })

type ApprovalTab = 'pending' | 'approved' | 'rejected' | 'all'

const { t, locale } = useI18n()

/** Short locale date/time for table/detail (not raw RFC3339). */
function formatApprovalTime (iso: string | undefined | null): string {
  if (iso == null || iso === '') return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return String(iso)
  const loc = locale.value?.toString().startsWith('zh') ? 'zh-CN' : 'en-US'
  return new Intl.DateTimeFormat(loc, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).format(d)
}

const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const pagination = reactive({
  page: 1,
  rowsPerPage: 50,
  rowsNumber: 0
})
const activeTab = ref<ApprovalTab>('pending')

const emptyLabel = computed(() => {
  switch (activeTab.value) {
    case 'pending': return t('approvalEmptyPending')
    case 'approved': return t('approvalEmptyApproved')
    case 'rejected': return t('approvalEmptyRejected')
    default: return t('approvalEmptyAll')
  }
})

const columns = [
  { name: 'id', label: 'ID', field: 'id', align: 'left' as const, sortable: false },
  { name: 'tool_name', label: 'Tool', field: 'tool_name', align: 'left' as const, sortable: false },
  { name: 'risk_level', label: 'Risk', field: 'risk_level', align: 'center' as const, sortable: false },
  { name: 'status', label: 'Status', field: 'status', align: 'center' as const, sortable: false },
  { name: 'input', label: 'Input', field: 'input', align: 'left' as const, sortable: false },
  {
    name: 'created_at',
    label: 'Time',
    field: 'created_at',
    align: 'left' as const,
    sortable: false,
    format: (val: unknown) => formatApprovalTime(val as string)
  },
  {
    name: 'actions',
    label: t('actions'),
    field: 'actions',
    align: 'right' as const,
    sortable: false,
    classes: 'approval-actions-col',
    headerClasses: 'approval-actions-col'
  }
]

const approveOpen = ref(false)
const rejectOpen = ref(false)
const detailOpen = ref(false)
const targetRow = ref<any>(null)
const detailRow = ref<any>(null)
const approveComment = ref('')
const rejectComment = ref('')

function onTabChange () {
  pagination.page = 1
  void load()
}

async function load () {
  loading.value = true
  try {
    const params: Record<string, string | number> = {
      page: pagination.page,
      page_size: pagination.rowsPerPage
    }
    const status = activeTab.value === 'all' ? '' : activeTab.value
    if (status) params.status = status

    const { data } = await api.get<APIResponse<any>>('/approvals', { params })
    const result = data.data
    rows.value = result?.requests ?? []
    total.value = result?.total ?? 0
    pagination.rowsNumber = total.value
  } catch {
    rows.value = []
    total.value = 0
    pagination.rowsNumber = 0
  } finally {
    loading.value = false
  }
}

function onRequest (props: { pagination: { page: number; rowsPerPage: number } }) {
  pagination.page = props.pagination.page
  pagination.rowsPerPage = props.pagination.rowsPerPage
  void load()
}

function openApproveDialog (row: any) {
  targetRow.value = row
  approveComment.value = ''
  approveOpen.value = true
}

function openRejectDialog (row: any) {
  targetRow.value = row
  rejectComment.value = ''
  rejectOpen.value = true
}

function openDetail (row: any) {
  detailRow.value = row
  detailOpen.value = true
}

async function doApprove () {
  try {
    await api.post(`/approvals/${targetRow.value.id}/approve`, null, { params: { comment: approveComment.value } })
    approveOpen.value = false
    void load()
  } catch (e) {
    console.error(e)
  }
}

async function doReject () {
  try {
    await api.post(`/approvals/${targetRow.value.id}/reject`, null, { params: { comment: rejectComment.value } })
    rejectOpen.value = false
    void load()
  } catch (e) {
    console.error(e)
  }
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

function getStatusColor (status: string) {
  switch (status) {
    case 'pending': return 'warning'
    case 'approved': return 'positive'
    case 'rejected': return 'negative'
    default: return 'grey'
  }
}

onMounted(() => {
  void load()
})
</script>

<style scoped>
.approvals-tabs :deep(.q-tab) {
  min-height: 44px;
}
.approvals-table :deep(.q-table__bottom) {
  border-top: 1px solid rgba(0, 0, 0, 0.08);
}
.ellipsis-2-lines {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* Narrow table cell + q-btn(icon+label) causes label to spill; text-only + min width fixes it */
.approvals-table :deep(th.approval-actions-col),
.approvals-table :deep(td.approval-actions-col) {
  min-width: 168px;
  width: 168px;
  vertical-align: middle;
}
.approvals-table :deep(.approval-inline-btn) {
  min-width: 4.25rem;
}
.approvals-table :deep(.approval-inline-btn .q-btn__content) {
  line-height: 1.25;
}
</style>
