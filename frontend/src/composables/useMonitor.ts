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

// 进程图标按路径缓存：后端每路径通常只在某一 tick 推送一次 iconDataUrl。
// 空路径统一键，对应应用 logo 回退。
const processIconByPath = new Map<string, string>()
const FALLBACK_ICON = '/appicon.png'

function mergeProcessIcons(list: ProcessInfo[]): ProcessInfo[] {
  return list.map((p) => {
    const path = (p.path || '').trim().toLowerCase()
    const key = path || '__fallback__'
    if (p.iconDataUrl) {
      processIconByPath.set(key, p.iconDataUrl)
      return p
    }
    const cached = processIconByPath.get(key)
    if (cached) return { ...p, iconDataUrl: cached }
    // 尚未推送时先用应用 logo，避免灰块闪烁。
    return { ...p, iconDataUrl: FALLBACK_ICON }
  })
}

let installed = false

function install() {
  if (installed) return
  installed = true
  const off = Events.On(MONITOR_TICK_EVENT, (ev: any) => {
    const tick = (ev && ev.data) ? (ev.data as MonitorTick) : (ev as unknown as MonitorTick)
    if (!tick || !tick.perf) return
    state.perf = tick.perf
    state.portStats = tick.portStats || state.portStats
    state.processes = markRaw(mergeProcessIcons(tick.processes || []))
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
