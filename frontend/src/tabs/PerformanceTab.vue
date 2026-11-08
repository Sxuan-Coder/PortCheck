<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMonitor } from '../composables/useMonitor'
import { HistoryService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { HistoryPoint } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { formatGB, formatNumber, formatPercent } from '../lib/format'

const { state } = useMonitor()

const bars = ref<number[]>(new Array(48).fill(0))

watch(
  () => state.perf.cpuPercent,
  (v) => {
    bars.value.push(Math.min(100, Math.max(0, v)))
    bars.value.shift()
  },
)

const memPct = () =>
  state.perf.memTotalGB > 0 ? (state.perf.memUsedGB / state.perf.memTotalGB) * 100 : 0

const commitPct = () =>
  state.perf.commitTotalGB > 0 ? (state.perf.commitUsedGB / state.perf.commitTotalGB) * 100 : 0

// ===== 历史趋势（后端 5s 帧按日落盘，此处按范围分桶查询） =====
const rangeOptions = [
  { value: 1, label: '1 小时' },
  { value: 24, label: '24 小时' },
  { value: 168, label: '7 天' },
]
const rangeHours = ref(1)
const history = ref<HistoryPoint[]>([])
const historyLoading = ref(false)
const VIEW = { w: 600, h: 160 }

async function loadHistory() {
  if (historyLoading.value || document.hidden) return
  historyLoading.value = true
  try {
    const r = await HistoryService.QuerySeries(rangeHours.value)
    history.value = r.points
  } catch {
    /* 查询失败保持旧数据，静默 */
  } finally {
    historyLoading.value = false
  }
}

watch(rangeHours, loadHistory)

let historyTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  loadHistory()
  // 窗口隐藏（托盘）时 loadHistory 直接返回，不产生查询与文件读取。
  historyTimer = setInterval(loadHistory, 30_000)
})
onBeforeUnmount(() => {
  if (historyTimer) clearInterval(historyTimer)
})

// 有效数据的时间域：查询范围可能早于记录起点（应用未运行/新功能上线），
// x 轴收缩到实际有数据的区间，避免折线挤在右侧、左侧大片空白像是渲染坏了。
const dataDomain = computed(() => {
  const pts = history.value
  let first: HistoryPoint | undefined
  let last: HistoryPoint | undefined
  for (const p of pts) {
    if (p.hasData) {
      if (!first) first = p
      last = p
    }
  }
  if (!first || !last) return null
  const step = pts.length > 1 ? pts[1].ts - pts[0].ts : 12_000
  return { t0: first.ts, t1: last.ts + step, trimmedStart: pts[0].ts !== first.ts }
})
const coverageLabel = computed(() => {
  const d = dataDomain.value
  if (!d || !d.trimmedStart) return ''
  return `数据自 ${new Date(d.t0).toLocaleTimeString('zh-CN', { hour12: false })} 起`
})

// 把分桶点转成 SVG 折线段：断档（hasData=false）处断线。
function buildSegments(key: (p: HistoryPoint) => number): string[] {
  const pts = history.value
  const domain = dataDomain.value
  if (pts.length === 0 || !domain) return []
  const { t0, t1 } = domain
  const x = (ts: number) => ((ts - t0) / (t1 - t0)) * VIEW.w
  const y = (v: number) => VIEW.h - (Math.min(100, Math.max(0, v)) / 100) * VIEW.h
  const segs: string[] = []
  let cur: string[] = []
  for (const p of pts) {
    if (!p.hasData) {
      if (cur.length >= 2) segs.push(cur.join(' '))
      cur = []
      continue
    }
    cur.push(`${x(p.ts).toFixed(1)},${y(key(p)).toFixed(1)}`)
  }
  if (cur.length >= 2) segs.push(cur.join(' '))
  return segs
}
const cpuSegments = () => buildSegments((p) => p.cpu)
const memSegments = () =>
  buildSegments((p) => (p.memTotalMB > 0 ? (p.memMB / p.memTotalMB) * 100 : 0))
</script>

<template>
  <div class="perf">
    <div class="cards">
      <div class="card acrylic-card">
        <div class="card-head">
          <span>CPU</span>
          <span class="tag" :style="{ color: 'var(--brand)' }">{{ formatPercent(state.perf.cpuPercent, 0) }}</span>
        </div>
        <div class="card-title">{{ state.perf.cpuName || '未知处理器' }}</div>
        <div class="card-sub">逻辑核心 {{ state.perf.numCores }}C</div>
        <div class="meter">
          <span :style="{ width: Math.min(100, state.perf.cpuPercent) + '%' }" />
        </div>
      </div>

      <div class="card acrylic-card">
        <div class="card-head">
          <span>物理内存</span>
          <span class="tag" :style="{ color: 'var(--purple)' }">{{ formatPercent(memPct(), 0) }}</span>
        </div>
        <div class="card-title">{{ formatGB(state.perf.memUsedGB) }} / {{ formatGB(state.perf.memTotalGB) }}</div>
        <div class="card-sub">已用 / 总量</div>
        <div class="meter">
          <span :style="{ width: memPct() + '%', background: 'var(--purple)' }" />
        </div>
      </div>

      <div class="card acrylic-card">
        <div class="card-head">
          <span>提交内存</span>
          <span class="tag" :style="{ color: 'var(--amber)' }">{{ formatPercent(commitPct(), 0) }}</span>
        </div>
        <div class="card-title">{{ formatGB(state.perf.commitUsedGB) }} / {{ formatGB(state.perf.commitTotalGB) }}</div>
        <div class="card-sub">已提交 / 提交限制（含页面文件）</div>
        <div class="meter">
          <span :style="{ width: commitPct() + '%', background: 'var(--amber)' }" />
        </div>
      </div>

      <div class="card acrylic-card">
        <div class="card-head">
          <span>网络端口概览</span>
        </div>
        <div class="kv"><span>TCP 连接</span><b class="mono">{{ formatNumber(state.portStats.tcpCount) }}</b></div>
        <div class="kv"><span>UDP 连接</span><b class="mono">{{ formatNumber(state.portStats.udpCount) }}</b></div>
        <div class="kv"><span>监听端口</span><b class="mono">{{ formatNumber(state.portStats.listeningCount) }}</b></div>
        <div class="kv"><span>关联进程</span><b class="mono">{{ formatNumber(state.portStats.processCount) }}</b></div>
      </div>
    </div>

    <div class="chart acrylic-card">
      <div class="chart-head">
        <span>CPU 占用率历史（近 48 秒）</span>
        <span class="legend">实时刷新 1s</span>
      </div>
      <div class="bars">
        <div
          v-for="(b, i) in bars"
          :key="i"
          class="bar"
          :style="{ height: Math.max(4, b) + '%' }"
          :title="b.toFixed(0) + '%'"
        />
      </div>
    </div>

    <div class="chart chart-history acrylic-card">
      <div class="chart-head">
        <span>历史趋势（CPU / 内存）</span>
        <div class="range">
          <button
            v-for="opt in rangeOptions"
            :key="opt.value"
            :class="{ on: rangeHours === opt.value }"
            @click="rangeHours = opt.value"
          >{{ opt.label }}</button>
        </div>
      </div>
      <div class="legend-row">
        <span class="dot cpu-dot" /> CPU
        <span class="dot mem-dot" /> 内存
        <span v-if="coverageLabel" class="coverage">{{ coverageLabel }}</span>
        <span v-if="historyLoading" class="loading-tip">查询中…</span>
      </div>
      <svg
        class="history-svg"
        :viewBox="`0 0 ${VIEW.w} ${VIEW.h}`"
        preserveAspectRatio="none"
      >
        <line x1="0" y1="25%" x2="100%" y2="25%" class="grid" />
        <line x1="0" y1="50%" x2="100%" y2="50%" class="grid" />
        <line x1="0" y1="75%" x2="100%" y2="75%" class="grid" />
        <g v-for="(seg, i) in cpuSegments()" :key="'c' + i">
          <polyline :points="seg" fill="none" class="cpu-line" stroke-width="1.5" />
        </g>
        <g v-for="(seg, i) in memSegments()" :key="'m' + i">
          <polyline :points="seg" fill="none" class="mem-line" stroke-width="1.5" />
        </g>
      </svg>
      <div v-if="!dataDomain" class="empty-history">该范围内暂无历史数据（记录随应用运行积累，最多保留 7 天）</div>
    </div>
  </div>
</template>

<style scoped>
.perf {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.card {
  border-radius: var(--radius-lg);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 120px;
}
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-3);
}
.tag {
  font-weight: 700;
}
.card-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-1);
}
.card-sub {
  font-size: 11px;
  color: var(--text-4);
}
.meter {
  margin-top: auto;
  height: 6px;
  border-radius: 99px;
  background: var(--field-bg);
  overflow: hidden;
}
.meter span {
  display: block;
  height: 100%;
  background: var(--brand);
  transition: width 0.4s ease;
}
.kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-3);
}
.kv b {
  color: var(--text-1);
}
.chart {
  /* 固定高度 + 裁剪：弹性压缩曾导致 SVG 溢出卡片边界；页面本身可滚动 */
  flex: none;
  height: 220px;
  position: relative;
  overflow: hidden;
  border-radius: var(--radius-lg);
  padding: 14px;
  display: flex;
  flex-direction: column;
}
/* 历史趋势卡片放大：主视图，页面滚动查看 */
.chart-history {
  height: 340px;
}
.chart-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  font-size: 12px;
  color: var(--text-2);
}
.legend {
  font-size: 10px;
  color: var(--brand);
  background: var(--brand-glow);
  padding: 2px 8px;
  border-radius: 99px;
}
.bars {
  flex: 1;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  min-height: 0;
}
.bar {
  flex: 1;
  background: linear-gradient(to top, var(--brand-dark), var(--brand));
  border-radius: 3px 3px 0 0;
  transition: height 0.4s ease;
  min-height: 3px;
}
.range {
  display: flex;
  gap: 4px;
}
.range button {
  padding: 3px 10px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: 99px;
  color: var(--text-3);
  font-size: 11px;
}
.range button.on {
  color: var(--brand);
  border-color: var(--brand-glow);
  background: var(--brand-glow);
}
.legend-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-3);
  margin-bottom: 6px;
}
.dot {
  width: 14px;
  height: 3px;
  border-radius: 99px;
  display: inline-block;
  margin-left: 8px;
}
.dot:first-child {
  margin-left: 0;
}
.cpu-dot { background: var(--brand); }
.mem-dot { background: var(--purple); }
.loading-tip { color: var(--text-4); margin-left: 8px; }
.coverage { color: var(--text-4); margin-left: 10px; font-size: 10px; }
.history-svg {
  flex: 1;
  width: 100%;
  min-height: 0;
}
.history-svg .grid {
  stroke: var(--hairline);
  stroke-width: 1;
  stroke-dasharray: 4 6;
}
.history-svg .cpu-line {
  stroke: var(--brand);
}
.history-svg .mem-line {
  stroke: var(--purple);
}
.empty-history {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 12px;
  text-align: center;
  font-size: 11px;
  color: var(--text-4);
}
</style>
