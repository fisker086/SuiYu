<template>
  <div class="node-config">
    <component
      :is="configComponent"
      v-if="configComponent"
      :config="config"
      :label="label"
      @update:config="onConfigUpdate"
      @update:label="onLabelUpdate"
    />
    <div v-else class="config-section">
      <div class="config-group">
        <div class="config-label">{{ t('wfNodeName') }}</div>
        <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
      </div>
    </div>

    <q-separator class="q-my-md" />

    <div class="config-section">
      <div class="config-section-title">
        <q-icon name="link" size="16px" class="q-mr-xs" />
        {{ t('wfInputMapping') }}
        <span class="config-hint">{{ t('wfInputMappingHint') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NodeConfig } from './types'
import { getNode } from './nodes'

const props = defineProps<{
  nodeType: string
  config: NodeConfig
  label: string
}>()

const emit = defineEmits<{
  'update:config': [config: NodeConfig]
  'update:label': [label: string]
}>()

const { t } = useI18n()

const nodeLabel = computed({
  get: () => props.label,
  set: (v: string) => emit('update:label', v)
})

const configComponent = computed(() => {
  const node = getNode(props.nodeType)
  return node?.Config
})

function onConfigUpdate (newConfig: NodeConfig) {
  emit('update:config', newConfig)
}

function onLabelUpdate (newLabel: string) {
  emit('update:label', newLabel)
}
</script>

<style scoped>
.node-config {
  background: white;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.config-section {
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.config-section-title {
  font-size: 13px;
  font-weight: 500;
  color: #333;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
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

.config-hint {
  font-size: 11px;
  color: #8c8c8c;
  font-weight: normal;
  margin-left: 8px;
}

.config-input {
  font-size: 13px;
}
</style>
