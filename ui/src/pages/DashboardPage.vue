<template>
  <q-page class="dashboard-page" padding>
    <div class="text-h5 q-mb-md">{{ t('dashboard') }}</div>

    <div class="row q-gutter-md q-mb-md">
      <q-card class="col" flat bordered>
        <q-card-section>
          <div class="text-subtitle2 text-grey">{{ t('totalChats') }}</div>
          <div class="text-h4">{{ stats.total_chats }}</div>
        </q-card-section>
      </q-card>
      <q-card class="col" flat bordered>
        <q-card-section>
          <div class="text-subtitle2 text-grey">{{ t('totalSessions') }}</div>
          <div class="text-h4">{{ stats.total_sessions }}</div>
        </q-card-section>
      </q-card>
      <q-card class="col" flat bordered>
        <q-card-section>
          <div class="text-subtitle2 text-grey">{{ t('totalMessages') }}</div>
          <div class="text-h4">{{ stats.total_messages }}</div>
        </q-card-section>
      </q-card>
      <q-card class="col" flat bordered>
        <q-card-section>
          <div class="text-subtitle2 text-grey">{{ t('totalAgents') }}</div>
          <div class="text-h4">{{ stats.total_agents }}</div>
        </q-card-section>
      </q-card>
    </div>

    <div class="row q-gutter-md q-mb-md">
      <q-card class="col-12" flat bordered>
        <q-card-section>
          <div class="text-h6">{{ t('chatActivity') }}</div>
        </q-card-section>
        <q-separator />
        <q-card-section>
          <div v-if="activityData.length === 0" class="text-grey text-center q-pa-md">
            {{ t('noData') }}
          </div>
          <div v-else class="dashboard-activity-line">
            <svg
              class="dashboard-activity-svg"
              viewBox="0 0 1000 260"
              preserveAspectRatio="xMidYMid meet"
              xmlns="http://www.w3.org/2000/svg"
            >
              <defs>
                <linearGradient id="dashboard-activity-fill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="#1976d2" stop-opacity="0.22" />
                  <stop offset="100%" stop-color="#1976d2" stop-opacity="0.02" />
                </linearGradient>
              </defs>
              <polygon
                v-if="activityFillPoints"
                :points="activityFillPoints"
                fill="url(#dashboard-activity-fill)"
              />
              <polyline
                fill="none"
                stroke="#1976d2"
                stroke-width="3"
                stroke-linecap="round"
                stroke-linejoin="round"
                vector-effect="non-scaling-stroke"
                :points="activityLinePoints"
              />
              <circle
                v-for="(pt, idx) in activityLineDots"
                :key="'dot-' + idx"
                :cx="pt.cx"
                :cy="pt.cy"
                r="5"
                fill="#fff"
                stroke="#1976d2"
                stroke-width="2"
              />
            </svg>
            <div class="row q-col-gutter-xs dashboard-activity-labels">
              <div
                v-for="item in activityData"
                :key="'lbl-' + item.date"
                class="col text-caption text-center text-grey-7"
              >
                {{ item.date }}
              </div>
            </div>
          </div>
        </q-card-section>
      </q-card>
    </div>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-6">
        <q-card flat bordered class="full-height">
          <q-card-section class="q-pb-sm">
            <div class="text-h6">{{ t('recentChats') }}</div>
            <div class="text-caption text-grey-7 q-mt-xs">{{ t('recentChatsHint') }}</div>
          </q-card-section>
          <q-separator />
          <q-card-section class="q-pa-none">
            <q-scroll-area class="dashboard-recent-scroll">
              <q-list separator class="q-py-sm">
                <q-item
                  v-for="chat in recentChats"
                  :key="chat.session_id"
                  class="dashboard-recent-item"
                >
                  <q-item-section avatar class="cursor-pointer dashboard-recent-avatar" @click="goChat(chat)">
                    <q-icon name="history" color="primary" size="sm" />
                  </q-item-section>
                  <q-item-section class="cursor-pointer dashboard-recent-main" @click="goChat(chat)">
                    <q-item-label lines="1" class="ellipsis">
                      {{ displaySessionTitle(chat) }}
                    </q-item-label>
                    <q-item-label caption lines="2" class="ellipsis-2-lines">
                      {{ t('agents') }}: {{ chat.agent_name }} · {{ chat.updated_at }}
                    </q-item-label>
                  </q-item-section>
                  <q-item-section side class="dashboard-recent-side">
                    <div class="row items-center no-wrap">
                      <q-btn
                        flat
                        dense
                        round
                        size="sm"
                        icon="edit"
                        color="grey-7"
                        class="dashboard-recent-edit"
                        :aria-label="t('renameSession')"
                        @click.stop="promptRenameSession(chat)"
                      />
                      <q-btn
                        flat
                        dense
                        round
                        size="sm"
                        icon="delete"
                        color="grey-7"
                        class="dashboard-recent-delete"
                        :aria-label="t('delete')"
                        @click.stop="confirmDeleteSession(chat)"
                      />
                      <q-icon
                        name="chevron_right"
                        color="grey-5"
                        size="sm"
                        class="cursor-pointer"
                        @click="goChat(chat)"
                      />
                    </div>
                  </q-item-section>
                </q-item>
                <q-item v-if="recentChats.length === 0">
                  <q-item-section class="text-grey">{{ t('noData') }}</q-item-section>
                </q-item>
              </q-list>
            </q-scroll-area>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-md-6">
        <q-card flat bordered class="full-height">
          <q-card-section class="q-pb-sm">
            <div class="text-h6">{{ t('dashboardAccessibleAgents') }}</div>
            <div class="text-caption text-grey-7 q-mt-xs">{{ t('dashboardAccessibleAgentsHint') }}</div>
          </q-card-section>
          <q-separator />
          <q-card-section class="q-pa-none">
            <q-scroll-area class="dashboard-recent-scroll">
              <q-list separator class="q-py-sm">
                <q-item
                  v-for="a in allowedAgents"
                  :key="a.id"
                  clickable
                  v-ripple
                  @click="goChatAgent(a)"
                >
                  <q-item-section avatar>
                    <q-icon name="smart_toy" color="secondary" />
                  </q-item-section>
                  <q-item-section>
                    <q-item-label>{{ a.name }}</q-item-label>
                    <q-item-label v-if="a.description" caption lines="2">{{ a.description }}</q-item-label>
                  </q-item-section>
                  <q-item-section side>
                    <q-icon name="chevron_right" color="grey-5" />
                  </q-item-section>
                </q-item>
                <q-item v-if="allowedAgents.length === 0">
                  <q-item-section class="text-grey">{{ t('noData') }}</q-item-section>
                </q-item>
              </q-list>
            </q-scroll-area>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { LocalStorage, useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { Agent, APIResponse } from 'src/api/types'

defineOptions({
  name: 'DashboardPage'
})

const { t } = useI18n()
const router = useRouter()
const $q = useQuasar()

interface ChatStats {
  total_chats: number
  total_sessions: number
  total_messages: number
  total_agents: number
}

interface RecentChat {
  session_id: string
  agent_id: number
  /** Opaque id for /chat/:public_id (preferred over agent_id in links). */
  agent_public_id?: string
  agent_name: string
  updated_at: string
  title?: string
}

interface ActivityItem {
  date: string
  count: number
}

const stats = ref<ChatStats>({
  total_chats: 0,
  total_sessions: 0,
  total_messages: 0,
  total_agents: 0
})

const recentChats = ref<RecentChat[]>([])
const allowedAgents = ref<Agent[]>([])
const activityData = ref<ActivityItem[]>([])

const VB = { w: 1000, h: 260, padX: 40, padY: 28 }

const activityLineDots = computed(() => {
  const data = activityData.value
  if (data.length === 0) return [] as { cx: number; cy: number }[]
  const max = Math.max(1, ...data.map(d => d.count))
  const { w, h, padX, padY } = VB
  const innerW = w - 2 * padX
  const innerH = h - 2 * padY
  const n = data.length
  return data.map((d, i) => {
    const x = padX + (n === 1 ? innerW / 2 : (i / (n - 1)) * innerW)
    const y = padY + innerH - (d.count / max) * innerH
    return { cx: x, cy: y }
  })
})

const activityLinePoints = computed(() =>
  activityLineDots.value.map(p => `${p.cx},${p.cy}`).join(' ')
)

/** 折线下方填充多边形（闭合到底边） */
const activityFillPoints = computed(() => {
  const dots = activityLineDots.value
  if (dots.length === 0) return ''
  const { h, padY } = VB
  const innerH = h - 2 * padY
  const yBottom = padY + innerH
  const firstX = dots[0].cx
  const lastX = dots[dots.length - 1].cx
  const top = dots.map(p => `${p.cx},${p.cy}`).join(' ')
  return `${firstX},${yBottom} ${top} ${lastX},${yBottom}`
})

function shortSessionId (sessionId: string): string {
  const s = (sessionId || '').trim()
  if (s.length <= 10) return s || '—'
  return `${s.slice(0, 8)}…`
}

function displaySessionTitle (chat: RecentChat): string {
  const raw = (chat.title || '').trim()
  if (raw) return raw
  return `${t('dashboardSessionPrimary')} · ${shortSessionId(chat.session_id)}`
}

function goChat (chat: RecentChat) {
  const pub = (chat.agent_public_id || '').trim()
  if (pub !== '') {
    LocalStorage.set('lastAgentPublicId', pub)
    void router.push({ name: 'chat', params: { agentId: pub } })
    return
  }
  if (chat.agent_id >= 1) {
    LocalStorage.set('lastAgentId', String(chat.agent_id))
    void router.push({ name: 'chat', params: { agentId: String(chat.agent_id) } })
  }
}

function goChatAgent (a: Agent) {
  const pub = (a.public_id || '').trim()
  if (pub !== '') {
    LocalStorage.set('lastAgentPublicId', pub)
    void router.push({ name: 'chat', params: { agentId: pub } })
    return
  }
  if (a.id >= 1) {
    LocalStorage.set('lastAgentId', String(a.id))
    void router.push({ name: 'chat', params: { agentId: String(a.id) } })
  }
}

function promptRenameSession (chat: RecentChat) {
  $q.dialog({
    title: t('renameSession'),
    message: t('renameSessionHint'),
    prompt: {
      model: (chat.title || '').trim(),
      type: 'text',
      maxlength: 512,
      isValid: (val: string) => val.trim().length <= 512
    },
    cancel: true,
    persistent: true
  }).onOk((newTitle: string) => {
    void renameSession(chat, newTitle)
  })
}

async function renameSession (chat: RecentChat, title: string) {
  try {
    await api.put(`/chat/sessions/${encodeURIComponent(chat.session_id)}`, { title })
    const idx = recentChats.value.findIndex(c => c.session_id === chat.session_id)
    if (idx >= 0) {
      recentChats.value[idx] = { ...recentChats.value[idx], title }
    }
    $q.notify({ type: 'positive', message: t('saveSuccess') })
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
  }
}

function confirmDeleteSession (chat: RecentChat) {
  $q.dialog({
    title: t('confirmDelete'),
    message: `${chat.agent_name}\n${t('deleteChatSessionConfirm')}`,
    cancel: { label: t('cancel'), flat: true },
    ok: { label: t('delete'), color: 'negative' },
    persistent: true
  }).onOk(() => {
    void deleteSession(chat)
  })
}

async function deleteSession (chat: RecentChat) {
  try {
    await api.delete(`/chat/sessions/${encodeURIComponent(chat.session_id)}`)
    recentChats.value = recentChats.value.filter(c => c.session_id !== chat.session_id)
    $q.notify({ type: 'positive', message: t('deleteSuccess') })
    try {
      const { data } = await api.get<{ data: ChatStats }>('/chat/stats')
      stats.value = data.data || stats.value
    } catch {}
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
  }
}

async function loadDashboard () {
  try {
    const { data } = await api.get<{ data: ChatStats }>('/chat/stats')
    stats.value = data.data || stats.value
  } catch {}

  try {
    const { data } = await api.get<{ data: RecentChat[] }>('/chat/recent')
    recentChats.value = data.data || []
  } catch {}

  try {
    const { data } = await api.get<APIResponse<Agent[]>>('/agents')
    allowedAgents.value = (data.data ?? []) as Agent[]
  } catch {
    allowedAgents.value = []
  }

  try {
    const { data } = await api.get<{ data: ActivityItem[] }>('/chat/activity')
    activityData.value = data.data || []
  } catch {}
}

onMounted(() => {
  void loadDashboard()
})
</script>

<style scoped>
.dashboard-recent-main {
  min-width: 0;
}
.dashboard-recent-side {
  flex: 0 0 auto !important;
  max-width: none !important;
  padding-left: 4px;
}
.dashboard-recent-side .row {
  flex-wrap: nowrap;
}
.dashboard-recent-avatar {
  flex-shrink: 0;
}
.dashboard-recent-edit,
.dashboard-recent-delete {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.dashboard-recent-item:hover .dashboard-recent-edit,
.dashboard-recent-item:hover .dashboard-recent-delete,
.dashboard-recent-item:focus-within .dashboard-recent-edit,
.dashboard-recent-item:focus-within .dashboard-recent-delete {
  opacity: 1;
}
@media (hover: none) {
  .dashboard-recent-edit,
  .dashboard-recent-delete {
    opacity: 0.65;
  }
}
.ellipsis-2-lines {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
