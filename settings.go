package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// 进程范围枚举。
const (
	processScopeCurrentUser = "currentUser" // 仅当前用户（普通权限，保持现状）
	processScopeSystem      = "system"      // 整个系统（需管理员权限，全量进程）
)

// 性能悬浮窗位置枚举。
const (
	overlayPositionTopLeft  = "topLeft"  // 屏幕左上角
	overlayPositionTopRight = "topRight" // 屏幕右上角（默认）
)

// 性能悬浮窗颜色预设枚举（id），实际色值由前端映射，后端只做白名单校验。
const (
	overlayColorWhite  = "white"
	overlayColorRed    = "red"
	overlayColorGreen  = "green"
	overlayColorBlue   = "blue"
	overlayColorYellow = "yellow"
)

// overlayFontSizeMin/Max 限定悬浮窗字号区间，超界回退默认。
const (
	overlayFontSizeMin = 10
	overlayFontSizeMax = 18
	overlayFontSizeDef = 12
)

// Settings 是应用的用户可配置项。
type Settings struct {
	Theme             string `json:"theme"`
	RefreshIntervalMs int    `json:"refreshIntervalMs"`
	Language          string `json:"language"`
	ProcessScope      string `json:"processScope"`    // currentUser / system
	OverlayEnabled    bool   `json:"overlayEnabled"`  // 性能悬浮窗开关，默认关闭
	OverlayPosition   string `json:"overlayPosition"` // topLeft / topRight，默认 topRight
	OverlayColor      string `json:"overlayColor"`    // white/red/green/blue/yellow，默认 white
	OverlayFontSize   int    `json:"overlayFontSize"` // 10-18，默认 12
}

// SettingsService 提供持久化配置读写与开机自启管理。
type SettingsService struct{}

// DefaultSettings 返回出厂默认设置。
func DefaultSettings() Settings {
	return Settings{
		Theme:             "dark",
		RefreshIntervalMs: 1000,
		Language:          "zh-CN",
		ProcessScope:      processScopeCurrentUser,
		OverlayEnabled:    false,
		OverlayPosition:   overlayPositionTopRight,
		OverlayColor:      overlayColorWhite,
		OverlayFontSize:   overlayFontSizeDef,
	}
}

// normalizeProcessScope 校验进程范围枚举，非法值回退为默认。
func normalizeProcessScope(v string) string {
	if v == processScopeSystem {
		return processScopeSystem
	}
	return processScopeCurrentUser
}

// normalizeOverlayPosition 校验悬浮窗位置枚举，非法值回退为 topRight。
func normalizeOverlayPosition(v string) string {
	if v == overlayPositionTopLeft {
		return overlayPositionTopLeft
	}
	return overlayPositionTopRight
}

// normalizeOverlayColor 校验悬浮窗颜色预设，非法值回退为 white。
func normalizeOverlayColor(v string) string {
	switch v {
	case overlayColorWhite, overlayColorRed, overlayColorGreen, overlayColorBlue, overlayColorYellow:
		return v
	default:
		return overlayColorWhite
	}
}

// normalizeOverlayFontSize 校验悬浮窗字号区间，超界回退默认值。
func normalizeOverlayFontSize(v int) int {
	if v < overlayFontSizeMin || v > overlayFontSizeMax {
		return overlayFontSizeDef
	}
	return v
}

// settingsPath 返回 %APPDATA%/PortCheck/settings.json。
func settingsPath() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "PortCheck")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// GetSettings 读取配置；文件不存在时返回默认值并写入一份。
func (s *SettingsService) GetSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return DefaultSettings(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			def := DefaultSettings()
			_ = s.SaveSettings(def) // 写入默认值，失败忽略
			return def, nil
		}
		return DefaultSettings(), nil
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), nil // 静默回退
	}
	settings.ProcessScope = normalizeProcessScope(settings.ProcessScope)
	settings.OverlayPosition = normalizeOverlayPosition(settings.OverlayPosition)
	settings.OverlayColor = normalizeOverlayColor(settings.OverlayColor)
	settings.OverlayFontSize = normalizeOverlayFontSize(settings.OverlayFontSize)
	return settings, nil
}

// SaveSettings 持久化设置到 JSON 文件。
func (s *SettingsService) SaveSettings(settings Settings) error {
	settings.ProcessScope = normalizeProcessScope(settings.ProcessScope)
	settings.OverlayPosition = normalizeOverlayPosition(settings.OverlayPosition)
	settings.OverlayColor = normalizeOverlayColor(settings.OverlayColor)
	settings.OverlayFontSize = normalizeOverlayFontSize(settings.OverlayFontSize)
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsElevated 返回当前进程是否以管理员权限运行（进程范围=system 的运行时前提）。
func (s *SettingsService) IsElevated() bool {
	return isElevated()
}

// RelaunchElevated 以管理员权限重启应用本体：runas 启动新实例后延时退出旧实例。
// 用于切换到"整个系统"进程范围；新实例以管理员身份启动后即可全量枚举进程。
func (s *SettingsService) RelaunchElevated() error {
	return relaunchElevated()
}
