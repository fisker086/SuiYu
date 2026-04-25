import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { LocalStorage, useQuasar } from 'quasar'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'
import type { Agent, APIResponse, CreateAgentRequest, UpdateAgentRequest, Skill, MCPConfig } from 'src/api/types'

export interface UserOption {
  id: number
  username: string
  email?: string
  full_name?: string
}

export function useAgentsPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const router = useRouter()
  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<Agent[]>([])
  const errorMsg = ref('')
  /** 客户端分页；Quasar 默认 rowsPerPage 为 5，显式设为 10 */
  const pagination = ref({
    sortBy: 'id' as string,
    descending: false,
    page: 1,
    rowsPerPage: 10
  })

  const columns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'name', label: t('name'), field: 'name', align: 'left' as const },
    { name: 'category', label: t('agentCategory'), field: 'category', align: 'left' as const },
    { name: 'description', label: t('description'), field: 'description', align: 'left' as const },
    { name: 'skill_count', label: t('colSkillCount'), field: (row: Agent) => row.skill_ids?.length ?? 0, align: 'center' as const },
    { name: 'mcp_count', label: t('colMcpConfigCount'), field: (row: Agent) => row.mcp_config_ids?.length ?? 0, align: 'center' as const },
    {
      name: 'is_active',
      label: t('status'),
      field: (row: Agent) => (row.is_active ? t('roleEnabled') : t('roleDisabled')),
      align: 'center' as const
    },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])

  const dialogOpen = ref(false)
  const isEdit = ref(false)
  const editId = ref(0)
  const form = reactive({
    name: '',
    category: '',
    description: '',
    is_active: true
  })

  const runtimeProfile = reactive({
    llm_model: '',
    temperature: 0.7,
    system_prompt: '',
    stream_enabled: true,
    memory_enabled: false,
    skill_ids: [] as string[],
    mcp_config_ids: [] as number[],
    execution_mode: 'single-call',
    max_iterations: 16,
    plan_prompt: '',
    reflection_depth: 0,
    approval_mode: 'auto',
    approvers: [] as string[],
    im_enabled: 'disabled',
    im_config: {
      webhook_url: '',
      secret: '',
      bot_name: '',
      app_id: '',
      app_secret: '',
      verification_token: '',
      encrypt_key: '',
      lark_open_domain: '',
      ws_enabled: false,
      telegram_token: '',
      telegram_chat_id: '',
      auto_reply: false,
      notify_on_approval: true
    }
  })

  const availableSkills = ref<Skill[]>([])
  const availableMcpConfigs = ref<MCPConfig[]>([])
  const availableUsers = ref<UserOption[]>([])
  const selectedSkillIds = ref<string[]>([])
  const selectedMcpConfigIds = ref<number[]>([])

  async function load () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<Agent[]>>('/agents')
      rows.value = (data.data ?? []) as Agent[]
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('loadFailed')
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  async function loadAvailableOptions () {
    try {
      const [skillsRes, mcpRes, usersRes] = await Promise.all([
        api.get<APIResponse<Skill[]>>('/skills'),
        api.get<APIResponse<MCPConfig[]>>('/mcp/configs'),
        api.get<APIResponse<{ list: UserOption[] }>>('/auth/users')
      ])
      availableSkills.value = (skillsRes.data.data ?? []) as Skill[]
      availableMcpConfigs.value = (mcpRes.data.data ?? []) as MCPConfig[]
      const usersData = usersRes.data.data as { list?: UserOption[] }
      availableUsers.value = usersData?.list ?? []
    } catch {
      availableSkills.value = []
      availableMcpConfigs.value = []
      availableUsers.value = []
    }
  }

  /** 编辑前清空运行时表单，避免与上次弹窗或异步加载竞态（否则后到的 load 会覆盖用户刚粘贴的 system prompt） */
  function resetRuntimeProfileDefaults () {
    Object.assign(runtimeProfile, {
      llm_model: '',
      temperature: 0.7,
      system_prompt: '',
      stream_enabled: true,
      memory_enabled: false,
      skill_ids: [] as string[],
      mcp_config_ids: [] as number[],
      execution_mode: 'single-call',
      max_iterations: 16,
      plan_prompt: '',
      reflection_depth: 0,
      approval_mode: 'auto',
      approvers: [] as string[]
    })
  }

  async function openDialog (agent?: Agent) {
    void loadAvailableOptions()
    if (agent) {
      isEdit.value = true
      editId.value = agent.id
      form.name = agent.name
      form.category = agent.category
      form.description = agent.description
      form.is_active = agent.is_active !== false

      selectedSkillIds.value = agent.skill_ids ?? []
      selectedMcpConfigIds.value = agent.mcp_config_ids ?? []

      resetRuntimeProfileDefaults()
      await loadAgentRuntime(agent.id)
      dialogOpen.value = true
    } else {
      isEdit.value = false
      editId.value = 0
      form.name = ''
      form.category = ''
      form.description = ''
      form.is_active = true

      selectedSkillIds.value = []
      selectedMcpConfigIds.value = []

      Object.assign(runtimeProfile, {
        source_agent: 'general_chat_agent',
        archetype: 'general_chat',
        role: '',
        goal: '',
        backstory: '',
        system_prompt: '',
        task_template: '',
        expected_output: '',
        llm_model: '',
        temperature: 0.7,
        stream_enabled: true,
        memory_enabled: false,
        skill_ids: [],
        mcp_config_ids: [],
        execution_mode: 'single-call',
        max_iterations: 16,
        plan_prompt: '',
        reflection_depth: 0,
        approval_mode: 'auto',
        approvers: [] as string[],
        im_enabled: 'disabled',
        im_config: {
          webhook_url: '',
          secret: '',
          bot_name: '',
          app_id: '',
          app_secret: '',
          verification_token: '',
          encrypt_key: '',
          lark_open_domain: '',
          ws_enabled: false,
          telegram_token: '',
          telegram_chat_id: '',
          auto_reply: false,
          notify_on_approval: true
        }
      })
      dialogOpen.value = true
    }
  }

  async function loadAgentRuntime (agentId: number) {
    try {
      const { data } = await api.get<APIResponse<Agent & { runtime_profile?: Record<string, unknown> }>>(`/agents/${agentId}`)
      const rp = data.data?.runtime_profile
      if (rp) {
        const imConfig = (rp.im_config as Record<string, unknown>) || {}
        Object.assign(runtimeProfile, {
          llm_model: rp.llm_model || '',
          temperature: rp.temperature ?? 0.7,
          system_prompt: typeof rp.system_prompt === 'string' ? rp.system_prompt : '',
          stream_enabled: rp.stream_enabled ?? true,
          memory_enabled: rp.memory_enabled ?? false,
          skill_ids: (rp.skill_ids as string[]) || [],
          mcp_config_ids: (rp.mcp_config_ids as number[]) || [],
          execution_mode: (rp.execution_mode && String(rp.execution_mode).trim() !== '') ? String(rp.execution_mode) : 'single-call',
          max_iterations: rp.max_iterations || 16,
          plan_prompt: rp.plan_prompt || '',
          reflection_depth: rp.reflection_depth || 0,
          approval_mode: typeof rp.approval_mode === 'string' && rp.approval_mode ? rp.approval_mode : 'auto',
          approvers: (rp.approvers as string[]) || [],
          im_enabled: typeof rp.im_enabled === 'string' && rp.im_enabled ? rp.im_enabled : 'disabled',
          im_config: {
            webhook_url: String(imConfig.webhook_url || ''),
            secret: String(imConfig.secret || ''),
            bot_name: String(imConfig.bot_name || ''),
            app_id: String(imConfig.app_id || ''),
            app_secret: String(imConfig.app_secret || ''),
            verification_token: String(imConfig.verification_token || ''),
            encrypt_key: String(imConfig.encrypt_key || ''),
            lark_open_domain: (() => {
              const d = String(imConfig.lark_open_domain || '').trim()
              if (d) return d
              const r = String(imConfig.lark_region || '').toLowerCase()
              if (r === 'intl' || r === 'larksuite' || r === 'global') {
                return 'https://open.larksuite.com'
              }
              return ''
            })(),
            ws_enabled: false,
            telegram_token: String(imConfig.telegram_token || ''),
            telegram_chat_id: String(imConfig.telegram_chat_id || ''),
            auto_reply: Boolean(imConfig.auto_reply),
            notify_on_approval: imConfig.notify_on_approval !== false
          }
        })
      }
    } catch {
      // use defaults
    }
  }

  async function saveAgent () {
    saving.value = true

    const runtimeProfileData = {
      source_agent: 'general_chat_agent',
      archetype: 'general_chat',
      role: null,
      goal: null,
      backstory: null,
      system_prompt: runtimeProfile.system_prompt || null,
      task_template: null,
      expected_output: null,
      llm_model: runtimeProfile.llm_model || null,
      temperature: runtimeProfile.temperature,
      stream_enabled: runtimeProfile.stream_enabled,
      memory_enabled: runtimeProfile.memory_enabled,
      skill_ids: selectedSkillIds.value,
      mcp_config_ids: selectedMcpConfigIds.value,
      execution_mode: runtimeProfile.execution_mode || 'single-call',
      max_iterations: runtimeProfile.max_iterations > 0 ? runtimeProfile.max_iterations : undefined,
      plan_prompt: runtimeProfile.plan_prompt || undefined,
      reflection_depth: runtimeProfile.reflection_depth > 0 ? runtimeProfile.reflection_depth : undefined,
      approval_mode: runtimeProfile.approval_mode || undefined,
      approvers: runtimeProfile.approvers && runtimeProfile.approvers.length > 0 ? runtimeProfile.approvers : undefined,
      im_enabled: runtimeProfile.im_enabled !== 'disabled' ? runtimeProfile.im_enabled : undefined,
      im_config: (() => {
        if (runtimeProfile.im_enabled === 'disabled') {
          return undefined
        }
        const c = { ...runtimeProfile.im_config }
        c.ws_enabled = false
        if (runtimeProfile.im_enabled !== 'telegram') {
          c.webhook_url = ''
          c.secret = ''
        }
        if (runtimeProfile.im_enabled !== 'lark') {
          c.verification_token = ''
          c.encrypt_key = ''
        }
        return c
      })()
    }

    try {
      if (isEdit.value) {
        const body: UpdateAgentRequest = {
          name: form.name,
          description: form.description,
          category: form.category,
          is_active: form.is_active,
          runtime_profile: runtimeProfileData as any
        }
        await api.put(`/agents/${editId.value}`, body)
        $q.notify({ type: 'positive', message: t('updateOk') })
      } else {
        const body: CreateAgentRequest = {
          name: form.name,
          description: form.description,
          category: form.category,
          is_active: form.is_active,
          runtime_profile: runtimeProfileData as any
        }
        await api.post('/agents', body)
        $q.notify({ type: 'positive', message: t('createOk') })
      }
      dialogOpen.value = false
      await load()
    } catch (e: unknown) {
      console.error('saveAgent error:', e)
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      saving.value = false
    }
  }

  function confirmDelete (agent: Agent) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${agent.name} (ID: ${agent.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/agents/${agent.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await load()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  function goChat (agent: { id: number; public_id?: string }) {
    const pub = (agent.public_id || '').trim()
    if (pub !== '') {
      LocalStorage.set('lastAgentPublicId', pub)
      void router.push({ name: 'chat', params: { agentId: pub } })
      return
    }
    LocalStorage.set('lastAgentId', String(agent.id))
    void router.push({ name: 'chat', params: { agentId: String(agent.id) } })
  }

  /**
   * 系统提示词：显式处理粘贴（写入 model + 恢复光标），避免部分环境下 QInput/对话框内默认粘贴不生效。
   * 纯图片剪贴板不拦截，交给浏览器默认行为（textarea 通常无效果，不会误清空）。
   */
  function onSystemPromptPaste (e: ClipboardEvent): void {
    const cd = e.clipboardData
    if (!cd) {
      return
    }
    const text = cd.getData('text/plain')
    if (text === '' && cd.files && cd.files.length > 0) {
      return
    }
    e.preventDefault()
    e.stopPropagation()
    const el = e.target as HTMLTextAreaElement
    const start = el.selectionStart ?? 0
    const end = el.selectionEnd ?? 0
    const cur = runtimeProfile.system_prompt
    runtimeProfile.system_prompt = cur.slice(0, start) + text + cur.slice(end)
    void nextTick(() => {
      el.focus()
      const pos = start + text.length
      el.setSelectionRange(pos, pos)
    })
  }

  onMounted(() => {
    void load()
    void loadAvailableOptions()
  })

  return {
    t,
    loading,
    rows,
    pagination,
    errorMsg,
    columns,
    load,
    goChat,
    dialogOpen,
    isEdit,
    form,
    runtimeProfile,
    saving,
    openDialog,
    saveAgent,
    confirmDelete,
    availableSkills,
    availableMcpConfigs,
    availableUsers,
    selectedSkillIds,
    selectedMcpConfigIds,
    onSystemPromptPaste
  }
}
