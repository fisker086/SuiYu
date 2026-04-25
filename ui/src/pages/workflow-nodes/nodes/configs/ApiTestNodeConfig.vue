<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfAPITestMethod') }}</div>
      <q-select :model-value="strField('method')" :options="[{'label':'GET','value':'GET'}]" outlined dense emit-value map-options class="config-input" @update:model-value="patchConfig('method', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfAPITestURL') }}</div>
      <q-input :model-value="strField('url')" outlined dense class="config-input" @update:model-value="patchConfig('url', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfAPITestHeaders') }}</div>
      <q-input :model-value="strField('headers')" outlined dense type="textarea" rows="2" class="config-input" @update:model-value="patchConfig('headers', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfAPITestBody') }}</div>
      <q-input :model-value="strField('body')" outlined dense type="textarea" rows="3" class="config-input" @update:model-value="patchConfig('body', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfAPITestAssertions') }}</div>
      <q-input :model-value="strField('assertions')" outlined dense type="textarea" rows="3" class="config-input" @update:model-value="patchConfig('assertions', $event)" />
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
