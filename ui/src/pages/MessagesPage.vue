<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('messages') }}</div>
      <q-space />
      <q-btn flat icon="refresh" round dense :loading="loading" @click="load" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-tabs v-model="activeTab" dense class="text-grey-7" active-color="primary" indicator-color="primary" align="left" narrow-indicator @update:model-value="onTabChange">
      <q-tab name="channels" :label="t('messageChannels')" />
      <q-tab name="messages" :label="t('agentMessages')" />
      <q-tab name="a2a" :label="t('a2aCards')" />
    </q-tabs>

    <q-separator />

    <q-tab-panels v-model="activeTab" animated>
      <!-- Channels Panel -->
      <q-tab-panel name="channels">
        <div class="row items-center q-mb-md">
          <q-space />
          <q-btn color="primary" :label="t('createChannel')" icon="add" class="q-mr-sm" unelevated rounded @click="openChannelDialog()" />
        </div>

        <q-table flat bordered :rows="channelRows" :columns="channelColumns" row-key="id" :loading="loading" no-data-label="暂无数据">
          <template #body-cell-is_public="props">
            <q-td :props="props">
              <q-badge :color="props.row.is_public ? 'blue' : 'grey'" :label="props.row.is_public ? '公开' : '私有'" />
            </q-td>
          </template>
          <template #body-cell-is_active="props">
            <q-td :props="props">
              <q-badge :color="props.row.is_active ? 'positive' : 'grey'" :label="props.row.is_active ? t('active') : t('inactive')" />
            </q-td>
          </template>
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn dense flat color="primary" :label="t('edit')" @click="openChannelDialog(props.row)" />
              <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDeleteChannel(props.row)" />
            </q-td>
          </template>
        </q-table>

        <q-dialog v-model="channelDialogOpen">
          <q-card style="min-width: 520px; max-width: 92vw;">
            <q-card-section class="row items-center">
              <div class="text-h6">{{ channelEditingId ? t('editChannel') : t('createChannel') }}</div>
              <q-space />
              <q-btn v-close-popup icon="close" flat round dense />
            </q-card-section>

            <q-card-section class="q-pt-none">
              <div class="row q-col-gutter-md">
                <div class="col-12">
                  <q-input v-model="channelForm.name" :label="t('channelName')" outlined dense :rules="[v => !!v]" />
                </div>
                <div class="col-12">
                  <q-select
                    v-model="channelForm.agent_id"
                    :label="t('messageAgent')"
                    :options="agentOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    :rules="[v => !!v]"
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-select
                    v-model="channelForm.kind"
                    :label="t('channelKind')"
                    :options="kindOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    :disable="!!channelEditingId"
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-input v-model="channelForm.description" :label="t('description')" outlined dense />
                </div>
              </div>

              <div class="row q-col-gutter-md q-mt-sm">
                <div class="col-6">
                  <q-checkbox v-model="channelForm.is_public" label="公开通道" />
                </div>
                <div class="col-6">
                  <q-checkbox v-model="channelForm.is_active" :label="t('active')" />
                </div>
              </div>
            </q-card-section>

            <q-card-actions align="right">
              <q-btn v-close-popup flat :label="t('cancel')" />
              <q-btn color="primary" :label="t('save')" :loading="saving" unelevated @click="saveChannel" />
            </q-card-actions>
          </q-card>
        </q-dialog>
      </q-tab-panel>

      <!-- Messages Panel -->
      <q-tab-panel name="messages">
        <div class="row items-center q-mb-md">
          <div class="row q-col-gutter-sm q-mr-md">
            <div class="col-auto">
              <q-select
                v-model="messageFilter.agent_id"
                :label="t('messageAgent')"
                :options="agentOptions"
                outlined
                dense
                emit-value
                map-options
                clearable
                style="min-width: 160px;"
                @update:model-value="loadMessages"
              />
            </div>
            <div class="col-auto">
              <q-select
                v-model="messageFilter.channel_id"
                label="通道"
                :options="channelOptions"
                outlined
                dense
                emit-value
                map-options
                clearable
                style="min-width: 160px;"
                @update:model-value="loadMessages"
              />
            </div>
            <div class="col-auto">
              <q-select
                v-model="messageFilter.status"
                label="状态"
                :options="[{ label: 'pending', value: 'pending' }, { label: 'delivered', value: 'delivered' }]"
                outlined
                dense
                emit-value
                map-options
                clearable
                style="min-width: 120px;"
                @update:model-value="loadMessages"
              />
            </div>
          </div>
          <q-space />
          <q-btn color="primary" label="发送消息" icon="send" class="q-mr-sm" unelevated rounded @click="openMessageDialog()" />
          <q-btn color="secondary" label="发送 Span" icon="timeline" unelevated rounded @click="openMessageDialog()" />
        </div>

        <q-table flat bordered :rows="messageRows" :columns="messageColumns" row-key="id" :loading="loading" no-data-label="暂无消息">
          <template #body-cell-content="props">
            <q-td :props="props">
              <span class="ellipsis block" style="max-width: 300px;">{{ props.row.content }}</span>
            </q-td>
          </template>
          <template #body-cell-status="props">
            <q-td :props="props">
              <q-badge :color="props.row.status === 'delivered' ? 'positive' : 'orange'" :label="props.row.status" />
            </q-td>
          </template>
        </q-table>

        <q-dialog v-model="messageDialogOpen">
          <q-card style="min-width: 520px; max-width: 92vw;">
            <q-card-section class="row items-center">
              <div class="text-h6">发送 Agent 消息</div>
              <q-space />
              <q-btn v-close-popup icon="close" flat round dense />
            </q-card-section>

            <q-card-section class="q-pt-none">
              <div class="row q-col-gutter-md">
                <div class="col-12">
                  <q-select
                    v-model="messageForm.from_agent_id"
                    label="发送方"
                    :options="agentOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    :rules="[v => !!v]"
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-select
                    v-model="messageForm.to_agent_id"
                    label="接收方"
                    :options="agentOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    clearable
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-select
                    v-model="messageForm.channel_id"
                    label="通道"
                    :options="channelOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    clearable
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-select
                    v-model="messageForm.kind"
                    label="消息类型"
                    :options="messageKindOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-input v-model.number="messageForm.priority" label="优先级" outlined dense type="number" />
                </div>
              </div>

              <q-input
                v-model="messageForm.content"
                label="消息内容"
                outlined
                dense
                type="textarea"
                rows="4"
                class="q-mt-md"
                :rules="[v => !!v]"
              />
            </q-card-section>

            <q-card-actions align="right">
              <q-btn v-close-popup flat :label="t('cancel')" />
              <q-btn color="primary" label="发送消息" :loading="saving" unelevated @click="sendMessage" />
              <q-btn color="secondary" label="发送 Span" :loading="saving" unelevated @click="sendSpan" />
            </q-card-actions>
          </q-card>
        </q-dialog>
      </q-tab-panel>

      <!-- A2A Cards Panel -->
      <q-tab-panel name="a2a">
        <div class="row items-center q-mb-md">
          <q-space />
          <q-btn color="primary" label="创建 A2A 卡片" icon="add" class="q-mr-sm" unelevated rounded @click="openA2ADialog()" />
        </div>

        <q-table flat bordered :rows="a2aRows" :columns="a2aColumns" row-key="id" :loading="loading" no-data-label="暂无数据">
          <template #body-cell-url="props">
            <q-td :props="props">
              <span class="ellipsis block" style="max-width: 250px;">{{ props.row.url || '—' }}</span>
            </q-td>
          </template>
          <template #body-cell-is_active="props">
            <q-td :props="props">
              <q-badge :color="props.row.is_active ? 'positive' : 'grey'" :label="props.row.is_active ? t('active') : t('inactive')" />
            </q-td>
          </template>
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn dense flat color="primary" :label="t('edit')" @click="openA2ADialog(props.row)" />
              <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDeleteA2ACard(props.row)" />
            </q-td>
          </template>
        </q-table>

        <q-dialog v-model="a2aDialogOpen">
          <q-card style="min-width: 520px; max-width: 92vw;">
            <q-card-section class="row items-center">
              <div class="text-h6">{{ a2aEditingId ? '编辑 A2A 卡片' : '创建 A2A 卡片' }}</div>
              <q-space />
              <q-btn v-close-popup icon="close" flat round dense />
            </q-card-section>

            <q-card-section class="q-pt-none">
              <div class="row q-col-gutter-md">
                <div class="col-12">
                  <q-select
                    v-model="a2aForm.agent_id"
                    :label="t('messageAgent')"
                    :options="agentOptions"
                    outlined
                    dense
                    emit-value
                    map-options
                    :rules="[v => !!v]"
                  />
                </div>
                <div class="col-12 col-sm-6">
                  <q-input v-model="a2aForm.name" label="卡片名称" outlined dense :rules="[v => !!v]" />
                </div>
                <div class="col-12 col-sm-6">
                  <q-input v-model="a2aForm.version" label="版本" outlined dense />
                </div>
                <div class="col-12">
                  <q-input v-model="a2aForm.url" label="URL" outlined dense />
                </div>
                <div class="col-12">
                  <q-input v-model="a2aForm.description" label="描述" outlined dense type="textarea" rows="3" />
                </div>
                <div class="col-12">
                  <q-checkbox v-model="a2aForm.is_active" :label="t('active')" />
                </div>
              </div>
            </q-card-section>

            <q-card-actions align="right">
              <q-btn v-close-popup flat :label="t('cancel')" />
              <q-btn color="primary" :label="t('save')" :loading="saving" unelevated @click="saveA2ACard" />
            </q-card-actions>
          </q-card>
        </q-dialog>
      </q-tab-panel>
    </q-tab-panels>
  </q-page>
</template>

<script setup lang="ts">
import { useMessagesPage } from 'pages/useMessagesPage'

defineOptions({ name: 'MessagesPage' })

const {
  t, loading, saving, errorMsg, activeTab,
  channelRows, messageRows, a2aRows,
  channelColumns, messageColumns, a2aColumns,
  channelDialogOpen, channelEditingId, channelForm,
  messageDialogOpen, messageForm,
  a2aDialogOpen, a2aEditingId, a2aForm,
  messageFilter, kindOptions, messageKindOptions,
  agentOptions, channelOptions,
  openChannelDialog, saveChannel, confirmDeleteChannel,
  openMessageDialog, sendMessage, sendSpan,
  openA2ADialog, saveA2ACard, confirmDeleteA2ACard,
  onTabChange, load, loadMessages
} = useMessagesPage()
</script>
