<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('channels') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createChannel')" icon="add" class="q-mr-sm" unelevated rounded @click="openDialog()" />
      <q-btn flat icon="refresh" round dense :loading="loading" @click="load" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <p class="text-caption text-grey-7 q-mb-md">{{ t('channelHelp') }}</p>

    <q-table flat bordered class="radius-sm" :rows="rows" :columns="columns" row-key="id" :loading="loading" :no-data-label="t('noData')">
      <template #body-cell-webhook_url="props">
        <q-td :props="props">
          <span class="ellipsis block" style="max-width: 220px;">{{ props.row.webhook_url || '—' }}</span>
        </q-td>
      </template>
      <template #body-cell-has_app_secret="props">
        <q-td :props="props">
          <q-icon :name="props.row.has_app_secret ? 'check_circle' : 'remove_circle_outline'" :color="props.row.has_app_secret ? 'positive' : 'grey'" />
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props">
          <q-badge :color="props.row.is_active ? 'positive' : 'grey'" :label="props.row.is_active ? t('active') : t('inactive')" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="primary" :label="t('edit')" @click="openDialog(props.row)" />
          <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="dialogOpen">
      <q-card style="min-width: 640px; max-width: 92vw;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editChannel') : t('createChannel') }}</div>
          <q-space />
          <q-btn v-close-popup icon="close" flat round dense />
        </q-card-section>

        <q-card-section class="q-pt-none">
          <div class="row q-col-gutter-md">
            <div class="col-12 col-sm-6">
              <q-input v-model="form.name" :label="t('channelName')" outlined dense :rules="[v => !!v]" />
            </div>
            <div class="col-12 col-sm-6">
              <q-select
                v-model="form.kind"
                :label="t('channelKind')"
                :options="kindOptions"
                outlined
                dense
                emit-value
                map-options
                :disable="!!editingId"
              />
            </div>
          </div>

          <q-input
            v-model="form.webhook_url"
            :label="t('channelWebhook')"
            outlined
            dense
            class="q-mt-md"
            type="textarea"
            autogrow
            :placeholder="t('channelWebhookPh')"
          />

          <div class="row q-col-gutter-md q-mt-sm">
            <div class="col-12 col-sm-6">
              <q-input v-model="form.app_id" :label="t('channelAppId')" outlined dense />
            </div>
            <div class="col-12 col-sm-6">
              <q-input
                v-model="form.app_secret"
                :label="t('channelAppSecret')"
                outlined
                dense
                type="password"
                :hint="editingId ? t('channelSecretHint') : undefined"
              />
            </div>
          </div>

          <q-input
            v-model="form.extra_json"
            :label="t('channelExtra')"
            outlined
            dense
            type="textarea"
            rows="5"
            class="q-mt-md"
            :hint="t('channelExtraHint')"
          />

          <q-checkbox v-model="form.is_active" :label="t('active')" class="q-mt-md" />

          <q-separator class="q-my-md" />

          <div v-if="editingId" class="row q-col-gutter-sm items-end">
            <div class="col">
              <q-input v-model="form.test_message" :label="t('channelTestMessage')" outlined dense />
            </div>
            <div class="col-auto">
              <q-btn color="secondary" :label="t('channelTestSend')" :loading="testing" @click="testSend" unelevated />
            </div>
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn v-close-popup flat :label="t('cancel')" />
          <q-btn color="primary" :label="t('save')" :loading="saving" unelevated @click="saveChannel" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useChannelsPage } from 'pages/useChannelsPage'

defineOptions({ name: 'ChannelsPage' })

const {
  t, loading, saving, testing, rows, columns, errorMsg, dialogOpen, form, editingId,
  openDialog, saveChannel, testSend, confirmDelete, load
} = useChannelsPage()

const kindOptions = computed(() => [
  { label: t('channelKindLark'), value: 'lark' },
  { label: t('channelKindDing'), value: 'dingtalk' },
  { label: t('channelKindWecom'), value: 'wecom' }
])
</script>
