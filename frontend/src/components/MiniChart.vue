<script setup lang="ts">
// CPU/RAM 迷你折线图：Canvas 绘制，requestAnimationFrame 节流重绘，
// 避免每次 monitor tick 都触发绘制。历史环形缓冲 60 点。
import { onBeforeUnmount, ref, watch } from 'vue'
import { useMonitor } from '../composables/useMonitor'

const { state } = useMonitor()

const cpuHistory: number[] = new Array(60).fill(0)
const ramHistory: number[] = new Array(60).fill(0)
const cpuCanvas = ref<HTMLCanvasElement | null>(null)
const ramCanvas = ref<HTMLCanvasElement | null>(null)

let raf = 0
let dirty = false
let lastDraw = 0

// watch shallow 字段 perf：仅标记脏，真正绘制交给 RAF 合并
watch(
  () => state.perf.cpuPercent,
  () => {
    cpuHistory.push(Math.min(100, Math.max(0, state.perf.cpuPercent)))
    cpuHistory.shift()
    ramHistory.push(
      state.perf.memTotalGB > 0
        ? (state.perf.memUsedGB / state.perf.memTotalGB) * 100
        : 0,
    )
    ramHistory.shift()
    dirty = true
    schedule()
  },
)

function schedule() {
  if (raf) return
  raf = requestAnimationFrame(loop)
}

function loop(t: number) {
  // 最多 ~30fps 绘制，足够流畅且省 CPU
  if (dirty && t - lastDraw >= 33) {
    dirty = false
    lastDraw = t
    draw(cpuCanvas.value, cpuHistory, 'rgba(20,184,166,1)')
    draw(ramCanvas.value, ramHistory, 'rgba(129,140,248,1)')
  }
  raf = dirty ? requestAnimationFrame(loop) : 0
}

function draw(canvas: HTMLCanvasElement | null, data: number[], stroke: string) {
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const dpr = window.devicePixelRatio || 1
  const w = canvas.offsetWidth
  const h = canvas.offsetHeight
  if (w === 0 || h === 0) return
  if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
    canvas.width = w * dpr
    canvas.height = h * dpr
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, w, h)

  // 网格
  ctx.strokeStyle = 'rgba(128,128,128,0.08)'
  ctx.lineWidth = 1
  for (let i = 1; i < 4; i++) {
    const y = (h / 4) * i
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(w, y)
    ctx.stroke()
  }

  // 曲线
  const step = w / (data.length - 1)
  ctx.beginPath()
  data.forEach((v, i) => {
    const x = i * step
    const y = h - (v / 100) * (h - 6) - 3
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  })
  ctx.strokeStyle = stroke
  ctx.lineWidth = 1.8
  ctx.shadowColor = stroke
  ctx.shadowBlur = 6
  ctx.stroke()
  ctx.shadowBlur = 0

  // 填充
  ctx.lineTo(w, h)
  ctx.lineTo(0, h)
  ctx.closePath()
  const grad = ctx.createLinearGradient(0, 0, 0, h)
  grad.addColorStop(0, stroke.replace('1)', '0.18)'))
  grad.addColorStop(1, stroke.replace('1)', '0)'))
  ctx.fillStyle = grad
  ctx.fill()
}

onBeforeUnmount(() => {
  if (raf) cancelAnimationFrame(raf)
})
</script>

<template>
  <section class="stat-grid">
    <div class="stat-card acrylic-card">
      <div class="stat-meta">
        <span class="stat-label">CPU 占用率</span>
        <div class="stat-value">
          <span class="mono num">{{ state.perf.cpuPercent.toFixed(0) }}%</span>
        </div>
        <span class="stat-sub">{{ state.perf.cpuName || 'CPU' }} · {{ state.perf.numCores }}C</span>
      </div>
      <div class="chart-wrap"><canvas ref="cpuCanvas" /></div>
    </div>

    <div class="stat-card acrylic-card">
      <div class="stat-meta">
        <span class="stat-label">物理内存</span>
        <div class="stat-value">
          <span class="mono num">{{ state.perf.memUsedGB.toFixed(1) }}</span>
          <span class="unit">/ {{ state.perf.memTotalGB.toFixed(1) }} GB</span>
        </div>
        <span class="stat-sub">
          已用
          {{ state.perf.memTotalGB > 0
            ? ((state.perf.memUsedGB / state.perf.memTotalGB) * 100).toFixed(0)
            : 0 }}%
        </span>
      </div>
      <div class="chart-wrap"><canvas ref="ramCanvas" /></div>
    </div>
  </section>
</template>

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  padding: 16px 16px 12px;
  flex-shrink: 0;
}
.stat-card {
  border-radius: var(--radius-lg);
  padding: 12px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.stat-meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.stat-label {
  font-size: 11px;
  color: var(--text-3);
  letter-spacing: 0.02em;
}
.stat-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
}
.num {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-1);
}
.unit {
  font-size: 11px;
  color: var(--brand);
}
.stat-sub {
  font-size: 10px;
  color: var(--text-4);
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chart-wrap {
  width: 50%;
  height: 48px;
}
canvas {
  width: 100%;
  height: 100%;
  display: block;
}
</style>
