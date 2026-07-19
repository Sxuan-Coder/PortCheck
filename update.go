package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/browser"
)

// 应用版本与仓库信息。
// 注意：发布新版本时需与 build/config.yml、frontend/package.json 保持一致。
const (
	appVersion = "2.2.1"
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
	info.HasUpdate = newerThan(latest, appVersion)
	return info, nil
}

// OpenURL 用系统默认浏览器打开指定网址（用于跳转 Release 下载页）。
func (s *UpdateService) OpenURL(url string) error {
	return browser.OpenURL(url)
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
