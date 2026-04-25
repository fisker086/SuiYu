import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { MCPConfig, APIResponse, MCPTool } from 'src/api/types'

export const transportOptions = ['sse', 'streamable-http', 'stdio']

const defaultForm = {
  key: '',
  name: '',
  description: '',
  transport: 'sse',
  endpoint: '',
  usage_hint: '',
  config_json: '{}',
  is_active: true,
  tools_json: '[]'
}

export const healthBadgeColor = (status: string | undefined): string => {
  switch (status) {
    case 'ready':
      return 'positive'
    case 'unknown':
      return 'warning'
    case 'error':
    case 'unhealthy':
      return 'negative'
    default:
      return status ? 'info' : 'grey'
  }
}

export function useMCPPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const loading = ref(false)
  const syncingId = ref<number | null>(null)
  const saving = ref(false)
  const rows = ref<MCPConfig[]>([])
  const errorMsg = ref('')
  const columns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'key', label: t('key'), field: 'key', align: 'left' as const },
    { name: 'name', label: t('name'), field: 'name', align: 'left' as const },
    { name: 'transport', label: t('transport'), field: 'transport', align: 'left' as const },
    { name: 'endpoint', label: t('endpoint'), field: 'endpoint', align: 'left' as const },
    { name: 'health_status', label: t('healthStatus'), field: 'health_status', align: 'left' as const },
    { name: 'tool_count', label: t('toolCount'), field: 'tool_count', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])
  const dialogOpen = ref(false)
  const editingId = ref<number | null>(null)
  const form = reactive({ ...defaultForm })
  const discoveringTools = ref(false)
  const endpointPlaceholder = computed(() =>
    form.transport === 'stdio' ? t('mcpEndpointPlaceholderStdio') : t('mcpEndpointPlaceholderSse')
  )

  async function load () {
    loading.value = true
    try {
      const { data } = await api.get<APIResponse<MCPConfig[]>>('/mcp/configs')
      rows.value = (data.data ?? []) as MCPConfig[]
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  async function openDialog (mcp?: MCPConfig) {
    if (mcp) {
      editingId.value = mcp.id
      form.key = mcp.key
      form.name = mcp.name
      form.transport = mcp.transport
      form.endpoint = mcp.endpoint || ''
      const m = mcp as MCPConfig & { config?: Record<string, unknown> }
      let rawConfig: Record<string, unknown> = {}
      if (m.config && typeof m.config === 'object') {
        rawConfig = { ...m.config }
      } else {
        try {
          rawConfig = mcp.config_json ? JSON.parse(String(mcp.config_json)) as Record<string, unknown> : {}
        } catch {
          rawConfig = {}
        }
      }
      // 描述持久化在 config.description，与顶层 description 无关
      form.description = typeof rawConfig.description === 'string'
        ? rawConfig.description
        : (mcp.description || '')
      delete rawConfig.description
      form.config_json = JSON.stringify(rawConfig, null, 2)
      form.usage_hint = typeof rawConfig.usage_hint === 'string' ? rawConfig.usage_hint : ''
      form.is_active = mcp.is_active
      form.tools_json = '[]'
      dialogOpen.value = true
      try {
        const { data } = await api.get<APIResponse<MCPTool[]>>(`/mcp/configs/${mcp.id}/tools`)
        form.tools_json = JSON.stringify(data.data ?? [], null, 2)
      } catch {
        form.tools_json = '[]'
      }
    } else {
      editingId.value = null
      Object.assign(form, defaultForm)
      dialogOpen.value = true
    }
  }

  function resetForm () {
    editingId.value = null
    Object.assign(form, defaultForm)
  }

  async function saveMCP () {
    if (!form.key || !form.name) return

    let parsedConfig: Record<string, unknown> = {}
    let parsedTools: any[] = []
    try {
      parsedConfig = JSON.parse(form.config_json || '{}') as Record<string, unknown>
    } catch {
      $q.notify({ type: 'negative', message: t('jsonConfigInvalid') })
      return
    }
    const hint = (form.usage_hint || '').trim()
    if (hint) {
      parsedConfig.usage_hint = hint
    } else {
      delete parsedConfig.usage_hint
    }
    const desc = (form.description || '').trim()
    if (desc) {
      parsedConfig.description = desc
    } else {
      delete parsedConfig.description
    }
    try {
      parsedTools = JSON.parse(form.tools_json || '[]')
    } catch {
      $q.notify({ type: 'negative', message: t('jsonToolsInvalid') })
      return
    }

    saving.value = true
    try {
      const body = {
        key: form.key,
        name: form.name,
        transport: form.transport,
        endpoint: form.endpoint,
        config: parsedConfig as Record<string, unknown>
      }

      if (editingId.value) {
        await api.put(`/mcp/configs/${editingId.value}`, body)
        if (parsedTools.length > 0) {
          await api.post(`/mcp/configs/${editingId.value}/sync`, { tools: parsedTools, create_capabilities: true })
        }
        $q.notify({ type: 'positive', message: t('updateOk') })
      } else {
        const { data } = await api.post<APIResponse<MCPConfig>>('/mcp/configs', body)
        if (data.data?.id && parsedTools.length > 0) {
          await api.post(`/mcp/configs/${data.data.id}/sync`, { tools: parsedTools, create_capabilities: true })
        }
        $q.notify({ type: 'positive', message: t('createOk') })
      }

      dialogOpen.value = false
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      saving.value = false
    }
  }

  async function syncMCP (mcp: MCPConfig) {
    syncingId.value = mcp.id
    try {
      await api.post(`/mcp/configs/${mcp.id}/sync`, {})
      $q.notify({ type: 'positive', message: t('syncSuccess') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('syncFailed') })
    } finally {
      syncingId.value = null
    }
  }

  const confirmDelete = (cfg: MCPConfig) => {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${cfg.name} (ID: ${cfg.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/mcp/configs/${cfg.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await load()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  const discoverTools = async () => {
    if (!editingId.value) return
    discoveringTools.value = true
    try {
      await api.post(`/mcp/configs/${editingId.value}/sync`, {})
      const { data } = await api.get<{ data?: unknown[] }>(`/mcp/configs/${editingId.value}/tools`)
      form.tools_json = JSON.stringify(data.data ?? [], null, 2)
      $q.notify({ type: 'positive', message: t('toolsListFetched') })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('syncToolsFailed') })
    } finally {
      discoveringTools.value = false
    }
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    syncingId,
    saving,
    rows,
    errorMsg,
    columns,
    load,
    dialogOpen,
    form,
    editingId,
    openDialog,
    resetForm,
    saveMCP,
    syncMCP,
    confirmDelete,
    discoveringTools,
    discoverTools,
    endpointPlaceholder
  }
}
