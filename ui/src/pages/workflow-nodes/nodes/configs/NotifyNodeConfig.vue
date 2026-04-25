<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfNotifyChannel') }}</div>
      <q-select :model-value="strField('channel')" :options="[{'label':'钉钉','value':'dingtalk'}]" outlined dense emit-value map-options class="config-input" @update:model-value="patchConfig('channel', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfNotifyTitle') }}</div>
      <q-input :model-value="strField('title')" outlined dense class="config-input" @update:model-value="patchConfig('title', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfNotifyMessage') }}</div>
      <q-input :model-value="strField('message')" outlined dense type="textarea" rows="3" class="config-input" @update:model-value="patchConfig('message', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfNotifyReceivers') }}</div>
      <q-input :model-value="strField('receivers')" outlined dense class="config-input" @update:model-value="patchConfig('receivers', $event)" />
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
