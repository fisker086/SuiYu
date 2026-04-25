<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfStartUserPrompt') }}</div>
      <div class="text-caption text-grey-7 q-mb-xs">{{ t('wfStartUserPromptHint') }}</div>
      <q-input
        :model-value="userPrompt"
        outlined
        dense
        type="textarea"
        rows="6"
        :placeholder="t('wfStartUserPromptPh')"
        class="config-input"
        @update:model-value="onUserPrompt"
      />
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

const emit = defineEmits<{
  'update:label': [v: string]
  'update:config': [v: Record<string, unknown>]
}>()

const { t } = useI18n()

const nodeLabel = computed({
  get: () => props.nodeLabel,
  set: (v) => emit('update:label', v)
})

const userPrompt = computed(() => {
  const v = props.config.user_prompt
  return v == null ? '' : String(v)
})

function onUserPrompt (v: string | number | null) {
  const s = v == null ? '' : String(v)
  emit('update:config', { ...props.config, user_prompt: s })
}
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
