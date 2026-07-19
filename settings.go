package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings 是应用的用户可配置项。
type Settings struct {
	Theme             string `json:"theme"`
	RefreshIntervalMs int    `json:"refreshIntervalMs"`
	Language          string `json:"language"`
}

// SettingsService 提供持久化配置读写与开机自启管理。
type SettingsService struct{}

// DefaultSettings 返回出厂默认设置。
func DefaultSettings() Settings {
	return Settings{
		Theme:             "dark",
		RefreshIntervalMs: 1000,
		Language:          "zh-CN",
	}
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
	return settings, nil
}

// SaveSettings 持久化设置到 JSON 文件。
func (s *SettingsService) SaveSettings(settings Settings) error {
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
