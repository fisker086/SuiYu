<template>
  <q-btn-dropdown
    flat
    dense
    label="插入变量"
    icon="data_object"
    color="primary"
    size="sm"
    class="var-insert-btn"
  >
    <q-list dense class="var-dropdown">
      <q-item-label header>系统变量</q-item-label>
      <q-item v-for="v in systemVars" :key="v.value" clickable v-close-popup @click="insertVar(v.value)">
        <q-item-section>
          <q-item-label>{{ v.label }}</q-item-label>
          <q-item-label caption class="var-expr">{{ v.display }}</q-item-label>
        </q-item-section>
      </q-item>

      <q-item-label header>输入参数</q-item-label>
      <q-item v-for="v in inputVars" :key="v.value" clickable v-close-popup @click="insertVar(v.value)">
        <q-item-section>
          <q-item-label>{{ v.label }}</q-item-label>
          <q-item-label caption class="var-expr">{{ v.display }}</q-item-label>
        </q-item-section>
      </q-item>

      <q-item-label header>上游节点输出 (Items 格式)</q-item-label>
      <template v-if="(upstreamNodes?.length ?? 0) > 0">
        <q-expansion-item
          v-for="node in (upstreamNodes ?? [])"
          :key="node.value"
          :label="node.label"
          dense
          expand-icon="expand_more"
          class="node-expansion"
        >
          <template #header>
            <div class="node-header">
              <q-icon :name="getNodeIcon(node.nodeType)" size="18px" class="q-mr-sm" />
              <span>{{ node.label }}</span>
              <q-badge :label="node.nodeType" color="grey-6" size="sm" class="q-ml-sm" />
            </div>
          </template>
          <q-item
            v-for="field in getNodeFields(node.value)"
            :key="field"
            clickable
            v-close-popup
            @click="insertVar(getNodeVarPath(node.label, field))"
          >
            <q-item-section>
              <q-item-label>{{ field }}</q-item-label>
              <q-item-label caption class="var-expr">{{ getNodeVarPath(node.label, field) }}</q-item-label>
            </q-item-section>
          </q-item>
          <q-separator />
          <q-item dense class="items-hint">
            <q-item-section>
              <q-item-label caption class="text-grey-6">
                也支持: {{ `.${node.label}.items[0].json.${getNodeFields(node.value)[0] || 'field'}` }}
              </q-item-label>
            </q-item-section>
          </q-item>
        </q-expansion-item>
      </template>
      <q-item v-else>
        <q-item-section>
          <q-item-label class="text-grey">无上游节点</q-item-label>
        </q-item-section>
      </q-item>

      <q-item-label header>快速引用</q-item-label>
      <q-item clickable v-close-popup @click="insertVar('{{last_output}}')">
        <q-item-section>
          <q-item-label>上一步输出</q-item-label>
          <q-item-label caption class="var-expr">{{ '{' + '{last_output}' + '}' }}</q-item-label>
        </q-item-section>
      </q-item>
      <q-item clickable v-close-popup @click="insertVar('{{last_output.json.data}}')">
        <q-item-section>
          <q-item-label>上一步 JSON.data</q-item-label>
          <q-item-label caption class="var-expr">{{ '{' + '{last_output.json.data}' + '}' }}</q-item-label>
        </q-item-section>
      </q-item>
      <q-item clickable v-close-popup @click="insertVar('{{last_output.items[0].json.result}}')">
        <q-item-section>
          <q-item-label>Items[0] 结果</q-item-label>
          <q-item-label caption class="var-expr">{{ '{' + '{last_output.items[0].json.result}' + '}' }}</q-item-label>
        </q-item-section>
      </q-item>

      <q-item-label header>批量数据 (Items)</q-item-label>
      <q-item clickable v-close-popup @click="insertVar('{{$items}}')">
        <q-item-section>
          <q-item-label>所有 Items 数组</q-item-label>
          <q-item-label caption class="var-expr">遍历处理批量数据</q-item-label>
        </q-item-section>
      </q-item>
      <q-item clickable v-close-popup @click="insertVar('{{$first.json.data}}')">
        <q-item-section>
          <q-item-label>第一个 Item 数据</q-item-label>
          <q-item-label caption class="var-expr">{{ '{' + '{$first.json.data}' + '}' }}</q-item-label>
        </q-item-section>
      </q-item>
    </q-list>
  </q-btn-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface UpstreamNode {
  label: string
  value: string
  nodeType: string
}

interface VarOption {
  label: string
  value: string
  display: string
}

const props = defineProps<{
  inputSchema?: Record<string, any>
  globalVars?: Record<string, any>
  upstreamNodes?: UpstreamNode[]
  nodeOutputFields?: Record<string, string[]>
}>()

const emit = defineEmits<{
  insert: [varPath: string]
}>()

const systemVars = computed((): VarOption[] => [
  { label: '用户输入', value: '{{sys.query}}', display: '{{sys.query}}' },
  { label: '工作流ID', value: '{{sys.workflow_id}}', display: '{{sys.workflow_id}}' },
  { label: '运行ID', value: '{{sys.workflow_run_id}}', display: '{{sys.workflow_run_id}}' }
])

const inputVars = computed((): VarOption[] => {
  if (!props.inputSchema) return []
  return Object.keys(props.inputSchema).map(key => ({
    label: key,
    value: `{{${key}}}`,
    display: `{{${key}}}`
  }))
})

function getNodeFields (nodeId: string): string[] {
  if (props.nodeOutputFields && props.nodeOutputFields[nodeId]) {
    return props.nodeOutputFields[nodeId]
  }
  const defaultFields: Record<string, string[]> = {
    start: ['content', 'data'],
    end: ['content', 'result'],
    agent: ['content', 'response', 'result'],
    llm: ['content', 'response', 'result'],
    condition: ['result', 'branch'],
    merge: ['outputs', 'merged'],
    http: ['response', 'body', 'status'],
    code: ['output', 'result'],
    tool: ['result', 'output'],
    knowledge: ['results', 'content'],
    notify: ['sent', 'message'],
    ssh: ['output', 'error'],
    variable: ['value'],
    template: ['output'],
    apitest: ['result', 'response'],
    datamask: ['masked', 'count']
  }
  return defaultFields[nodeId] || ['content', 'data', 'result']
}

function getNodeVarPath (nodeLabel: string, field: string): string {
  return `{{.${nodeLabel}.${field}}}`
}

function getNodeIcon (nodeType: string): string {
  const icons: Record<string, string> = {
    start: 'play_circle',
    end: 'stop_circle',
    agent: 'smart_topy',
    llm: 'psychology',
    condition: 'call_split',
    merge: 'merge_type',
    http: 'http',
    code: 'code',
    tool: 'build',
    knowledge: 'library_books',
    notify: 'send',
    ssh: 'terminal',
    variable: 'data_object',
    template: 'text_snippet',
    apitest: 'api',
    datamask: 'security'
  }
  return icons[nodeType] || 'hub'
}

function insertVar (varPath: string) {
  emit('insert', varPath)
}
</script>

<style scoped>
.var-insert-btn {
  margin-bottom: 4px;
}
.var-dropdown {
  min-width: 260px;
  max-width: 360px;
}
.var-expr {
  font-family: monospace;
  font-size: 11px;
  color: #4fc3f7;
}
.node-expansion {
  font-size: 13px;
}
.node-header {
  display: flex;
  align-items: center;
  width: 100%;
}
.items-hint {
  background: #f5f5f5;
  padding: 4px 8px;
}
</style>
