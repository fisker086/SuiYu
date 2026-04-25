const fs = require('fs')
const path = require('path')

const nodes = [
  {
    name: 'Start',
    value: 'start',
    label: 'Start',
    icon: 'play_arrow',
    color: '#4caf50',
    desc: 'Workflow entry',
    category: 'flow',
    configFields: []
  },
  {
    name: 'End',
    value: 'end',
    label: 'End',
    icon: 'stop',
    color: '#f44336',
    desc: 'Workflow output',
    category: 'flow',
    configFields: []
  },
  {
    name: 'Agent',
    value: 'agent',
    label: 'Agent',
    icon: 'smart_toy',
    color: '#2196f3',
    desc: 'Execute AI agent task',
    category: 'ai',
    configFields: [
      // 绑定智能体：下拉数据须来自 /agents（见 configs/useAgentNodeConfig.ts），勿在此处写死 GPT
      { name: 'agentId', label: 'wfBindAgent', type: 'select', key: 'agent_id', options: [{ label: 'GPT-4', value: 'gpt-4' }] },
      { name: 'promptTemplate', label: 'wfPromptTemplate', type: 'textarea', key: 'prompt_template', rows: 6 }
    ]
  },
  {
    name: 'LLM',
    value: 'llm',
    label: 'LLM',
    icon: 'psychology',
    color: '#9c27b0',
    desc: 'Direct LLM call',
    category: 'ai',
    configFields: [
      { name: 'model', label: 'wfModelSelect', type: 'select', key: 'model', options: [{ label: 'GPT-4', value: 'gpt-4' }] },
      { name: 'systemPrompt', label: 'wfLLMSystemPrompt', type: 'textarea', key: 'system_prompt', rows: 3 },
      { name: 'temperature', label: 'wfLLMTemperature', type: 'number', key: 'temperature' }
    ]
  },
  {
    name: 'Knowledge',
    value: 'knowledge',
    label: 'Knowledge',
    icon: 'menu_book',
    color: '#8bc34a',
    desc: 'Retrieve from knowledge base',
    category: 'ai',
    configFields: [
      { name: 'query', label: 'wfKnowledgeQuery', type: 'input', key: 'query' },
      { name: 'topK', label: 'wfKnowledgeTopK', type: 'number', key: 'top_k' }
    ]
  },
  {
    name: 'Condition',
    value: 'condition',
    label: 'Condition',
    icon: 'git_branch',
    color: '#ff5722',
    desc: 'Branch based on condition',
    category: 'flow',
    configFields: [
      { name: 'condition', label: 'wfCondition', type: 'input', key: 'condition' }
    ]
  },
  {
    name: 'Merge',
    value: 'merge',
    label: 'Merge',
    icon: 'merge_type',
    color: '#673ab7',
    desc: 'Merge multiple branches',
    category: 'flow',
    configFields: [
      { name: 'mergeMode', label: 'wfMergeMode', type: 'select', key: 'merge_mode', options: [{ label: 'All', value: 'all' }] }
    ]
  },
  {
    name: 'Tool',
    value: 'tool',
    label: 'Tool',
    icon: 'build',
    color: '#ff9800',
    desc: 'Execute a tool or skill',
    category: 'action',
    configFields: [
      { name: 'toolName', label: 'wfToolName', type: 'input', key: 'tool_name' },
      { name: 'toolInput', label: 'wfToolInput', type: 'textarea', key: 'tool_input', rows: 3 }
    ]
  },
  {
    name: 'Http',
    value: 'http',
    label: 'HTTP Request',
    icon: 'http',
    color: '#00bcd4',
    desc: 'Make HTTP requests',
    category: 'action',
    configFields: [
      { name: 'method', label: 'wfHTTPMethod', type: 'select', key: 'method', options: [{ label: 'GET', value: 'GET' }] },
      { name: 'url', label: 'wfHTTPURL', type: 'input', key: 'url' },
      { name: 'headers', label: 'wfHTTPHeaders', type: 'textarea', key: 'headers', rows: 2 },
      { name: 'body', label: 'wfHTTPBody', type: 'textarea', key: 'body', rows: 3 }
    ]
  },
  {
    name: 'Code',
    value: 'code',
    label: 'Code',
    icon: 'code',
    color: '#607d8b',
    desc: 'Execute Python/JS code',
    category: 'action',
    configFields: [
      { name: 'language', label: 'wfCodeLanguage', type: 'select', key: 'language', options: [{ label: 'Python', value: 'python' }] },
      { name: 'code', label: 'wfCode', type: 'textarea', key: 'code', rows: 8 }
    ]
  },
  {
    name: 'Template',
    value: 'template',
    label: 'Template',
    icon: 'description',
    color: '#795548',
    desc: 'Transform data with template',
    category: 'data',
    configFields: [
      { name: 'template', label: 'wfTemplate', type: 'textarea', key: 'template', rows: 4 }
    ]
  },
  {
    name: 'Variable',
    value: 'variable',
    label: 'Variable',
    icon: 'data_object',
    color: '#ffc107',
    desc: 'Set or update variables',
    category: 'data',
    configFields: [
      { name: 'assignments', label: 'wfVariableAssignments', type: 'textarea', key: 'assignments', rows: 3 }
    ]
  },
  {
    name: 'Ssh',
    value: 'ssh',
    label: 'SSH Execute',
    icon: 'terminal',
    color: '#4caf50',
    desc: 'Remote command execution',
    category: 'ops',
    configFields: [
      { name: 'host', label: 'wfSSHHost', type: 'input', key: 'host' },
      { name: 'port', label: 'wfSSHPort', type: 'number', key: 'port' },
      { name: 'username', label: 'wfSSHUser', type: 'input', key: 'username' },
      { name: 'password', label: 'wfSSHPassword', type: 'password', key: 'password' },
      { name: 'command', label: 'wfSSHCommand', type: 'textarea', key: 'command', rows: 3 },
      { name: 'timeout', label: 'wfSSHTimeout', type: 'number', key: 'timeout' }
    ]
  },
  {
    name: 'Notify',
    value: 'notify',
    label: 'Notification',
    icon: 'notifications',
    color: '#ff9800',
    desc: 'Send notification',
    category: 'notify',
    configFields: [
      { name: 'channel', label: 'wfNotifyChannel', type: 'select', key: 'channel', options: [{ label: '钉钉', value: 'dingtalk' }] },
      { name: 'title', label: 'wfNotifyTitle', type: 'input', key: 'title' },
      { name: 'message', label: 'wfNotifyMessage', type: 'textarea', key: 'message', rows: 3 },
      { name: 'receivers', label: 'wfNotifyReceivers', type: 'input', key: 'receivers' }
    ]
  },
  {
    name: 'ApiTest',
    value: 'apitest',
    label: 'API Test',
    icon: 'science',
    color: '#2196f3',
    desc: 'Execute API test',
    category: 'test',
    configFields: [
      { name: 'method', label: 'wfAPITestMethod', type: 'select', key: 'method', options: [{ label: 'GET', value: 'GET' }] },
      { name: 'url', label: 'wfAPITestURL', type: 'input', key: 'url' },
      { name: 'headers', label: 'wfAPITestHeaders', type: 'textarea', key: 'headers', rows: 2 },
      { name: 'body', label: 'wfAPITestBody', type: 'textarea', key: 'body', rows: 3 },
      { name: 'assertions', label: 'wfAPITestAssertions', type: 'textarea', key: 'assertions', rows: 3 }
    ]
  },
  {
    name: 'DataMask',
    value: 'datamask',
    label: 'Data Mask',
    icon: 'visibility_off',
    color: '#9c27b0',
    desc: 'Sensitive data masking',
    category: 'data',
    configFields: [
      { name: 'fields', label: 'wfDataMaskFields', type: 'input', key: 'fields' },
      { name: 'maskType', label: 'wfDataMaskType', type: 'select', key: 'mask_type', options: [{ label: '手机号', value: 'phone' }] },
      { name: 'pattern', label: 'wfDataMaskPattern', type: 'input', key: 'pattern' }
    ]
  }
]

function esc (s) {
  return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'")
}

function generatePalette (node) {
  const nodeData = `{
  value: '${esc(node.value)}',
  label: '${esc(node.label)}',
  icon: '${esc(node.icon)}',
  color: '${esc(node.color)}',
  desc: '${esc(node.desc)}',
  category: '${esc(node.category)}'
}`
  return `<template>
  <div class="node-item" draggable="true" @dragstart="onDragStart" @click="onClick">
    <div class="node-icon" :style="{ backgroundColor: node.color + '20', color: node.color }">
      <q-icon :name="node.icon" size="18px" />
    </div>
    <div class="node-info">
      <div class="node-name">${node.label}</div>
      <div class="node-desc">${node.desc}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
const node = ${nodeData}

const emit = defineEmits(['drag', 'click'])

function onDragStart (event: DragEvent) {
  event.dataTransfer?.setData('application/vueflow', JSON.stringify(node))
  emit('drag', event)
}

function onClick () {
  emit('click')
}
</script>

<style scoped>
.node-item {
  display: flex;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: grab;
  transition: all 0.2s;
  border: 1px solid transparent;
}
.node-item:hover {
  background: #f5f5f5;
  border-color: #e8e8e8;
}
.node-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 10px;
  flex-shrink: 0;
}
.node-name {
  font-size: 13px;
  font-weight: 500;
  color: #333;
}
.node-desc {
  font-size: 11px;
  color: #8c8c8c;
}
</style>
`
}

function generateConfig (node) {
  const fields = node.configFields.map(f => {
    if (f.type === 'select') {
      return `    <div class="config-group">
      <div class="config-label">{{ t('${f.label}') }}</div>
      <q-select :model-value="strField('${f.key}')" :options="${JSON.stringify(f.options).replace(/"/g, "'")}" outlined dense emit-value map-options class="config-input" @update:model-value="patchConfig('${f.key}', $event)" />
    </div>`
    } else if (f.type === 'textarea') {
      return `    <div class="config-group">
      <div class="config-label">{{ t('${f.label}') }}</div>
      <q-input :model-value="strField('${f.key}')" outlined dense type="textarea" rows="${f.rows || 3}" class="config-input" @update:model-value="patchConfig('${f.key}', $event)" />
    </div>`
    } else if (f.type === 'password') {
      return `    <div class="config-group">
      <div class="config-label">{{ t('${f.label}') }}</div>
      <q-input :model-value="strField('${f.key}')" outlined dense type="password" class="config-input" @update:model-value="patchConfig('${f.key}', $event)" />
    </div>`
    } else if (f.type === 'number') {
      return `    <div class="config-group">
      <div class="config-label">{{ t('${f.label}') }}</div>
      <q-input :model-value="numField('${f.key}')" type="number" outlined dense class="config-input" @update:model-value="patchNumber('${f.key}', $event)" />
    </div>`
    } else {
      return `    <div class="config-group">
      <div class="config-label">{{ t('${f.label}') }}</div>
      <q-input :model-value="strField('${f.key}')" outlined dense class="config-input" @update:model-value="patchConfig('${f.key}', $event)" />
    </div>`
    }
  }).join('\n')

  const bodyFields = fields ? `\n${fields}` : ''
  const hasConfigFields = node.configFields.length > 0
  const hasNumberField = node.configFields.some(f => f.type === 'number')
  const needsStrField = node.configFields.some(f => f.type !== 'number')

  let patchConfigFn = ''
  if (hasConfigFields) {
    patchConfigFn = `
function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}
`
  }

  let strFieldFn = ''
  if (hasConfigFields && needsStrField) {
    strFieldFn = `
function strField (key: string): string {
  const v = props.config[key]
  return v == null ? '' : String(v)
}
`
  }

  let numFieldFn = ''
  if (hasConfigFields && hasNumberField) {
    numFieldFn = `
function numField (key: string): number {
  const v = props.config[key]
  if (v == null || v === '') return 0
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isNaN(n) ? 0 : n
}
`
  }

  let patchNumberFn = ''
  if (hasNumberField) {
    patchNumberFn = `
function patchNumber (key: string, raw: string | number | null | undefined) {
  if (raw === '' || raw === null || raw === undefined) {
    patchConfig(key, 0)
    return
  }
  const n = typeof raw === 'number' ? raw : Number(raw)
  patchConfig(key, Number.isNaN(n) ? 0 : n)
}
`
  }

  return `<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>${bodyFields}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits(['update:label', 'update:config'])

const { t } = useI18n()
${patchConfigFn}${strFieldFn}${numFieldFn}${patchNumberFn}
const nodeLabel = computed({
  get: () => props.nodeLabel,
  set: (v) => emit('update:label', v)
})
</script>

<style scoped>
.config-content {
  padding: 12px 16px;
}
.config-group {
  margin-bottom: 16px;
}
.config-label {
  font-size: 13px;
  color: #333;
  margin-bottom: 6px;
  font-weight: 500;
}
.config-input {
  font-size: 13px;
}
</style>
`
}

const nodesDir = __dirname
const configsDir = path.join(nodesDir, 'configs')
const palettesDir = path.join(nodesDir, 'palettes')

if (!fs.existsSync(configsDir)) fs.mkdirSync(configsDir)
if (!fs.existsSync(palettesDir)) fs.mkdirSync(palettesDir)

for (const node of nodes) {
  const palettePath = path.join(palettesDir, `${node.name}Palette.vue`)
  const configPath = path.join(configsDir, `${node.name}NodeConfig.vue`)

  fs.writeFileSync(palettePath, generatePalette(node))
  fs.writeFileSync(configPath, generateConfig(node))

  console.log(`Generated: ${node.name}`)
}

console.log('Done!')
