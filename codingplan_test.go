package main

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("测试 JSON 解析失败：%v", err)
	}
	return m
}

// ── 智谱：unit 字段分桶 ──

// cc-switch #3036 真实案例：周期末尾周桶比 5h 桶更早重置，
// 按 nextResetTime 排序必然把两桶标反，unit 字段必须优先。
func TestZhipuUnit分类_周桶先重置不翻转(t *testing.T) {
	data := mustJSON(t, `{
		"limits": [
			{ "type": "TOKENS_LIMIT", "unit": 6, "number": 7, "percentage": 42.0, "nextResetTime": 1000003600000 },
			{ "type": "TOKENS_LIMIT", "unit": 3, "number": 5, "percentage": 1.0,  "nextResetTime": 1000018000000 }
		]}`)
	q5, q7 := parseZhipuTiers(data)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶，got 5h=%v weekly=%v", q5, q7)
	}
	if q5.UsedPercent != 1.0 || q7.UsedPercent != 42.0 {
		t.Errorf("分桶错误：5h=%.1f（期望 1.0），周=%.1f（期望 42.0）", q5.UsedPercent, q7.UsedPercent)
	}
}

// 老套餐（2026-02-12 前订阅）只回 1 条 TOKENS_LIMIT，自然降级为仅 5h 桶。
func TestZhipu老套餐单条降级FiveHour(t *testing.T) {
	data := mustJSON(t, `{
		"limits": [
			{ "type": "TOKENS_LIMIT", "percentage": 2.0, "nextResetTime": 1774967594803 },
			{ "type": "TIME_LIMIT", "percentage": 0.0 }
		]}`)
	q5, q7 := parseZhipuTiers(data)
	if q5 == nil || q7 != nil {
		t.Fatalf("期望仅 5h 桶，got 5h=%v weekly=%v", q5, q7)
	}
	if q5.UsedPercent != 2.0 {
		t.Errorf("5h 百分比 = %.1f，期望 2.0", q5.UsedPercent)
	}
}

// 5h 桶 0% 时可能没有 nextResetTime：无 reset 的条目必须归 5h 桶，
// 不能按 reset 升序把周桶误判为 5h。
func TestZhipu无Reset条目归FiveHour(t *testing.T) {
	data := mustJSON(t, `{
		"limits": [
			{ "type": "TOKENS_LIMIT", "percentage": 25.0, "nextResetTime": 2000000000000 },
			{ "type": "TOKENS_LIMIT", "percentage": 0.0 }
		]}`)
	q5, q7 := parseZhipuTiers(data)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶，got 5h=%v weekly=%v", q5, q7)
	}
	if q5.UsedPercent != 0.0 || q5.ResetsAt != "" {
		t.Errorf("5h 桶应为 0%% 且无重置时间，got %.1f / %q", q5.UsedPercent, q5.ResetsAt)
	}
	if q7.UsedPercent != 25.0 || q7.ResetsAt == "" {
		t.Errorf("周桶应为 25%% 带重置时间，got %.1f / %q", q7.UsedPercent, q7.ResetsAt)
	}
}

// unit 缺失/不认识时兜底：按 reset 升序（无 reset 优先）填空缺槽位。
func TestZhipuUnit缺失按Reset升序兜底(t *testing.T) {
	data := mustJSON(t, `{
		"limits": [
			{ "type": "tokens_limit", "percentage": 44.0, "nextResetTime": 1000000000000 },
			{ "type": "Tokens_Limit", "percentage": 53.0, "nextResetTime": 2000000000000 }
		]}`)
	q5, q7 := parseZhipuTiers(data)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶")
	}
	if q5.UsedPercent != 44.0 || q7.UsedPercent != 53.0 {
		t.Errorf("兜底排序错误：5h=%.1f 周=%.1f", q5.UsedPercent, q7.UsedPercent)
	}
}

// type 大小写不敏感；非 TOKENS_LIMIT 条目忽略。
func TestZhipu忽略非TokensLimit条目(t *testing.T) {
	data := mustJSON(t, `{ "limits": [{ "type": "TIME_LIMIT", "percentage": 5.0 }] }`)
	q5, q7 := parseZhipuTiers(data)
	if q5 != nil || q7 != nil {
		t.Errorf("不应产生任何桶")
	}
}

// percentage 为字符串等异常形态时按 0 处理且不崩溃。
func TestZhipu异常Percentage按零处理(t *testing.T) {
	data := mustJSON(t, `{
		"limits": [
			{ "type": "TOKENS_LIMIT", "percentage": "invalid", "nextResetTime": 1000000000000 },
			{ "type": "TOKENS_LIMIT", "percentage": null, "nextResetTime": 2000000000000 }
		]}`)
	q5, q7 := parseZhipuTiers(data)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶")
	}
	if q5.UsedPercent != 0 || q7.UsedPercent != 0 {
		t.Errorf("异常 percentage 应按 0 处理")
	}
}

// base_url 路由：bigmodel.cn → 国内站；z.ai / 其他 → 国际站。
func TestZhipuBase路由(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn"},
		{"https://Open.BigModel.cn/api/paas/v4", "https://open.bigmodel.cn"},
		{"https://api.z.ai/api/paas/v4", "https://api.z.ai"},
		{"https://example.com/zhipu", "https://api.z.ai"},
	}
	for _, c := range cases {
		if got := zhipuQuotaBase(c.in); got != c.want {
			t.Errorf("zhipuQuotaBase(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// ── Kimi：limit/remaining 推百分比 ──

func TestKimi用量与重置时间(t *testing.T) {
	body := mustJSON(t, `{
		"limits": [{ "type": "FIVE_HOUR", "detail": { "limit": 100, "remaining": 40, "resetTime": 1774967594803 } }],
		"usage": { "limit": 1000, "remaining": "350", "resetTime": "2026-09-10T00:00:00Z" }
	}`)
	q5, q7 := parseKimiTiers(body)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶")
	}
	if q5.UsedPercent != 60 {
		t.Errorf("5h 已用 = %.1f，期望 60", q5.UsedPercent)
	}
	if q7.UsedPercent != 65 {
		t.Errorf("周已用 = %.1f，期望 65（remaining 为字符串也应解析）", q7.UsedPercent)
	}
	if q7.ResetsAt != "2026-09-10T00:00:00Z" {
		t.Errorf("字符串 resetTime 应原样透传，got %q", q7.ResetsAt)
	}
}

// remaining > limit（异常数据）时已用截断为 0，不出现负百分比。
func TestKimi负用量截断为零(t *testing.T) {
	body := mustJSON(t, `{ "usage": { "limit": 100, "remaining": 120 } }`)
	_, q7 := parseKimiTiers(body)
	if q7 == nil || q7.UsedPercent != 0 {
		t.Errorf("负用量应截断为 0")
	}
}

// ── MiniMax：剩余百分比反转 + 周桶激活判定 ──

func TestMiniMax两桶从剩余百分比反推(t *testing.T) {
	body := mustJSON(t, `{
		"model_remains": [
			{ "model_name": "general", "current_interval_remaining_percent": 98.0,
			  "current_weekly_remaining_percent": 95.0, "current_weekly_status": 1,
			  "end_time": 1780329600000, "weekly_end_time": 1780848000000 },
			{ "model_name": "video", "current_interval_remaining_percent": 100.0 }
		]}`)
	q5, q7 := parseMiniMaxTiers(body)
	if q5 == nil || q7 == nil {
		t.Fatalf("期望两个桶")
	}
	if q5.UsedPercent != 2 || q7.UsedPercent != 5 {
		t.Errorf("已用 = %.1f / %.1f，期望 2 / 5", q5.UsedPercent, q7.UsedPercent)
	}
	if q5.ResetsAt == "" || q7.ResetsAt == "" {
		t.Errorf("重置时间不应为空")
	}
}

// 无周限额套餐：current_weekly_status=3（剩余恒 100%），不应展示假 0% 周桶。
func TestMiniMax无周限额跳过周桶(t *testing.T) {
	body := mustJSON(t, `{
		"model_remains": [{
			"model_name": "general",
			"current_interval_remaining_percent": 99,
			"current_weekly_status": 3,
			"current_weekly_remaining_percent": 100,
			"end_time": 1780365600000,
			"weekly_end_time": 1780848000000
		}]}`)
	q5, q7 := parseMiniMaxTiers(body)
	if q5 == nil || q7 != nil {
		t.Fatalf("期望仅 5h 桶，got 5h=%v weekly=%v", q5, q7)
	}
	if q5.UsedPercent != 1 {
		t.Errorf("5h 已用 = %.1f，期望 1", q5.UsedPercent)
	}
}

// 缺 general 桶（只有 video / 空数组 / 缺字段）不崩溃、不产生桶。
func TestMiniMax缺General返回空(t *testing.T) {
	for _, s := range []string{
		`{ "model_remains": [{ "model_name": "video" }] }`,
		`{ "model_remains": [] }`,
		`{}`,
	} {
		if q5, q7 := parseMiniMaxTiers(mustJSON(t, s)); q5 != nil || q7 != nil {
			t.Errorf("%s 不应产生桶", s)
		}
	}
}

// ── ZenMux：0-1 小数 ×100 + USD 额度 ──

func TestZenMux小数百分比与USD(t *testing.T) {
	body := mustJSON(t, `{
		"success": true,
		"data": {
			"quota_5_hour": { "usage_percentage": 0.42, "resets_at": "2026-09-06T12:00:00Z",
				"used_value_usd": 3.2, "max_value_usd": 7.5 },
			"quota_7_day": { "usage_percentage": 0.55, "resets_at": "2026-09-10T00:00:00Z",
				"used_value_usd": 22.1, "max_value_usd": 40 },
			"plan": { "tier": "pro" },
			"account_status": "active"
		}}`)
	data := jObj(body, "data")
	q5o := jObj(data, "quota_5_hour")
	q7o := jObj(data, "quota_7_day")
	if pct, _ := jNum(q5o, "usage_percentage"); pct*100 != 42 {
		t.Errorf("5h 已用 = %v，期望 42", pct*100)
	}
	if got := quotaUSDLabel(q5o); got != "$3.20 / $7.50" {
		t.Errorf("USD 文本 = %q", got)
	}
	if got := quotaUSDLabel(q7o); got != "$22.10 / $40.00" {
		t.Errorf("USD 文本 = %q", got)
	}
}

// ── DeepSeek：余额解析与币种挑选 ──

// 官方示例形态：金额为字符串数字、is_available 布尔、赠金/充值拆分。
func TestDeepSeek余额解析(t *testing.T) {
	body := mustJSON(t, `{
		"is_available": true,
		"balance_infos": [{
			"currency": "CNY",
			"total_balance": "110.00",
			"granted_balance": "10.00",
			"topped_up_balance": "100.00"
		}]}`)
	b := parseDeepSeekBalance(body)
	if b == nil {
		t.Fatalf("期望余额结构")
	}
	if b.Currency != "CNY" || !b.Available {
		t.Errorf("币种/可用态解析错误：%q / %v", b.Currency, b.Available)
	}
	if b.Total != 110 || b.Granted != 10 || b.ToppedUp != 100 {
		t.Errorf("金额解析错误：total=%v granted=%v toppedUp=%v", b.Total, b.Granted, b.ToppedUp)
	}
}

// 双币种时优先 CNY；仅 USD 账户取 USD 条目。
func TestDeepSeek优先CNY币种(t *testing.T) {
	body := mustJSON(t, `{
		"is_available": true,
		"balance_infos": [
			{ "currency": "USD", "total_balance": "5.50", "granted_balance": "0", "topped_up_balance": "5.50" },
			{ "currency": "CNY", "total_balance": "39.80", "granted_balance": "0", "topped_up_balance": "39.80" }
		]}`)
	if b := parseDeepSeekBalance(body); b == nil || b.Currency != "CNY" || b.Total != 39.8 {
		t.Errorf("双币种应优先 CNY，got %+v", b)
	}

	onlyUSD := mustJSON(t, `{
		"is_available": false,
		"balance_infos": [{ "currency": "USD", "total_balance": "5.50" }]}`)
	b := parseDeepSeekBalance(onlyUSD)
	if b == nil || b.Currency != "USD" || b.Available {
		t.Errorf("仅 USD 时应取 USD 且 is_available=false，got %+v", b)
	}
}

// balance_infos 缺失 / 空数组 / 全为非对象时不崩溃，返回 nil 由上层报错。
func TestDeepSeek缺BalanceInfos不崩溃(t *testing.T) {
	for _, s := range []string{`{}`, `{ "balance_infos": [] }`, `{ "balance_infos": [42] }`} {
		if b := parseDeepSeekBalance(mustJSON(t, s)); b != nil {
			t.Errorf("%s 不应产生余额结构", s)
		}
	}
}

// ── 供应商注册表 ──

// 五家供应商全部注册且有默认显示名；查询函数非空。
func Test供应商注册表全覆盖(t *testing.T) {
	for _, p := range []string{
		codingPlanZhipu, codingPlanKimi, codingPlanMiniMax, codingPlanZenMux, codingPlanDeepseek,
	} {
		provider, ok := codingPlanProviders[p]
		if !ok {
			t.Errorf("供应商 %q 未注册", p)
			continue
		}
		if provider.label == "" || provider.query == nil {
			t.Errorf("供应商 %q 注册不完整：label/query 为空", p)
		}
	}
}
