import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse, MessageChannel, AgentMessage, A2ACard, Agent } from 'src/api/types'

const messageColumns = [
  { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
  { name: 'from_agent_name', label: '发送方', field: 'from_agent_name', align: 'left' as const },
  { name: 'to_agent_name', label: '接收方', field: 'to_agent_name', align: 'left' as const },
  { name: 'kind', label: '类型', field: 'kind', align: 'left' as const },
  { name: 'content', label: '内容', field: 'content', align: 'left' as const },
  { name: 'status', label: '状态', field: 'status', align: 'center' as const },
  { name: 'created_at', label: '时间', field: 'created_at', align: 'left' as const }
]

const defaultChannelForm = {
  name: '',
  agent_id: 0,
  kind: 'direct',
  description: '',
  is_public: false,
  is_active: true
}

const defaultMessageForm = {
  from_agent_id: 0,
  to_agent_id: 0,
  channel_id: 0,
  kind: 'text',
  content: '',
  priority: 0
}

const defaultA2AForm = {
  agent_id: 0,
  name: '',
  description: '',
  url: '',
  version: '1.0.0',
  is_active: true
}

export function useMessagesPage () {
  const { t } = useI18n()
  const $q = useQuasar()

  const channelColumns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'name', label: '名称', field: 'name', align: 'left' as const },
    { name: 'agent_name', label: '所属智能体', field: 'agent_name', align: 'left' as const },
    { name: 'kind', label: '类型', field: 'kind', align: 'left' as const },
    { name: 'is_public', label: '公开', field: 'is_public', align: 'center' as const },
    { name: 'is_active', label: '启用', field: 'is_active', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])

  const a2aColumns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'name', label: '名称', field: 'name', align: 'left' as const },
    { name: 'url', label: 'URL', field: 'url', align: 'left' as const },
    { name: 'version', label: '版本', field: 'version', align: 'center' as const },
    { name: 'is_active', label: '启用', field: 'is_active', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])

  const loading = ref(false)
  const saving = ref(false)
  const errorMsg = ref('')

  const activeTab = ref('channels')

  const channelRows = ref<MessageChannel[]>([])
  const messageRows = ref<AgentMessage[]>([])
  const a2aRows = ref<A2ACard[]>([])
  const agents = ref<Agent[]>([])

  const channelDialogOpen = ref(false)
  const channelEditingId = ref<number | null>(null)
  const channelForm = reactive({ ...defaultChannelForm })

  const messageDialogOpen = ref(false)
  const messageForm = reactive({ ...defaultMessageForm })

  const a2aDialogOpen = ref(false)
  const a2aEditingId = ref<number | null>(null)
  const a2aForm = reactive({ ...defaultA2AForm })

  const messageFilter = reactive({
    agent_id: 0,
    channel_id: 0,
    status: ''
  })

  const kindOptions = [
    { label: '直连 (direct)', value: 'direct' },
    { label: '广播 (broadcast)', value: 'broadcast' },
    { label: '主题 (topic)', value: 'topic' }
  ]

  const messageKindOptions = [
    { label: '文本 (text)', value: 'text' },
    { label: '命令 (command)', value: 'command' },
    { label: '事件 (event)', value: 'event' },
    { label: '结果 (result)', value: 'result' }
  ]

  const agentOptions = ref<{ label: string; value: number }[]>([])
  const channelOptions = ref<{ label: string; value: number }[]>([])

  async function loadAgents () {
    try {
      const { data } = await api.get<APIResponse<Agent[]>>('/agents')
      agents.value = (data.data ?? []) as Agent[]
      agentOptions.value = agents.value.map(a => ({ label: a.name, value: a.id }))
    } catch {
      agents.value = []
      agentOptions.value = []
    }
  }

  async function loadChannels () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<MessageChannel[]>>('/message-channels')
      channelRows.value = (data.data ?? []) as MessageChannel[]
      channelOptions.value = channelRows.value.map(ch => ({ label: `${ch.name} (${ch.kind})`, value: ch.id }))
    } catch (e: unknown) {
      channelRows.value = []
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? '加载通道失败'
    } finally {
      loading.value = false
    }
  }

  async function loadMessages () {
    loading.value = true
    errorMsg.value = ''
    try {
      const params: Record<string, string | number> = { limit: 100 }
      if (messageFilter.agent_id) params.agent_id = messageFilter.agent_id
      if (messageFilter.channel_id) params.channel_id = messageFilter.channel_id
      if (messageFilter.status) params.status = messageFilter.status

      const { data } = await api.get<APIResponse<{ messages: AgentMessage[]; total: number }>>('/messages', { params })
      const result = data.data
      messageRows.value = (result?.messages ?? []) as AgentMessage[]
    } catch (e: unknown) {
      messageRows.value = []
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? '加载消息失败'
    } finally {
      loading.value = false
    }
  }

  async function loadA2ACards () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<A2ACard[]>>('/a2a-cards')
      a2aRows.value = (data.data ?? []) as A2ACard[]
    } catch (e: unknown) {
      a2aRows.value = []
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? '加载 A2A 卡片失败'
    } finally {
      loading.value = false
    }
  }

  async function load () {
    await loadAgents()
    if (activeTab.value === 'channels') await loadChannels()
    else if (activeTab.value === 'messages') await loadMessages()
    else if (activeTab.value === 'a2a') await loadA2ACards()
  }

  function openChannelDialog (row?: MessageChannel) {
    if (row) {
      channelEditingId.value = row.id
      channelForm.name = row.name
      channelForm.agent_id = row.agent_id
      channelForm.kind = row.kind
      channelForm.description = row.description
      channelForm.is_public = row.is_public
      channelForm.is_active = row.is_active
    } else {
      channelEditingId.value = null
      Object.assign(channelForm, { ...defaultChannelForm })
    }
    channelDialogOpen.value = true
  }

  async function saveChannel () {
    if (!channelForm.name?.trim()) {
      $q.notify({ type: 'negative', message: '请填写通道名称' })
      return
    }
    if (!channelForm.agent_id) {
      $q.notify({ type: 'negative', message: '请选择所属智能体' })
      return
    }

    saving.value = true
    try {
      if (channelEditingId.value) {
        await api.put(`/message-channels/${channelEditingId.value}`, {
          name: channelForm.name.trim(),
          description: channelForm.description.trim(),
          is_public: channelForm.is_public,
          is_active: channelForm.is_active
        })
        $q.notify({ type: 'positive', message: t('saveOk') })
      } else {
        await api.post('/message-channels', {
          name: channelForm.name.trim(),
          agent_id: channelForm.agent_id,
          kind: channelForm.kind,
          description: channelForm.description.trim(),
          is_public: channelForm.is_public,
          is_active: channelForm.is_active
        })
        $q.notify({ type: 'positive', message: t('createOk') })
      }
      channelDialogOpen.value = false
      await loadChannels()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    } finally {
      saving.value = false
    }
  }

  function confirmDeleteChannel (row: MessageChannel) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${row.name} (ID: ${row.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/message-channels/${row.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await loadChannels()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  function openMessageDialog () {
    Object.assign(messageForm, { ...defaultMessageForm })
    messageDialogOpen.value = true
  }

  async function sendMessage () {
    if (!messageForm.from_agent_id) {
      $q.notify({ type: 'negative', message: '请选择发送方' })
      return
    }
    if (!messageForm.to_agent_id && !messageForm.channel_id) {
      $q.notify({ type: 'negative', message: '请选择接收方或通道' })
      return
    }
    if (!messageForm.content?.trim()) {
      $q.notify({ type: 'negative', message: '请填写消息内容' })
      return
    }

    saving.value = true
    try {
      const body: Record<string, unknown> = {
        from_agent_id: messageForm.from_agent_id,
        kind: messageForm.kind,
        content: messageForm.content.trim(),
        priority: messageForm.priority
      }
      if (messageForm.to_agent_id) body.to_agent_id = messageForm.to_agent_id
      if (messageForm.channel_id) body.channel_id = messageForm.channel_id

      await api.post('/messages/send', body)
      $q.notify({ type: 'positive', message: '消息已发送' })
      messageDialogOpen.value = false
      await loadMessages()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? '发送失败' })
    } finally {
      saving.value = false
    }
  }

  async function sendSpan () {
    if (!messageForm.from_agent_id) {
      $q.notify({ type: 'negative', message: '请选择发送方' })
      return
    }
    if (!messageForm.to_agent_id && !messageForm.channel_id) {
      $q.notify({ type: 'negative', message: '请选择接收方或通道' })
      return
    }
    if (!messageForm.content?.trim()) {
      $q.notify({ type: 'negative', message: '请填写消息内容' })
      return
    }

    saving.value = true
    try {
      const body: Record<string, unknown> = {
        from_agent_id: messageForm.from_agent_id,
        content: messageForm.content.trim()
      }
      if (messageForm.to_agent_id) body.to_agent_id = messageForm.to_agent_id
      if (messageForm.channel_id) body.channel_id = messageForm.channel_id

      await api.post('/messages/span', body)
      $q.notify({ type: 'positive', message: 'Span 消息已发送' })
      messageDialogOpen.value = false
      await loadMessages()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? '发送失败' })
    } finally {
      saving.value = false
    }
  }

  function openA2ADialog (row?: A2ACard) {
    if (row) {
      a2aEditingId.value = row.id
      a2aForm.agent_id = row.agent_id
      a2aForm.name = row.name
      a2aForm.description = row.description
      a2aForm.url = row.url
      a2aForm.version = row.version
      a2aForm.is_active = row.is_active
    } else {
      a2aEditingId.value = null
      Object.assign(a2aForm, { ...defaultA2AForm })
    }
    a2aDialogOpen.value = true
  }

  async function saveA2ACard () {
    if (!a2aForm.agent_id) {
      $q.notify({ type: 'negative', message: '请选择智能体' })
      return
    }
    if (!a2aForm.name?.trim()) {
      $q.notify({ type: 'negative', message: '请填写卡片名称' })
      return
    }

    saving.value = true
    try {
      if (a2aEditingId.value) {
        await api.put(`/a2a-cards/${a2aEditingId.value}`, {
          name: a2aForm.name.trim(),
          description: a2aForm.description.trim(),
          url: a2aForm.url.trim(),
          version: a2aForm.version.trim(),
          is_active: a2aForm.is_active
        })
        $q.notify({ type: 'positive', message: t('saveOk') })
      } else {
        await api.post('/a2a-cards', {
          agent_id: a2aForm.agent_id,
          name: a2aForm.name.trim(),
          description: a2aForm.description.trim(),
          url: a2aForm.url.trim(),
          version: a2aForm.version.trim(),
          is_active: a2aForm.is_active
        })
        $q.notify({ type: 'positive', message: t('createOk') })
      }
      a2aDialogOpen.value = false
      await loadA2ACards()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    } finally {
      saving.value = false
    }
  }

  function confirmDeleteA2ACard (row: A2ACard) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${row.name} (ID: ${row.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/a2a-cards/${row.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await loadA2ACards()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  function onTabChange (tab: string) {
    activeTab.value = tab
    void load()
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    saving,
    errorMsg,
    activeTab,
    channelRows,
    messageRows,
    a2aRows,
    agents,
    channelColumns,
    messageColumns,
    a2aColumns,
    channelDialogOpen,
    channelEditingId,
    channelForm,
    messageDialogOpen,
    messageForm,
    a2aDialogOpen,
    a2aEditingId,
    a2aForm,
    messageFilter,
    kindOptions,
    messageKindOptions,
    agentOptions,
    channelOptions,
    openChannelDialog,
    saveChannel,
    confirmDeleteChannel,
    openMessageDialog,
    sendMessage,
    sendSpan,
    openA2ADialog,
    saveA2ACard,
    confirmDeleteA2ACard,
    onTabChange,
    load,
    loadMessages
  }
}
