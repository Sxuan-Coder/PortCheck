package main

// ProcessInfo 描述单个运行进程的实时快照，由 MonitorService 采样后批量推送给前端。
type ProcessInfo struct {
	PID         uint32  `json:"pid"`
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	CPU         float64 `json:"cpu"`         // 单核基准百分比（原始值，可 >100%）；前端 ÷ 核心数 得整机基准
	MemBytes    uint64  `json:"memBytes"`    // 工作集 (WorkingSetSize)，单位字节
	IconDataURL string  `json:"iconDataUrl"` // 应用图标 data URL；进程流中每路径通常只推送一次，前端需缓存
}

// PerfSnapshot 描述整机 CPU 与内存的实时指标。
type PerfSnapshot struct {
	CPUPercent float64 `json:"cpuPercent"` // 0-100，整机占用
	MemUsedGB  float64 `json:"memUsedGB"`  // 物理内存已用 (GB)
	MemTotalGB float64 `json:"memTotalGB"` // 物理内存总量 (GB)
	CPUName    string  `json:"cpuName"`    // CPU 型号名
	NumCores   int     `json:"numCores"`   // 逻辑核心数
}

// ServiceEntry 描述一条 Windows 服务，v2 支持停止/启动。
type ServiceEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`     // 运行中 / 已停止 / 已暂停 / 未知
	StartType   string `json:"startType"` // 自动 / 手动 / 禁用 / 未知
}

// StartupEntry 描述一条开机启动项，v2 支持删除/禁用/启用。
type StartupEntry struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Location    string `json:"location"`    // HKCU / HKLM / StartupFolder
	Disabled    bool   `json:"disabled"`    // 来自 StartupApproved
	IconDataURL string `json:"iconDataUrl"` // 应用图标 data:image/png;base64,... ；失败为空
}

// ServiceOpResult 是服务停止/启动操作的结果。
type ServiceOpResult struct {
	Name    string `json:"name"`
	Action  string `json:"action"` // "stop" / "start"
	Message string `json:"message"`
}

// StartupOpResult 是启动项删除/禁用/启用操作的结果。
type StartupOpResult struct {
	Name    string `json:"name"`
	Action  string `json:"action"` // "delete" / "disable" / "enable"
	Message string `json:"message"`
}

// MonitorTick 是后端单 ticker 每秒组装并一次性推送给前端的合并载荷，
// 避免前端为进程/性能/端口分别发起高频 RPC。
type MonitorTick struct {
	Timestamp int64          `json:"timestamp"`
	Processes []ProcessInfo  `json:"processes"`
	Perf      PerfSnapshot   `json:"perf"`
	PortStats PortListResult `json:"portStats"`
}

// ServicesService 提供 Windows 服务枚举与停止/启动（v2）。
type ServicesService struct{}

// StartupService 提供开机启动项枚举与删除/禁用/启用（v2）。
type StartupService struct{}
