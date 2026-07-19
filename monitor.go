package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// monitorTickEvent 是后端每秒批量推送事件的名字，前端通过 Events.On 订阅一次。
const monitorTickEvent = "monitor:tick"

// MonitorService 负责每秒采样进程/性能/端口统计，合并为单个事件推送给前端，
// 是高频刷新场景下"批量推送取代频繁 RPC"的核心。
type MonitorService struct {
	mu sync.Mutex

	cancel    context.CancelFunc
	intervalCh chan time.Duration // 运行时调整推送间隔

	// 进程 CPU 差值状态：上一秒每个 PID 的累计 CPU 时间（100ns 刻度）。
	prevProcTimes map[uint32]int64
	// 整机 CPU 差值状态：上一秒的 idle/kernel/user 累计时间。
	prevIdle   int64
	prevKernel int64
	prevUser   int64
	prevWhen   time.Time

	// 启动时读取一次的静态信息。
	numCores int
	cpuName  string
}

// ServiceStartup 在应用启动时读取 CPU 信息并启动后台采样循环。
func (s *MonitorService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.intervalCh = make(chan time.Duration, 1)
	s.numCores, s.cpuName = readCPUInfo()
	s.prevProcTimes = map[uint32]int64{}
	s.prevWhen = time.Now()

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.tickLoop(ctx)
	return nil
}

// ServiceShutdown 在应用退出时停止采样循环。
func (s *MonitorService) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *MonitorService) tickLoop(ctx context.Context) {
	interval := time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.emitTick(now)
		case newInterval := <-s.intervalCh:
			if newInterval < 200*time.Millisecond {
				newInterval = 200 * time.Millisecond
			}
			if newInterval > 10*time.Second {
				newInterval = 10 * time.Second
			}
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
		}
	}
}

// emitTick 执行一次采样并合并推送；持有锁以保证差值状态一致性。
func (s *MonitorService) emitTick(now time.Time) {
	s.mu.Lock()
	elapsed := now.Sub(s.prevWhen).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	processes, perf := sampleProcessesAndPerf(s, elapsed)
	s.prevWhen = now
	s.mu.Unlock()

	// 端口统计走轻量计数路径，不解析进程名，避免拖慢采样。
	portStats := portCountsLight()
	portStats.Ports = nil // 推送不含明细，Ports Tab 按需调用 ListPorts

	payload := MonitorTick{
		Timestamp: now.UnixMilli(),
		Processes: processes,
		Perf:      perf,
		PortStats: portStats,
	}

	if app := application.Get(); app != nil {
		app.Event.EmitEvent(&application.CustomEvent{Name: monitorTickEvent, Data: payload})
	}
}

// SetInterval 动态调整采样推送间隔（毫秒），范围 200-10000；超范围自动钳位。
func (s *MonitorService) SetInterval(ms int) error {
	select {
	case s.intervalCh <- time.Duration(ms) * time.Millisecond:
	default:
	}
	return nil
}
