<script setup lang="ts">
import { computed } from 'vue'
import { useMonitor } from '../composables/useMonitor'
import { formatPercent } from '../lib/format'

// 性能悬浮窗：极简纯文本视图，透明背景，仅显示三项百分比。
// 与 PerformanceTab 共享同一 monitor:tick 数据源。
const { state } = useMonitor()

const memPct = computed(() =>
  state.perf.memTotalGB > 0 ? (state.perf.memUsedGB / state.perf.memTotalGB) * 100 : 0,
)
const commitPct = computed(() =>
  state.perf.commitTotalGB > 0 ? (state.perf.commitUsedGB / state.perf.commitTotalGB) * 100 : 0,
)
</script>

<template>
  <!-- 透明背景；--wails-draggable: drag 允许拖动微调（不持久化）。 -->
  <div class="overlay" style="--wails-draggable: drag">
    <div class="line"><span class="k">CPU</span><span class="v">{{ formatPercent(state.perf.cpuPercent, 0) }}</span></div>
    <div class="line"><span class="k">内存</span><span class="v">{{ formatPercent(memPct, 0) }}</span></div>
    <div class="line"><span class="k">提交</span><span class="v">{{ formatPercent(commitPct, 0) }}</span></div>
  </div>
</template>

<style scoped>
.overlay {
  background: transparent;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  height: 100vh;
  width: 100vw;
  box-sizing: border-box;
  justify-content: center;
  font-family: "Segoe UI", "Inter", -apple-system, "Microsoft YaHei", sans-serif;
  user-select: none;
  /* 轻微文字阴影提升在任意桌面背景上的可读性 */
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
}

.line {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
  line-height: 1.4;
  color: rgba(255, 255, 255, 0.92);
  white-space: nowrap;
}

.k {
  opacity: 0.7;
}

.v {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
</style>
