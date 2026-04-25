import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type {
  APIResponse, WorkflowDefinition, WorkflowNode, WorkflowEdge,
  CreateWorkflowDefinitionRequest, UpdateWorkflowDefinitionRequest,
  WorkflowExecution, ExecuteWorkflowResponse
} from 'src/api/types'
import type { Connection, Edge, Node } from '@vue-flow/core'
import { getNodeType } from './workflow-nodes/index'
import { getDefaultConfig } from './workflow-nodes/configs'

let nodeCounter = 0

export function newWorkflowKey (): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID().replace(/-/g, '')
  }
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    const bytes = new Uint8Array(16)
    crypto.getRandomValues(bytes)
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  }
  return `wf${Date.now().toString(36)}${Math.random().toString(36).slice(2, 14)}`
}

export function useWorkflowEditor () {
  const { t } = useI18n()
  const $q = useQuasar()

  const loading = ref(false)
  const saving = ref(false)
  const errorMsg = ref('')
  const activeTab = ref('list')
  const workflowList = ref<WorkflowDefinition[]>([])

  const currentWorkflow = ref<WorkflowDefinition | null>(null)
  const workflowName = ref('')
  const workflowKey = ref('')
  const workflowDesc = ref('')

  const nodes = ref<any[]>([])
  const edges = ref<any[]>([])
  const selectedNodeId = ref<string | null>(null)
  const configPanelOpen = ref(false)

  const selectedNode = ref<any | null>(null)

  const selectedNodeData = ref<Record<string, unknown>>({})

  watch(selectedNodeId, (newId) => {
    if (!newId) {
      selectedNodeData.value = {}
      return
    }
    const node = nodes.value.find(n => n.id === newId)
    selectedNodeData.value = node ? { ...node.data } : {}
  }, { immediate: true })

  function updateSelectedNodeData (updater: (data: Record<string, unknown>) => Record<string, unknown>) {
    if (!selectedNodeId.value) return
    const node = nodes.value.find(n => n.id === selectedNodeId.value)
    if (node) {
      const next = updater({ ...(node.data as Record<string, unknown>) })
      node.data = next
      selectedNodeData.value = { ...next }
    }
  }

  async function loadWorkflows () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<WorkflowDefinition[]>>('/workflows/graph')
      workflowList.value = (data.data ?? []) as WorkflowDefinition[]
    } catch (e: unknown) {
      workflowList.value = []
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('wfLoadFailed')
    } finally {
      loading.value = false
    }
  }

  function openEditor (wf?: WorkflowDefinition) {
    if (wf) {
      currentWorkflow.value = wf
      workflowName.value = wf.name
      workflowKey.value = wf.key
      workflowDesc.value = wf.description || ''
      nodes.value = wf.nodes.map(n => {
        const cfg = (n.config || {}) as Record<string, unknown>
        let agentId: number | null = n.agent_id != null && n.agent_id >= 1 ? n.agent_id : null
        if (agentId == null && n.type === 'agent') {
          for (const key of ['agent_id', 'agentId'] as const) {
            const raw = cfg[key]
            if (raw != null && raw !== '' && raw !== 0) {
              const num = typeof raw === 'number' ? raw : Number(raw)
              if (!Number.isNaN(num) && num >= 1) {
                agentId = num
                break
              }
            }
          }
        }
        return {
          id: n.id,
          type: 'default',
          position: n.position || { x: 100, y: 100 },
          data: {
            label: n.label,
            nodeType: n.type,
            agentId,
            config: n.config || {},
            inputSchema: n.input_schema,
            outputSchema: n.output_schema
          }
        }
      }) as Node[]
      edges.value = wf.edges.map(e => ({
        id: e.id,
        source: e.source_node_id,
        target: e.target_node_id,
        sourceHandle: e.source_port || undefined,
        targetHandle: e.target_port || undefined,
        label: e.label || undefined,
        animated: e.condition ? true : undefined,
        style: e.condition ? { stroke: '#FF9800' } : undefined,
        data: { condition: e.condition }
      })) as Edge[]
    } else {
      currentWorkflow.value = null
      workflowName.value = ''
      workflowKey.value = newWorkflowKey()
      workflowDesc.value = ''
      nodes.value = []
      edges.value = []
    }
    activeTab.value = 'editor'
  }

  function addNode (type: string, position: { x: number; y: number }) {
    nodeCounter++
    const id = `node_${nodeCounter}_${Date.now()}`
    const nodeTypeInfo = getNodeType(type)
    const label = nodeTypeInfo?.label || type

    const newNode: Node = {
      id,
      type: 'default',
      position,
      data: {
        label: `${label}`,
        nodeType: type,
        agentId: null,
        config: { ...getDefaultConfig(type) },
        inputSchema: null,
        outputSchema: null
      }
    }

    nodes.value = [...nodes.value, newNode]
    selectedNodeId.value = id
    configPanelOpen.value = true
  }

  function onNodeClick (_event: MouseEvent, node: Node) {
    selectedNodeId.value = node.id
    configPanelOpen.value = true
  }

  function onPaneClick () {
    selectedNodeId.value = null
    configPanelOpen.value = false
  }

  const onEdgeClick = () => {
    selectedNodeId.value = null
    configPanelOpen.value = false
  }

  function onConnect (connection: Connection) {
    const exists = edges.value.some(
      e => e.source === connection.source && e.target === connection.target
    )
    if (exists) return

    const edge: Edge = {
      id: `edge_${connection.source}_${connection.target}_${Date.now()}`,
      source: connection.source,
      target: connection.target,
      sourceHandle: connection.sourceHandle || undefined,
      targetHandle: connection.targetHandle || undefined,
      animated: true
    }
    edges.value = [...edges.value, edge]
  }

  function onNodesChange (changes: any[]) {
    for (const change of changes) {
      if (change.type === 'position' && 'position' in change) {
        const node = nodes.value.find(n => n.id === change.id)
        if (node) {
          node.position = {
            x: change.position.x,
            y: change.position.y
          }
        }
      }
    }
  }

  function deleteSelectedNode () {
    if (!selectedNodeId.value) return
    deleteNodeById(selectedNodeId.value)
  }

  function deleteNodeById (nodeId: string) {
    if (!nodeId) return
    if (!nodes.value.some(n => n.id === nodeId)) return
    nodes.value = nodes.value.filter(n => n.id !== nodeId)
    edges.value = edges.value.filter(
      e => e.source !== nodeId && e.target !== nodeId
    )
    if (selectedNodeId.value === nodeId) {
      selectedNodeId.value = null
      configPanelOpen.value = false
    }
  }

  function deleteEdge (edgeId: string) {
    edges.value = edges.value.filter(e => e.id !== edgeId)
  }

  /** Top-level agent_id is required by the API; also accept config.agent_id when data.agentId was never synced. */
  function resolveNodeAgentId (d: Record<string, unknown>, nodeType: string): number | undefined {
    const fromData = d.agentId
    if (fromData != null && fromData !== '') {
      const n = typeof fromData === 'number' ? fromData : Number(fromData)
      if (!Number.isNaN(n) && n >= 1) return n
    }
    if (nodeType === 'agent') {
      const cfg = (d.config as Record<string, unknown>) || {}
      for (const key of ['agent_id', 'agentId'] as const) {
        const raw = cfg[key]
        if (raw != null && raw !== '' && raw !== 0) {
          const n = typeof raw === 'number' ? raw : Number(raw)
          if (!Number.isNaN(n) && n >= 1) return n
        }
      }
    }
    return undefined
  }

  function validateAgentNodesForExecute (): boolean {
    for (const n of nodes.value) {
      const d = n.data as Record<string, unknown>
      const nodeType = (d.nodeType as string) || ''
      if (nodeType !== 'agent') continue
      if (resolveNodeAgentId(d, nodeType) == null) {
        const label = (d.label as string) || n.id
        $q.notify({
          type: 'negative',
          message: t('wfAgentNodeNeedsAgent', { label })
        })
        return false
      }
    }
    return true
  }

  function buildSaveBody (): CreateWorkflowDefinitionRequest | UpdateWorkflowDefinitionRequest | null {
    if (!workflowName.value?.trim()) {
      $q.notify({ type: 'negative', message: t('wfNameRequired') })
      return null
    }
    if (nodes.value.length === 0) {
      $q.notify({ type: 'negative', message: t('wfNeedOneNode') })
      return null
    }
    const key = (workflowKey.value || '').trim() || newWorkflowKey()
    workflowKey.value = key
    return {
      key,
      name: workflowName.value.trim(),
      description: workflowDesc.value.trim(),
      kind: 'graph',
      nodes: toWorkflowNodes(),
      edges: toWorkflowEdges(),
      is_active: true
    }
  }

  /** Persist graph to server. When silent, does not switch tab or show success toast (for run-before-save). */
  async function persistWorkflowGraph (options: { silent?: boolean } = {}): Promise<boolean> {
    const body = buildSaveBody()
    if (!body) return false
    saving.value = true
    try {
      if (currentWorkflow.value) {
        await api.put(`/workflows/graph/${currentWorkflow.value.id}`, body)
        if (!options.silent) {
          $q.notify({ type: 'positive', message: t('saveOk') })
          activeTab.value = 'list'
          await loadWorkflows()
        }
        return true
      }
      const { data } = await api.post<APIResponse<WorkflowDefinition>>('/workflows/graph', body)
      const created = data.data
      if (created) {
        currentWorkflow.value = created
        workflowName.value = created.name
        workflowKey.value = created.key
        workflowDesc.value = created.description || ''
      }
      $q.notify({ type: 'positive', message: t('createOk') })
      activeTab.value = 'list'
      await loadWorkflows()
      return true
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
      return false
    } finally {
      saving.value = false
    }
  }

  function toWorkflowNodes (): WorkflowNode[] {
    return nodes.value.map(n => {
      const d = n.data as Record<string, unknown>
      const nodeType = (d.nodeType as string) || 'agent'
      return {
        id: n.id,
        type: nodeType,
        label: (d.label as string) || '',
        agent_id: resolveNodeAgentId(d, nodeType),
        config: (d.config as Record<string, unknown>) || {},
        position: n.position
          ? { x: n.position.x, y: n.position.y }
          : undefined,
        input_schema: d.inputSchema as Record<string, unknown> | undefined,
        output_schema: d.outputSchema as Record<string, unknown> | undefined
      }
    })
  }

  function toWorkflowEdges (): WorkflowEdge[] {
    return edges.value.map(e => {
      const lbl = e.label
      let labelStr: string | undefined
      if (typeof lbl === 'string') {
        labelStr = lbl || undefined
      }
      return {
        id: e.id,
        source_node_id: e.source,
        target_node_id: e.target,
        source_port: e.sourceHandle || undefined,
        target_port: e.targetHandle || undefined,
        condition: (e.data as Record<string, unknown>)?.condition as string | undefined,
        label: labelStr
      }
    })
  }

  async function saveWorkflow () {
    await persistWorkflowGraph({ silent: false })
  }

  function confirmDelete (wf: WorkflowDefinition) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${wf.name} (ID: ${wf.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/workflows/graph/${wf.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await loadWorkflows()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  function onTabChange (tab: string) {
    activeTab.value = tab
    if (tab === 'list') {
      void loadWorkflows()
    }
  }

  const executing = ref(false)
  const executionResult = ref<ExecuteWorkflowResponse | null>(null)
  const executionHistory = ref<WorkflowExecution[]>([])
  const executionDialogOpen = ref(false)

  /** Runs workflow on server; does not open the result dialog (caller may run canvas animation first). */
  async function executeWorkflowDirect (): Promise<ExecuteWorkflowResponse | null> {
    if (!currentWorkflow.value?.id) {
      $q.notify({ type: 'negative', message: t('wfNoWorkflowToExecute') })
      return null
    }
    if (!validateAgentNodesForExecute()) return null
    const saved = await persistWorkflowGraph({ silent: true })
    if (!saved) return null
    executing.value = true
    executionResult.value = null
    try {
      const { data } = await api.post<APIResponse<ExecuteWorkflowResponse>>(
        `/workflows/graph/${currentWorkflow.value.id}/execute`,
        { workflow_id: currentWorkflow.value.id, message: '' }
      )
      const payload = data.data ?? null
      executionResult.value = payload
      $q.notify({ type: 'positive', message: t('wfExecuteSuccess') })
      return payload
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('wfExecuteFailed') })
      return null
    } finally {
      executing.value = false
    }
  }

  async function executeWorkflow (): Promise<ExecuteWorkflowResponse | null> {
    return executeWorkflowDirect()
  }

  async function loadExecutionHistory (workflowId: number) {
    try {
      const { data } = await api.get<APIResponse<WorkflowExecution[]>>(
        `/workflows/graph/${workflowId}/executions`
      )
      executionHistory.value = (data.data ?? []) as WorkflowExecution[]
    } catch {
      executionHistory.value = []
    }
  }

  async function getExecutionDetail (execId: number) {
    try {
      const { data } = await api.get<APIResponse<WorkflowExecution>>(
        `/workflows/graph/executions/${execId}`
      )
      return (data.data ?? null) as WorkflowExecution | null
    } catch {
      return null
    }
  }

  function getUpstreamNodes (): Array<{ label: string; value: string; nodeType: string }> {
    if (!selectedNodeId.value) return []

    const visited = new Set<string>()
    const upstream: Array<{ label: string; value: string; nodeType: string }> = []

    function dfs (nodeId: string) {
      if (visited.has(nodeId)) return
      visited.add(nodeId)

      const node = nodes.value.find(n => n.id === nodeId)
      if (node && node.id !== selectedNodeId.value) {
        const data = node.data as Record<string, unknown>
        upstream.push({
          label: (data.label as string) || node.id,
          value: node.id,
          nodeType: (data.nodeType as string) || ''
        })
      }

      const incomingEdges = edges.value.filter(e => e.target === nodeId)
      for (const edge of incomingEdges) {
        dfs(edge.source)
      }
    }

    const incomingEdges = edges.value.filter(e => e.target === selectedNodeId.value)
    for (const edge of incomingEdges) {
      dfs(edge.source)
    }

    return upstream
  }

  function getNodeOutputFields (nodeId: string): string[] {
    const node = nodes.value.find(n => n.id === nodeId)
    if (!node) return []

    const data = node.data as Record<string, unknown>
    const nodeType = data.nodeType as string
    const outputSchema = data.outputSchema as Record<string, unknown> | undefined

    const defaultFields: Record<string, string[]> = {
      input: ['content'],
      output: ['content'],
      agent: ['content', 'type'],
      llm: ['content', 'type'],
      tool: ['tool', 'input', 'result', 'type'],
      condition: ['result', 'branch'],
      merge: ['outputs', 'count']
    }

    const fields = defaultFields[nodeType] || ['content']

    if (outputSchema?.properties) {
      const props = outputSchema.properties as Record<string, unknown>
      return [...fields, ...Object.keys(props)]
    }

    return fields
  }

  onMounted(() => {
    void loadWorkflows()
  })

  return {
    t,
    loading,
    saving,
    errorMsg,
    activeTab,
    workflowList,
    currentWorkflow,
    workflowName,
    workflowDesc,
    nodes,
    edges,
    selectedNodeId,
    configPanelOpen,
    selectedNode,
    selectedNodeData,
    updateSelectedNodeData,
    openEditor,
    addNode,
    onNodeClick,
    onPaneClick,
    onEdgeClick,
    onConnect,
    onNodesChange,
    deleteSelectedNode,
    deleteNodeById,
    deleteEdge,
    saveWorkflow,
    confirmDelete,
    onTabChange,
    loadWorkflows,
    getUpstreamNodes,
    getNodeOutputFields,
    executing,
    executionResult,
    executionHistory,
    executionDialogOpen,
    executeWorkflow,
    executeWorkflowDirect,
    loadExecutionHistory,
    getExecutionDetail
  }
}
