<script setup lang="ts">
import { ref, watch } from 'vue'
import { useMonitor } from '../composables/useMonitor'
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
  </div>
</template>

<style scoped>
.perf {
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: 100%;
}
.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
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
  flex: 1;
  border-radius: var(--radius-lg);
  padding: 14px;
  display: flex;
  flex-direction: column;
  min-height: 0;
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
  min-height: 120px;
}
.bar {
  flex: 1;
  background: linear-gradient(to top, var(--brand-dark), var(--brand));
  border-radius: 3px 3px 0 0;
  transition: height 0.4s ease;
  min-height: 3px;
}
</style>
