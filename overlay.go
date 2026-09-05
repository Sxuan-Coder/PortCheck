package main

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 悬浮窗尺寸与边距（DIP，与屏幕坐标一致）。
// 极简文本视图：三行短文本，窗口尽量小以减少遮挡。
const (
	overlayWidth  = 130
	overlayHeight = 72
	overlayMargin = 8

	// 用量悬浮窗（Coding Plan 圆环图表）：高度随账号行数动态变化（SetUsageRows）。
	usageOverlayWidth   = 200
	usageOverlayRowH    = 56
	usageOverlayPad     = 8
	usageOverlayMinRows = 1
	usageOverlayMaxRows = 4 // 前端最多展示 3 个账号 + 1 行"更多"提示

	// 悬浮窗加载的 URL：与主窗口共享同一 SPA bundle，靠 hash 路由分发到极简浮窗视图。
	overlayURL      = "/#/overlay"
	usageOverlayURL = "/#/usage-overlay"
)

// OverlayService 负责管理「性能悬浮窗」与「用量悬浮窗」这两个独立、置顶、
// 无边框的小窗口的生命周期。它们与主窗口完全解耦：主窗口最小化到托盘不影响
// 悬浮窗；悬浮窗显隐只由设置项驱动。
type OverlayService struct {
	mu sync.Mutex

	win      application.Window
	enabled  bool
	position string

	usageWin      application.Window
	usageEnabled  bool
	usagePosition string
	usageRows     int // 用量悬浮窗当前行数，决定窗口高度
}

// pendingRestore / pendingUsageRestore 标记启动时读到的「需要恢复悬浮窗」意图。
// 真正的创建推迟到主窗口 WebView 导航完成（见 main.go 的 RestoreIfEnabled 调用），
// 避免启动期主窗口与悬浮窗两个 WebView2 同时冷启动、争抢同一 WebView2 环境/资源
// 导致主窗口首帧白屏（表现为开启悬浮窗后下次启动主窗口无页面）。
var (
	pendingRestore      bool
	pendingUsageRestore bool
)

// ServiceStartup 在应用启动时读取持久化设置，仅记录恢复意图，不立即创建窗口。
// 窗口创建由 main.go 在主窗口 NavigationCompleted 后调用 RestoreIfEnabled 触发。
func (s *OverlayService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	ss := &SettingsService{}
	settings, err := ss.GetSettings()
	if err != nil {
		return nil // 读不到设置时静默跳过，不阻塞启动
	}
	if settings.OverlayEnabled {
		s.position = normalizeOverlayPosition(settings.OverlayPosition)
		pendingRestore = true
	}
	if settings.UsageOverlayEnabled {
		s.usagePosition = normalizeOverlayPosition(settings.UsageOverlayPosition)
		pendingUsageRestore = true
	}
	return nil
}

// RestoreIfEnabled 在主窗口前端加载完成后恢复悬浮窗。
// 仅在启动时记录了恢复意图时生效，调用一次后清空标记，防止重复触发。
// 先恢复性能悬浮窗再恢复用量悬浮窗，保证同角错开计算拿到前者最终状态。
func (s *OverlayService) RestoreIfEnabled() {
	if pendingRestore {
		pendingRestore = false
		_ = s.Apply(true, s.position)
	}
	if pendingUsageRestore {
		pendingUsageRestore = false
		_ = s.ApplyUsage(true, s.usagePosition)
	}
}

// Apply 是前端可调用的 RPC：按 (enabled, position) 创建 / 移动 / 关闭悬浮窗。
//   - enabled=false：关闭并丢弃窗口引用。
//   - enabled=true 且窗口不存在：按 position 计算坐标后创建置顶无边框窗口。
//   - enabled=true 且窗口已存在：仅按 position 重新定位。
func (s *OverlayService) Apply(enabled bool, position string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	position = normalizeOverlayPosition(position)
	s.enabled = enabled
	s.position = position

	app := application.Get()
	if app == nil {
		return nil
	}

	if !enabled {
		if s.win != nil {
			s.win.Close()
			s.win = nil
		}
		// 性能悬浮窗关闭后，用量悬浮窗不再需要下移错开，重新贴角。
		if s.usageWin != nil {
			ux, uy := s.computeUsageXY(s.usagePosition)
			s.usageWin.SetPosition(ux, uy)
		}
		return nil
	}

	x, y := computeOverlayXY(position)

	if s.win == nil {
		s.win = app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:         "PortCheck Overlay",
			Width:         overlayWidth,
			Height:        overlayHeight,
			Frameless:     true,
			AlwaysOnTop:   true,
			DisableResize: true,
			Hidden:        true, // 先隐藏创建，避免首帧白闪，随后显式 Show
			Windows: application.WindowsWindow{
				HiddenOnTaskbar: true, // 悬浮窗为辅助小窗，不在任务栏占位
			},
			BackgroundType:   application.BackgroundTypeTransparent,
			BackgroundColour: application.NewRGBA(0, 0, 0, 0),
			URL:              overlayURL,
		})
		s.win.Show()
		// 某些 webview 实现会忽略创建时的 X/Y 而默认居中，显式定位一次确保贴角。
		s.win.SetPosition(x, y)
		return nil
	}

	// 已存在：仅更新位置。
	s.win.SetPosition(x, y)
	// 性能悬浮窗移动后，同角的用量悬浮窗需要跟着重新错开。
	if s.usageWin != nil {
		ux, uy := s.computeUsageXY(s.usagePosition)
		s.usageWin.SetPosition(ux, uy)
	}
	return nil
}

// ApplyUsage 是前端可调用的 RPC：按 (enabled, position) 创建 / 移动 / 关闭用量悬浮窗。
// 生命周期语义与 Apply 一致；与性能悬浮窗同角时自动下移一行错开，避免互相遮挡。
func (s *OverlayService) ApplyUsage(enabled bool, position string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	position = normalizeOverlayPosition(position)
	s.usageEnabled = enabled
	s.usagePosition = position

	app := application.Get()
	if app == nil {
		return nil
	}

	if !enabled {
		if s.usageWin != nil {
			s.usageWin.Close()
			s.usageWin = nil
		}
		return nil
	}

	x, y := s.computeUsageXY(position)

	if s.usageWin == nil {
		s.usageWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:         "PortCheck Usage Overlay",
			Width:         usageOverlayWidth,
			Height:        usageOverlayHeightFor(s.usageRows),
			Frameless:     true,
			AlwaysOnTop:   true,
			DisableResize: true,
			Hidden:        true, // 先隐藏创建，避免首帧白闪，随后显式 Show
			Windows: application.WindowsWindow{
				HiddenOnTaskbar: true, // 悬浮窗为辅助小窗，不在任务栏占位
			},
			BackgroundType:   application.BackgroundTypeTransparent,
			BackgroundColour: application.NewRGBA(0, 0, 0, 0),
			URL:              usageOverlayURL,
		})
		s.usageWin.Show()
		// 某些 webview 实现会忽略创建时的 X/Y 而默认居中，显式定位一次确保贴角。
		s.usageWin.SetPosition(x, y)
		return nil
	}

	// 已存在：仅更新位置。
	s.usageWin.SetPosition(x, y)
	return nil
}

// SetUsageRows 由用量悬浮窗前端上报需要展示的账号行数，后端据此调整窗口高度。
// 账号增删时行数变化，窗口高度跟随；行数在 [minRows, maxRows] 内收敛。
func (s *OverlayService) SetUsageRows(rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rows < usageOverlayMinRows {
		rows = usageOverlayMinRows
	}
	if rows > usageOverlayMaxRows {
		rows = usageOverlayMaxRows
	}
	s.usageRows = rows
	if s.usageWin != nil {
		s.usageWin.SetSize(usageOverlayWidth, usageOverlayHeightFor(rows))
	}
	return nil
}

// usageOverlayHeightFor 按行数计算用量悬浮窗高度（DIP）。
func usageOverlayHeightFor(rows int) int {
	if rows < usageOverlayMinRows {
		rows = usageOverlayMinRows
	}
	return rows*usageOverlayRowH + usageOverlayPad*2
}

// computeUsageXY 计算用量悬浮窗坐标：贴主屏 WorkArea 角落；当性能悬浮窗开启且
// 同处一角时，在其下方错开一行，避免两个置顶小窗互相遮挡。
func (s *OverlayService) computeUsageXY(position string) (int, int) {
	screen := application.GetScreenByIndex(0)
	if screen == nil {
		return overlayMargin, overlayMargin
	}
	wa := screen.WorkArea
	x, y := wa.X+overlayMargin, wa.Y+overlayMargin
	if position != overlayPositionTopLeft {
		x = wa.X + wa.Width - usageOverlayWidth - overlayMargin
	}
	if s.enabled && s.position == position {
		y += overlayHeight + overlayMargin
	}
	return x, y
}

// computeOverlayXY 依据主屏 WorkArea（已排除任务栏）与位置枚举计算窗口左上角坐标。
// 找不到主屏时回退到 (margin, margin)，保证仍可见。
func computeOverlayXY(position string) (int, int) {
	screen := application.GetScreenByIndex(0)
	if screen == nil {
		return overlayMargin, overlayMargin
	}
	wa := screen.WorkArea
	switch position {
	case overlayPositionTopLeft:
		return wa.X + overlayMargin, wa.Y + overlayMargin
	default:
		return wa.X + wa.Width - overlayWidth - overlayMargin, wa.Y + overlayMargin
	}
}

// ServiceShutdown 关闭悬浮窗，避免退出时残留句柄。
func (s *OverlayService) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.win != nil {
		s.win.Close()
		s.win = nil
	}
	if s.usageWin != nil {
		s.usageWin.Close()
		s.usageWin = nil
	}
	return nil
}
