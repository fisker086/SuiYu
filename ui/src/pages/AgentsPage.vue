<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('agents') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createAgent')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="load" :loading="loading" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-table
      v-model:pagination="pagination"
      flat
      bordered
      class="radius-sm"
      :rows="rows"
      :columns="columns"
      row-key="id"
      :loading="loading"
      :rows-per-page-options="[5, 10, 15, 25, 50]"
      :no-data-label="t('noData')"
    >
      <template #body-cell-skill_count="props">
        <q-td :props="props">
          <q-badge color="blue" :label="props.row.skill_ids?.length ?? 0" />
        </q-td>
      </template>
      <template #body-cell-mcp_count="props">
        <q-td :props="props">
          <q-badge color="green" :label="props.row.mcp_config_ids?.length ?? 0" />
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props">
          <q-badge :color="props.row.is_active !== false ? 'positive' : 'grey'" :label="props.row.is_active !== false ? t('roleEnabled') : t('roleDisabled')" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="primary" :label="t('chat')" @click="goChat(props.row)" />
          <q-btn dense flat color="secondary" :label="t('edit')" @click="openDialog(props.row)" />
          <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="dialogOpen">
      <q-card class="agent-edit-dialog-card">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ isEdit ? t('editAgent') : t('createAgent') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none q-pt-none" style="max-height: 70vh; overflow-y: auto;">
          <q-tabs v-model="agentTab" dense align="left" class="text-grey" active-color="primary" indicator-color="primary">
            <q-tab name="basic" :label="t('basicInfo')" />
            <q-tab name="runtime" :label="t('runtimeConfig')" />
            <q-tab name="capabilities" :label="t('capabilities')" />
            <q-tab name="im" :label="t('imBot')" />
          </q-tabs>

          <q-separator />

          <q-tab-panels v-model="agentTab" :animated="false">
            <!-- Basic Info Tab -->
            <q-tab-panel name="basic">
              <div class="row q-col-gutter-md">
                <div class="col-6">
                  <q-input v-model="form.name" :label="t('agentName')" outlined dense :rules="[v => !!v]" />
                </div>
                <div class="col-6">
                  <q-input v-model="form.category" :label="t('category')" outlined dense />
                </div>
              </div>
              <q-input v-model="form.description" :label="t('description')" outlined dense type="textarea" autogrow :rules="[v => !!v]" class="q-mt-md" />
              <q-checkbox v-model="form.is_active" :label="t('isActive')" class="q-mt-md" />
            </q-tab-panel>

            <!-- Runtime Config Tab -->
            <q-tab-panel name="runtime">
              <div class="row q-col-gutter-md">
                <div class="col-6">
                  <q-input v-model="runtimeProfile.llm_model" :label="t('llmModel')" outlined dense />
                </div>
                <div class="col-6">
                  <q-input v-model.number="runtimeProfile.temperature" :label="t('temperature')" outlined dense type="number" step="0.1" min="0" max="2" />
                </div>
              </div>
              <div class="q-mt-md">
                <div class="text-caption text-grey-7 q-mb-xs">{{ t('systemPrompt') }}</div>
                <textarea
                  v-model="runtimeProfile.system_prompt"
                  class="agent-system-prompt-textarea"
                  rows="10"
                  spellcheck="false"
                  autocomplete="off"
                  @paste="onSystemPromptPaste"
                />
              </div>

              <q-separator class="q-my-md" />
              <div class="text-subtitle2 q-mb-sm text-primary">{{ t('agentExecutionModeSection') }}</div>
              <q-select
                v-model="runtimeProfile.execution_mode"
                :options="executionModeOptions"
                :label="t('agentExecutionModeLabel')"
                outlined
                dense
                emit-value
                map-options
                :hint="t('agentExecutionModeHint')"
              />
              <q-input
                v-if="runtimeProfile.execution_mode === 'react'"
                v-model.number="runtimeProfile.max_iterations"
                :label="t('agentMaxIterations')"
                outlined
                dense
                type="number"
                min="1"
                max="50"
                class="q-mt-md"
                :hint="t('agentMaxIterationsHint')"
              />
              <q-input
                v-if="runtimeProfile.execution_mode === 'plan-and-execute'"
                v-model="runtimeProfile.plan_prompt"
                :label="t('agentPlanPromptLabel')"
                outlined
                dense
                type="textarea"
                rows="3"
                class="q-mt-md"
                :hint="t('agentPlanPromptHint')"
              />

              <q-separator class="q-my-md" />
              <div class="text-subtitle2 q-mb-sm text-primary">{{ t('approvalMode') }}</div>
              <q-select
                v-model="runtimeProfile.approval_mode"
                :options="approvalModeOptions"
                :label="t('approvalMode')"
                outlined
                dense
                emit-value
                map-options
                :hint="t('approvalModeHelp')"
              />

              <div v-if="runtimeProfile.approval_mode !== 'auto'" class="q-mt-md">
                <div class="text-subtitle2 q-mb-sm text-primary">{{ t('approvers') }}</div>
                <q-select
                  v-model="runtimeProfile.approvers"
                  :options="availableUsers"
                  :label="t('approversLabel')"
                  outlined
                  dense
                  multiple
                  use-chips
                  emit-value
                  map-options
                  :hint="t('approversHint')"
                  option-value="username"
                  option-label="username"
                />
              </div>

              <div class="row q-col-gutter-md q-mt-md">
                <div class="col-6">
                  <q-checkbox v-model="runtimeProfile.stream_enabled" :label="t('streamEnabled')" />
                </div>
                <div class="col-6">
                  <q-checkbox v-model="runtimeProfile.memory_enabled" :label="t('memoryEnabled')" />
                </div>
              </div>
            </q-tab-panel>

            <!-- Capabilities Tab (MCP & Skills) -->
            <q-tab-panel name="capabilities">
              <div class="q-mb-md">
                <div class="row items-center q-mb-sm">
                  <div class="text-subtitle2 text-primary">{{ t('bindSkills') }} ({{ selectedSkillIds.length }}/{{ availableSkills.length }})</div>
                  <div class="text-caption text-grey-7 q-mb-xs">
                    {{ t('agentCapabilitiesRiskHint') }}
                  </div>
                  <q-space />
                  <q-input
                    v-model="skillSearch"
                    outlined
                    dense
                    :placeholder="t('agentSearchSkillsPlaceholder')"
                    class="q-mr-sm"
                    style="width: 200px;"
                    clearable
                  >
                    <template #prepend>
                      <q-icon name="search" />
                    </template>
                  </q-input>
                  <q-btn dense outline color="primary" :label="selectedSkillIds.length ? t('agentClearSkills') : t('agentSelectAllSkills')" @click="toggleAllSkills" size="sm" />
                </div>

                <div v-if="availableSkills.length === 0" class="text-caption text-grey q-pa-md text-center">
                  <q-icon name="info" size="sm" /> {{ t('agentNoSkillsHint') }}
                </div>

                <q-scroll-area style="height: 350px;">
                  <div v-for="group in skillsByCategory" :key="group.key" class="q-mb-md">
                    <div class="text-subtitle2 text-weight-medium q-mb-xs" :class="group.colorClass">
                      <q-icon :name="group.icon" size="xs" class="q-mr-xs" />
                      {{ group.label }} ({{ group.items.length }})
                    </div>
                    <div class="row q-col-gutter-sm">
                      <div
                        v-for="skill in group.items"
                        :key="skill.id"
                        class="col-12"
                      >
                        <div
                          class="skill-card q-pa-sm q-pl-md cursor-pointer row items-center no-wrap"
                          :class="{ 'skill-card--selected': selectedSkillIds.includes(skill.key) }"
                          @click="toggleSkill(skill.key)"
                        >
                          <q-checkbox
                            v-model="selectedSkillIds"
                            :val="skill.key"
                            color="primary"
                            class="q-mr-sm"
                            @click.stop
                            @update:model-value="onSkillToggle(skill.key, $event)"
                          />
                          <div class="col">
                            <div class="row items-center">
                              <span class="text-body2 text-weight-medium">{{ skill.name }}</span>
                              <q-badge
                                :color="getSkillRiskColor(skill)"
                                :label="getSkillRiskLabel(skill)"
                                class="q-ml-xs"
                                size="xs"
                              />
                              <q-badge
                                :color="execModeForSkill(skill) === 'client' ? 'teal' : 'blue-grey'"
                                :label="execModeForSkill(skill) === 'client' ? t('agentLocal') : t('agentRemoteServer')"
                                class="q-ml-xs"
                                size="xs"
                              />
                            </div>
                            <div class="text-caption text-grey-7">{{ skill.description || t('noneDescription') }}</div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-if="filteredSkills.length === 0 && availableSkills.length > 0" class="text-center text-grey q-pa-md">
                    {{ t('agentNoMatch') }}
                  </div>
                </q-scroll-area>
              </div>

              <q-separator class="q-my-md" />

              <div>
                <div class="text-subtitle2 q-mb-sm text-secondary">{{ t('bindMCP') }} ({{ selectedMcpConfigIds.length }})</div>
                <div v-if="availableMcpConfigs.length === 0" class="text-caption text-grey q-pa-md text-center">
                  <q-icon name="info" size="sm" /> {{ t('agentNoMcpHint') }}
                </div>
                <div v-for="mcp in availableMcpConfigs" :key="mcp.id" class="q-mb-sm">
                  <q-checkbox
                    v-model="selectedMcpConfigIds"
                    :val="mcp.id"
                    color="secondary"
                  >
                    <template #default>
                      <div class="text-body2">{{ mcp.name }}</div>
                      <div class="text-caption text-grey">{{ mcp.description || mcp.endpoint || t('noneDescription') }}</div>
                    </template>
                  </q-checkbox>
                </div>
              </div>
            </q-tab-panel>

            <!-- IM Bot Tab -->
            <q-tab-panel name="im">
              <div class="row q-col-gutter-md">
                <div class="col-12">
                  <q-select
                    v-model="runtimeProfile.im_enabled"
                    :options="imEnabledOptions"
                    :label="t('imEnabled')"
                    outlined
                    dense
                    emit-value
                    map-options
                  />
                </div>
              </div>

              <div v-if="runtimeProfile.im_enabled !== 'disabled'" class="q-mt-md">
                <q-separator class="q-my-md" />
                <div class="text-subtitle2 q-mb-sm text-primary">{{ t('imConfig') }}</div>

                <q-input
                  v-if="runtimeProfile.im_enabled === 'telegram'"
                  v-model="runtimeProfile.im_config.telegram_token"
                  :label="t('imTelegramToken')"
                  outlined
                  dense
                  class="q-mb-md"
                />

                <q-input
                  v-if="runtimeProfile.im_enabled === 'telegram'"
                  v-model="runtimeProfile.im_config.telegram_chat_id"
                  :label="t('imTelegramChatId')"
                  outlined
                  dense
                  class="q-mb-md"
                />

                <q-input
                  v-if="runtimeProfile.im_enabled === 'lark'"
                  v-model="runtimeProfile.im_config.lark_open_domain"
                  :label="t('imLarkOpenDomain')"
                  outlined
                  dense
                  :placeholder="t('imLarkOpenDomainPh')"
                  :hint="t('imLarkOpenDomainHint')"
                  class="q-mb-md"
                />

                <template v-if="imShowsAppCredentials">
                  <q-input
                    v-model="runtimeProfile.im_config.app_id"
                    :label="t('imAppId')"
                    outlined
                    dense
                    :hint="t('imAppIdHint')"
                    class="q-mb-md"
                  />
                  <q-input
                    v-model="runtimeProfile.im_config.app_secret"
                    :label="t('imAppSecret')"
                    outlined
                    dense
                    type="password"
                    autocomplete="new-password"
                    :hint="t('imAppSecretHint')"
                    class="q-mb-md"
                  />
                </template>

                <q-input
                  v-if="runtimeProfile.im_enabled === 'lark'"
                  v-model="runtimeProfile.im_config.verification_token"
                  :label="t('imLarkVerificationToken')"
                  outlined
                  dense
                  :hint="t('imLarkVerificationTokenHint')"
                  class="q-mb-md"
                />

                <q-input
                  v-if="runtimeProfile.im_enabled === 'lark'"
                  v-model="runtimeProfile.im_config.encrypt_key"
                  :label="t('imLarkEncryptKey')"
                  outlined
                  dense
                  type="password"
                  autocomplete="new-password"
                  :hint="t('imLarkEncryptKeyHint')"
                  class="q-mb-md"
                />

                <q-input
                  v-model="runtimeProfile.im_config.bot_name"
                  :label="t('imBotName')"
                  outlined
                  dense
                  :hint="t('imBotNameHint')"
                  class="q-mt-md"
                />

                <div class="row q-col-gutter-md q-mt-md items-center">
                  <div class="col-12 col-sm-6">
                    <q-toggle
                      v-model="runtimeProfile.im_config.auto_reply"
                      :label="t('imAutoReply')"
                      class="full-width"
                    />
                  </div>
                  <div class="col-12 col-sm-6">
                    <q-toggle
                      v-model="runtimeProfile.im_config.notify_on_approval"
                      :label="t('imNotifyOnApproval')"
                      class="full-width"
                    />
                  </div>
                </div>
              </div>

              <q-banner v-if="runtimeProfile.im_enabled === 'disabled'" class="bg-grey-2 text-grey-8 q-mt-md" rounded>
                {{ t('imNotConfigured') }}
              </q-banner>
            </q-tab-panel>
          </q-tab-panels>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveAgent" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Skill } from 'src/api/types'
import { useAgentsPage } from './useAgentsPage'
defineOptions({ name: 'AgentsPage' })

const agentTab = ref('basic')
const skillSearch = ref('')

const agentsPage = useAgentsPage()
/** Assigned separately so vue-tsc exposes it to the template (destructuring can lose bindings). */
const pagination = agentsPage.pagination
const {
  t,
  loading,
  rows,
  errorMsg,
  columns,
  load,
  goChat,
  dialogOpen,
  isEdit,
  form,
  runtimeProfile,
  saving,
  openDialog,
  saveAgent,
  confirmDelete,
  availableSkills,
  availableMcpConfigs,
  availableUsers,
  selectedSkillIds,
  selectedMcpConfigIds,
  onSystemPromptPaste
} = agentsPage

const executionModeOptions = computed(() => [
  { label: t('execModeSingleCall'), value: 'single-call' },
  { label: t('execModeReact'), value: 'react' },
  { label: t('execModePlanExecute'), value: 'plan-and-execute' }
])

const approvalModeOptions = computed(() => [
  { label: t('approvalModeAuto'), value: 'auto' },
  { label: t('approvalModeHighDetail'), value: 'high_and_above' },
  { label: t('approvalModeAll'), value: 'all' }
])

const imEnabledOptions = computed(() => [
  { label: t('imEnabledOptionsDisabled'), value: 'disabled' },
  { label: t('imEnabledOptionsTelegram'), value: 'telegram' },
  { label: t('imEnabledOptionsLark'), value: 'lark' },
  { label: t('imEnabledOptionsDingtalk'), value: 'dingtalk' },
  { label: t('imEnabledOptionsWecom'), value: 'wecom' }
])

/** 飞书 / 钉钉 / 企微：仅填应用 App ID + App Secret */
const imShowsAppCredentials = computed(() =>
  runtimeProfile.im_enabled === 'lark' ||
  runtimeProfile.im_enabled === 'dingtalk' ||
  runtimeProfile.im_enabled === 'wecom'
)

const filteredSkills = computed(() => {
  if (!skillSearch.value) return availableSkills.value
  const q = skillSearch.value.toLowerCase()
  return availableSkills.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    (s.description || '').toLowerCase().includes(q) ||
    s.key.toLowerCase().includes(q)
  )
})

const skillCategoryMap = computed((): Record<string, { label: string; icon: string; colorClass: string }> => ({
  safe: { label: t('skillCatSafe'), icon: 'check_circle', colorClass: 'text-positive' },
  read_local: { label: t('skillCatReadLocal'), icon: 'desktop_windows', colorClass: 'text-info' },
  read_remote: { label: t('skillCatReadRemote'), icon: 'cloud', colorClass: 'text-warning' },
  write: { label: t('skillCatWrite'), icon: 'edit', colorClass: 'text-negative' }
}))

const skillsByCategory = computed(() => {
  const skills = filteredSkills.value
  const groups: Record<string, any[]> = {}
  for (const skill of skills) {
    const cat = skill.category || 'safe'
    if (!groups[cat]) groups[cat] = []
    groups[cat].push(skill)
  }
  const order = ['safe', 'read_local', 'read_remote', 'write']
  const map = skillCategoryMap.value
  return order
    .filter(k => groups[k])
    .map(k => ({
      key: k,
      ...map[k],
      items: groups[k]
    }))
})

function toggleSkill (key: string) {
  const idx = selectedSkillIds.value.indexOf(key)
  if (idx >= 0) {
    selectedSkillIds.value.splice(idx, 1)
  } else {
    selectedSkillIds.value.push(key)
  }
}

function onSkillToggle (key: string, val: any) {
  const checked = Array.isArray(val) ? val.includes(key) : !!val
  const idx = selectedSkillIds.value.indexOf(key)
  if (checked && idx < 0) {
    selectedSkillIds.value.push(key)
  } else if (!checked && idx >= 0) {
    selectedSkillIds.value.splice(idx, 1)
  }
}

function toggleAllSkills () {
  if (selectedSkillIds.value.length > 0) {
    selectedSkillIds.value = []
  } else {
    selectedSkillIds.value = availableSkills.value.map(s => s.key)
  }
}

const skillRiskMap: Record<string, string> = {
  'builtin_skill.search': 'low',
  'builtin_skill.calculator': 'low',
  'builtin_skill.code_interpreter': 'low',
  'builtin_skill.datetime': 'low',
  'builtin_skill.regex': 'low',
  'builtin_skill.json_parser': 'low',
  'builtin_skill.log_analyzer': 'low',
  'builtin_skill.csv_analyzer': 'low',
  'builtin_skill.file_parser': 'low',
  'builtin_skill.image_analyzer': 'low',
  'builtin_skill.terraform_plan': 'low',
  'builtin_skill.docker_operator': 'medium',
  'builtin_skill.git_operator': 'medium',
  'builtin_skill.system_monitor': 'medium',
  'builtin_skill.cron_manager': 'medium',
  'builtin_skill.network_tools': 'medium',
  'builtin_skill.cert_checker': 'medium',
  'builtin_skill.nginx_diagnose': 'medium',
  'builtin_skill.dns_lookup': 'medium',
  'builtin_skill.http_client': 'medium',
  'builtin_skill.ssh_executor': 'high',
  'builtin_skill.k8s_operator': 'high',
  'builtin_skill.db_query': 'high',
  'builtin_skill.prometheus_query': 'high',
  'builtin_skill.grafana_reader': 'high',
  'builtin_skill.aws_readonly': 'high',
  'builtin_skill.gcp_readonly': 'high',
  'builtin_skill.azure_readonly': 'high',
  'builtin_skill.loki_query': 'high',
  'builtin_skill.argocd_readonly': 'high',
  'builtin_skill.alert_sender': 'critical',
  'builtin_skill.slack_notify': 'critical',
  'builtin_skill.jira_connector': 'high',
  'builtin_skill.github_issue': 'high'
}

/** Prefer server-filled risk_level / execution_mode from GET /skills (SkillService enrich). */
const getSkillRiskLevel = (skill: Skill): string => {
  const fromApi = skill.risk_level != null && String(skill.risk_level).trim() !== '' ? String(skill.risk_level).trim() : ''
  if (fromApi) return fromApi
  return skillRiskMap[skill.key] || 'medium'
}

const execModeForSkill = (skill: Skill): string => {
  const fromApi = skill.execution_mode != null && String(skill.execution_mode).trim() !== '' ? String(skill.execution_mode).trim() : ''
  if (fromApi) return fromApi
  return 'server'
}

const getSkillRiskColor = (skill: Skill): string => {
  const level = getSkillRiskLevel(skill)
  const colors: Record<string, string> = { low: 'green', medium: 'orange', high: 'deep-orange', critical: 'red' }
  return colors[level] || 'grey'
}

const getSkillRiskLabel = (skill: Skill): string => {
  const level = getSkillRiskLevel(skill)
  const labels: Record<string, string> = {
    low: t('riskLow'),
    medium: t('riskMedium'),
    high: t('riskHigh'),
    critical: t('riskCritical')
  }
  return labels[level] || level
}
</script>

<style scoped>
.agent-edit-dialog-card {
  width: min(92vw, 560px);
  max-width: 92vw;
}
.skill-card {
  border: 1px solid rgba(0, 0, 0, 0.12);
  border-radius: 6px;
  transition: background-color 0.15s, border-color 0.15s;
}
.skill-card:hover {
  background-color: rgba(0, 0, 0, 0.04);
}
.skill-card--selected {
  background-color: rgba(24, 143, 255, 0.08);
  border-color: var(--q-primary);
}
.agent-system-prompt-textarea {
  width: 100%;
  box-sizing: border-box;
  min-height: 200px;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.5;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: rgba(0, 0, 0, 0.87);
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.24);
  border-radius: 4px;
  resize: vertical;
  outline: none;
}
.agent-system-prompt-textarea:focus {
  border-color: var(--q-primary);
}
</style>
