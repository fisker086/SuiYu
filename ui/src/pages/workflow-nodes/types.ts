export interface WorkflowNodeType {
  value: string
  label: string
  icon: string
  color: string
  desc: string
  category: string
}

export interface WorkflowNodeCategory {
  name: string
  icon: string
  nodes: WorkflowNodeType[]
}

export interface NodeConfig {
  [key: string]: unknown
}

export interface WorkflowNodeDefinition {
  type: WorkflowNodeType
  defaultConfig: NodeConfig
}
