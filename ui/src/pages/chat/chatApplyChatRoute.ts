import type { Ref } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'
import { LocalStorage } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse, Agent, ChatGroup, ChatHistoryMessage, ChatSession } from 'src/api/types'
import { findAgentFromRouteParam, LAST_AGENT_PUBLIC_ID_KEY } from './chatRouteHelpers'
import {
  isLikelySessionUUID,
  normalizeRouteQuery,
  ROUTE_GROUP_Q,
  ROUTE_SESSION_Q
} from './chatRouteQuery'

/**
 * 根据路径 `/chat/...`、`/chat/group/...`、`?session=`（重定向）、`?group=` 恢复上下文。
 * 原 useChatPage.applyChatRouteAfterLoadGroups 整段迁入，行为不变。
 */
export type ApplyChatRouteDeps = {
  route: RouteLocationNormalizedLoaded
  router: Router
  routePathSegment: () => string
  routeParamSessionId: () => string
  loadChatGroups: () => Promise<void>
  loadMessages: () => Promise<void>
  loadSessions: () => Promise<void>
  selectChatGroup: (group: ChatGroup) => Promise<void>
  applyAgentFromPublicIdPath: (found: Agent) => void
  clearThoughtSteps: () => void
  syncAgentIdToRoute: () => void
  setStoredGroupSessionId: (groupId: number, sid: string) => void

  agents: Ref<Agent[]>
  chatGroups: Ref<ChatGroup[]>
  currentGroup: Ref<ChatGroup | null>
  sessionId: Ref<string | null>
  agentId: Ref<number | null>
  workflowId: Ref<number | null>
  chatMode: Ref<'agent' | 'workflow'>
  sessionListModeTab: Ref<'single' | 'group'>
  history: Ref<ChatHistoryMessage[]>
  localTurns: Ref<unknown[]>
  lastTurnDurationMs: Ref<number | null>
  draft: Ref<string>
  pendingImages: Ref<unknown[]>
  pendingFiles: Ref<unknown[]>
  sessionDrawerOpen: Ref<boolean>
  sessionFullDialogOpen: Ref<boolean>
}

export async function applyChatRouteAfterLoadGroups (d: ApplyChatRouteDeps): Promise<void> {
  const qSess = normalizeRouteQuery(d.route.query[ROUTE_SESSION_Q] as string | string[] | undefined)
  if (qSess !== '') {
    void d.router.replace({ name: 'chat', params: { agentId: qSess }, query: {} })
    return
  }

  if (d.route.name === 'chat-group') {
    const sid = d.routeParamSessionId().trim()
    if (sid === '' || !isLikelySessionUUID(sid)) {
      void d.router.replace({ path: '/chat', query: {} })
      return
    }
    if (d.chatGroups.value.length === 0) await d.loadChatGroups()
    try {
      const { data } = await api.get<APIResponse<ChatSession>>(
        `/chat/sessions/${encodeURIComponent(sid)}`
      )
      const sess = data.data as ChatSession
      if (sess.group_id == null || sess.group_id < 1) {
        void d.router.replace({ name: 'chat', params: { agentId: sid }, query: {} })
        return
      }
      const group = d.chatGroups.value.find(x => x.id === sess.group_id)
      if (!group) {
        void d.router.replace({ path: '/chat', query: {} })
        return
      }
      if (d.currentGroup.value?.id === group.id && d.sessionId.value === sess.session_id) {
        d.sessionListModeTab.value = 'group'
        return
      }
      d.currentGroup.value = group
      d.agentId.value = null
      d.workflowId.value = null
      d.chatMode.value = 'agent'
      d.sessionId.value = sess.session_id
      d.setStoredGroupSessionId(group.id, sess.session_id)
      d.history.value = []
      d.localTurns.value = []
      d.lastTurnDurationMs.value = null
      d.draft.value = ''
      d.pendingImages.value = []
      d.pendingFiles.value = []
      d.clearThoughtSteps()
      d.sessionDrawerOpen.value = false
      d.sessionFullDialogOpen.value = false
      d.sessionListModeTab.value = 'group'
      void d.loadMessages()
      return
    } catch {
      void d.router.replace({ path: '/chat', query: {} })
      return
    }
  }

  const pathSeg = d.routePathSegment()
  const sidFromPath = pathSeg !== '' && isLikelySessionUUID(pathSeg) ? pathSeg : ''

  if (sidFromPath !== '') {
    const agentFromPath = findAgentFromRouteParam(sidFromPath, d.agents.value)
    if (agentFromPath) {
      d.applyAgentFromPublicIdPath(agentFromPath)
      return
    }
    try {
      const { data } = await api.get<APIResponse<ChatSession>>(
        `/chat/sessions/${encodeURIComponent(sidFromPath)}`
      )
      const sess = data.data as ChatSession
      if (sess.group_id != null && sess.group_id >= 1) {
        const group = d.chatGroups.value.find(x => x.id === sess.group_id)
        if (group) {
          if (d.currentGroup.value?.id === group.id && d.sessionId.value === sess.session_id) {
            if (d.route.name !== 'chat-group' || d.routeParamSessionId() !== sess.session_id) {
              void d.router.replace({ name: 'chat-group', params: { sessionId: sess.session_id } })
            }
            d.sessionListModeTab.value = 'group'
            return
          }
          d.currentGroup.value = group
          d.agentId.value = null
          d.workflowId.value = null
          d.chatMode.value = 'agent'
          d.sessionId.value = sess.session_id
          d.setStoredGroupSessionId(group.id, sess.session_id)
          d.history.value = []
          d.localTurns.value = []
          d.lastTurnDurationMs.value = null
          d.draft.value = ''
          d.pendingImages.value = []
          d.pendingFiles.value = []
          d.clearThoughtSteps()
          d.sessionDrawerOpen.value = false
          d.sessionFullDialogOpen.value = false
          d.sessionListModeTab.value = 'group'
          void d.loadMessages()
          void d.router.replace({ name: 'chat-group', params: { sessionId: sess.session_id } })
          return
        }
      }
      d.currentGroup.value = null
      d.sessionListModeTab.value = 'single'
      if (sess.agent_id >= 1) {
        d.agentId.value = sess.agent_id
      }
      d.sessionId.value = sess.session_id
      d.history.value = []
      d.localTurns.value = []
      d.lastTurnDurationMs.value = null
      d.draft.value = ''
      d.pendingImages.value = []
      d.pendingFiles.value = []
      d.clearThoughtSteps()
      void d.loadMessages()
      d.syncAgentIdToRoute()
      void d.loadSessions()
      return
    } catch {
      // 例如 agents 尚未就绪时未命中上方 agentFromPath；GET 404 后再按 public_id 解析
      const found = findAgentFromRouteParam(sidFromPath, d.agents.value)
      if (found) {
        d.applyAgentFromPublicIdPath(found)
      }
      return
    }
  }

  const gRaw = normalizeRouteQuery(d.route.query[ROUTE_GROUP_Q] as string | string[] | undefined)
  if (gRaw === '') {
    // 从群聊切到无参 /chat 时 loadAgents 不会重跑，agentId 仍为 null，侧栏会话列表无法拉取
    if (
      d.route.name === 'chat' &&
      d.chatMode.value === 'agent' &&
      d.routePathSegment() === '' &&
      !d.currentGroup.value &&
      (d.agentId.value == null || d.agentId.value < 1) &&
      d.agents.value.length > 0
    ) {
      const fromStoragePub = LocalStorage.getItem<string>(LAST_AGENT_PUBLIC_ID_KEY)
      const raw = (fromStoragePub ?? '').trim()
      let picked = raw !== '' ? findAgentFromRouteParam(raw, d.agents.value) : null
      if (!picked) picked = d.agents.value[0]
      if (picked) {
        d.agentId.value = picked.id
        void d.loadSessions()
        d.syncAgentIdToRoute()
      }
    }
    return
  }
  const gid = parseInt(gRaw, 10)
  if (Number.isNaN(gid) || gid < 1) return
  const group = d.chatGroups.value.find(x => x.id === gid)
  if (!group) return
  if (d.currentGroup.value?.id === gid) return
  await d.selectChatGroup(group)
}
