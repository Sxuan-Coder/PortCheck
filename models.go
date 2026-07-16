package main

// ProcessInfo 描述单个运行进程的实时快照，由 MonitorService 采样后批量推送给前端。
type ProcessInfo struct {
	PID      uint32  `json:"pid"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	CPU      float64 `json:"cpu"`      // 单核基准百分比（原始值，可 >100%）；前端 ÷ 核心数 得整机基准
	MemBytes uint64  `json:"memBytes"` // 工作集 (WorkingSetSize)，单位字节
}

// PerfSnapshot 描述整机 CPU 与内存的实时指标。
type PerfSnapshot struct {
	CPUPercent float64 `json:"cpuPercent"` // 0-100，整机占用
	MemUsedGB  float64 `json:"memUsedGB"`  // 物理内存已用 (GB)
	MemTotalGB float64 `json:"memTotalGB"` // 物理内存总量 (GB)
	CPUName    string  `json:"cpuName"`    // CPU 型号名
	NumCores   int     `json:"numCores"`   // 逻辑核心数
}

// ServiceEntry 描述一条 Windows 服务，v1 只读。
type ServiceEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`    // 运行中 / 已停止 / 已暂停 / 未知
	StartType   string `json:"startType"` // 自动 / 手动 / 禁用 / 未知
}

// StartupEntry 描述一条开机启动项，v1 只读。
type StartupEntry struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"` // HKCU / HKLM / StartupFolder
}

// MonitorTick 是后端单 ticker 每秒组装并一次性推送给前端的合并载荷，
// 避免前端为进程/性能/端口分别发起高频 RPC。
type MonitorTick struct {
	Timestamp int64          `json:"timestamp"`
	Processes []ProcessInfo  `json:"processes"`
	Perf      PerfSnapshot   `json:"perf"`
	PortStats PortListResult `json:"portStats"`
}

// ServicesService 提供只读的 Windows 服务枚举（v1 不含启停写操作）。
type ServicesService struct{}

// StartupService 提供只读的开机启动项枚举（v1 不含启用/禁用写操作）。
type StartupService struct{}
