export interface NodeConfigField {
  name: string
  label: string
  type: 'input' | 'number' | 'textarea' | 'select' | 'password'
  key: string
  options?: Array<{ label: string; value: string }>
  placeholder?: string
  rows?: number
  min?: number
  max?: number
  step?: number
}

export interface NodeDefinition {
  value: string
  label: string
  icon: string
  color: string
  desc: string
  category: string
  defaultConfig: Record<string, unknown>
  configFields: NodeConfigField[]
}
