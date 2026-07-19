// 前端本地定义 monitor 事件载荷类型。
// 后端 MonitorService 没有导出的 RPC 方法（仅生命周期），wails 绑定生成器不会为其导出类型，
// 因此这里手动镜像 models.go 的结构，供 Events.On("monitor:tick") 消费。

export interface ProcessInfo {
  pid: number
  name: string
  path: string
  cpu: number // 单核基准百分比
  memBytes: number // 工作集字节
  iconDataUrl?: string // 可选：后端每路径通常只推一次，前端缓存
}

export interface PerfSnapshot {
  cpuPercent: number
  memUsedGB: number
  memTotalGB: number
  cpuName: string
  numCores: number
}

export interface PortStats {
  ports: unknown[]
  tcpCount: number
  udpCount: number
  listeningCount: number
  processCount: number
  warnings: string[]
}

export interface MonitorTick {
  timestamp: number
  processes: ProcessInfo[]
  perf: PerfSnapshot
  portStats: PortStats
}

/** monitor:tick 事件名，与后端 monitor.go 的 monitorTickEvent 保持一致。 */
export const MONITOR_TICK_EVENT = 'monitor:tick'
