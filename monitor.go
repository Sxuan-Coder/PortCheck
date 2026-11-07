package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// monitorTickEvent 是后端每秒批量推送事件的名字，前端通过 Events.On 订阅一次。
const monitorTickEvent = "monitor:tick"

// backgroundSampleInterval 是主窗口最小化/隐藏到托盘且无悬浮窗时的采样间隔；
// 显著降低常驻托盘时的 CPU 唤醒与采样开销，窗口恢复可见后自动回到前台间隔。
const backgroundSampleInterval = 5 * time.Second

// monitorSvc 是 MonitorService 的包级单例：设置保存时需要通知它更新前台采样间隔。
var monitorSvc = &MonitorService{}

// MonitorService 负责采样进程/性能/端口统计，合并为单个事件推送给前端，
// 是高频刷新场景下"批量推送取代频繁 RPC"的核心。
// 采样频率随窗口状态自适应：前台用用户设置间隔，最小化/托盘隐藏且无悬浮窗时降频。
type MonitorService struct {
	mu sync.Mutex

	cancel    context.CancelFunc
	intervalCh chan time.Duration // 运行时调整推送间隔

	// foregroundIntervalMs 是前台状态下的采样间隔（毫秒），来自用户设置；
	// atomic 供 tickLoop 与设置保存路径无锁共享。
	foregroundIntervalMs atomic.Int64

	// 进程 CPU 差值状态：上一秒每个 PID 的累计 CPU 时间（100ns 刻度）。
	prevProcTimes map[uint32]int64
	// 进程 I/O 差值状态：上一秒每个 PID 的累计读+写传输字节。
	prevIoTimes map[uint32]uint64
	// 整机 CPU 差值状态：上一秒的 idle/kernel/user 累计时间。
	prevIdle   int64
	prevKernel int64
	prevUser   int64
	prevWhen   time.Time

	// 启动时读取一次的静态信息。
	numCores int
	cpuName  string
}

// ServiceStartup 在应用启动时读取 CPU 信息与采样间隔设置，启动后台采样循环。
func (s *MonitorService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.intervalCh = make(chan time.Duration, 1)
	s.numCores, s.cpuName = readCPUInfo()
	s.prevProcTimes = map[uint32]int64{}
	s.prevIoTimes = map[uint32]uint64{}
	s.prevWhen = time.Now()
	if settings, err := (&SettingsService{}).GetSettings(); err == nil {
		s.foregroundIntervalMs.Store(int64(settings.RefreshIntervalMs))
	} else {
		s.foregroundIntervalMs.Store(1000)
	}

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

// SetForegroundIntervalMs 由设置保存路径调用：更新前台采样间隔（毫秒），
// 超范围钳位到 200-10000。不直接动 ticker，由 tickLoop 的 cadence 对比生效。
func (s *MonitorService) SetForegroundIntervalMs(ms int) {
	if ms < 200 {
		ms = 200
	}
	if ms > 10000 {
		ms = 10000
	}
	s.foregroundIntervalMs.Store(int64(ms))
}

// foregroundInterval 返回钳位后的前台采样间隔。
func (s *MonitorService) foregroundInterval() time.Duration {
	ms := s.foregroundIntervalMs.Load()
	if ms < 200 {
		ms = 200
	}
	if ms > 10000 {
		ms = 10000
	}
	return time.Duration(ms) * time.Millisecond
}

// cadence 依据窗口状态返回当前期望采样间隔。
// 窗口状态 API 是到主线程的同步调用，只能在 emitTick 释放锁之后调用（本函数即如此），
// 且每个 tick 至多调用一次；悬浮窗开启时视为始终有观察者，不查询窗口状态。
func (s *MonitorService) cadence() time.Duration {
	if overlaySvc.AnyOverlayActive() {
		return s.foregroundInterval()
	}
	if mainWindow != nil {
		// 短路顺序：先 IsVisible（隐藏到托盘最常见），再 IsMinimised。
		if !mainWindow.IsVisible() || mainWindow.IsMinimised() {
			return backgroundSampleInterval
		}
	}
	return s.foregroundInterval()
}

func (s *MonitorService) tickLoop(ctx context.Context) {
	interval := s.foregroundInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.emitTick(now)
			// cadence 决策放在锁外的 tickLoop 上：幂等对比，值变化才重建 ticker，
			// 避免逐 tick 重建造成的相位漂移。
			if desired := s.cadence(); desired != interval {
				interval = desired
				ticker.Stop()
				ticker = time.NewTicker(interval)
			}
		case newInterval := <-s.intervalCh:
			if newInterval < 200*time.Millisecond {
				newInterval = 200 * time.Millisecond
			}
			if newInterval > 10*time.Second {
				newInterval = 10 * time.Second
			}
			interval = newInterval
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

// SetInterval 动态调整前台采样间隔（毫秒），范围 200-10000；超范围自动钳位。
// 设置面板保存设置后调用；前台间隔变更也会经 SaveSettings 通知到本服务，两路幂等。
func (s *MonitorService) SetInterval(ms int) error {
	s.SetForegroundIntervalMs(ms)
	// 立即生效：若当前正以前台间隔运行，直接重建 ticker；后台降频状态下
	// 只更新缓存，恢复前台时由 cadence 采用新值。
	select {
	case s.intervalCh <- s.foregroundInterval():
	default:
	}
	return nil
}
