<template>
  <q-page padding>
    <q-banner dense rounded class="bg-grey-2 text-dark q-mb-md text-body2">
      {{ t('botsPageGlobalHint') }}
    </q-banner>

    <div class="row q-col-gutter-md q-mb-md items-center">
      <div class="col-12 col-md-6">
        <div class="text-h6 text-text2">{{ t('botTitle') }}</div>
      </div>
      <div class="col-12 col-md-6 row q-gutter-sm justify-end">
        <q-btn
          color="positive"
          :icon="running ? 'stop' : 'play_arrow'"
          :label="running ? t('stopAll') : t('startAll')"
          :loading="loading"
          @click="running ? stopAll() : startAll()"
        />
      </div>
    </div>

    <p class="text-caption text-grey-7 q-mb-md">
      {{ botCount }} {{ t('agents') }}
    </p>

    <q-table
      flat
      bordered
      class="radius-sm"
      :rows="bots"
      :columns="columns"
      row-key="agent_id"
      :loading="loading"
      :no-data-label="t('botsPageNoData')"
    >
      <template #body-cell-bot_type="props">
        <q-td :props="props">
          <q-badge :color="props.row.bot_type === 'lark' ? 'blue' : 'primary'" :label="props.row.bot_type === 'lark' ? '飞书' : 'Telegram'" />
        </q-td>
      </template>
      <template #body-cell-app_id="props">
        <q-td :props="props">
          <code v-if="props.row.app_id" class="text-xs">{{ props.row.app_id }}</code>
          <span v-else class="text-grey-5">-</span>
        </q-td>
      </template>
      <template #body-cell-is_running="props">
        <q-td :props="props">
          <q-icon
            :name="props.row.is_running ? 'check_circle' : 'remove_circle_outline'"
            :color="props.row.is_running ? 'positive' : 'grey'"
          />
        </q-td>
      </template>
      <template #body-cell-ops="props">
        <q-td :props="props">
          <q-btn
            v-if="!props.row.is_running"
            dense
            flat
            color="positive"
            icon="play_arrow"
            :label="t('botRowStart')"
            :loading="rowBusyAgentId === props.row.agent_id"
            :disable="rowBusyAgentId !== null && rowBusyAgentId !== props.row.agent_id"
            @click="startRowAgent(props.row.agent_id, props.row.bot_type)"
          />
          <q-btn
            v-else
            dense
            flat
            color="orange"
            icon="stop"
            :label="t('botRowStop')"
            :loading="rowBusyAgentId === props.row.agent_id"
            :disable="rowBusyAgentId !== null && rowBusyAgentId !== props.row.agent_id"
            @click="stopRowAgent(props.row.agent_id, props.row.bot_type)"
          />
          <q-btn
            dense
            flat
            color="negative"
            :label="t('botUnregister')"
            :loading="rowBusyAgentId === props.row.agent_id"
            :disable="rowBusyAgentId !== null && rowBusyAgentId !== props.row.agent_id"
            class="q-ml-xs"
            @click="confirmUnregister(props.row.agent_id, props.row.bot_type, props.row.agent_name || props.row.app_id || props.row.chat_id)"
          />
        </q-td>
      </template>
      <template #no-data>
        <div class="full-width row flex-center q-pa-lg text-grey-6">
          <div class="text-center">
            <q-icon name="hub" size="48px" class="q-mb-md" />
            <div>{{ t('botsPageNoData') }}</div>
            <div class="text-caption">{{ t('botsPageNoDataHint') }}</div>
          </div>
        </div>
      </template>
    </q-table>
  </q-page>
</template>

<script setup lang="ts">
import { useBotPage } from 'pages/useBotPage'

defineOptions({ name: 'BotPage' })

const {
  t,
  loading,
  rowBusyAgentId,
  bots,
  botCount,
  running,
  columns,
  confirmUnregister,
  startRowAgent,
  stopRowAgent,
  startAll,
  stopAll
} = useBotPage()
</script>
