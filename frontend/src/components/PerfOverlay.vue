<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Events } from '@wailsio/runtime'
import { useMonitor } from '../composables/useMonitor'
import { formatPercent } from '../lib/format'
import { SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'

// 性能悬浮窗：极简纯文本视图，透明背景，仅显示三项百分比。
// 颜色 / 字号由设置项驱动：启动时读取一次，运行时通过全局事件 overlay:config 实时更新。

// 颜色预设 id -> 实际 CSS 色值。与后端 settings.go 的白名单保持一致。
const OVERLAY_COLORS: Record<string, string> = {
  white: '#ffffff',
  red: '#ff5a5a',
  green: '#4ade80',
  blue: '#60a5fa',
  yellow: '#facc15',
}

const OVERLAY_CONFIG_EVENT = 'overlay:config'

const { state } = useMonitor()

const colorId = ref<string>('white')
const fontSize = ref<number>(12)

const colorHex = computed(() => OVERLAY_COLORS[colorId.value] || OVERLAY_COLORS.white)
const memPct = computed(() =>
  state.perf.memTotalGB > 0 ? (state.perf.memUsedGB / state.perf.memTotalGB) * 100 : 0,
)
const commitPct = computed(() =>
  state.perf.commitTotalGB > 0 ? (state.perf.commitUsedGB / state.perf.commitTotalGB) * 100 : 0,
)

onMounted(async () => {
  try {
    const s = await SettingsService.GetSettings()
    if (OVERLAY_COLORS[s.overlayColor]) colorId.value = s.overlayColor
    const fs = Number(s.overlayFontSize)
    if (Number.isFinite(fs) && fs >= 10 && fs <= 18) fontSize.value = fs
  } catch {
    /* 保持默认 */
  }
})

function onConfig(ev: any) {
  // Wails 事件载荷可能直接是数据，也可能包在 .data 里，兼容两种形态。
  const raw = ev && ev.data ? ev.data : ev
  const cfg = (raw && typeof raw === 'object' ? raw : {}) as { color?: string; fontSize?: number }
  if (cfg.color && OVERLAY_COLORS[cfg.color]) colorId.value = cfg.color
  if (typeof cfg.fontSize === 'number' && cfg.fontSize >= 10 && cfg.fontSize <= 18) {
    fontSize.value = cfg.fontSize
  }
}

// On 返回一个取消订阅函数；悬浮窗生命周期内常驻，卸载时调用取消。
const cancelOverlayConfig = Events.On(OVERLAY_CONFIG_EVENT, onConfig)

onBeforeUnmount(() => {
  cancelOverlayConfig()
})
</script>

<template>
  <!-- 透明背景；--wails-draggable: drag 允许拖动微调（不持久化）。
       文字颜色与字号由设置项驱动，配 text-shadow 保证在任意桌面背景上可读。 -->
  <div
    class="overlay"
    :style="{ '--oc': colorHex, '--ofs': fontSize + 'px' }"
    style="--wails-draggable: drag"
  >
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
  font-size: var(--ofs);
  color: var(--oc);
  user-select: none;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.85), 0 0 4px rgba(0, 0, 0, 0.5);
}

.line {
  display: flex;
  align-items: baseline;
  gap: 6px;
  line-height: 1.4;
  white-space: nowrap;
}

.k {
  opacity: 0.65;
}

.v {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
</style>
