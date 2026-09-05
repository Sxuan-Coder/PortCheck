package main

// Coding Plan 用量查询服务：对接 Kimi For Coding、智谱 GLM（国内/国际）、
// MiniMax（国内/国际）、ZenMux 四家供应商的用量接口，统一抽象为
// 「5 小时窗口 + 周窗口」两个配额桶（已用百分比 + 重置时间）。
// 接口语义与解析逻辑对齐 cc-switch 的 Rust 实现，并保留其踩坑修复：
//   - 智谱 TOKENS_LIMIT 按 unit 字段分桶（unit=3 五小时 / unit=6 周），不能按
//     nextResetTime 排序分桶：周期末尾周桶会比 5h 桶更早重置（cc-switch #3036）。
//   - MiniMax 周桶仅 current_weekly_status==1 时展示（==3 表示无周限额，剩余恒 100%）。
//   - 智谱监控接口的鉴权头不加 Bearer 前缀；ZenMux 的 usage_percentage 是 0-1 小数。

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 供应商标识（存储于 CodingPlanAccount.Provider）。
const (
	codingPlanZhipu   = "zhipu"
	codingPlanKimi    = "kimi"
	codingPlanMiniMax = "minimax"
	codingPlanZenMux  = "zenmux"
)

// 查询结果状态。
const (
	codingPlanStatusOK      = "ok"
	codingPlanStatusExpired = "expired" // HTTP 401/403：密钥无效或已过期
	codingPlanStatusError   = "error"   // 网络 / 业务 / 解析错误
)

const codingPlanHTTPTimeout = 15 * time.Second

// errCodingPlanAuth 表示鉴权失败（HTTP 401/403），前端按"密钥失效"黄色态展示。
var errCodingPlanAuth = errors.New("鉴权失败，API Key 无效或已过期")

var codingPlanHTTPClient = &http.Client{Timeout: codingPlanHTTPTimeout}

// codingPlanMu 保护 codingplans.json 的读改写（前端可能并发触发保存/删除）。
var codingPlanMu sync.Mutex

// ── 配置持久化：%APPDATA%/PortCheck/codingplans.json ─────────────

type codingPlanStore struct {
	Accounts []CodingPlanAccount `json:"accounts"`
}

func codingPlansPath() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "PortCheck")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "codingplans.json"), nil
}

func loadCodingPlans() ([]CodingPlanAccount, error) {
	path, err := codingPlansPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var store codingPlanStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("解析 codingplans.json 失败：%w", err)
	}
	return store.Accounts, nil
}

func saveCodingPlans(accounts []CodingPlanAccount) error {
	path, err := codingPlansPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(codingPlanStore{Accounts: accounts}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ── 服务方法 ───────────────────────────────────────────────────

// ListCodingPlans 返回已配置的账号列表。
func (s *CodingPlanService) ListCodingPlans() ([]CodingPlanAccount, error) {
	return loadCodingPlans()
}

// SaveCodingPlan 新增或更新账号；ID 为空时生成新 ID，返回落库后的账号。
func (s *CodingPlanService) SaveCodingPlan(account CodingPlanAccount) (CodingPlanAccount, error) {
	account.Provider = strings.ToLower(strings.TrimSpace(account.Provider))
	switch account.Provider {
	case codingPlanZhipu, codingPlanKimi, codingPlanMiniMax, codingPlanZenMux:
	default:
		return account, fmt.Errorf("不支持的供应商：%s", account.Provider)
	}
	account.APIKey = strings.TrimSpace(account.APIKey)
	account.BaseURL = strings.TrimSpace(account.BaseURL)
	account.Name = strings.TrimSpace(account.Name)
	if account.Name == "" {
		account.Name = codingPlanProviderLabel(account.Provider)
	}
	if account.APIKey == "" {
		return account, errors.New("API Key 不能为空")
	}
	if account.Provider == codingPlanZenMux {
		if !strings.HasPrefix(strings.ToLower(account.BaseURL), "https://") {
			return account, errors.New("ZenMux 需要完整的 https:// 查询端点 URL")
		}
	}

	codingPlanMu.Lock()
	defer codingPlanMu.Unlock()
	accounts, err := loadCodingPlans()
	if err != nil {
		return account, err
	}
	if account.ID == "" {
		account.ID = fmt.Sprintf("cp-%x", time.Now().UnixNano())
		account.CreatedAt = time.Now().Unix()
		accounts = append(accounts, account)
	} else {
		found := false
		for i := range accounts {
			if accounts[i].ID == account.ID {
				accounts[i] = account
				found = true
				break
			}
		}
		if !found {
			return account, fmt.Errorf("账号不存在：%s", account.ID)
		}
	}
	if err := saveCodingPlans(accounts); err != nil {
		return account, err
	}
	return account, nil
}

// DeleteCodingPlan 删除账号配置。
func (s *CodingPlanService) DeleteCodingPlan(id string) error {
	codingPlanMu.Lock()
	defer codingPlanMu.Unlock()
	accounts, err := loadCodingPlans()
	if err != nil {
		return err
	}
	kept := accounts[:0]
	found := false
	for _, a := range accounts {
		if a.ID == id {
			found = true
			continue
		}
		kept = append(kept, a)
	}
	if !found {
		return fmt.Errorf("账号不存在：%s", id)
	}
	return saveCodingPlans(kept)
}

// QueryCodingPlanUsage 查询单个账号的实时用量。账号不存在返回 Go 错误；
// 查询失败（网络/鉴权/业务/解析）不返回错误，而是带 Status/Error 的结果。
func (s *CodingPlanService) QueryCodingPlanUsage(id string) (CodingPlanUsage, error) {
	accounts, err := loadCodingPlans()
	if err != nil {
		return CodingPlanUsage{}, err
	}
	for i := range accounts {
		if accounts[i].ID == id {
			usage := queryCodingPlanQuota(accounts[i])
			usage.AccountID = id
			return usage, nil
		}
	}
	return CodingPlanUsage{}, fmt.Errorf("账号不存在：%s", id)
}

// queryCodingPlanQuota 按供应商分发查询并归一为统一的用量结构。
func queryCodingPlanQuota(account CodingPlanAccount) CodingPlanUsage {
	usage := CodingPlanUsage{
		AccountID: account.ID,
		Status:    codingPlanStatusError,
		QueriedAt: time.Now().Unix(),
	}
	if account.APIKey == "" {
		usage.Error = "API Key 为空"
		return usage
	}

	var q5, q7 *CodingPlanQuota
	var planName string
	var err error
	switch account.Provider {
	case codingPlanZhipu:
		q5, q7, planName, err = queryZhipuUsage(account.BaseURL, account.APIKey)
	case codingPlanKimi:
		q5, q7, planName, err = queryKimiUsage(account.APIKey)
	case codingPlanMiniMax:
		q5, q7, planName, err = queryMiniMaxUsage(account.BaseURL, account.APIKey)
	case codingPlanZenMux:
		q5, q7, planName, err = queryZenMuxUsage(account.BaseURL, account.APIKey)
	default:
		err = fmt.Errorf("不支持的供应商：%s", account.Provider)
	}
	if err != nil {
		usage.Error = err.Error()
		if errors.Is(err, errCodingPlanAuth) {
			usage.Status = codingPlanStatusExpired
		}
		return usage
	}

	usage.Status = codingPlanStatusOK
	usage.FiveHour = q5
	usage.Weekly = q7
	usage.PlanName = planName
	return usage
}

// codingPlanProviderLabel 返回供应商默认显示名（备注名留空时回填）。
func codingPlanProviderLabel(provider string) string {
	switch provider {
	case codingPlanZhipu:
		return "智谱 GLM"
	case codingPlanKimi:
		return "Kimi For Coding"
	case codingPlanMiniMax:
		return "MiniMax"
	case codingPlanZenMux:
		return "ZenMux"
	}
	return provider
}

// ── HTTP 与 JSON 工具 ──────────────────────────────────────────

// codingPlanGet 发起 GET 并解析 JSON；非 2xx 返回错误（401/403 包装为 errCodingPlanAuth）。
func codingPlanGet(url string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := codingPlanHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络错误：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w (HTTP %d)", errCodingPlanAuth, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API 错误 (HTTP %d)：%s", resp.StatusCode, truncateRunes(string(body), 200))
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("响应解析失败：%w", err)
	}
	return parsed, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// jObj / jArr / jStr / jNum / jInt 是 map[string]any 的宽松取值助手，
// 字段缺失或类型不符时返回零值，与 cc-switch 的 serde_json 兜底语义一致。
func jObj(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func jArr(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	v, _ := m[key].([]any)
	return v
}

func jStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// jNum 兼容数字与数字字符串（如 100 与 "100"）；解析失败视为缺失。
func jNum(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	switch n := m[key].(type) {
	case float64:
		return n, true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func jInt(m map[string]any, key string) (int64, bool) {
	f, ok := jNum(m, key)
	if !ok {
		return 0, false
	}
	return int64(f), true
}

// codingPlanMillisToISO 把毫秒时间戳转为带本地时区偏移的 ISO 8601。
func codingPlanMillisToISO(ms int64) string {
	return time.UnixMilli(ms).Format(time.RFC3339)
}

// kimiResetISO 提取 Kimi 的 resetTime：字符串直接透传，数字按秒(<1e12)/毫秒自适应。
func kimiResetISO(m map[string]any) string {
	if s := jStr(m, "resetTime"); s != "" {
		return s
	}
	if ms, ok := jInt(m, "resetTime"); ok {
		if ms < 1_000_000_000_000 {
			ms *= 1000
		}
		return codingPlanMillisToISO(ms)
	}
	return ""
}

// kimiUtilization 由 limit/remaining 计算已用百分比（负数截断为 0）。
func kimiUtilization(detail map[string]any) float64 {
	limit, _ := jNum(detail, "limit")
	remaining, _ := jNum(detail, "remaining")
	used := limit - remaining
	if used < 0 {
		used = 0
	}
	if limit <= 0 {
		return 0
	}
	return used / limit * 100
}

// quotaUSDLabel 生成 ZenMux 的 "$已用 / $总额" 文本；字段缺失返回空。
func quotaUSDLabel(m map[string]any) string {
	used, ok1 := jNum(m, "used_value_usd")
	max, ok2 := jNum(m, "max_value_usd")
	if !ok1 || !ok2 {
		return ""
	}
	return fmt.Sprintf("$%.2f / $%.2f", used, max)
}

// ── Kimi For Coding ───────────────────────────────────────────

// queryKimiUsage 查询 Kimi：GET https://api.kimi.com/coding/v1/usages（Bearer）。
// 5h 桶取 limits[].detail（limit-remaining 推百分比），周桶取顶层 usage。
func queryKimiUsage(apiKey string) (*CodingPlanQuota, *CodingPlanQuota, string, error) {
	body, err := codingPlanGet("https://api.kimi.com/coding/v1/usages", map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
	})
	if err != nil {
		return nil, nil, "", err
	}
	q5, q7 := parseKimiTiers(body)
	return q5, q7, "", nil
}

func parseKimiTiers(body map[string]any) (q5, q7 *CodingPlanQuota) {
	for _, it := range jArr(body, "limits") {
		obj, _ := it.(map[string]any)
		detail := jObj(obj, "detail")
		if detail == nil {
			continue
		}
		q5 = &CodingPlanQuota{
			UsedPercent: kimiUtilization(detail),
			ResetsAt:    kimiResetISO(detail),
		}
		break
	}
	if usage := jObj(body, "usage"); usage != nil {
		q7 = &CodingPlanQuota{
			UsedPercent: kimiUtilization(usage),
			ResetsAt:    kimiResetISO(usage),
		}
	}
	return q5, q7
}

// ── 智谱 GLM ──────────────────────────────────────────────────

// zhipuQuotaBase 由配置的 base_url 路由监控端点：含 bigmodel.cn 走国内站，否则国际站。
func zhipuQuotaBase(baseURL string) string {
	if strings.Contains(strings.ToLower(baseURL), "bigmodel.cn") {
		return "https://open.bigmodel.cn"
	}
	return "https://api.z.ai"
}

// queryZhipuUsage 查询智谱：GET {base}/api/monitor/usage/quota/limit。
// 鉴权头直接放 API Key，不加 Bearer 前缀（与官方开放平台监控接口约定一致）。
func queryZhipuUsage(baseURL, apiKey string) (*CodingPlanQuota, *CodingPlanQuota, string, error) {
	url := zhipuQuotaBase(baseURL) + "/api/monitor/usage/quota/limit"
	body, err := codingPlanGet(url, map[string]string{
		"Authorization":   apiKey,
		"Content-Type":    "application/json",
		"Accept-Language": "en-US,en",
	})
	if err != nil {
		return nil, nil, "", err
	}
	if v, exists := body["success"]; exists {
		if ok, _ := v.(bool); !ok {
			msg := jStr(body, "msg")
			if msg == "" {
				msg = "未知错误"
			}
			return nil, nil, "", fmt.Errorf("API 错误：%s", msg)
		}
	}
	data := jObj(body, "data")
	if data == nil {
		return nil, nil, "", errors.New("响应缺少 data 字段")
	}
	q5, q7 := parseZhipuTiers(data)
	return q5, q7, jStr(data, "level"), nil
}

// parseZhipuTiers 解析智谱 data.limits 中的 TOKENS_LIMIT 条目为 (5h, 周) 两个桶。
//
// 分类优先级：
//  1. 显式 unit 字段：3=五小时滚动窗，6=周窗。不能按 nextResetTime 排序代替——
//     周期末末尾周桶会比 5h 桶更早重置（cc-switch #3036），按时间排序必然标反。
//  2. 兜底（unit 缺失或不识别）：无 nextResetTime 的条目优先归五小时桶
//     （0% 等状态下可能没有 reset），其余按 reset 升序填入仍空缺的槽位。
//
// 老套餐只回 1 条 TOKENS_LIMIT 时自然降级为仅展示五小时桶；超出的条目丢弃。
func parseZhipuTiers(data map[string]any) (q5, q7 *CodingPlanQuota) {
	type zhipuEntry struct {
		resetMS  int64
		hasReset bool
		pct      float64
		resetISO string
	}
	var fiveHour, weekly *zhipuEntry
	var unclassified []zhipuEntry

	for _, it := range jArr(data, "limits") {
		obj, _ := it.(map[string]any)
		if obj == nil || !strings.EqualFold(jStr(obj, "type"), "TOKENS_LIMIT") {
			continue
		}
		pct, _ := jNum(obj, "percentage")
		e := zhipuEntry{pct: pct}
		if ms, ok := jInt(obj, "nextResetTime"); ok {
			e.resetMS, e.hasReset = ms, true
			e.resetISO = codingPlanMillisToISO(ms)
		}
		unit, _ := jInt(obj, "unit")
		switch {
		case unit == 3 && fiveHour == nil:
			fiveHour = &e
		case unit == 6 && weekly == nil:
			weekly = &e
		default:
			unclassified = append(unclassified, e)
		}
	}

	sort.SliceStable(unclassified, func(i, j int) bool {
		if unclassified[i].hasReset != unclassified[j].hasReset {
			return !unclassified[i].hasReset // 无 reset 的优先归五小时桶
		}
		return unclassified[i].resetMS < unclassified[j].resetMS
	})
	for _, e := range unclassified {
		if fiveHour == nil {
			fiveHour = &e
		} else if weekly == nil {
			weekly = &e
		}
	}

	if fiveHour != nil {
		q5 = &CodingPlanQuota{UsedPercent: fiveHour.pct, ResetsAt: fiveHour.resetISO}
	}
	if weekly != nil {
		q7 = &CodingPlanQuota{UsedPercent: weekly.pct, ResetsAt: weekly.resetISO}
	}
	return q5, q7
}

// ── MiniMax ───────────────────────────────────────────────────

// queryMiniMaxUsage 查询 MiniMax：GET {域名}/v1/api/openplatform/coding_plan/remains（Bearer）。
// 国内站 api.minimaxi.com，国际站 api.minimax.io（由 base_url 是否含 minimax.io 判断）。
func queryMiniMaxUsage(baseURL, apiKey string) (*CodingPlanQuota, *CodingPlanQuota, string, error) {
	domain := "https://api.minimaxi.com"
	if strings.Contains(strings.ToLower(baseURL), "minimax.io") {
		domain = "https://api.minimax.io"
	}
	body, err := codingPlanGet(domain+"/v1/api/openplatform/coding_plan/remains", map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	})
	if err != nil {
		return nil, nil, "", err
	}
	if baseResp := jObj(body, "base_resp"); baseResp != nil {
		code, _ := jInt(baseResp, "status_code") // 缺失按 -1 处理，同样视为业务错误
		if code != 0 {
			msg := jStr(baseResp, "status_msg")
			if msg == "" {
				msg = "未知错误"
			}
			return nil, nil, "", fmt.Errorf("API 错误 (code %d)：%s", code, msg)
		}
	}
	q5, q7 := parseMiniMaxTiers(body)
	return q5, q7, "", nil
}

// parseMiniMaxTiers 解析 model_remains 中的 general 桶（跳过 video 等）。
// 接口直接给"剩余百分比"，反转为已用；周桶仅 current_weekly_status==1 激活，
// ==3 表示无周限额（剩余恒 100%，不应展示假 0% 周桶）。
func parseMiniMaxTiers(body map[string]any) (q5, q7 *CodingPlanQuota) {
	for _, it := range jArr(body, "model_remains") {
		obj, _ := it.(map[string]any)
		if obj == nil || jStr(obj, "model_name") != "general" {
			continue
		}
		if remain, ok := jNum(obj, "current_interval_remaining_percent"); ok {
			iso := ""
			if ms, ok2 := jInt(obj, "end_time"); ok2 {
				iso = codingPlanMillisToISO(ms)
			}
			q5 = &CodingPlanQuota{UsedPercent: 100 - remain, ResetsAt: iso}
		}
		if status, _ := jInt(obj, "current_weekly_status"); status == 1 {
			if remain, ok := jNum(obj, "current_weekly_remaining_percent"); ok {
				iso := ""
				if ms, ok2 := jInt(obj, "weekly_end_time"); ok2 {
					iso = codingPlanMillisToISO(ms)
				}
				q7 = &CodingPlanQuota{UsedPercent: 100 - remain, ResetsAt: iso}
			}
		}
		break
	}
	return q5, q7
}

// ── ZenMux ────────────────────────────────────────────────────

// queryZenMuxUsage 查询 ZenMux：直接 GET 用户配置的完整端点（Bearer）。
// usage_percentage 是 0-1 小数需乘 100，另附带 $已用/$总额 额度文本。
func queryZenMuxUsage(baseURL, apiKey string) (*CodingPlanQuota, *CodingPlanQuota, string, error) {
	body, err := codingPlanGet(baseURL, map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
	})
	if err != nil {
		return nil, nil, "", err
	}
	if ok, _ := body["success"].(bool); !ok {
		msg := jStr(body, "message")
		if msg == "" {
			msg = "未知错误"
		}
		return nil, nil, "", fmt.Errorf("API 错误：%s", msg)
	}
	data := jObj(body, "data")
	if data == nil {
		return nil, nil, "", errors.New("响应缺少 data 字段")
	}

	var q5, q7 *CodingPlanQuota
	if q5o := jObj(data, "quota_5_hour"); q5o != nil {
		pct, _ := jNum(q5o, "usage_percentage")
		q5 = &CodingPlanQuota{
			UsedPercent: pct * 100,
			ResetsAt:    jStr(q5o, "resets_at"),
			UsedLabel:   quotaUSDLabel(q5o),
		}
	}
	if q7o := jObj(data, "quota_7_day"); q7o != nil {
		pct, _ := jNum(q7o, "usage_percentage")
		q7 = &CodingPlanQuota{
			UsedPercent: pct * 100,
			ResetsAt:    jStr(q7o, "resets_at"),
			UsedLabel:   quotaUSDLabel(q7o),
		}
	}

	planName := ""
	if tier := jStr(jObj(data, "plan"), "tier"); tier != "" {
		planName = fmt.Sprintf("%s (%s)", tier, jStr(data, "account_status"))
	}
	return q5, q7, planName, nil
}
