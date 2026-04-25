<template>
  <q-page class="usage-page" padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6">{{ t('usageStats') }}</div>
      <q-space />
      <q-btn-toggle
        v-model="period"
        flat
        toggle-color="primary"
        :options="[
          { label: t('day'), value: 'day' },
          { label: t('week'), value: 'week' },
          { label: t('month'), value: 'month' }
        ]"
        @update:model-value="loadUsage"
      />
      <q-btn flat round dense icon="refresh" @click="loadUsage" :loading="loading" class="q-ml-md">
        <q-tooltip>刷新</q-tooltip>
      </q-btn>
    </div>

    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner size="lg" />
    </div>

    <div v-else-if="stats.length === 0" class="text-grey text-center q-pa-xl">
      {{ t('noData') }}
    </div>

    <div v-else>
      <div class="row q-gutter-md q-mb-md">
        <q-card class="col" flat bordered>
          <q-card-section>
            <div class="text-subtitle2 text-grey">{{ t('totalTokens') }}</div>
            <div class="text-h4">{{ formatNumber(totalTokens) }}</div>
          </q-card-section>
        </q-card>
        <q-card class="col" flat bordered>
          <q-card-section>
            <div class="text-subtitle2 text-grey">{{ t('totalCost') }}</div>
            <div class="text-h4">${{ totalCost.toFixed(4) }}</div>
          </q-card-section>
        </q-card>
        <q-card class="col" flat bordered>
          <q-card-section>
            <div class="text-subtitle2 text-grey">{{ t('totalUsers') }}</div>
            <div class="text-h4">{{ userCount }}</div>
          </q-card-section>
        </q-card>
      </div>

      <q-table
        :rows="stats"
        :columns="columns"
        row-key="id"
        flat
        bordered
        :pagination="{ rowsPerPage: 20 }"
      >
        <template #body-cell-date="props">
          <q-td :props="props">
            {{ props.row.date }}
          </q-td>
        </template>
        <template #body-cell-user_name="props">
          <q-td :props="props">
            {{ props.row.user_name || '—' }}
          </q-td>
        </template>
        <template #body-cell-agent_name="props">
          <q-td :props="props">
            {{ props.row.agent_name || '-' }}
          </q-td>
        </template>
        <template #body-cell-prompt_tokens="props">
          <q-td :props="props">
            {{ formatNumber(props.row.prompt_tokens) }}
          </q-td>
        </template>
        <template #body-cell-completion_tokens="props">
          <q-td :props="props">
            {{ formatNumber(props.row.completion_tokens) }}
          </q-td>
        </template>
        <template #body-cell-total_tokens="props">
          <q-td :props="props">
            {{ formatNumber(props.row.total_tokens) }}
          </q-td>
        </template>
        <template #body-cell-cost="props">
          <q-td :props="props">
            ${{ props.row.cost.toFixed(4) }}
          </q-td>
        </template>
      </q-table>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'

const { t } = useI18n()

interface UsageStat {
  id: number
  date: string
  user_id: string
  user_name: string
  agent_id: number
  agent_name: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost: number
}

interface UsageResponse {
  data: {
    stats: UsageStat[]
    summary: Record<string, {
      total_tokens: number
      total_cost: number
    }>
  }
}

const loading = ref(false)
const period = ref<'day' | 'week' | 'month'>('day')
const stats = ref<UsageStat[]>([])

const columns: { name: string; label: string; field: string | ((row: UsageStat) => unknown); align: 'left' | 'center' | 'right'; sortable?: boolean }[] = [
  { name: 'date', label: '日期', field: 'date', align: 'left', sortable: true },
  { name: 'user_name', label: '用户', field: 'user_name', align: 'left' },
  { name: 'agent_name', label: '智能体', field: 'agent_name', align: 'left' },
  { name: 'model', label: '模型', field: 'model', align: 'left' },
  { name: 'prompt_tokens', label: 'Prompt Tokens', field: 'prompt_tokens', align: 'right' },
  { name: 'completion_tokens', label: 'Completion Tokens', field: 'completion_tokens', align: 'right' },
  { name: 'total_tokens', label: 'Total Tokens', field: 'total_tokens', align: 'right' },
  { name: 'cost', label: '费用', field: 'cost', align: 'right' }
]

const totalTokens = computed(() => {
  return stats.value.reduce((sum, s) => sum + s.total_tokens, 0)
})

const totalCost = computed(() => {
  return stats.value.reduce((sum, s) => sum + s.cost, 0)
})

const userCount = computed(() => {
  const users = new Set(stats.value.map(s => s.user_id))
  return users.size
})

function formatNumber (num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(2) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(2) + 'K'
  }
  return num.toString()
}

async function loadUsage () {
  loading.value = true
  try {
    const { data } = await api.get<UsageResponse>('/usage', {
      params: { period: period.value }
    })
    stats.value = data.data?.stats || []
  } catch (e) {
    console.error('Failed to load usage:', e)
    stats.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadUsage()
})
</script>

<style scoped>
.usage-page {
  min-height: 100vh;
}
</style>
