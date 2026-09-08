package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 应用版本与仓库信息。
// 注意：发布新版本时需与 build/config.yml、frontend/package.json 保持一致。
const (
	appVersion = "2.4.2"
	repoOwner  = "Sxuan-Coder"
	repoName   = "PortCheck"
	githubAPI  = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// UpdateService 提供检查更新能力：请求 GitHub 最新 Release，与本地版本对比。
type UpdateService struct{}

// UpdateInfo 是检查更新的返回结果。
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"` // 本地版本
	LatestVersion  string `json:"latestVersion"`  // 最新 Release 版本（已去掉前缀 v）
	HasUpdate      bool   `json:"hasUpdate"`      // 是否有新版本
	ReleaseURL     string `json:"releaseUrl"`     // Release 页面地址
	DownloadURL    string `json:"downloadUrl"`    // 第一个资产下载地址（可能为空）
	InstallerURL   string `json:"installerUrl"`   // Windows 一键安装包地址（-installer.exe，可能为空）
	Notes          string `json:"notes"`          // Release 说明
}

// CheckUpdate 查询 GitHub 最新 Release 并与本地版本对比。
// 使用未认证请求，受 GitHub 60 次/小时/IP 限制；手动触发场景足够。
func (s *UpdateService) CheckUpdate() (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: appVersion}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, githubAPI, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", repoName+"/"+appVersion)

	resp, err := client.Do(req)
	if err != nil {
		return info, fmt.Errorf("无法连接更新服务器：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("更新服务返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}

	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return info, err
	}

	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info.LatestVersion = latest
	info.ReleaseURL = rel.HTMLURL
	info.Notes = strings.TrimSpace(rel.Body)
	if len(rel.Assets) > 0 {
		info.DownloadURL = rel.Assets[0].BrowserDownloadURL
	}
	// 优先找 Windows 一键安装包（-installer.exe），供应用内下载安装使用。
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, "installer") && strings.HasSuffix(a.Name, ".exe") {
			info.InstallerURL = a.BrowserDownloadURL
			break
		}
	}
	info.HasUpdate = newerThan(latest, appVersion)
	return info, nil
}

// OpenURL 用系统默认浏览器打开指定网址（用于跳转 Release 下载页）。
func (s *UpdateService) OpenURL(url string) error {
	return browser.OpenURL(url)
}

// updateDownloadProgressEvent 是下载进度事件名，前端 useUpdate 订阅以驱动进度条。
const updateDownloadProgressEvent = "update:download"

// downloadInstallerBytes 触发前端进度条的粒度：每收到该字节数广播一次。
const downloadProgressStep = 64 * 1024

// DownloadInstaller 下载安装包到系统临时目录并自动启动安装程序。
// 通过 update:download 事件广播下载进度 {received,total,percent}；
// 返回本地文件路径。仅 Windows 会自动打开安装包，其他平台仅下载（占位行为）。
func (s *UpdateService) DownloadInstaller(url string) (string, error) {
	if url == "" {
		return "", errors.New("未找到安装包下载地址")
	}

	// 不设整体超时：安装包可能十几 MB 且用户网络较慢；用连接阶段超时即可。
	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", repoName+"/"+appVersion)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败：服务器返回状态码 %d", resp.StatusCode)
	}

	// 文件名取 URL 末段（形如 PortCheck-2.4.0-windows-amd64-installer.exe），
	// 取不到时回退固定名。
	name := path.Base(strings.Split(url, "?")[0])
	if name == "" || name == "." || name == "/" {
		name = "PortCheck-installer.exe"
	}
	dst := filepath.Join(os.TempDir(), name)
	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败：%w", err)
	}
	defer f.Close()

	app := application.Get()
	total := resp.ContentLength // 未知时为 -1
	var received int64
	buf := make([]byte, 32*1024)
	lastEmit := -int64(downloadProgressStep)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("写入文件失败：%w", werr)
			}
			received += int64(n)
			// 按固定字节粒度节流广播，避免每 32KB 一次事件刷爆前端。
			if app != nil && received-lastEmit >= downloadProgressStep {
				lastEmit = received
				percent := 0
				if total > 0 {
					percent = int(received * 100 / total)
				}
				app.Event.EmitEvent(&application.CustomEvent{
					Name: updateDownloadProgressEvent,
					Data: map[string]any{
						"received": received,
						"total":    total,
						"percent":  percent,
					},
				})
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", fmt.Errorf("下载中断：%w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("写入文件失败：%w", err)
	}
	if app != nil {
		percent := 100
		if total <= 0 {
			percent = 0
		}
		app.Event.EmitEvent(&application.CustomEvent{
			Name: updateDownloadProgressEvent,
			Data: map[string]any{"received": received, "total": total, "percent": percent},
		})
	}

	if err := openInstaller(dst); err != nil {
		return dst, fmt.Errorf("已下载到 %s，但启动安装程序失败：%w", dst, err)
	}
	return dst, nil
}

// CurrentVersion 返回当前应用版本号，供前端展示或诊断使用。
func (s *UpdateService) CurrentVersion() string {
	return appVersion
}

// newerThan 判断 a 是否严格新于 b（语义化版本比较，忽略前缀 v）。
func newerThan(a, b string) bool {
	pa := parseSemver(a)
	pb := parseSemver(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		xa, xb := 0, 0
		if i < len(pa) {
			xa = pa[i]
		}
		if i < len(pb) {
			xb = pb[i]
		}
		if xa != xb {
			return xa > xb
		}
	}
	return false
}

// parseSemver 把 "v2.0.1" / "2.0.1" 解析为 [2,0,1]，无法解析的段按 0 处理。
func parseSemver(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s = strings.SplitN(s, "-", 2)[0] // 去掉预发布后缀
	parts := strings.Split(s, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		v, _ := strconv.Atoi(strings.TrimSpace(p))
		out[i] = v
	}
	return out
}
