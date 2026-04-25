<template>
  <q-dialog
    ref="dialogRef"
    maximized
    transition-show="fade"
    transition-hide="fade"
    @hide="onDialogHide"
  >
    <q-card class="column no-wrap full-height chat-img-preview-card">
      <q-card-section class="row items-center q-py-sm q-px-md text-white">
        <span class="text-caption text-weight-medium">{{ t('chatImagePreviewHint') }}</span>
        <q-space />
        <q-btn
          flat
          round
          dense
          icon="restart_alt"
          color="white"
          :aria-label="t('chatImagePreviewReset')"
          @click="resetZoom"
        />
        <q-btn
          flat
          round
          dense
          icon="close"
          color="white"
          :aria-label="t('cancel')"
          @click="onDialogCancel"
        />
      </q-card-section>
      <q-separator dark />
      <q-card-section class="col q-pa-none scroll chat-img-preview-body">
        <div
          class="chat-img-preview-viewport"
          @wheel="onWheel"
          @dblclick="resetZoom"
        >
          <div class="chat-img-preview-inner flex flex-center">
            <img
              :src="src"
              class="chat-img-preview-img"
              :class="{ 'chat-img-preview-img--sized': naturalW > 0 }"
              alt=""
              loading="eager"
              draggable="false"
              :width="naturalW > 0 ? displayW : undefined"
              :height="naturalW > 0 ? displayH : undefined"
              @load="onImgLoad"
            >
          </div>
        </div>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useDialogPluginComponent } from 'quasar'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  src: string
}>()

defineEmits([...useDialogPluginComponent.emits])

const { t } = useI18n()
const { dialogRef, onDialogHide, onDialogCancel } = useDialogPluginComponent()

const naturalW = ref(0)
const naturalH = ref(0)
const scale = ref(1)

const MIN_SCALE = 0.25
const MAX_SCALE = 8

const displayW = computed(() => Math.max(1, Math.round(naturalW.value * scale.value)))
const displayH = computed(() => Math.max(1, Math.round(naturalH.value * scale.value)))

function onImgLoad (e: Event): void {
  const el = e.target as HTMLImageElement
  naturalW.value = el.naturalWidth || 1
  naturalH.value = el.naturalHeight || 1
}

function resetZoom (): void {
  scale.value = 1
}

function onWheel (e: WheelEvent): void {
  e.preventDefault()
  const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1
  const next = scale.value * factor
  scale.value = Math.min(MAX_SCALE, Math.max(MIN_SCALE, next))
}

watch(
  () => props.src,
  () => {
    naturalW.value = 0
    naturalH.value = 0
    scale.value = 1
  }
)
</script>

<style scoped lang="sass">
.chat-img-preview-card
  background: #121214 !important

.chat-img-preview-body
  min-height: 0
  flex: 1 1 auto

.chat-img-preview-viewport
  width: 100%
  height: calc(100vh - 52px)
  max-height: calc(100vh - 52px)
  overflow: auto
  cursor: zoom-in
  box-sizing: border-box
  -webkit-overflow-scrolling: touch

.chat-img-preview-inner
  min-width: 100%
  min-height: 100%
  padding: 16px
  box-sizing: border-box

.chat-img-preview-img
  display: block
  user-select: none

.chat-img-preview-img:not(.chat-img-preview-img--sized)
  max-width: min(96vw, 1400px)
  width: auto
  height: auto

.chat-img-preview-img--sized
  max-width: none
</style>
