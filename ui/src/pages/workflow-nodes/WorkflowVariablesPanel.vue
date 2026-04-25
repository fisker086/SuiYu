<template>
  <div class="variables-panel">
    <div class="panel-header">
      <q-icon name="data_object" size="20px" />
      <span>变量</span>
    </div>

    <q-tabs v-model="activeTab" dense align="justify" class="var-tabs">
      <q-tab name="system" label="系统" />
      <q-tab name="input" label="输入" />
      <q-tab name="global" label="全局" />
    </q-tabs>

    <q-tab-panels v-model="activeTab" animated class="var-panels">
      <q-tab-panel name="system" class="var-list">
        <div v-for="(val, key) in systemVars" :key="key" class="var-item">
          <span class="var-key">{{ key }}</span>
          <span class="var-value">{{ truncate(val) }}</span>
        </div>
        <div v-if="Object.keys(systemVars).length === 0" class="empty-hint">
          无系统变量
        </div>
      </q-tab-panel>

      <q-tab-panel name="input" class="var-list">
        <div v-for="(val, key) in inputVars" :key="key" class="var-item">
          <span class="var-key">{{ key }}</span>
          <span class="var-value">{{ truncate(val) }}</span>
        </div>
        <div v-if="Object.keys(inputVars).length === 0" class="empty-hint">
          在 Start 节点配置输入参数
        </div>
      </q-tab-panel>

      <q-tab-panel name="global" class="var-list">
        <div v-for="(val, key) in globalVars" :key="key" class="var-item">
          <span class="var-key">{{ key }}</span>
          <span class="var-value">{{ truncate(val) }}</span>
        </div>
        <div class="add-var">
          <q-input v-model="newVarKey" dense outlined placeholder="变量名" class="var-input" />
          <q-btn flat dense icon="add" @click="addVariable" />
        </div>
      </q-tab-panel>
    </q-tab-panels>

    <div class="panel-section">
      <div class="section-title">引用语法</div>
      <div class="syntax-hint">
        <code v-pre>{{'{{变量名}}' }}</code> 引用变量
      </div>
      <div class="syntax-hint">
        <code v-pre>{{'{{节点ID.field}}' }}</code> 引用节点输出
      </div>
      <div class="syntax-hint">
        <code v-pre>{{'{{sys.query}}' }}</code> 系统变量
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  variables?: Record<string, any>
  inputSchema?: Record<string, any>
}>()

const emit = defineEmits<{
  'add-variable': [key: string]
}>()

const activeTab = ref('system')
const newVarKey = ref('')

const systemVars = computed(() => ({
  'sys.query': props.variables?.sys?.query || '',
  'sys.workflow_id': props.variables?.sys?.workflow_id || '',
  'sys.workflow_run_id': props.variables?.sys?.workflow_run_id || ''
}))

const inputVars = computed(() => props.inputSchema || {})

const globalVars = computed(() => {
  const vars: Record<string, any> = {}
  if (props.variables) {
    for (const [key, val] of Object.entries(props.variables)) {
      if (!key.startsWith('sys.') && !key.startsWith('input_')) {
        vars[key] = val
      }
    }
  }
  return vars
})

function truncate (val: any): string {
  if (val === null || val === undefined) return '-'
  const str = typeof val === 'object' ? JSON.stringify(val) : String(val)
  return str.length > 30 ? str.slice(0, 30) + '...' : str
}

function addVariable () {
  if (newVarKey.value.trim()) {
    emit('add-variable', newVarKey.value.trim())
    newVarKey.value = ''
  }
}
</script>

<style scoped>
.variables-panel {
  width: 240px;
  background: #1e1e1e;
  border-left: 1px solid #333;
  display: flex;
  flex-direction: column;
}

.panel-header {
  padding: 12px 16px;
  font-weight: 600;
  color: #fff;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid #333;
}

.var-tabs {
  background: #252525;
}

.var-panels {
  flex: 1;
  overflow: auto;
}

.var-list {
  padding: 8px;
}

.var-item {
  display: flex;
  justify-content: space-between;
  padding: 6px 8px;
  background: #2a2a2a;
  border-radius: 4px;
  margin-bottom: 4px;
  font-size: 12px;
}

.var-key {
  color: #4fc3f7;
  font-family: monospace;
}

.var-value {
  color: #aaa;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty-hint {
  color: #666;
  text-align: center;
  padding: 20px;
  font-size: 12px;
}

.add-var {
  display: flex;
  gap: 4px;
  margin-top: 8px;
}

.var-input {
  flex: 1;
}

.panel-section {
  padding: 12px;
  border-top: 1px solid #333;
}

.section-title {
  color: #888;
  font-size: 11px;
  margin-bottom: 8px;
  text-transform: uppercase;
}

.syntax-hint {
  color: #666;
  font-size: 11px;
  margin-bottom: 4px;
}

.syntax-hint code {
  color: #4fc3f7;
  background: #2a2a2a;
  padding: 2px 4px;
  border-radius: 3px;
}
</style>
