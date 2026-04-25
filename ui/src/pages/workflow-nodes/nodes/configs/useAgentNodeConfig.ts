import { computed, onMounted, ref } from 'vue'
import { api } from 'boot/axios'
import type { APIResponse, Agent } from 'src/api/types'

export type AgentNodeConfigProps = {
  nodeLabel: string
  config: Record<string, unknown>
}

export type AgentNodeConfigEmit = {
  (e: 'update:label', v: string): void
  (e: 'update:config', v: Record<string, unknown>): void
}

/** 绑定智能体下拉：GET /agents */
export function useAgentNodeAgentOptions () {
  const agents = ref<Agent[]>([])
  const loading = ref(false)

  const agentOptions = computed(() =>
    agents.value.map(a => ({ label: a.name, value: a.id }))
  )

  async function loadAgents (): Promise<void> {
    loading.value = true
    try {
      const { data } = await api.get<APIResponse<Agent[]>>('/agents')
      agents.value = (data.data ?? []) as Agent[]
    } catch {
      agents.value = []
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void loadAgents()
  })

  return {
    agents,
    agentOptions,
    loading,
    loadAgents
  }
}

export function useAgentNodeForm (props: AgentNodeConfigProps, emit: AgentNodeConfigEmit) {
  const bindAgentId = computed(() => {
    const v = props.config.agent_id
    if (v == null || v === '' || v === 0) return null
    const n = typeof v === 'number' ? v : Number(v)
    return Number.isNaN(n) ? null : n
  })

  function patchConfig (key: string, value: unknown) {
    emit('update:config', { ...props.config, [key]: value })
  }

  function onAgentId (val: number | null) {
    patchConfig('agent_id', val == null || val === undefined ? 0 : val)
  }

  function strField (key: string): string {
    const v = props.config[key]
    return v == null ? '' : String(v)
  }

  function numField (key: string): number {
    const v = props.config[key]
    if (v == null || v === '') return 0
    const n = typeof v === 'number' ? v : Number(v)
    return Number.isNaN(n) ? 0 : n
  }

  function patchNumber (key: string, raw: string | number | null | undefined) {
    if (raw === '' || raw === null || raw === undefined) {
      patchConfig(key, 0)
      return
    }
    const n = typeof raw === 'number' ? raw : Number(raw)
    patchConfig(key, Number.isNaN(n) ? 0 : n)
  }

  const nodeLabel = computed({
    get: () => props.nodeLabel,
    set: (v) => emit('update:label', v)
  })

  return {
    bindAgentId,
    patchConfig,
    onAgentId,
    strField,
    numField,
    patchNumber,
    nodeLabel
  }
}
