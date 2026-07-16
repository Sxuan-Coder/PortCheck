import { onBeforeUnmount } from 'vue'
import { Events } from '@wailsio/runtime'
import { shallowReactive, markRaw } from 'vue'
import { MONITOR_TICK_EVENT, type MonitorTick, type ProcessInfo } from '../lib/types'

// 全局 monitor store：整生命周期只订阅一次 monitor:tick。
// 大数组用 markRaw 关闭深响应式，避免 Vue 对 1000+ 行建深层 proxy 拖垮刷新。
// 进程表/迷你图分别按需从这些浅字段派生，互不触发全量重算。

interface MonitorState {
  processes: ProcessInfo[]
  perf: MonitorTick['perf']
  portStats: MonitorTick['portStats']
  ready: boolean
}

const state = shallowReactive<MonitorState>({
  processes: markRaw([]),
  perf: { cpuPercent: 0, memUsedGB: 0, memTotalGB: 0, cpuName: '', numCores: 0 },
  portStats: { ports: [], tcpCount: 0, udpCount: 0, listeningCount: 0, processCount: 0, warnings: [] },
  ready: false,
})

let installed = false

function install() {
  if (installed) return
  installed = true
  const off = Events.On(MONITOR_TICK_EVENT, (ev: any) => {
    const tick = (ev && ev.data) ? (ev.data as MonitorTick) : (ev as unknown as MonitorTick)
    if (!tick || !tick.perf) return
    state.perf = tick.perf
    state.portStats = tick.portStats || state.portStats
    state.processes = markRaw(tick.processes || [])
    state.ready = true
  })
  // 复用 off 句柄，便于卸载（应用常驻，实际不卸载）
  void off
}

export function useMonitor() {
  install()
  onBeforeUnmount(() => {
    // 全局 store 在应用生命周期内常驻，单组件卸载不取消订阅
  })
  return { state }
}
