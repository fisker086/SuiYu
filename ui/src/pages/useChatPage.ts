import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded } from 'vue-router'
import { LocalStorage, useQuasar } from 'quasar'
import ChatImagePreviewDialog from 'src/components/ChatImagePreviewDialog.vue'
import { api } from 'boot/axios'
import { isCancel } from 'axios'
import type { Agent, WorkflowDefinition, APIResponse, ChatHistoryMessage, ChatReactStep, ChatSession, Skill, ChatGroup, CreateGroupRequest } from 'src/api/types'
import { hydrateReactStepsFromServer } from 'src/utils/reactStepsHydrate'
import { buildSkillRiskLookup, resolveClientToolRiskLevel } from 'src/utils/toolRisk'
import { onChatInputEnterToSend } from 'src/utils/chatComposer'
import { logChatAttach } from './chat/chatAttachDebug'
import {
  CHAT_DOCUMENT_INPUT_ACCEPT,
  CHAT_IMAGE_INPUT_ACCEPT,
  inferImageMimeFromFile,
  isAllowedDocumentButtonFile,
  isAllowedImageButtonFile
} from './chat/chatAttachmentRules'
import {
  getFileIcon,
  getFileName,
  isSafeImagePreviewSrc,
  resolveChatImageUrl,
  userMessageImageUrls,
  userMessageTextToDisplay as userMessageTextToDisplayI18n
} from './chat/chatMessageDisplay'
import {
  findAgentFromRouteParam,
  LAST_AGENT_PUBLIC_ID_KEY
} from './chat/chatRouteHelpers'
import {
  SESSION_BROWSE_PAGE_SIZE,
  SESSION_LIST_FETCH_LIMIT,
  SESSION_MESSAGES_PAGE_SIZE,
  SESSION_RAIL_PREVIEW_MAX
} from './chat/chatSessionConstants'
import {
  groupChatGroupsByDay,
  groupRailBlocksFromGrouped,
  groupSessionsByDay,
  sessionBlocksFromGrouped
} from './chat/chatSessionGroup'
import {
  chatDateDividerText,
  chatMessageTimeLabel,
  formatChatMessageTime,
  formatSessionTime,
  groupCaptionTime,
  messageDateDividerAt,
  sessionTitle
} from './chat/chatSessionTime'
import { formatPendingFileSize } from './chat/chatFormatBytes'
import {
  ROUTE_GROUP_Q,
  ROUTE_SESSION_Q,
  isLikelySessionUUID,
  normalizeRouteQuery
} from './chat/chatRouteQuery'
import { chatReactStepsToThoughtSteps } from './chat/chatReactStepsSync'
import {
  initTasksFromPlanTasksPayload,
  mergePlanDetailFromReActPayload,
  type PlanTaskRowWeb
} from 'src/utils/planExecuteMerge'
import {
  processReActEvent as processReActEventImpl,
  processStreamEvent as processStreamEventImpl
} from './chat/chatReactStreamProcessors'
import { createParseStreamEvents } from './chat/chatParseStreamEvents'
import { createSendStreamLifecycle } from './chat/chatSendStreamRuntime'
import {
  computeStreamMessageLabel,
  uploadPendingChatAttachments
} from './chat/chatSendAttachments'
import { buildChatStreamHttpRequest, buildChatStreamUrl } from './chat/chatSendStreamRequest'
import { createAssistantStreamSetup } from './chat/chatSendStreamAssistantSetup'
import { applyChatRouteAfterLoadGroups as applyChatRouteFromUrl } from './chat/chatApplyChatRoute'
import {
  handleComposerDragOver,
  handleComposerDrop,
  handleComposerPaste
} from './chat/chatComposerAttach'

const THOUGHT_STEPS_MAX = 200

function createChatPageState () {
  const { t } = useI18n()
  const $q = useQuasar()
  const route = useRoute()
  const router = useRouter()

  const chatMode = ref<'agent' | 'workflow'>('agent')

  const agentsLoading = ref(false)

  const agents = ref<Agent[]>([])
  const agentId = ref<number | null>(null)

  const agentOptions = computed(() =>
    agents.value.map(a => ({
      id: a.id,
      label: a.public_id ? `${a.name}` : `${a.name} (#${a.id})`
    }))
  )

  const workflowsLoading = ref(false)
  const workflows = ref<WorkflowDefinition[]>([])
  const workflowId = ref<number | null>(null)
  const workflowOptions = computed(() =>
    workflows.value.filter(w => w.is_active).map(w => ({
      id: w.id,
      label: `${w.name} (#${w.id})`
    }))
  )

  // Chat groups（须在 canStartSession 之前声明）
  const chatGroups = ref<ChatGroup[]>([])
  const currentGroup = ref<ChatGroup | null>(null)
  const groupDialogOpen = ref(false)
  const groupForm = ref<{ name: string; agentIds: number[] }>({ name: '', agentIds: [] })
  const groupInviteDialogOpen = ref(false)
  const groupInviteAgentIds = ref<number[]>([])
  const currentUserLabel = ref('')

  const groupInviteSelectOptions = computed(() => {
    const g = currentGroup.value
    const inGroup = new Set((g?.members ?? []).map(m => m.agent_id))
    return agents.value
      .filter(a => !inGroup.has(a.id))
      .map(a => ({
        id: a.id,
        label: a.public_id ? `${a.name}` : `${a.name} (#${a.id})`
      }))
  })

  function openGroupInviteDialog (): void {
    if (!currentGroup.value) return
    groupInviteAgentIds.value = []
    groupInviteDialogOpen.value = true
  }

  async function submitGroupInvite (): Promise<void> {
    const g = currentGroup.value
    if (!g) return
    if (groupInviteAgentIds.value.length === 0) {
      $q.notify({ type: 'warning', message: t('groupInvitePickAgents') })
      return
    }
    const existing = [...(g.members ?? []).map(m => m.agent_id)]
    const merged = [...new Set([...existing, ...groupInviteAgentIds.value])]
    try {
      await api.put<APIResponse<ChatGroup>>(`/chat/groups/${g.id}`, { agent_ids: merged })
      await loadChatGroups()
      const upd = chatGroups.value.find(x => x.id === g.id)
      if (upd) currentGroup.value = upd
      groupInviteDialogOpen.value = false
      groupInviteAgentIds.value = []
      $q.notify({ type: 'positive', message: t('groupInviteSuccess') })
    } catch {
      $q.notify({ type: 'negative', message: t('groupInviteFailed') })
    }
  }

  /** 已选 Agent/Workflow 即可输入与附件；群聊仅须已选群组；不要求已有 session（首条消息由服务端创建会话） */
  const canStartSession = computed(() => {
    if (currentGroup.value && currentGroup.value.id > 0) {
      return true
    }
    if (chatMode.value === 'agent') {
      return agentId.value != null && agentId.value >= 1
    }
    return workflowId.value != null && workflowId.value >= 1
  })

  const sessionId = ref<string | null>(null)
  const sessionBusy = ref(false)
  const sessionsList = ref<ChatSession[]>([])
  const sessionsLoading = ref(false)
  /** selectSession 过程中避免 loadSessions 把 sessionId 抢改成 list[0]（与列表刷新竞态） */
  const suppressSessionListAutoPick = ref(false)
  const sessionDrawerOpen = ref(false)
  const SESSION_RAIL_LEGACY = 'sya_chat_session_rail_collapsed'
  const SESSION_RAIL_COLLAPSE_KEY = 'aitaskmeta_chat_session_rail_collapsed'
  /** 群聊会话 id（与 GET /chat/sessions/:id/messages 一致），刷新后从本地恢复 */
  const LEGACY_GROUP_PREFIX = 'sya_group_session_'
  const GROUP_SESSION_STORAGE_PREFIX = 'aitaskmeta_group_session_'

  function getStoredGroupSessionId (groupId: number): string | null {
    const k = `${GROUP_SESSION_STORAGE_PREFIX}${groupId}`
    const cur = LocalStorage.getItem<string>(k)
    if (cur != null && String(cur).trim() !== '') return String(cur).trim()
    const leg = LocalStorage.getItem<string>(`${LEGACY_GROUP_PREFIX}${groupId}`)
    if (leg != null && String(leg).trim() !== '') {
      const s = String(leg).trim()
      LocalStorage.set(k, s)
      LocalStorage.removeItem(`${LEGACY_GROUP_PREFIX}${groupId}`)
      return s
    }
    return null
  }

  function setStoredGroupSessionId (groupId: number, sid: string): void {
    LocalStorage.set(`${GROUP_SESSION_STORAGE_PREFIX}${groupId}`, sid)
    LocalStorage.removeItem(`${LEGACY_GROUP_PREFIX}${groupId}`)
  }
  const sessionRailCollapsed = ref((() => {
    let v = LocalStorage.getItem(SESSION_RAIL_COLLAPSE_KEY)
    if (v !== '1' && v !== '0') v = LocalStorage.getItem(SESSION_RAIL_LEGACY)
    if (v === '1' || v === '0') {
      LocalStorage.set(SESSION_RAIL_COLLAPSE_KEY, v)
      LocalStorage.removeItem(SESSION_RAIL_LEGACY)
    }
    return v === '1'
  })())

  /** 路由参数 `agentId` 复用：智能体 public_id / 数字 id，或单聊会话 UUID（群聊会话见 `routeParamSessionId`） */
  function routePathSegment (): string {
    const raw = route.params.agentId
    return typeof raw === 'string' ? raw : Array.isArray(raw) ? raw[0] ?? '' : ''
  }

  /** 仅 `chat-group` 路由：`/chat/group/:sessionId` */
  function routeParamSessionId (): string {
    const raw = route.params.sessionId
    return typeof raw === 'string' ? raw : Array.isArray(raw) ? raw[0] ?? '' : ''
  }

  /** 群聊：`/chat/group/<session_id>`（单聊仍为 `/chat/<agent|单聊 session>`） */
  function syncGroupChatToRoute (): void {
    const g = currentGroup.value
    if (!g || g.id < 1) return
    const sid = sessionId.value?.trim()
    if (!sid) return
    if (route.name === 'chat-group') {
      const cur = routeParamSessionId()
      if (cur === sid) return
    } else {
      const cur = routePathSegment()
      if (cur === sid) return
    }
    void router.replace({ name: 'chat-group', params: { sessionId: sid }, query: {} })
  }

  async function loadChatGroups () {
    try {
      const { data } = await api.get<APIResponse<ChatGroup[]>>('/chat/groups')
      chatGroups.value = data.data || []
    } catch (err) {
      console.warn('loadChatGroups failed:', err)
    }
  }

  /** 路径 UUID 实为智能体 public_id：按单聊打开，不请求 GET /sessions/:id，避免无意义 404 */
  function applyAgentFromPublicIdPath (found: Agent): void {
    currentGroup.value = null
    sessionListModeTab.value = 'single'
    const prevAgent = agentId.value
    const prevSid = sessionId.value
    agentId.value = found.id
    const keepSession = prevAgent === found.id && prevSid != null && prevSid !== ''
    if (!keepSession) {
      sessionId.value = null
      history.value = []
      localTurns.value = []
      lastTurnDurationMs.value = null
      draft.value = ''
      pendingImages.value = []
      pendingFiles.value = []
      clearThoughtSteps()
    }
    void loadSessions()
    syncAgentIdToRoute()
  }

  /** 根据路径 `/chat/...`、`/chat/group/...`、`?session=`（重定向）、`?group=` 恢复上下文 */
  async function applyChatRouteAfterLoadGroups (): Promise<void> {
    return applyChatRouteFromUrl({
      route: route as RouteLocationNormalizedLoaded,
      router,
      routePathSegment,
      routeParamSessionId,
      loadChatGroups,
      loadMessages,
      loadSessions,
      selectChatGroup,
      applyAgentFromPublicIdPath,
      clearThoughtSteps,
      syncAgentIdToRoute,
      setStoredGroupSessionId,
      agents,
      chatGroups,
      currentGroup,
      sessionId,
      agentId,
      workflowId,
      chatMode,
      sessionListModeTab,
      history,
      localTurns,
      lastTurnDurationMs,
      draft,
      pendingImages,
      pendingFiles,
      sessionDrawerOpen,
      sessionFullDialogOpen
    })
  }

  async function createChatGroup () {
    if (!groupForm.value.name || groupForm.value.agentIds.length === 0) {
      $q.notify({ type: 'warning', message: t('groupNameAndAgentsRequired') })
      return
    }
    try {
      const req: CreateGroupRequest = {
        name: groupForm.value.name,
        agent_ids: groupForm.value.agentIds
      }
      const { data } = await api.post<APIResponse<ChatGroup>>('/chat/groups', req)
      if (data.data) {
        chatGroups.value.unshift(data.data)
      }
      groupDialogOpen.value = false
      groupForm.value = { name: '', agentIds: [] }
      $q.notify({ type: 'positive', message: t('groupCreated') })
    } catch (err) {
      $q.notify({ type: 'negative', message: t('groupCreateFailed') })
    }
  }

  async function deleteChatGroup (id: number) {
    try {
      await api.delete(`/chat/groups/${id}`)
      chatGroups.value = chatGroups.value.filter(g => g.id !== id)
      if (currentGroup.value?.id === id) {
        currentGroup.value = null
        if (agentId.value == null && agents.value.length > 0) {
          agentId.value = agents.value[0].id
        }
        if (agentId.value != null && agentId.value >= 1) {
          syncAgentIdToRoute()
        } else {
          void router.replace({ path: '/chat', query: {} })
        }
      }
      $q.notify({ type: 'positive', message: t('groupDeleted') })
    } catch {
      $q.notify({ type: 'negative', message: t('groupDeleteFailed') })
    }
  }

  function clearCurrentGroup () {
    currentGroup.value = null
    sessionListModeTab.value = 'single'
    if (chatMode.value === 'agent' && agentId.value != null && agentId.value >= 1) {
      syncAgentIdToRoute()
    } else {
      void router.replace({ path: '/chat', query: {} })
    }
  }

  const history = ref<ChatHistoryMessage[]>([])
  /** 与 ChatHistoryMessage 附件字段一致：image_urls / file_urls / image_data_urls（仅本地预览） */
  const localTurns = ref<
    {
      role: string
      content: string
      displayedContent: string
      duration?: number
      image_urls?: string[]
      image_data_urls?: string[]
      file_urls?: string[]
      createdAt?: string
      /** ReAct / ADK 步骤快照，供 ThoughtSidebar；主气泡只渲染最终正文 */
      reactSteps?: ChatReactStep[]
      /** Agent ID for group chat responses (to display which agent sent the message) */
      agentId?: number
    }[]
  >([])

  const displayMessages = computed(() => {
    const fromHistory = history.value.map(h => ({
      role: h.role,
      content: h.content,
      displayedContent: h.content,
      agentId: h.agent_id != null && h.agent_id >= 1 ? h.agent_id : undefined,
      duration: undefined as number | undefined,
      image_urls: h.image_urls ?? [],
      image_data_urls: undefined as string[] | undefined,
      file_urls: h.file_urls ?? [],
      createdAt: h.created_at,
      reactSteps: hydrateReactStepsFromServer(h.react_steps)
    }))
    return [...fromHistory, ...localTurns.value]
  })

  const maxChatImages = 12
  const pendingImages = ref<{ dataUrl: string; base64: string; mime: string }[]>([])
  const pendingFiles = ref<{ name: string; size: number; base64: string; mime: string; url?: string }[]>([])
  const imageInputRef = ref<HTMLInputElement | null>(null)
  const fileInputRef = ref<HTMLInputElement | null>(null)

  const maxChatImageBytes = 8 * 1024 * 1024

  const sending = ref(false)
  const stopping = ref(false)
  /** 用于中止 /chat/stream 的 fetch（非响应式，仅 sendStream/stopStream 使用） */
  let streamAbortController: AbortController | null = null
  const draft = ref('')
  /** Quasar QInput 实例，用于定位 textarea 做 @ 成员提示 */
  const composerInputRef = ref<{ $el?: HTMLElement } | null>(null)
  /** 群聊输入框 @ 提及菜单 */
  const mentionOpen = ref(false)
  const mentionQuery = ref('')
  const mentionAnchorStart = ref(0)
  const mentionIndex = ref(0)
  const chatScrollRef = ref<HTMLElement | null>(null)

  watch(
    () => [pendingImages.value.length, pendingFiles.value.length] as const,
    async ([ni, nf]) => {
      logChatAttach(`pending counts images=${ni} files=${nf}`)
      await nextTick()
      const strip =
        typeof document !== 'undefined' ? document.querySelector('.chat-pending-strip') : null
      const h = strip instanceof HTMLElement ? strip.offsetHeight : null
      const disp = strip instanceof HTMLElement ? window.getComputedStyle(strip).display : null
      logChatAttach(
        `DOM after tick scrollRef=${!!chatScrollRef.value} strip=${!!strip} stripH=${h} stripDisplay=${disp}`
      )
    }
  )

  const currentStreamModelName = ref('')

  /** 与 GET /skills（库表 + 服务端 enrich）一致，用于侧栏风险；未加载前回退 medium */
  const skillRiskLookup = ref<Record<string, string>>({})

  async function loadSkillRiskLookup (): Promise<void> {
    try {
      const { data } = await api.get<APIResponse<Skill[]>>('/skills')
      if (data.code === 0 && Array.isArray(data.data)) {
        skillRiskLookup.value = buildSkillRiskLookup(data.data)
      }
    } catch {
      /* 保持已有 lookup，避免刷屏 */
    }
  }

  function resolveToolRiskForStream (toolName: string, payloadRisk: unknown): string {
    return resolveClientToolRiskLevel(toolName, payloadRisk, skillRiskLookup.value)
  }

  const THOUGHT_SIDEBAR_LEGACY = 'sya_chat_thought_sidebar_open'
  const THOUGHT_SIDEBAR_KEY = 'aitaskmeta_chat_thought_sidebar_open'
  const thoughtSidebarOpen = ref((() => {
    let v = LocalStorage.getItem(THOUGHT_SIDEBAR_KEY)
    if (v !== '1' && v !== '0') v = LocalStorage.getItem(THOUGHT_SIDEBAR_LEGACY)
    if (v === '1' || v === '0') {
      LocalStorage.set(THOUGHT_SIDEBAR_KEY, v)
      LocalStorage.removeItem(THOUGHT_SIDEBAR_LEGACY)
    }
    return v === '1'
  })())
  const thoughtSteps = ref<{ type: string; data: Record<string, unknown>; meta?: Record<string, unknown>; timestamp?: string }[]>([])
  /** 与 chatParseStreamEvents 共用：去掉主气泡内与 ADK tool_result 完全重复的文本 */
  const lastServerToolResultText = { value: '' }
  const thoughtStatus = ref<'running' | 'completed'>('completed')
  const lastTurnDurationMs = ref<number | null>(null)

  function toggleThoughtSidebar (): void {
    thoughtSidebarOpen.value = !thoughtSidebarOpen.value
    LocalStorage.set(THOUGHT_SIDEBAR_KEY, thoughtSidebarOpen.value ? '1' : '0')
    LocalStorage.removeItem(THOUGHT_SIDEBAR_LEGACY)
  }

  function pushThoughtStep (step: { type: string; data: Record<string, unknown>; meta?: Record<string, unknown>; timestamp?: string }): void {
    if (!step || !step.type || !step.data) return
    thoughtSteps.value.push({
      ...step,
      timestamp: step.timestamp || new Date().toISOString()
    })
    if (thoughtSteps.value.length > THOUGHT_STEPS_MAX) {
      thoughtSteps.value.splice(0, thoughtSteps.value.length - THOUGHT_STEPS_MAX)
    }
  }

  /** Plan-and-execute：与桌面端 PlanExecuteTaskPanel 一致，合并 plan_tasks / plan_step 为一条可更新清单 */
  function upsertPlanExecute (payload: Record<string, unknown>): void {
    const reactType = payload.type as string
    if (reactType === 'plan_tasks') {
      const tasks = initTasksFromPlanTasksPayload(payload)
      if (tasks.length === 0) return
      // 新一轮 plan_tasks（含 tool_result/stream 重发）时先清掉旧清单，避免多条 plan_execute 并存；
      // 否则 reactSteps.find 取第一条、getCurrentPlanTasks 取最后一条，主气泡会叠两张卡或状态错位。
      thoughtSteps.value = thoughtSteps.value.filter(
        (s) => !(s.type === 'plan' && s.data?.kind === 'plan_execute')
      )
      pushThoughtStep({
        type: 'plan',
        data: { kind: 'plan_execute', tasks }
      })
      return
    }
    if (reactType === 'plan_step') {
      const stepNum = payload.step as number | undefined
      const st = payload.plan_step_status as string | undefined
      if (stepNum == null || !st) return
      const list = thoughtSteps.value
      for (let i = list.length - 1; i >= 0; i--) {
        const s = list[i]
        if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
          const tasks = (s.data.tasks as PlanTaskRowWeb[]) || []
          const idx = stepNum - 1
          if (idx < 0 || idx >= tasks.length) return
          const next = [...tasks]
          const cur = next[idx]
          let nextStatus: 'pending' | 'running' | 'done' | 'error' = cur.status
          if (st === 'running') nextStatus = 'running'
          else if (st === 'done') nextStatus = 'done'
          else if (st === 'error') nextStatus = 'error'
          next[idx] = { ...cur, status: nextStatus }
          list[i] = { ...s, data: { ...s.data, tasks: next } }
          thoughtSteps.value = [...list]
          return
        }
      }
    }
  }

  function mergePlanExecuteReActEvent (payload: Record<string, unknown>): boolean {
    const list = thoughtSteps.value
    for (let i = list.length - 1; i >= 0; i--) {
      const s = list[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        const tasks = (s.data.tasks as PlanTaskRowWeb[]) || []
        const merged = mergePlanDetailFromReActPayload(tasks, payload)
        if (!merged) return false
        list[i] = { ...s, data: { ...s.data, tasks: merged } }
        thoughtSteps.value = [...list]
        return true
      }
    }
    return false
  }

  function appendThoughtText (content: string, source = 'stream'): void {
    if (!content || !content.trim()) return
    const lastStep = thoughtSteps.value[thoughtSteps.value.length - 1]
    if (lastStep && lastStep.type === 'thought' && lastStep.data?.source === source) {
      lastStep.data.content = `${lastStep.data.content || ''}${content}`
      return
    }
    pushThoughtStep({
      type: 'thought',
      data: { content, source }
    })
  }

  function clearThoughtSteps (): void {
    thoughtSteps.value = []
    lastServerToolResultText.value = ''
    thoughtStatus.value = 'completed'
  }

  /** 用户手动停止流式回复时，结束当前计划项的转圈状态，避免 UI 一直显示为 running。 */
  function finalizeRunningPlanTasksOnStop (): void {
    const list = thoughtSteps.value
    for (let i = list.length - 1; i >= 0; i--) {
      const s = list[i]
      if (s.type !== 'plan' || s.data?.kind !== 'plan_execute') continue
      const tasks = ((s.data.tasks as PlanTaskRowWeb[]) || []).map((row) => {
        if (row.status !== 'running') return row
        const details = row.details ?? []
        const stopText = '已手动停止回复'
        const nextDetails = details.some(d => d.text === stopText)
          ? details
          : [...details, { text: stopText, tone: 'muted' as const }]
        return { ...row, status: 'pending' as const, details: nextDetails }
      })
      list[i] = { ...s, data: { ...s.data, tasks } }
      thoughtSteps.value = [...list]
      return
    }
  }

  async function selectChatGroup (group: ChatGroup) {
    currentGroup.value = group
    agentId.value = null
    workflowId.value = null
    chatMode.value = 'agent'
    const stored = getStoredGroupSessionId(group.id)
    let sid = stored
    if (!sid) {
      const first = group.members?.[0]?.agent_id
      if (first != null && first >= 1) {
        try {
          const { data } = await api.post<APIResponse<ChatSession>>('/chat/sessions', {
            agent_id: first,
            group_id: group.id
          })
          const sess = data.data as ChatSession
          if (sess?.session_id) {
            sid = sess.session_id
            setStoredGroupSessionId(group.id, sid)
          }
        } catch (err) {
          console.warn('create group chat session failed:', err)
        }
      }
    }
    sessionId.value = sid
    if (sessionId.value) {
      setStoredGroupSessionId(group.id, sessionId.value)
    }
    history.value = []
    localTurns.value = []
    lastTurnDurationMs.value = null
    draft.value = ''
    pendingImages.value = []
    pendingFiles.value = []
    clearThoughtSteps()
    sessionDrawerOpen.value = false
    sessionFullDialogOpen.value = false
    if (sessionId.value) {
      void loadMessages()
    }
    sessionListModeTab.value = 'group'
    syncGroupChatToRoute()
  }

  /** 打开会话或刷新消息后，用最后一条含 react_steps 的助手消息驱动 ThoughtSidebar（与流式过程共用同一数据源） */
  function syncThoughtSidebarFromLoadedHistory (): void {
    if (sending.value) return
    const msgs = displayMessages.value
    for (let i = msgs.length - 1; i >= 0; i--) {
      const m = msgs[i] as { role: string; reactSteps?: ChatReactStep[] }
      if (m.role !== 'assistant') continue
      const rs = m.reactSteps
      if (rs != null && rs.length > 0) {
        thoughtSteps.value = chatReactStepsToThoughtSteps(rs)
        thoughtStatus.value = 'completed'
        return
      }
    }
    thoughtSteps.value = []
    thoughtStatus.value = 'completed'
  }

  function processReActEvent (payload: Record<string, unknown>): void {
    processReActEventImpl(payload, {
      pushThoughtStep,
      resolveToolRiskForStream,
      upsertPlanExecute,
      mergePlanExecuteReActEvent
    })
  }

  function processStreamEvent (payload: Record<string, unknown>): void {
    processStreamEventImpl(payload, {
      pushThoughtStep,
      resolveToolRiskForStream,
      setLastServerToolResult: (t: string) => {
        lastServerToolResultText.value = t
      }
    })
  }

  function scrollChatToBottom (): void {
    void nextTick(() => {
      const el = chatScrollRef.value
      if (!el) return
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    })
  }

  function fillDraft (text: string): void {
    draft.value = text
  }

  function clearPendingImages (): void {
    pendingImages.value = []
  }

  function removePendingImageAt (idx: number): void {
    pendingImages.value.splice(idx, 1)
  }

  /** 与「上传文件」按钮、粘贴、拖放共用；内部校验类型与大小 */
  function enqueuePendingDocumentFromFile (file: File): void {
    const maxFileSize = 10 * 1024 * 1024 // 10MB，与后端上传一致
    if (!isAllowedDocumentButtonFile(file)) {
      $q.notify({ type: 'warning', message: t('chatDocumentTypeNotAllowed') })
      return
    }
    if (file.size > maxFileSize) {
      $q.notify({ type: 'warning', message: t('chatFileTooLarge') })
      return
    }
    const reader = new FileReader()
    reader.onerror = () => {
      $q.notify({ type: 'negative', message: `${t('chatAttachFile')}: 读取失败` })
    }
    reader.onload = () => {
      const dataUrl = reader.result as string
      const comma = dataUrl.indexOf(',')
      if (comma < 0) return
      const base64 = dataUrl.slice(comma + 1)
      const rawMime = (file.type || '').trim()
      const lower = file.name.toLowerCase()
      let fm = rawMime
      if (!fm) {
        if (lower.endsWith('.pdf')) fm = 'application/pdf'
        else if (lower.endsWith('.json')) fm = 'application/json'
        else if (lower.endsWith('.md')) fm = 'text/markdown'
        else fm = 'text/plain'
      }
      pendingFiles.value = [
        ...pendingFiles.value,
        { name: file.name, size: file.size, base64, mime: fm }
      ]
      logChatAttach('pending document', { name: file.name, mime: fm })
      void nextTick(() => scrollChatToBottom())
    }
    reader.readAsDataURL(file)
  }

  function onFileSelected (e: Event): void {
    const input = e.target as HTMLInputElement
    // 必须先拷贝 File[]：input.value='' 会清空同一 FileList，随后 files.length 变为 0
    const fileArray = input.files?.length ? Array.from(input.files) : []
    input.value = ''
    logChatAttach(
      `onFileSelected files=${fileArray.length} canStart=${canStartSession.value} mode=${chatMode.value} agentId=${agentId.value} workflowId=${workflowId.value} inputDisabled=${input.disabled}`
    )
    if (fileArray.length === 0) return
    for (const file of fileArray) {
      enqueuePendingDocumentFromFile(file)
    }
  }

  function clearPendingFiles (): void {
    pendingFiles.value = []
  }

  function removePendingFile (idx: number): void {
    pendingFiles.value.splice(idx, 1)
  }

  async function uploadFile (file: File): Promise<{ url: string; filename: string } | null> {
    const formData = new FormData()
    formData.append('file', file)
    try {
      const { data } = await api.post<APIResponse<{ url: string; filename: string }>>('/chat/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })
      if (data.code === 0 && data.data) {
        return data.data
      }
      $q.notify({ type: 'negative', message: data.message || '上传失败' })
      return null
    } catch (e) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '上传失败' })
      return null
    }
  }

  function setPendingImageFromFile (file: File): void {
    const mime = file.type || ''
    logChatAttach(
      `setPendingImageFromFile enter name=${file.name} size=${file.size} mime=${mime || '(empty)'}`
    )
    if (!isAllowedImageButtonFile(file)) {
      logChatAttach('setPendingImageFromFile abort: type not allowed')
      $q.notify({ type: 'warning', message: t('chatImageTypeNotAllowed') })
      return
    }
    if (file.size > maxChatImageBytes) {
      logChatAttach(`setPendingImageFromFile abort: file too large (>${maxChatImageBytes})`)
      $q.notify({ type: 'warning', message: t('chatImageTooLarge') })
      return
    }
    const reader = new FileReader()
    reader.onerror = () => {
      logChatAttach('setPendingImageFromFile FileReader.onerror', reader.error?.message ?? 'unknown')
      $q.notify({ type: 'negative', message: `${t('chatAttachImage')}: 读取失败` })
    }
    reader.onload = () => {
      const dataUrl = reader.result as string
      const comma = dataUrl.indexOf(',')
      if (comma < 0) {
        logChatAttach('setPendingImageFromFile: invalid dataUrl (no comma)')
        return
      }
      const base64 = dataUrl.slice(comma + 1)
      if (pendingImages.value.length >= maxChatImages) {
        $q.notify({ type: 'warning', message: t('chatMaxImages', { n: maxChatImages }) })
        return
      }
      pendingImages.value = [...pendingImages.value, {
        dataUrl,
        base64,
        mime: inferImageMimeFromFile(file)
      }]
      logChatAttach(
        `setPendingImageFromFile ok name=${file.name} mime=${mime || 'image/png'} dataUrlLen=${dataUrl.length} prefix=${dataUrl.slice(0, 40)} pendingTotal=${pendingImages.value.length}`
      )
      void nextTick(() => scrollChatToBottom())
    }
    logChatAttach('setPendingImageFromFile FileReader.readAsDataURL(...) start')
    reader.readAsDataURL(file)
  }

  function onImageSelected (e: Event): void {
    const input = e.target as HTMLInputElement
    const picked = input.files?.length ? Array.from(input.files) : []
    input.value = ''
    logChatAttach(
      `onImageSelected files=${picked.length} canStart=${canStartSession.value} mode=${chatMode.value} agentId=${agentId.value} workflowId=${workflowId.value} inputDisabled=${input.disabled}`
    )
    if (picked.length === 0) return
    for (let i = 0; i < picked.length; i++) {
      if (pendingImages.value.length >= maxChatImages) {
        $q.notify({ type: 'warning', message: t('chatMaxImages', { n: maxChatImages }) })
        break
      }
      setPendingImageFromFile(picked[i])
    }
  }

  const composerAttachDeps = () => ({
    canStartSession: () => canStartSession.value,
    maxChatImages,
    maxChatImageBytes,
    getPendingImageCount: () => pendingImages.value.length,
    t,
    logChatAttach,
    notifyWarning: (message: string) => { $q.notify({ type: 'warning', message }) },
    setPendingImageFromFile,
    enqueuePendingDocumentFromFile
  })

  /** 输入框内粘贴：图片走待发送预览；PDF/TXT/MD/JSON 走「上传文件」同款逻辑（实现见 chatComposerAttach）。 */
  function onComposerPaste (e: ClipboardEvent): void {
    handleComposerPaste(e, composerAttachDeps())
  }

  function onComposerDragOver (e: DragEvent): void {
    handleComposerDragOver(e, composerAttachDeps())
  }

  /** 拖入输入区：与文件夹按钮、粘贴一致，支持图片与允许类型的文档 */
  function onComposerDrop (e: DragEvent): void {
    handleComposerDrop(e, composerAttachDeps())
  }

  /** 将地址栏与当前选中智能体对齐，避免下拉切换后刷新仍回到旧 ID */
  function syncAgentIdToRoute (): void {
    if (chatMode.value !== 'agent') return
    if (currentGroup.value && currentGroup.value.id > 0) {
      syncGroupChatToRoute()
      return
    }
    const id = agentId.value
    const curRaw = route.params.agentId
    const cur = typeof curRaw === 'string' ? curRaw : Array.isArray(curRaw) ? (curRaw[0] ?? '') : ''
    if (id != null && id >= 1) {
      const ag = agents.value.find(a => a.id === id)
      const pub = (ag?.public_id ?? '').trim()
      const want = pub !== '' ? pub : String(id)
      if (
        cur === want &&
        !normalizeRouteQuery(route.query[ROUTE_GROUP_Q] as string | string[] | undefined) &&
        !normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)
      ) {
        return
      }
      void router.replace({ name: 'chat', params: { agentId: want }, query: {} })
    }
  }

  async function loadAgents () {
    agentsLoading.value = true
    try {
      const { data } = await api.get<APIResponse<Agent[]>>('/agents')
      agents.value = (data.data ?? []) as Agent[]
      const pathStr = routePathSegment()
      const looksLikeSessionUuidPath = pathStr !== '' && isLikelySessionUUID(pathStr)
      const hasSessionInQuery =
        normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined) !== ''
      const hasGroupInQuery =
        normalizeRouteQuery(route.query[ROUTE_GROUP_Q] as string | string[] | undefined) !== ''
      const fromStoragePub = LocalStorage.getItem<string>(LAST_AGENT_PUBLIC_ID_KEY)
      const legacyNum = LocalStorage.getItem<string>('lastAgentId')
      if (route.name === 'chat-group') {
        agentId.value = null
      } else if (hasSessionInQuery || hasGroupInQuery) {
        agentId.value = null
      } else {
        const raw =
          pathStr !== '' ? pathStr : (fromStoragePub ?? '')
        let picked = findAgentFromRouteParam(raw, agents.value)
        if (!picked && (fromStoragePub == null || fromStoragePub === '') && legacyNum != null && legacyNum !== '') {
          picked = findAgentFromRouteParam(legacyNum, agents.value)
        }
        if (picked) {
          agentId.value = picked.id
          if ((picked.public_id || '').trim() !== '') {
            const routeStr = pathStr
            if (routeStr !== picked.public_id && /^\d+$/.test(routeStr.trim())) {
              void router.replace({ name: 'chat', params: { agentId: picked.public_id }, query: {} })
            }
          }
        } else if (looksLikeSessionUuidPath) {
          // 可能是会话 id（群/单聊），也可能是与 session 同形的 public_id；后者由 applyChatRoute 在 GET 失败后回退解析
          agentId.value = null
        } else if (agents.value.length > 0) {
          agentId.value = agents.value[0].id
        } else {
          agentId.value = null
        }
      }
    } finally {
      agentsLoading.value = false
    }
  }

  async function loadWorkflows () {
    workflowsLoading.value = true
    try {
      const { data } = await api.get<APIResponse<WorkflowDefinition[]>>('/workflows/graph')
      workflows.value = (data.data ?? []) as WorkflowDefinition[]
    } finally {
      workflowsLoading.value = false
    }
  }

  const canSend = computed(() => {
    if (sending.value) return false
    // Group chat or agent/workflow chat requires different conditions
    if (currentGroup.value && currentGroup.value.id > 0) {
      // Group chat: need group and either text or @mentions
      const hasText = draft.value.trim().length > 0
      const hasMentions = /@\S+/.test(draft.value)
      const hasImg = pendingImages.value.length > 0
      const hasFiles = pendingFiles.value.length > 0
      return hasText || hasMentions || hasImg || hasFiles
    }
    if (!canStartSession.value) return false
    const hasText = draft.value.trim().length > 0
    const hasImg = pendingImages.value.length > 0
    const hasFiles = pendingFiles.value.length > 0
    return hasText || hasImg || hasFiles
  })

  /** 当前群成员（可 @），名称与 extractMentions 所用 agents 列表一致 */
  const groupMentionAgents = computed(() => {
    const g = currentGroup.value
    if (!g?.members?.length) return [] as { id: number; name: string }[]
    const out: { id: number; name: string }[] = []
    const seen = new Set<number>()
    for (const m of g.members) {
      const ag = agents.value.find(a => a.id === m.agent_id)
      const name = (m.agent_name ?? ag?.name ?? '').trim()
      if (!name || seen.has(m.agent_id)) continue
      seen.add(m.agent_id)
      out.push({ id: m.agent_id, name })
    }
    return out
  })

  const mentionFiltered = computed(() => {
    if (!mentionOpen.value || !currentGroup.value) return [] as { id: number; name: string }[]
    const q = mentionQuery.value.toLowerCase()
    return groupMentionAgents.value.filter(
      a => q === '' || a.name.toLowerCase().includes(q)
    )
  })

  const showSessionRail = computed(() => chatMode.value === 'agent' && $q.screen.gt.sm)

  function getComposerTextarea (): HTMLTextAreaElement | null {
    const q = composerInputRef.value as
      | { nativeEl?: HTMLTextAreaElement | null; $el?: HTMLElement }
      | null
    if (!q) return null
    const native = q.nativeEl
    if (native instanceof HTMLTextAreaElement) return native
    const root = q.$el
    if (!root) return null
    return root.querySelector('textarea')
  }

  /** 半角 @ 与全角 ＠（IME）均触发提及 */
  const MENTION_AT_FULL = '\uFF20'

  function updateMentionFromDraft (): void {
    const g = currentGroup.value
    if (!g?.id || !(g.members?.length ?? 0)) {
      mentionOpen.value = false
      return
    }
    const ta = getComposerTextarea()
    if (!ta) return
    const cursor = ta.selectionStart ?? draft.value.length
    const before = draft.value.slice(0, cursor)
    const iAt = before.lastIndexOf('@')
    const iFull = before.lastIndexOf(MENTION_AT_FULL)
    const at = Math.max(iAt, iFull)
    if (at < 0) {
      mentionOpen.value = false
      return
    }
    if (at > 0 && !/[\s\n]/.test(before[at - 1] ?? '')) {
      mentionOpen.value = false
      return
    }
    const afterAt = before.slice(at + 1)
    if (/\s/.test(afterAt)) {
      mentionOpen.value = false
      return
    }
    mentionAnchorStart.value = at
    mentionQuery.value = afterAt
    mentionOpen.value = true
    mentionIndex.value = 0
  }

  function insertMentionByAgent (agent: { id: number; name: string }): void {
    const ta = getComposerTextarea()
    const end = ta?.selectionStart ?? draft.value.length
    const start = mentionAnchorStart.value
    const name = agent.name
    const before = draft.value.slice(0, start)
    const after = draft.value.slice(end)
    draft.value = `${before}@${name} ${after}`
    mentionOpen.value = false
    void nextTick(() => {
      const pos = start + name.length + 2
      const t = getComposerTextarea()
      t?.setSelectionRange(pos, pos)
      t?.focus()
    })
  }

  function onComposerKeyup (): void {
    if (currentGroup.value?.id) {
      void nextTick(() => updateMentionFromDraft())
    }
  }

  watch(
    () => draft.value,
    () => {
      if (currentGroup.value?.id) {
        void nextTick(() => updateMentionFromDraft())
      }
    }
  )

  watch(currentGroup, () => {
    mentionOpen.value = false
  })

  watch(mentionFiltered, (list) => {
    if (mentionIndex.value >= list.length) {
      mentionIndex.value = Math.max(0, list.length - 1)
    }
  })

  const groupChatRailBlocks = computed(() =>
    groupRailBlocksFromGrouped(groupChatGroupsByDay(chatGroups.value), t)
  )

  const sessionGroupBlocks = computed(() =>
    sessionBlocksFromGrouped(groupSessionsByDay(sessionsList.value), t)
  )

  const sessionsBrowseList = ref<ChatSession[]>([])
  const sessionsBrowseLoading = ref(false)
  const sessionsBrowseLoadingMore = ref(false)
  const sessionsBrowseHasMore = ref(false)
  const sessionsBrowseOffset = ref(0)
  const browseSelectedSessionId = ref<string | null>(null)
  const browseMessages = ref<ChatHistoryMessage[]>([])
  const browseMessagesLoading = ref(false)

  const browseMessagesForDisplay = computed(() =>
    browseMessages.value.map(h => ({
      ...h,
      reactSteps: hydrateReactStepsFromServer(h.react_steps)
    }))
  )

  const sessionBrowseGroupBlocks = computed(() =>
    sessionBlocksFromGrouped(groupSessionsByDay(sessionsBrowseList.value), t)
  )

  let browseScrollDebounce: ReturnType<typeof setTimeout> | null = null

  const sessionRailPreviewBlocks = computed(() => {
    const blocks = sessionGroupBlocks.value
    const out: { key: string; label: string; items: ChatSession[] }[] = []
    let count = 0
    for (const b of blocks) {
      if (count >= SESSION_RAIL_PREVIEW_MAX) break
      const slice = b.items.slice(0, SESSION_RAIL_PREVIEW_MAX - count)
      if (slice.length === 0) continue
      out.push({ ...b, items: slice })
      count += slice.length
    }
    return out
  })

  const showViewAllSessions = computed(() => sessionsList.value.length > SESSION_RAIL_PREVIEW_MAX)

  const sessionFullDialogOpen = ref(false)

  /** 历史会话侧栏/抽屉底部：单聊 | 群聊，默认单聊 */
  const sessionListModeTab = ref<'single' | 'group'>('single')

  watch(sessionListModeTab, (mode) => {
    // 从群聊切到单聊侧栏时须退出群聊上下文，否则 agentId 为空，侧栏拉不到会话列表（刷新后无缓存列表时表现为「看不到历史」）
    if (mode === 'single' && currentGroup.value) {
      clearCurrentGroup()
      return
    }
    if (mode === 'group' && chatGroups.value.length === 0) {
      void loadChatGroups()
    }
    if (mode === 'single' && chatMode.value === 'agent' && agentId.value != null && agentId.value >= 1) {
      void loadSessions()
    }
  })

  function toggleSessionRailCollapse (): void {
    sessionRailCollapsed.value = !sessionRailCollapsed.value
    LocalStorage.set(SESSION_RAIL_COLLAPSE_KEY, sessionRailCollapsed.value ? '1' : '0')
    LocalStorage.removeItem(SESSION_RAIL_LEGACY)
  }

  async function fetchSessionsPage (offset: number, limit: number): Promise<ChatSession[]> {
    if (agentId.value == null || agentId.value < 1) return []
    const attempts = 3
    for (let i = 0; i < attempts; i++) {
      try {
        const { data } = await api.get<APIResponse<ChatSession[]>>(
          `/chat/sessions?agent_id=${agentId.value}&limit=${limit}&offset=${offset}`
        )
        return (data.data ?? []) as ChatSession[]
      } catch (e: unknown) {
        const err = e as { response?: { status?: number }; message?: string; code?: string }
        const isCanceled = err.code === 'ERR_CANCELED' || err.message?.includes('canceled') || err.message?.includes('Cancel')
        if (isCanceled && i < attempts - 1) {
          await new Promise(resolve => setTimeout(resolve, 100 * (i + 1)))
          continue
        }
        throw e
      }
    }
    return []
  }

  async function fetchSessionsList (): Promise<ChatSession[]> {
    return fetchSessionsPage(0, SESSION_LIST_FETCH_LIMIT)
  }

  /** 拉取某会话的全部历史（多页 offset），与单聊/群聊一致 */
  async function fetchAllSessionMessages (sid: string): Promise<ChatHistoryMessage[]> {
    const all: ChatHistoryMessage[] = []
    let offset = 0
    while (true) {
      const { data } = await api.get<APIResponse<ChatHistoryMessage[]>>(
        `/chat/sessions/${encodeURIComponent(sid)}/messages`,
        { params: { limit: SESSION_MESSAGES_PAGE_SIZE, offset } }
      )
      const batch = (data.data ?? []) as ChatHistoryMessage[]
      all.push(...batch)
      if (batch.length < SESSION_MESSAGES_PAGE_SIZE) break
      offset += batch.length
    }
    return all
  }

  async function loadBrowseMessages (sid: string) {
    browseMessagesLoading.value = true
    try {
      browseMessages.value = await fetchAllSessionMessages(sid)
    } catch (e: unknown) {
      if (isCancel(e)) return
      browseMessages.value = []
    } finally {
      browseMessagesLoading.value = false
    }
  }

  async function loadMoreSessionsBrowse () {
    if (!sessionsBrowseHasMore.value || sessionsBrowseLoadingMore.value || agentId.value == null) return
    sessionsBrowseLoadingMore.value = true
    try {
      const batch = await fetchSessionsPage(sessionsBrowseOffset.value, SESSION_BROWSE_PAGE_SIZE)
      const seen = new Set(sessionsBrowseList.value.map(s => s.session_id))
      for (const s of batch) {
        if (!seen.has(s.session_id)) {
          sessionsBrowseList.value.push(s)
          seen.add(s.session_id)
        }
      }
      sessionsBrowseOffset.value += batch.length
      if (batch.length === 0 || batch.length < SESSION_BROWSE_PAGE_SIZE) {
        sessionsBrowseHasMore.value = false
      }
    } finally {
      sessionsBrowseLoadingMore.value = false
    }
  }

  function onSessionBrowseScroll (info: { verticalPercentage: number }) {
    if (info.verticalPercentage < 0.88) return
    if (browseScrollDebounce != null) return
    browseScrollDebounce = setTimeout(() => {
      browseScrollDebounce = null
      void loadMoreSessionsBrowse()
    }, 200)
  }

  async function selectBrowseSession (sid: string) {
    if (browseSelectedSessionId.value === sid) return
    browseSelectedSessionId.value = sid
    await loadBrowseMessages(sid)
  }

  function openBrowseSessionInChat () {
    const sid = browseSelectedSessionId.value
    if (!sid) return
    void selectSession(sid)
  }

  watch(sessionFullDialogOpen, async (open) => {
    if (!open) {
      browseSelectedSessionId.value = null
      browseMessages.value = []
      return
    }
    sessionsBrowseLoading.value = true
    try {
      if (sessionsList.value.length === 0) {
        const batch = await fetchSessionsPage(0, SESSION_LIST_FETCH_LIMIT)
        sessionsBrowseList.value = batch
        sessionsBrowseOffset.value = batch.length
        sessionsBrowseHasMore.value = batch.length >= SESSION_LIST_FETCH_LIMIT
      } else {
        sessionsBrowseList.value = [...sessionsList.value]
        sessionsBrowseOffset.value = sessionsList.value.length
        sessionsBrowseHasMore.value = sessionsList.value.length >= SESSION_LIST_FETCH_LIMIT
      }
      const pick =
        sessionId.value && sessionsBrowseList.value.some(s => s.session_id === sessionId.value)
          ? sessionId.value
          : sessionsBrowseList.value[0]?.session_id ?? null
      browseSelectedSessionId.value = pick
      if (pick) await loadBrowseMessages(pick)
      else browseMessages.value = []
    } finally {
      sessionsBrowseLoading.value = false
    }
  })

  async function loadSessions () {
    // 群聊没有 agentId；若按「无 agent」清空 sessionId/history，会在刷新 /chat/group/:id 后抹掉刚加载的消息
    if (currentGroup.value != null && currentGroup.value.id > 0) {
      return
    }
    if (agentId.value == null || agentId.value < 1) {
      sessionsList.value = []
      sessionId.value = null
      history.value = []
      lastTurnDurationMs.value = null
      return
    }
    sessionsLoading.value = true
    try {
      const list = await fetchSessionsList()
      sessionsList.value = list
      if (list.length === 0) {
        history.value = []
        // 首次对话时列表仍可能为空：流式进行中、或 refresh 尚未返回。若此时清空 localTurns / sessionId，会抹掉界面上的首条消息与 X-Session-ID，表现为「发完没反应」。
        if (!sending.value && localTurns.value.length === 0) {
          sessionId.value = null
          localTurns.value = []
          lastTurnDurationMs.value = null
        }
        return
      }
      const before = sessionId.value
      const exists = before != null && list.some(s => s.session_id === before)
      if (!exists && list.length > 0 && !suppressSessionListAutoPick.value) {
        sessionId.value = list[0].session_id
        localTurns.value = []
        lastTurnDurationMs.value = null
        await loadMessages()
      }
    } catch (e: unknown) {
      sessionsList.value = []
      sessionId.value = null
      history.value = []
      lastTurnDurationMs.value = null
      const err = e as { response?: { status?: number }; message?: string }
      console.warn('loadSessions failed:', err)
      if (err.response?.status === 503) {
        $q.notify({ type: 'warning', message: '会话服务不可用，请检查数据库配置' })
      }
    } finally {
      sessionsLoading.value = false
    }
  }

  async function refreshSessionsList () {
    if (chatMode.value !== 'agent' || agentId.value == null || agentId.value < 1) return
    try {
      sessionsList.value = await fetchSessionsList()
    } catch {
      /* ignore */
    }
  }

  async function selectSession (sid: string) {
    if (sessionId.value === sid) {
      sessionDrawerOpen.value = false
      sessionFullDialogOpen.value = false
      return
    }
    currentGroup.value = null
    sessionListModeTab.value = 'single'
    // 群聊时 agentId 为空，侧栏 sessionsList 未拉取；必须从接口取 session 才能设置 agentId，否则 URL 仍停在 /chat/<群会话uuid>，syncAgentIdToRoute 不生效
    let sess = sessionsList.value.find(s => s.session_id === sid)
    if (!sess) {
      try {
        const { data } = await api.get<APIResponse<ChatSession>>(
          `/chat/sessions/${encodeURIComponent(sid)}`
        )
        sess = data.data as ChatSession
      } catch {
        /* 无权限或不存在 */
      }
    }
    if (!sess) {
      $q.notify({ type: 'warning', message: t('chatSessionNotFound') })
      void refreshSessionsList()
      return
    }
    suppressSessionListAutoPick.value = true
    try {
      // 先写 sessionId，再写 agentId，避免 watch(agentId)→loadSessions 在旧 sessionId 上抢改 list[0]
      sessionId.value = sid
      localTurns.value = []
      lastTurnDurationMs.value = null
      if (sess.agent_id >= 1) {
        agentId.value = sess.agent_id
      }
      await loadMessages()
    } finally {
      suppressSessionListAutoPick.value = false
    }
    sessionDrawerOpen.value = false
    sessionFullDialogOpen.value = false
    syncAgentIdToRoute()
  }

  async function loadMessages () {
    if (!sessionId.value) return
    try {
      history.value = await fetchAllSessionMessages(sessionId.value)
    } catch (e: unknown) {
      // axios 拦截器对同 URL 重复请求会取消上一次；勿清空 history、勿当错误打日志
      if (isCancel(e)) return
      history.value = []
      const err = e as { response?: { status?: number } }
      console.warn('loadMessages failed:', err)
      if (err.response?.status === 404) {
        $q.notify({ type: 'warning', message: t('chatSessionNotFound') })
      }
      if (err.response?.status === 503) {
        $q.notify({ type: 'warning', message: '无法加载聊天记录' })
      }
    }
    await nextTick()
    syncThoughtSidebarFromLoadedHistory()
  }

  async function createSession () {
    if (chatMode.value === 'agent') {
      if (agentId.value == null || agentId.value < 1) return
      sessionBusy.value = true
      try {
        const { data } = await api.post<APIResponse<ChatSession>>('/chat/sessions', { agent_id: agentId.value })
        const sess = data.data as ChatSession
        sessionId.value = sess.session_id
        localTurns.value = []
        history.value = []
        lastTurnDurationMs.value = null
        clearThoughtSteps()
        await refreshSessionsList()
        $q.notify({ type: 'positive', message: '已创建会话' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string }; status?: number } }
        if (err.response?.status === 503) {
          $q.notify({ type: 'warning', message: '暂无法创建会话' })
        } else {
          $q.notify({ type: 'negative', message: err.response?.data?.message ?? '创建会话失败' })
        }
      } finally {
        sessionBusy.value = false
      }
    } else {
      if (workflowId.value == null || workflowId.value < 1) return
      sessionBusy.value = true
      try {
        const { data } = await api.post<APIResponse<ChatSession>>('/chat/sessions', { workflow_id: workflowId.value })
        const sess = data.data as ChatSession
        sessionId.value = sess.session_id
        localTurns.value = []
        history.value = []
        lastTurnDurationMs.value = null
        clearThoughtSteps()
        $q.notify({ type: 'positive', message: '已创建工作流会话' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string }; status?: number } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? '创建工作流会话失败' })
      } finally {
        sessionBusy.value = false
      }
    }
  }

  async function createSessionFromSidebar () {
    if (chatMode.value === 'agent') {
      currentGroup.value = null
    }
    await createSession()
    if (chatMode.value === 'agent') {
      syncAgentIdToRoute()
    }
  }

  function onSessionRailAddClick () {
    if (sessionListModeTab.value === 'group') {
      groupDialogOpen.value = true
    } else {
      void createSessionFromSidebar()
    }
  }

  async function sendStream (text: string, fileUrls?: string[], imageUrlsForHistory?: string[]) {
    const baseURL = (api.defaults.baseURL ?? '/api/v1').replace(/\/$/, '')
    const isGroupChat = !!(currentGroup.value && currentGroup.value.id > 0)
    const url = buildChatStreamUrl(baseURL, isGroupChat)
    const token = LocalStorage.getItem('access') as string | null

    const {
      assistantIdx,
      assistantEntries,
      groupAssistantBlockStartIdx,
      pendingAgentIds,
      ensureAssistantRow
    } = createAssistantStreamSetup(isGroupChat, text, agents.value, localTurns)

    const groupNoAgentReply = isGroupChat && pendingAgentIds.length === 0
    thoughtStatus.value = groupNoAgentReply ? 'completed' : 'running'
    lastTurnDurationMs.value = null
    if (!groupNoAgentReply) {
      clearThoughtSteps()
    }

    const parseStreamEvents = createParseStreamEvents({
      isGroupChat,
      groupAssistantBlockStartIdx,
      pendingAgentIds,
      agents,
      localTurns,
      assistantEntries,
      processStreamEvent,
      processReActEvent,
      thoughtSteps,
      currentStreamModelName,
      appendThoughtText,
      scrollChatToBottom,
      ensureAssistantRow,
      lastServerToolResultText
    })

    const { runStreamFetch } = createSendStreamLifecycle({
      isGroupChat,
      assistantIdx,
      pendingAgentIds,
      assistantEntries,
      localTurns,
      thoughtStatus,
      thoughtSteps,
      sessionId,
      currentGroup,
      lastTurnDurationMs,
      parseStreamEvents,
      scrollChatToBottom,
      t,
      setStoredGroupSessionId,
      setStreamAbortController: (ac) => { streamAbortController = ac }
    })

    const { body, headers } = buildChatStreamHttpRequest({
      text,
      sessionId: sessionId.value,
      isGroupChat,
      currentGroup: currentGroup.value,
      agentId: agentId.value,
      imageUrlsForHistory,
      fileUrls,
      agents: agents.value,
      token
    })

    const ac = new AbortController()
    await runStreamFetch({ url, headers, body, ac })
    // 流结束：不在此处 stopTypewriter / 同步全文，否则正常流结束时会在同一轮取消 rAF，打字机失效（见 applyParsed isFinal）。
  }

  /** Enter 发送；Shift+Enter 换行；IME 组字时 Enter 不发送（与桌面端一致）。群聊时 @ 成员菜单优先。 */
  function onComposerKeydown (e: KeyboardEvent) {
    if (currentGroup.value?.id && mentionOpen.value) {
      const list = mentionFiltered.value
      if (e.key === 'ArrowDown') {
        if (list.length > 0) {
          e.preventDefault()
          mentionIndex.value = (mentionIndex.value + 1) % list.length
        }
        return
      }
      if (e.key === 'ArrowUp') {
        if (list.length > 0) {
          e.preventDefault()
          mentionIndex.value = (mentionIndex.value - 1 + list.length) % list.length
        }
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        mentionOpen.value = false
        return
      }
      if (e.key === 'Tab' && list.length > 0) {
        e.preventDefault()
        insertMentionByAgent(list[mentionIndex.value])
        return
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        if (e.isComposing) return
        if (list.length > 0) {
          e.preventDefault()
          insertMentionByAgent(list[mentionIndex.value])
          return
        }
        mentionOpen.value = false
      }
    }
    onChatInputEnterToSend(e, send)
  }

  async function send () {
    if (!canSend.value) return
    const text = draft.value.trim()
    const imgs = [...pendingImages.value]
    const files = [...pendingFiles.value]
    const streamMessage = computeStreamMessageLabel(text, imgs, files, t)
    const userLabel = streamMessage
    draft.value = ''
    pendingImages.value = []
    pendingFiles.value = []
    const lenBeforeUser = localTurns.value.length
    localTurns.value.push({
      role: 'user',
      content: userLabel,
      displayedContent: userLabel,
      image_data_urls: imgs.map(i => i.dataUrl),
      image_urls: [],
      file_urls: [],
      createdAt: new Date().toISOString()
    })
    sending.value = true
    try {
      const { image_urls: uploadedImageUrls, file_urls: uploadedFileUrls } = await uploadPendingChatAttachments(
        imgs,
        files,
        uploadFile,
        {
          t,
          onPartialImageUpload: () => {
            $q.notify({ type: 'warning', message: t('chatPartialImageUploadFailed') })
          }
        }
      )
      const lastUser = localTurns.value[localTurns.value.length - 1]
      lastUser.image_urls = uploadedImageUrls
      lastUser.image_data_urls = undefined
      lastUser.file_urls = uploadedFileUrls
      await sendStream(streamMessage, uploadedFileUrls, uploadedImageUrls)
      await refreshSessionsList()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } }; message?: string }
      while (localTurns.value.length > lenBeforeUser) {
        localTurns.value.pop()
      }
      draft.value = text
      pendingImages.value = imgs
      pendingFiles.value = files
      const msg =
        err.response?.data?.message ??
        (e instanceof Error ? e.message : null) ??
        '请求失败'
      $q.notify({ type: 'negative', message: msg, timeout: 9000 })
    } finally {
      sending.value = false
    }
  }

  async function stopStream () {
    if (!sending.value) return
    stopping.value = true
    try {
      streamAbortController?.abort()
      const sid = sessionId.value
      if (sid) {
        const token = LocalStorage.getItem('access') as string | null
        await api.post('/chat/stop', { session_id: sid }, {
          headers: token ? { Authorization: `Bearer ${token}` } : {}
        })
      }
    } catch {
      // ignore stop errors
    } finally {
      finalizeRunningPlanTasksOnStop()
      thoughtStatus.value = 'completed'
      stopping.value = false
      sending.value = false
    }
  }

  watch(agentId, () => {
    localTurns.value = []
    lastTurnDurationMs.value = null
    if (agentId.value != null && agentId.value >= 1) {
      const ag = agents.value.find(a => a.id === agentId.value)
      if (ag?.public_id) {
        LocalStorage.set(LAST_AGENT_PUBLIC_ID_KEY, ag.public_id)
      }
      syncAgentIdToRoute()
      void loadSessions()
    }
  })

  watch(
    () => route.params.agentId,
    () => {
      if (route.name === 'chat-group') return
      if (normalizeRouteQuery(route.query[ROUTE_GROUP_Q] as string | string[] | undefined)) return
      if (normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)) return
      const rawStr = routePathSegment()
      if (chatMode.value !== 'agent' || agents.value.length === 0) return
      // UUID 形路径可能是会话 id（由 applyChatRoute GET 解析），也可能是智能体 public_id
      if (rawStr !== '' && isLikelySessionUUID(rawStr)) {
        const found = findAgentFromRouteParam(rawStr, agents.value)
        if (!found) return
        if (currentGroup.value) {
          currentGroup.value = null
        }
        if (agentId.value !== found.id) {
          agentId.value = found.id
        }
        return
      }
      const found = findAgentFromRouteParam(rawStr, agents.value)
      if (!found) return
      if (currentGroup.value) {
        currentGroup.value = null
      }
      if (agentId.value !== found.id) {
        agentId.value = found.id
      }
      if ((found.public_id || '').trim() !== '' && /^\d+$/.test(rawStr.trim())) {
        void router.replace({ name: 'chat', params: { agentId: found.public_id }, query: {} })
      }
    }
  )

  watch(
    () =>
      `${String(route.name)}\x00${route.name === 'chat-group' ? routeParamSessionId() : ''}\x00${routePathSegment()}\x00${normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)}\x00${normalizeRouteQuery(route.query[ROUTE_GROUP_Q] as string | string[] | undefined)}`,
    async () => {
      if (route.name !== 'chat' && route.name !== 'chat-group') return
      if (chatGroups.value.length === 0) await loadChatGroups()
      await applyChatRouteAfterLoadGroups()
    }
  )

  watch(sessionId, () => {
    if (!currentGroup.value?.id) return
    syncGroupChatToRoute()
  })

  watch(workflowId, () => {
    localTurns.value = []
    lastTurnDurationMs.value = null
  })

  watch(chatMode, m => {
    if (m === 'workflow') {
      sessionsList.value = []
      sessionDrawerOpen.value = false
    } else {
      syncAgentIdToRoute()
      void loadSessions()
    }
  })

  function onDocumentVisibility () {
    if (document.visibilityState !== 'visible') return
    void loadSkillRiskLookup()
    if (chatMode.value !== 'agent' || agentId.value == null || agentId.value < 1) return
    void refreshSessionsList()
  }

  function onEscStopStream (e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    if (!sending.value || stopping.value) return
    e.preventDefault()
    void stopStream()
  }

  onMounted(async () => {
    await loadAgents()
    try {
      const { data } = await api.get<{ user: { username: string } }>('/auth/me')
      currentUserLabel.value = (data.user?.username ?? '').trim()
    } catch {
      currentUserLabel.value = ''
    }
    void loadWorkflows()
    await loadChatGroups()
    await applyChatRouteAfterLoadGroups()
    void loadSessions()
    void loadSkillRiskLookup()
    document.addEventListener('visibilitychange', onDocumentVisibility)
    window.addEventListener('keydown', onEscStopStream)
  })

  onUnmounted(() => {
    document.removeEventListener('visibilitychange', onDocumentVisibility)
    window.removeEventListener('keydown', onEscStopStream)
  })

  watch(
    displayMessages,
    () => {
      scrollChatToBottom()
    },
    { deep: true }
  )

  watch(sending, v => {
    if (!v) scrollChatToBottom()
  })

  function promptRenameSession (s: ChatSession) {
    $q.dialog({
      title: t('renameSession'),
      message: t('renameSessionHint'),
      prompt: {
        model: sessionTitle(s),
        type: 'text',
        maxlength: 512,
        isValid: (val: string) => val.trim().length <= 512
      },
      cancel: true,
      persistent: true
    }).onOk((newTitle: string) => {
      void renameSession(s.session_id, newTitle)
    })
  }

  function confirmDeleteSession (s: ChatSession) {
    $q.dialog({
      title: t('confirmDelete'),
      message: t('deleteChatSessionConfirm'),
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' },
      persistent: true
    }).onOk(() => {
      void deleteSession(s.session_id)
    })
  }

  async function deleteSession (sid: string) {
    try {
      await api.delete(`/chat/sessions/${encodeURIComponent(sid)}`)
      await loadSessions()
      if (sessionId.value === sid) {
        sessionId.value = null
      }
      $q.notify({ type: 'positive', message: t('deleteSuccess') })
    } catch (e) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || t('deleteFailed') })
    }
  }

  async function renameSession (sid: string, title: string) {
    try {
      await api.put(`/chat/sessions/${encodeURIComponent(sid)}`, { title })
      await refreshSessionsList()
      const upd = sessionsList.value.find(s => s.session_id === sid)
      if (upd) {
        const idx = sessionsBrowseList.value.findIndex(s => s.session_id === sid)
        if (idx >= 0) sessionsBrowseList.value[idx] = { ...upd }
      }
      $q.notify({ type: 'positive', message: t('saveSuccess') })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    }
  }

  /** 助手气泡尚无可见正文时显示等待动画。群聊可有多条助手行，各自在出字前显示波浪。 */
  function isAssistantTypingSlot (idx: number): boolean {
    if (!sending.value) return false
    const msgs = displayMessages.value
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    const text = ((m as { displayedContent?: string }).displayedContent || m.content || '').trim()
    if (text !== '') return false
    // 单聊仍只让「最后一条」显示动画，避免历史里空壳气泡误显
    const isGroup = !!(currentGroup.value && currentGroup.value.id > 0)
    if (!isGroup && idx !== msgs.length - 1) return false
    // Plan-and-execute：已有执行计划清单时主气泡应展示 PlanExecutePanel（与桌面端一致），
    // 不能仅用打字占位挡住 v-else-if，否则第一次流式中步骤状态/详情无法在主气泡更新。
    if (idx === msgs.length - 1) {
      for (let i = thoughtSteps.value.length - 1; i >= 0; i--) {
        const s = thoughtSteps.value[i]
        if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
          const tasks = (s.data.tasks as unknown[] | undefined) ?? []
          if (tasks.length > 0) return false
        }
      }
    }
    return true
  }

  /** 从 thoughtSteps 中提取当前流式中的 plan 任务列表 */
  function getCurrentPlanTasks (): { index: number; task: string; status: 'pending' | 'running' | 'done' | 'error'; details?: { text: string; tone: 'error' | 'muted' }[] }[] {
    for (let i = thoughtSteps.value.length - 1; i >= 0; i--) {
      const s = thoughtSteps.value[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        return (s.data.tasks as { index: number; task: string; status: 'pending' | 'running' | 'done' | 'error'; details?: { text: string; tone: 'error' | 'muted' }[] }[]) || []
      }
    }
    return []
  }

  /** 与 getCurrentPlanTasks 一致：取最后一条 plan_execute（避免 react_steps 里多条清单时主气泡显示旧的） */
  function getPlanExecuteTasksFromMessage (m: { reactSteps?: ChatReactStep[] }): PlanTaskRowWeb[] {
    const rs = m.reactSteps
    if (!rs?.length) return []
    for (let i = rs.length - 1; i >= 0; i--) {
      const s = rs[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        const raw = (s.data.tasks as PlanTaskRowWeb[]) || []
        return raw.map((r) => ({ ...r }))
      }
    }
    return []
  }

  const GENERIC_STREAM_FAILURE_ZH = '抱歉，本次回复未能生成。请稍后重试。'

  function isGenericStreamFailureText (s: string): boolean {
    const t = String(s ?? '').replace(/\s+/g, '').trim()
    if (t === '') return true
    return t === GENERIC_STREAM_FAILURE_ZH.replace(/\s+/g, '')
  }

  /** 同一轮若有多条 assistant 历史消息带 plan，只显示最后那一张；流式中则由底部当前 plan 卡接管。 */
  function shouldRenderPlanExecuteForMessage (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    if (getPlanExecuteTasksFromMessage(m).length === 0) return false
    for (let i = idx + 1; i < msgs.length; i++) {
      const next = msgs[i]
      if (next.role === 'user') return true
      if (next.role === 'assistant' && getPlanExecuteTasksFromMessage(next).length > 0) return false
    }
    if (sending.value && getCurrentPlanTasks().length > 0) return false
    return true
  }

  /** 被后续计划卡覆盖且仅含通用失败占位文案的 assistant 历史消息整条隐藏。 */
  function shouldHideAssistantPlanMessage (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, content?: string, displayedContent?: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    if (getPlanExecuteTasksFromMessage(m).length === 0) return false
    if (shouldRenderPlanExecuteForMessage(idx)) return false
    const text = String(m.displayedContent || m.content || '')
    return isGenericStreamFailureText(text)
  }

  /** 计划卡存在时，隐藏其下方的通用失败占位文案（与桌面端一致）。 */
  function shouldHideAssistantMessageText (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, content?: string, displayedContent?: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    const text = String(m.displayedContent || m.content || '')
    if (!isGenericStreamFailureText(text)) return false
    const hasHistoryPlan = getPlanExecuteTasksFromMessage(m).length > 0
    const hasStreamingPlan = sending.value && idx === msgs.length - 1 && getCurrentPlanTasks().length > 0
    return hasHistoryPlan || hasStreamingPlan
  }

  function userMessageTextToDisplay (m: Parameters<typeof userMessageTextToDisplayI18n>[0]): string {
    return userMessageTextToDisplayI18n(m, t)
  }

  function openImagePreview (url: string): void {
    const src = resolveChatImageUrl(url)
    if (!isSafeImagePreviewSrc(src)) {
      $q.notify({ type: 'warning', message: t('chatImagePreviewUnsafe') })
      return
    }
    $q.dialog({
      component: ChatImagePreviewDialog,
      componentProps: { src }
    })
  }

  function openFilePreview (url: string): void {
    window.open(url, '_blank')
  }

  return {
    t,
    chatMode,
    agents,
    agentsLoading,
    workflowsLoading,
    agentId,
    workflowId,
    agentOptions,
    workflowOptions,
    sessionId,
    sessionBusy,
    sessionsList,
    sessionsLoading,
    sessionDrawerOpen,
    selectSession,
    formatSessionTime,
    formatChatMessageTime,
    chatMessageTimeLabel,
    chatDateDividerText,
    messageDateDividerAt,
    sessionTitle,
    promptRenameSession,
    confirmDeleteSession,
    deleteSession,
    showSessionRail,
    sessionRailCollapsed,
    sessionGroupBlocks,
    sessionRailPreviewBlocks,
    showViewAllSessions,
    sessionFullDialogOpen,
    sessionListModeTab,
    createSessionFromSidebar,
    onSessionRailAddClick,
    sessionBrowseGroupBlocks,
    sessionsBrowseList,
    sessionsBrowseLoading,
    sessionsBrowseLoadingMore,
    sessionsBrowseHasMore,
    browseSelectedSessionId,
    browseMessages,
    browseMessagesForDisplay,
    browseMessagesLoading,
    onSessionBrowseScroll,
    selectBrowseSession,
    openBrowseSessionInChat,
    toggleSessionRailCollapse,
    displayMessages,
    sending,
    stopping,
    draft,
    composerInputRef,
    mentionOpen,
    mentionFiltered,
    mentionIndex,
    insertMentionByAgent,
    onComposerKeyup,
    chatScrollRef,
    fillDraft,
    pendingImages,
    maxChatImages,
    imageInputRef,
    clearPendingImages,
    removePendingImageAt,
    onImageSelected,
    onComposerPaste,
    onComposerDragOver,
    onComposerDrop,
    chatImageInputAccept: CHAT_IMAGE_INPUT_ACCEPT,
    chatDocumentInputAccept: CHAT_DOCUMENT_INPUT_ACCEPT,
    canSend,
    canStartSession,
    createSession,
    onComposerKeydown,
    send,
    stopStream,
    thoughtSidebarOpen,
    thoughtSteps,
    thoughtStatus,
    toggleThoughtSidebar,
    currentStreamModelName,
    lastTurnDurationMs,
    isAssistantTypingSlot,
    getCurrentPlanTasks,
    getPlanExecuteTasksFromMessage,
    shouldRenderPlanExecuteForMessage,
    shouldHideAssistantPlanMessage,
    shouldHideAssistantMessageText,
    pendingFiles,
    fileInputRef,
    clearPendingFiles,
    removePendingFile,
    formatPendingFileSize,
    onFileSelected,
    uploadFile,
    getFileIcon,
    getFileName,
    resolveChatImageUrl,
    userMessageImageUrls,
    userMessageTextToDisplay,
    openImagePreview,
    openFilePreview,
    chatGroups,
    groupChatRailBlocks,
    groupCaptionTime,
    currentGroup,
    groupDialogOpen,
    groupForm,
    loadChatGroups,
    createChatGroup,
    deleteChatGroup,
    selectChatGroup,
    clearCurrentGroup,
    currentUserLabel,
    groupInviteDialogOpen,
    groupInviteAgentIds,
    groupInviteSelectOptions,
    openGroupInviteDialog,
    submitGroupInvite
  }
}

export type UseChatPageReturn = ReturnType<typeof createChatPageState>

export function useChatPage (): UseChatPageReturn {
  return createChatPageState()
}
