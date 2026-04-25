<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfBindAgent') }}</div>
      <q-select
        :model-value="bindAgentId"
        :options="agentOptions"
        :loading="agentsLoading"
        outlined
        dense
        emit-value
        map-options
        clearable
        :placeholder="t('wfSelectAgentPh')"
        class="config-input"
        @update:model-value="onAgentId"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfPromptTemplate') }}</div>
      <q-input :model-value="strField('prompt_template')" outlined dense type="textarea" rows="6" class="config-input" @update:model-value="patchConfig('prompt_template', $event)" />
    </div>
    <q-separator class="q-my-md" />
    <div class="config-group">
      <div class="config-label">{{ t('wfRetryConfig') }}</div>
      <div class="row q-col-gutter-sm">
        <div class="col-6">
          <q-input
            :model-value="numField('retry_count')"
            outlined dense
            type="number"
            :label="t('wfRetryCount')"
            min="0" max="5"
            class="config-input"
            @update:model-value="patchNumber('retry_count', $event)"
          />
        </div>
        <div class="col-6">
          <q-input
            :model-value="numField('retry_delay_ms')"
            outlined dense
            type="number"
            :label="t('wfRetryDelayMs')"
            min="0" max="60000"
            class="config-input"
            @update:model-value="patchNumber('retry_delay_ms', $event)"
          />
        </div>
      </div>
      <div class="text-caption text-grey-6 q-mt-xs">{{ t('wfRetryHint') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAgentNodeAgentOptions, useAgentNodeForm } from './useAgentNodeConfig'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:label': [v: string]
  'update:config': [v: Record<string, unknown>]
}>()

const { t } = useI18n()

const { agentOptions, loading: agentsLoading } = useAgentNodeAgentOptions()

const {
  bindAgentId,
  patchConfig,
  patchNumber,
  onAgentId,
  strField,
  numField,
  nodeLabel
} = useAgentNodeForm(props, emit)
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
