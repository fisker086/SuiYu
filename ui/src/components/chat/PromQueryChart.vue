<script setup lang="ts">
/**
 * Renders Prometheus range query matrix (array of { metric, values: [[ts, v], ...] }).
 * Mirrors auto-sre-agent ChatStep.vue chart logic.
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Chart from 'chart.js/auto'

export interface PromSeries {
  metric: Record<string, string>
  values: [number, string][]
}

const props = defineProps<{
  series: PromSeries[]
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const CHART_COLORS = [
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

function metricLegendLabel (metric: Record<string, string>, index: number): string {
  const label =
    Object.entries(metric)
      .filter(([k]) => k !== '__name__')
      .map(([k, v]) => `${k}=${v}`)
      .join(', ') ||
    metric.__name__ ||
    `series ${index + 1}`
  return label.length > 56 ? `${label.slice(0, 53)}…` : label
}

function renderChart (): void {
  destroyChart()
  if (!canvasRef.value || !props.series?.length) return
  const results = props.series
  const ctx = canvasRef.value.getContext('2d')
  if (!ctx) return

  const isInstantVector = results.every(r => r.values.length === 1)

  if (isInstantVector) {
    const labels = results.map((r, i) => metricLegendLabel(r.metric, i))
    const data = results.map(r => parseFloat(r.values[0][1]))
    const horizontal = labels.length > 6 || labels.some(l => l.length > 28)
    chart = new Chart(ctx, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'value',
            data,
            backgroundColor: results.map((_, i) =>
              CHART_COLORS[i % CHART_COLORS.length].replace('rgb', 'rgba').replace(')', ', 0.45)')
            ),
            borderColor: results.map((_, i) => CHART_COLORS[i % CHART_COLORS.length]),
            borderWidth: 1
          }
        ]
      },
      options: {
        indexAxis: horizontal ? 'y' : 'x',
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: horizontal
            ? { beginAtZero: true, grid: { color: 'rgba(0,0,0,0.06)' }, ticks: { font: { size: 10 } } }
            : { grid: { display: false }, ticks: { maxRotation: 45, minRotation: 0, font: { size: 9 } } },
          y: horizontal
            ? { grid: { display: false }, ticks: { font: { size: 9 } } }
            : { beginAtZero: true, grid: { color: 'rgba(0,0,0,0.06)' }, ticks: { font: { size: 10 } } }
        },
        plugins: {
          legend: { display: false }
        }
      }
    })
    return
  }

  const datasets = results.map((r, i) => {
    const label = metricLegendLabel(r.metric, i)
    const color = CHART_COLORS[i % CHART_COLORS.length]
    return {
      label,
      data: r.values.map(v => ({ x: v[0] * 1000, y: parseFloat(v[1]) })),
      borderColor: color,
      backgroundColor: color.replace('rgb', 'rgba').replace(')', ', 0.08)'),
      borderWidth: 2,
      pointRadius: 0,
      pointHoverRadius: 4,
      tension: 0.3,
      fill: true
    }
  })

  chart = new Chart(ctx, {
    type: 'line',
    data: { datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { intersect: false, mode: 'index' },
      scales: {
        x: {
          type: 'linear',
          grid: { display: false },
          ticks: {
            callback: value =>
              new Date(value as number).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
            maxRotation: 0,
            font: { size: 10 }
          }
        },
        y: {
          beginAtZero: false,
          grid: { color: 'rgba(0,0,0,0.06)' },
          ticks: { font: { size: 10 } }
        }
      },
      plugins: {
        legend: {
          display: datasets.length <= 6,
          position: 'bottom',
          labels: { boxWidth: 12, padding: 12, font: { size: 11 } }
        },
        tooltip: {
          padding: 10,
          callbacks: {
            title: items => {
              const x = items[0]?.parsed.x
              return typeof x === 'number' ? new Date(x).toLocaleString() : ''
            }
          }
        }
      }

    }
  })
}

onMounted(() => {
  renderChart()
})

watch(
  () => props.series,
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
  <div class="prom-chart-wrap">
    <canvas ref="canvasRef" class="prom-chart-canvas" />
  </div>
</template>

<style scoped>
.prom-chart-wrap {
  position: relative;
  width: 100%;
  height: 240px;
}
.prom-chart-canvas {
  width: 100% !important;
  height: 100% !important;
}
</style>
