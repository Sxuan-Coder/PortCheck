package main

// ProcessInfo 描述单个运行进程的实时快照，由 MonitorService 采样后批量推送给前端。
type ProcessInfo struct {
	PID         uint32  `json:"pid"`
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	CPU         float64 `json:"cpu"`         // 单核基准百分比（原始值，可 >100%）；前端 ÷ 核心数 得整机基准
	MemBytes    uint64  `json:"memBytes"`    // 工作集 (WorkingSetSize)，单位字节
	CommitBytes uint64  `json:"commitBytes"` // 提交内存 (PagefileUsage)，对应任务管理器"提交大小"，单位字节
	IconDataURL string  `json:"iconDataUrl"` // 应用图标 data URL；进程流中每路径通常只推送一次，前端需缓存
}

// PerfSnapshot 描述整机 CPU 与内存的实时指标。
type PerfSnapshot struct {
	CPUPercent    float64 `json:"cpuPercent"`    // 0-100，整机占用
	MemUsedGB     float64 `json:"memUsedGB"`     // 物理内存已用 (GB)
	MemTotalGB    float64 `json:"memTotalGB"`    // 物理内存总量 (GB)
	CommitUsedGB  float64 `json:"commitUsedGB"`  // 已提交内存 (GB)，可超过物理内存（含页面文件）
	CommitTotalGB float64 `json:"commitTotalGB"` // 提交限制 (GB) = 物理内存 + 页面文件上限
	CPUName       string  `json:"cpuName"`       // CPU 型号名
	NumCores      int     `json:"numCores"`      // 逻辑核心数
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

// CodingPlanAccount 描述一条 Coding Plan 用量监控账号的配置。
type CodingPlanAccount struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`      // 显示名，留空时由前端回退为供应商名
	Provider    string  `json:"provider"`  // zhipu / kimi / minimax / zenmux / deepseek
	BaseURL     string  `json:"baseUrl"`   // zenmux 必填（完整查询端点）；zhipu/minimax 用于区分国内/国际站
	APIKey      string  `json:"apiKey"`
	AlertAmount float64 `json:"alertAmount"` // 余额预警阈值（元），0 = 不提醒；仅余额型供应商（deepseek）使用
	CreatedAt   int64   `json:"createdAt"`
}

// CodingPlanQuota 描述一个用量窗口（5 小时 / 周）的百分比与重置时间。
type CodingPlanQuota struct {
	UsedPercent float64 `json:"usedPercent"` // 已用百分比（0-100 基准；超界不裁剪，展示策略交给前端）
	ResetsAt    string  `json:"resetsAt"`    // 重置时间 ISO 8601，空表示未知
	UsedLabel   string  `json:"usedLabel"`   // 补充用量文本（如 ZenMux 的 "$3.20 / $7.50"），无则空
}

// CodingPlanBalance 描述余额型供应商（DeepSeek）的账户余额，金额单位为账户币种。
type CodingPlanBalance struct {
	Currency  string  `json:"currency"`  // "CNY" / "USD"
	Total     float64 `json:"total"`     // 总余额（赠金 + 充值）
	Granted   float64 `json:"granted"`   // 未过期赠金
	ToppedUp  float64 `json:"toppedUp"`  // 充值余额
	Available bool    `json:"available"` // 余额是否足以进行 API 调用
}

// CodingPlanUsage 是单账号一次用量查询的结果；失败不返回 Go 错误，
// 而是 Status=expired/error + Error 带原因，便于前端按卡片展示失败态。
// 配额型供应商填 FiveHour/Weekly，余额型供应商填 Balance（互斥，nil 区分形态）。
type CodingPlanUsage struct {
	AccountID string             `json:"accountId"`
	PlanName  string             `json:"planName"` // 套餐等级（Lite/Pro/Max 等），无则空
	Status    string             `json:"status"`   // ok / expired / error
	FiveHour  *CodingPlanQuota   `json:"fiveHour"` // 5 小时窗口；nil 表示无该桶
	Weekly    *CodingPlanQuota   `json:"weekly"`   // 周（7 天）窗口；nil 表示套餐无周限额
	Balance   *CodingPlanBalance `json:"balance"`  // 余额（DeepSeek）；nil 表示配额型
	Error     string             `json:"error"`
	QueriedAt int64              `json:"queriedAt"`
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

// CodingPlanService 提供 Coding Plan 账号配置管理与用量查询。
type CodingPlanService struct{}
