<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfKnowledgeQuery') }}</div>
      <q-input :model-value="strField('query')" outlined dense class="config-input" @update:model-value="patchConfig('query', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfKnowledgeTopK') }}</div>
      <q-input :model-value="numField('top_k')" type="number" outlined dense class="config-input" @update:model-value="patchNumber('top_k', $event)" />
    </div>
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

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
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
