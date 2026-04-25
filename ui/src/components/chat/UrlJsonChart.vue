<script setup lang="ts">
/**
 * 由 HTTP 等接口返回的 JSON（数值序列 / Chart.js 形 / series 形）绘制折线图。
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Chart from 'chart.js/auto'
import type { UrlChartPayload } from 'src/utils/urlResponseChart'

const props = defineProps<{
  payload: UrlChartPayload
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const COLORS = [
  'rgb(25, 118, 210)',
  'rgb(211, 47, 47)',
  'rgb(56, 142, 60)',
  'rgb(245, 124, 0)',
  'rgb(106, 27, 154)',
  'rgb(194, 163, 0)'
]

function destroyChart (): void {
  if (chart) {
    chart.destroy()
    chart = null
  }
}

function renderChart (): void {
  destroyChart()
  if (!canvasRef.value || !props.payload?.datasets?.length) return
  const ctx = canvasRef.value.getContext('2d')
  if (!ctx) return

  const { labels, datasets } = props.payload
  chart = new Chart(ctx, {
    type: 'line',
    data: {
      labels,
      datasets: datasets.map((ds, i) => {
        const c = COLORS[i % COLORS.length]
        return {
          label: ds.label,
          data: ds.data,
          borderColor: c,
          backgroundColor: c.replace('rgb', 'rgba').replace(')', ', 0.12)'),
          borderWidth: 2,
          pointRadius: 3,
          pointHoverRadius: 5,
          tension: 0.2,
          fill: datasets.length === 1
        }
      })
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { intersect: false, mode: 'index' },
      scales: {
        x: {
          grid: { color: 'rgba(0,0,0,0.06)' },
          ticks: { maxRotation: 45, minRotation: 0, font: { size: 10 } }
        },
        y: {
          beginAtZero: false,
          grid: { color: 'rgba(0,0,0,0.06)' },
          ticks: { font: { size: 10 } }
        }
      },
      plugins: {
        legend: {
          display: datasets.length > 1,
          position: 'bottom',
          labels: { boxWidth: 12, padding: 12, font: { size: 11 } }
        }
      }
    }
  })
}

onMounted(() => {
  renderChart()
})

watch(
  () => props.payload,
  () => {
    renderChart()
  },
  { deep: true }
)

onBeforeUnmount(() => {
  destroyChart()
})
</script>

<template>
  <div class="url-json-chart-wrap">
    <canvas ref="canvasRef" class="url-json-chart-canvas" />
  </div>
</template>

<style scoped>
.url-json-chart-wrap {
  position: relative;
  width: 100%;
  height: 240px;
}
.url-json-chart-canvas {
  width: 100% !important;
  height: 100% !important;
}
</style>
