<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('skills') }}</div>
      <q-space />
      <q-btn
        color="primary"
        :label="t('createSkill')"
        icon="add"
        class="q-mr-sm"
        unelevated
        @click="openCreateDialog"
      />
      <q-input
        v-model="search"
        outlined
        dense
        style="max-width: 260px;"
        :placeholder="t('search')"
        clearable
      >
        <template #prepend>
          <q-icon name="search" />
        </template>
      </q-input>
      <q-btn
        outline
        color="secondary"
        icon="library_books"
        class="q-mr-sm"
        dense
        :label="t('skillSyncBuiltins')"
        :loading="syncLoading"
        @click="syncBuiltinSkills"
      />
      <q-btn flat icon="refresh" round dense class="q-ml-sm" @click="load" :loading="loading" />
    </div>

    <div class="text-body2 text-grey q-mb-md">
      <q-icon name="info" class="q-mr-sm" />
      {{ t('skillsPageIntro') }}
    </div>

    <q-table
      v-model:pagination="pagination"
      :filter="search"
      flat
      bordered
      class="radius-sm skill-table-wrap"
      dense
      wrap-cells
      :rows="rows"
      :columns="columns"
      row-key="id"
      :loading="loading"
      :rows-per-page-options="[5, 10, 15, 25, 50]"
      :no-data-label="t('noData')"
    >
      <template #body-cell-key="props">
        <q-td :props="props" class="skill-table-col-key">
          <div class="skill-key-text text-body2" :title="props.row.key">
            {{ props.row.key }}
          </div>
        </q-td>
      </template>
      <template #body-cell-kind="props">
        <q-td :props="props">
          <q-badge
            v-if="props.row.key?.startsWith('builtin_skill.')"
            color="deep-purple"
            outline
            :label="t('skillKindBuiltin')"
          />
          <q-badge v-else color="grey-7" outline :label="t('skillKindCustom')" />
        </q-td>
      </template>
      <template #body-cell-name="props">
        <q-td :props="props" class="skill-table-col-name">
          <div class="skill-name-text" :title="props.row.name || ''">
            {{ props.row.name }}
          </div>
        </q-td>
      </template>
      <template #body-cell-risk_level="props">
        <q-td :props="props" class="skill-table-col-risk">
          <q-badge :color="riskBadgeColor(props.row.risk_level)" :label="riskLevelLabel(props.row.risk_level)" />
        </q-td>
      </template>
      <template #body-cell-execution_mode="props">
        <q-td :props="props">
          <q-badge :color="props.row.execution_mode === 'client' ? 'teal' : 'blue-grey'" outline>
            {{ props.row.execution_mode === 'client' ? t('execClient') : t('execServer') }}
          </q-badge>
        </q-td>
      </template>
      <template #body-cell-description="props">
        <q-td :props="props" class="skill-table-col-desc">
          <div class="skill-desc-cell">{{ props.row.description }}</div>
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props" class="skill-table-col-status">
          <div class="row items-center justify-center">
            <q-toggle
              :model-value="props.row.is_active"
              color="primary"
              @update:model-value="(v: boolean) => toggleActive(props.row, v)"
            />
          </div>
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props" class="skill-table-col-actions">
          <div class="skill-actions-cell row items-center justify-center no-wrap">
            <q-btn dense flat color="primary" :label="t('editContent')" @click="openEditor(props.row)" class="q-mr-xs" />
            <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
          </div>
        </q-td>
      </template>
    </q-table>

    <!-- Create/Edit Dialog -->
    <q-dialog v-model="dialogOpen" persistent>
      <q-card style="min-width: 500px;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editSkill') : t('createSkill') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input
            v-model="form.key"
            :label="t('skillKey')"
            outlined
            dense
            :readonly="!!editingId"
            :class="{ 'bg-grey-2': editingId }"
            class="q-mb-md"
          />
          <q-input
            v-model="form.name"
            :label="t('skillName')"
            outlined
            dense
            class="q-mb-md"
          />
          <q-input
            v-model="form.description"
            :label="t('description')"
            outlined
            dense
            type="textarea"
            rows="2"
          />
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveSkill" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Content Editor Dialog -->
    <q-dialog v-model="editorOpen" maximized>
      <q-card>
        <q-card-section class="row items-center q-pa-sm">
          <div class="text-h6">{{ t('editSkillContent') }}: {{ editingSkill?.name }}</div>
          <q-space />
          <q-btn flat :label="t('cancel')" v-close-popup class="q-mr-sm" />
          <q-btn color="primary" :label="t('save')" @click="saveSkillContent" :loading="saving" unelevated />
        </q-card-section>

        <q-card-section class="q-pa-sm">
          <div class="q-mb-md">
            <div class="text-caption text-grey">{{ t('labelKey') }}</div>
            <div class="text-body1 text-wrap">{{ editingSkill?.key }}</div>
          </div>
          <div class="row q-col-gutter-md q-mb-md">
            <div class="col-6">
              <q-select
                v-model="editingSkill!.risk_level"
                :options="skillRiskOptions"
                :label="t('riskLevel')"
                outlined
                dense
                emit-value
                map-options
              />
            </div>
            <div class="col-6">
              <q-select
                v-model="editingSkill!.execution_mode"
                :options="skillExecutionModeOptions"
                :label="t('executionLocation')"
                outlined
                dense
                emit-value
                map-options
                :hint="t('skillsExecHint')"
              />
            </div>
          </div>
        </q-card-section>

        <q-separator />

        <q-card-section class="q-pa-none skill-editor-wrap">
          <q-splitter
            v-model="editorSplit"
            unit="%"
            :limits="[28, 72]"
            :horizontal="$q.screen.lt.md"
            class="skill-md-splitter"
          >
            <template #before>
              <div class="q-pa-md column full-height">
                <div class="text-caption text-grey q-mb-xs">{{ t('skillMarkdownSource') }}</div>
                <q-input
                  v-model="editingSkill!.content"
                  outlined
                  type="textarea"
                  autogrow
                  class="skill-md-source col"
                  input-class="skill-md-textarea"
                  :input-style="{ minHeight: 'min(60vh, 520px)' }"
                />
              </div>
            </template>
            <template #after>
              <div class="q-pa-md column full-height skill-md-preview-col">
                <div class="text-caption text-grey q-mb-xs">{{ t('skillMarkdownPreview') }}</div>
                <!-- eslint-disable vue/no-v-html -- sanitized via renderChatMarkdown (DOMPurify) -->
                <div class="skill-md-preview col scroll" v-html="previewHtml" />
              </div>
            </template>
          </q-splitter>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { useQuasar } from 'quasar'
import { useSkillsPage } from './useSkillsPage'

defineOptions({ name: 'SkillsPage' })

const $q = useQuasar()

const {
  t,
  loading,
  saving,
  rows,
  pagination,
  columns,
  search,
  load,
  syncBuiltinSkills,
  syncLoading,
  editorSplit,
  previewHtml,
  dialogOpen,
  form,
  editingId,
  openCreateDialog,
  saveSkill,
  editorOpen,
  editingSkill,
  openEditor,
  saveSkillContent,
  toggleActive,
  confirmDelete,
  riskLevelLabel,
  skillRiskOptions,
  skillExecutionModeOptions
} = useSkillsPage()

function riskBadgeColor (raw: string | undefined): string {
  const r = (raw || 'medium').toLowerCase()
  if (r === 'low') return 'positive'
  if (r === 'medium') return 'warning'
  if (r === 'high') return 'deep-orange'
  if (r === 'critical') return 'negative'
  return 'grey'
}
</script>

<style scoped>
/* 不用 table-layout:fixed，避免描述列长文本在部分浏览器下溢出盖住相邻列 */
.skill-table-wrap :deep(table.q-table) {
  width: 100%;
  table-layout: auto;
}

.skill-table-col-key {
  width: 220px;
  min-width: 220px;
  max-width: 220px;
  vertical-align: middle;
}
.skill-key-text {
  word-break: break-all;
  overflow-wrap: anywhere;
  line-height: 1.35;
  font-size: 12px;
}

.skill-editor-wrap {
  min-height: min(70vh, 640px);
}
.skill-md-splitter {
  height: calc(100vh - 200px);
  min-height: 360px;
}
.skill-md-textarea {
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.45;
}
/* 预览区：覆盖浏览器对 h1 的默认超大字号，与左侧源码密度接近 */
.skill-md-preview {
  font-size: 13px;
  line-height: 1.5;
  padding: 10px 12px;
  border: 1px solid rgba(0, 0, 0, 0.12);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.02);
  color: rgba(0, 0, 0, 0.87);
}
.body--dark .skill-md-preview {
  border-color: rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(255, 255, 255, 0.88);
}
.skill-md-preview :deep(h1) {
  font-size: 1.125rem;
  font-weight: 600;
  margin: 0.5em 0 0.35em;
  line-height: 1.35;
}
.skill-md-preview :deep(h2) {
  font-size: 1.05rem;
  font-weight: 600;
  margin: 0.55em 0 0.3em;
  line-height: 1.35;
}
.skill-md-preview :deep(h3) {
  font-size: 1rem;
  font-weight: 600;
  margin: 0.5em 0 0.28em;
  line-height: 1.35;
}
.skill-md-preview :deep(h4),
.skill-md-preview :deep(h5),
.skill-md-preview :deep(h6) {
  font-size: 0.95rem;
  font-weight: 600;
  margin: 0.45em 0 0.25em;
  line-height: 1.35;
}
.skill-md-preview :deep(p) {
  margin: 0.35em 0;
  font-size: inherit;
}
.skill-md-preview :deep(ul),
.skill-md-preview :deep(ol) {
  margin: 0.35em 0;
  padding-left: 1.25em;
  font-size: inherit;
}
.skill-md-preview :deep(li) {
  margin: 0.2em 0;
}
.skill-md-preview :deep(blockquote) {
  margin: 0.5em 0;
  padding: 0.25em 0 0.25em 0.75em;
  border-left: 3px solid rgba(0, 0, 0, 0.15);
  font-size: inherit;
}
.body--dark .skill-md-preview :deep(blockquote) {
  border-left-color: rgba(255, 255, 255, 0.2);
}
.skill-md-preview :deep(pre),
.skill-md-preview :deep(code) {
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
  font-size: 12px;
}
.skill-md-preview :deep(pre) {
  padding: 8px 10px;
  overflow: auto;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.06);
  margin: 0.5em 0;
}
.body--dark .skill-md-preview :deep(pre) {
  background: rgba(0, 0, 0, 0.35);
}
.skill-md-preview :deep(a) {
  font-size: inherit;
}

.skill-table-col-risk {
  min-width: 120px;
  width: 120px;
  max-width: 120px;
  text-align: center;
  vertical-align: middle;
}

.skill-table-col-name {
  max-width: 140px;
  width: 9%;
  vertical-align: middle;
}
.skill-name-text {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  line-height: 1.35;
  font-size: 13px;
  word-break: break-word;
}

.skill-table-col-desc {
  min-width: 0;
  max-width: 100%;
}
.skill-desc-cell {
  word-break: break-word;
  overflow-wrap: anywhere;
  line-height: 1.4;
  font-size: 13px;
}

.skill-table-col-status {
  width: 80px;
  min-width: 80px;
  max-width: 80px;
  text-align: center;
  vertical-align: middle;
}

.skill-table-col-actions {
  min-width: 200px;
  width: 200px;
  max-width: 200px;
  text-align: center;
  vertical-align: middle;
}

/* 拉近「状态」与「操作」两列的间距（减小相邻单元格内侧 padding） */
.skill-table-wrap :deep(td.skill-table-col-status) {
  padding-right: 4px;
}
.skill-table-wrap :deep(td.skill-table-col-actions) {
  padding-left: 4px;
}
.skill-actions-cell {
  width: 100%;
}
</style>
