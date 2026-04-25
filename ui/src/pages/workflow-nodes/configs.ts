import type { NodeConfig } from './types'

export const defaultNodeConfigs: Record<string, NodeConfig> = {
  start: { user_prompt: '', input_schema: {} },
  end: { output_mapping: '' },
  agent: { agent_id: 0, prompt_template: '' },
  llm: { agent_id: 0, prompt: '', system_prompt: '', temperature: 0.7 },
  tool: { tool_name: '', tool_input: '' },
  http: { method: 'GET', url: '', headers: {}, body: '', timeout_ms: 30000 },
  code: { language: 'python', code: '' },
  condition: { condition: '' },
  knowledge: { knowledge_base_id: 0, query: '', top_k: 5 },
  template: { template: '' },
  variable: { assignments: {} },
  merge: { merge_mode: 'all' },
  input: {},
  output: {},
  loop: { mode: 'count', count: 5, max_iterations: 10 },
  branch: { condition: '', true_node: '', false_node: '' },
  parallel: { nodes: '', timeout_ms: 60000 },
  wait: { duration_ms: 1000, condition: '', max_wait_ms: 300000 },
  ssh: { host: '', port: 22, username: '', password: '', command: '', timeout: 30 },
  notify: { channel: 'dingtalk', title: '', message: '', receivers: '' },
  apitest: { url: '', method: 'GET', headers: {}, body: '', assertions: '' },
  datamask: { fields: '', mask_type: 'phone', pattern: '' }
}

export function getDefaultConfig (type: string): NodeConfig {
  return defaultNodeConfigs[type] || {}
}
