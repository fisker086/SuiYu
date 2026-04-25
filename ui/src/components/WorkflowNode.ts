import { computed } from 'vue'

export interface WorkflowNodeData {
  nodeType: string
  label?: string
  agentId?: number | null
  config?: Record<string, unknown>
  inputSchema?: Record<string, unknown>
  outputSchema?: Record<string, unknown>
}

export function useWorkflowNode (data: WorkflowNodeData, _selected: boolean) {
  const headerColor = computed(() => {
    const type = data.nodeType
    const colors: Record<string, string> = {
      input: '#52C41A',
      output: '#1890FF',
      agent: '#722ED1',
      condition: '#FA8C16',
      merge: '#607D8B',
      llm: '#EB2F96',
      tool: '#8C8C8C'
    }
    return colors[type] || '#999'
  })

  const nodeIcon = computed(() => {
    const type = data.nodeType
    const icons: Record<string, string> = {
      agent: 'smart_toy',
      input: 'input',
      output: 'output',
      condition: 'call_split',
      merge: 'merge_type',
      llm: 'psychology',
      tool: 'build'
    }
    return icons[type] || 'circle'
  })

  const config = computed(() => {
    const c = data.config
    if (c && typeof c === 'object') return c as Record<string, unknown>
    return {}
  })

  const hasPrompt = computed(() => {
    const val = config.value.prompt_template
    return typeof val === 'string' && val.length > 0
  })

  const promptPreview = computed(() => {
    const val = config.value.prompt_template as string | undefined
    if (!val) return ''
    return val.length > 40 ? val.substring(0, 40) + '...' : val
  })

  const hasCondition = computed(() => {
    const val = config.value.condition
    return typeof val === 'string' && val.length > 0
  })

  const conditionText = computed(() => {
    const val = config.value.condition as string
    return val.length > 30 ? val.substring(0, 30) + '...' : val
  })

  const hasToolName = computed(() => {
    const val = config.value.tool_name
    return typeof val === 'string' && val.length > 0
  })

  const hasInputSchema = computed(() => {
    const s = data.inputSchema
    return s && typeof s === 'object' && 'properties' in s
  })

  const inputSchemaFieldCount = computed(() => {
    const s = data.inputSchema as Record<string, unknown> | undefined
    if (!s) return 0
    const p = s.properties as Record<string, unknown> | undefined
    return p ? Object.keys(p).length : 0
  })

  const hasOutputSchema = computed(() => {
    const s = data.outputSchema
    return s && typeof s === 'object' && 'properties' in s
  })

  const outputSchemaFieldCount = computed(() => {
    const s = data.outputSchema as Record<string, unknown> | undefined
    if (!s) return 0
    const p = s.properties as Record<string, unknown> | undefined
    return p ? Object.keys(p).length : 0
  })

  return {
    headerColor,
    nodeIcon,
    config,
    hasPrompt,
    promptPreview,
    hasCondition,
    conditionText,
    hasToolName,
    hasInputSchema,
    inputSchemaFieldCount,
    hasOutputSchema,
    outputSchemaFieldCount
  }
}
