import type { WorkflowNodeCategory, WorkflowNodeType } from './types'

const builtinNodes: WorkflowNodeType[] = [
  { value: 'start', label: 'Start', icon: 'play_arrow', color: '#4caf50', desc: 'Workflow entry', category: 'flow' },
  { value: 'end', label: 'End', icon: 'stop', color: '#f44336', desc: 'Workflow output', category: 'flow' },
  { value: 'condition', label: 'Condition', icon: 'git_branch', color: '#ff5722', desc: 'Branch based on condition', category: 'flow' },
  { value: 'merge', label: 'Merge', icon: 'merge_type', color: '#673ab7', desc: 'Merge multiple branches', category: 'flow' },
  { value: 'loop', label: 'Loop', icon: 'loop', color: '#e91e63', desc: 'Iterate over items or count', category: 'flow' },
  { value: 'branch', label: 'Branch', icon: 'call_split', color: '#ff9800', desc: 'Execute different paths based on condition', category: 'flow' },
  { value: 'parallel', label: 'Parallel', icon: 'parallelize', color: '#673ab7', desc: 'Execute multiple nodes concurrently', category: 'flow' },
  { value: 'wait', label: 'Wait', icon: 'hourglass_empty', color: '#9e9e9e', desc: 'Wait for a duration or condition', category: 'flow' },
  { value: 'agent', label: 'Agent', icon: 'smart_toy', color: '#2196f3', desc: 'Call an AI agent', category: 'ai' },
  { value: 'llm', label: 'LLM', icon: 'psychology', color: '#9c27b0', desc: 'Direct LLM call', category: 'ai' },
  { value: 'knowledge', label: 'Knowledge', icon: 'menu_book', color: '#8bc34a', desc: 'Retrieve from knowledge base', category: 'ai' },
  { value: 'tool', label: 'Tool', icon: 'build', color: '#ff9800', desc: 'Execute a tool/skill', category: 'action' },
  { value: 'http', label: 'HTTP Request', icon: 'http', color: '#00bcd4', desc: 'Make HTTP requests', category: 'action' },
  { value: 'code', label: 'Code', icon: 'code', color: '#607d8b', desc: 'Execute Python/JS code', category: 'action' },
  { value: 'template', label: 'Template', icon: 'description', color: '#795548', desc: 'Transform data with template', category: 'data' },
  { value: 'variable', label: 'Variable', icon: 'data_object', color: '#ffc107', desc: 'Set or update variables', category: 'data' },
  { value: 'ssh', label: 'SSH Execute', icon: 'terminal', color: '#4caf50', desc: 'Remote command execution', category: 'ops' },
  { value: 'notify', label: 'Notification', icon: 'notifications', color: '#ff9800', desc: 'Send notification', category: 'notify' },
  { value: 'apitest', label: 'API Test', icon: 'science', color: '#2196f3', desc: 'Execute API test', category: 'test' },
  { value: 'datamask', label: 'Data Mask', icon: 'visibility_off', color: '#9c27b0', desc: 'Sensitive data masking', category: 'data' }
]

const nodeRegistry = new Map<string, WorkflowNodeType>()

for (const node of builtinNodes) {
  nodeRegistry.set(node.value, node)
}

export function registerNodeType (node: WorkflowNodeType) {
  nodeRegistry.set(node.value, node)
}

export function getNodeType (value: string): WorkflowNodeType | undefined {
  return nodeRegistry.get(value)
}

export function getAllNodeTypes (): WorkflowNodeType[] {
  return Array.from(nodeRegistry.values())
}

export function getNodeTypesByCategory (category: string): WorkflowNodeType[] {
  return builtinNodes.filter(n => n.category === category)
}

export function getCategories (): WorkflowNodeCategory[] {
  const categoryMap = new Map<string, WorkflowNodeCategory>()

  for (const node of builtinNodes) {
    if (!categoryMap.has(node.category)) {
      categoryMap.set(node.category, {
        name: node.category,
        icon: getCategoryIcon(node.category),
        nodes: []
      })
    }
    const cat = categoryMap.get(node.category)
    if (cat) {
      cat.nodes.push(node)
    }
  }

  return Array.from(categoryMap.values())
}

function getCategoryIcon (category: string): string {
  const icons: Record<string, string> = {
    flow: 'account_tree',
    ai: 'smart_toy',
    action: 'build',
    data: 'data_usage',
    ops: 'monitor',
    notify: 'notifications',
    test: 'science'
  }
  return icons[category] || 'widgets'
}

export { builtinNodes }
export type { WorkflowNodeType, WorkflowNodeCategory }
