<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Events } from '@wailsio/runtime'
import { CodingPlanService, SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { SetUsageRows } from '../../bindings/github.com/Sxuan-Coder/PortCheck/overlayservice'
import type { CodingPlanAccount, CodingPlanQuota, CodingPlanUsage } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { usageLevel, LEVEL_COLOR, countdown, providerMeta, balanceLevel, balancePct, currencySymbol } from '../lib/codingplans'

// 用量悬浮窗：Coding Plan 圆环图表视图，透明背景置顶。
// 每个账号一行：左侧并排展示全部可用窗口（5 小时 / 周 / 月）的圆环百分比
// （已用/剩余由设置项 usageOverlayMode 决定，圆环颜色始终按「已用程度」分级），
// 右侧账号名 + 各窗口重置倒计时；余额型供应商（DeepSeek）的圆环按
// 「余额余量 / 预警值 ×10」填充、颜色按预警分档。
// 数据自取：启动查询一次 + 按设置间隔定时轮询（默认 5 分钟）；主窗口增删账号时通过 codingplan:changed
// 事件即时刷新。窗口高度由后端按行数调整（SetUsageRows）。

const TICK_MS = 30 * 1000
const MAX_ACCOUNTS = 3

const accounts = ref<CodingPlanAccount[]>([])
const usages = ref<Record<string, CodingPlanUsage>>({})
const now = ref(Date.now())
// 百分比展示模式：used=已用（默认）/ remaining=剩余；启动读一次，设置页通过事件实时推送。
const mode = ref<'used' | 'remaining'>('used')
// 自动刷新间隔（分钟）：启动读一次设置，设置页经 usage-overlay:config 事件实时推送。
const refreshMinutes = ref(5)

let refreshTimer: number | undefined
let tickTimer: number | undefined

function scheduleRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  refreshTimer = window.setInterval(refresh, refreshMinutes.value * 60 * 1000)
}

// 圆环几何参数（viewBox 36x36）
const R = 15.5
const C = 2 * Math.PI * R

async function refresh() {
  try {
    const list = await CodingPlanService.ListCodingPlans()
    const map: Record<string, CodingPlanUsage> = {}
    await Promise.all(
      list.map(async (a) => {
        try {
          map[a.id] = await CodingPlanService.QueryCodingPlanUsage(a.id)
        } catch {
          /* 单账号失败静默，保持其它行可用 */
        }
      }),
    )
    accounts.value = list
    usages.value = map
  } catch {
    /* 网络异常保持上次数据 */
  }
  syncRows()
}

// 行数上报：账号 0 个 → 1 行提示；>3 个 → 3 行 + 1 行"更多"。
function syncRows() {
  const n = accounts.value.length
  const rows = Math.max(1, Math.min(n, MAX_ACCOUNTS) + (n > MAX_ACCOUNTS ? 1 : 0))
  SetUsageRows(rows).catch(() => {})
}

const shown = () => accounts.value.slice(0, MAX_ACCOUNTS)
const moreCount = () => accounts.value.length - MAX_ACCOUNTS

// 配额桶：按 5 小时 / 周 / 月顺序全部并排展示，缺失的桶自动跳过。
// 普通供应商最多两环，OpenCode Go 显示三环。
interface OverlayBucket {
  label: string
  quota: CodingPlanQuota
}
function quotaBuckets(u: CodingPlanUsage | null | undefined): OverlayBucket[] {
  if (!u || u.status !== 'ok' || u.balance) return []
  const list: OverlayBucket[] = []
  if (u.fiveHour) list.push({ label: '5h', quota: u.fiveHour })
  if (u.weekly) list.push({ label: '7d', quota: u.weekly })
  if (u.monthly) list.push({ label: '本月', quota: u.monthly })
  return list
}
// 余额型（DeepSeek）：环填充 = 余额余量（满环 = 预警值 ×10），不受已用/剩余模式影响。
function balanceOf(u: CodingPlanUsage | null | undefined) {
  if (!u || u.status !== 'ok') return null
  return u.balance ?? null
}
// 按展示模式换算百分比：remaining 模式显示 100-已用。
const dispPct = (q: CodingPlanQuota) =>
  Math.min(100, Math.max(0, mode.value === 'remaining' ? 100 - q.usedPercent : q.usedPercent))
// 颜色：余额型按预警分档（跌破预警值即红），配额型始终按已用程度分级。
const quotaColor = (q: CodingPlanQuota) => LEVEL_COLOR[usageLevel(q.usedPercent)]
const balanceColor = (a: CodingPlanAccount, u: CodingPlanUsage | null | undefined) => {
  const b = balanceOf(u)
  return b ? LEVEL_COLOR[balanceLevel(b.total, a.alertAmount, b.available)] : 'rgba(255,255,255,0.25)'
}
const quotaPct = (a: CodingPlanAccount, u: CodingPlanUsage | null | undefined) => {
  const b = balanceOf(u)
  return b ? balancePct(b.total, a.alertAmount) : 0
}
const ringOffset = (pct: number) => C * (1 - pct / 100)

// 第二行文案：显示各窗口的重置倒计时（百分比已在圆环内）；
// 余额型显示「余额 ¥xx · 预警 ¥xx」。
const line2 = (a: CodingPlanAccount, u: CodingPlanUsage | null | undefined) => {
  if (!u) return '加载中…'
  if (u.status === 'expired') return '密钥失效'
  if (u.status === 'error') return '查询失败'
  if (u.balance) {
    if (!u.balance.available) return '余额不足，无法调用'
    const s = currencySymbol(u.balance.currency)
    const base = `余额 ${s}${u.balance.total.toFixed(2)}`
    return a.alertAmount > 0 ? `${base} · 预警 ${s}${a.alertAmount}` : base
  }
  // 倒计时按圆环顺序（5h / 7d / 本月）依次展示，避免与环下标签重复。
  const cds = quotaBuckets(u)
    .map((b) => countdown(b.quota.resetsAt, now.value))
    .filter(Boolean)
  if (cds.length) return `重置 ${cds.join(' · ')}`
  return '无配额数据'
}

// 主窗口事件分两类载荷：
//   - { usages: map }：主窗口刷新拿到最新用量，直接采用，不重复请求接口
//   - 无载荷：账号增删，需要重新拉账号列表并全量查询
function onChanged(ev: any) {
  const raw = ev && ev.data ? ev.data : ev
  const payload = raw && typeof raw === 'object' ? raw : null
  if (payload && payload.usages && typeof payload.usages === 'object') {
    usages.value = payload.usages as Record<string, CodingPlanUsage>
    syncRows()
  } else {
    refresh()
  }
}

// 设置页切换显示模式 / 刷新间隔时实时推送（载荷兼容 ev.data 包装形态）。
function onUsageConfig(ev: any) {
  const raw = ev && ev.data ? ev.data : ev
  const cfg = (raw && typeof raw === 'object' ? raw : {}) as { mode?: string; refreshMinutes?: number }
  if (cfg.mode === 'used' || cfg.mode === 'remaining') mode.value = cfg.mode
  if (Number.isFinite(cfg.refreshMinutes) && (cfg.refreshMinutes as number) > 0) {
    refreshMinutes.value = cfg.refreshMinutes as number
    scheduleRefresh()
  }
}

onMounted(async () => {
  try {
    const s = await SettingsService.GetSettings()
    if (s.usageOverlayMode === 'remaining') mode.value = 'remaining'
    if ([1, 3, 5, 10, 15, 30].includes(s.codingPlanRefreshMinutes)) {
      refreshMinutes.value = s.codingPlanRefreshMinutes
    }
  } catch {
    /* 保持默认 */
  }
  refresh()
  scheduleRefresh()
  tickTimer = window.setInterval(() => (now.value = Date.now()), TICK_MS)
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (tickTimer) window.clearInterval(tickTimer)
  cancelChanged()
  cancelUsageConfig()
})

// 主窗口增删账号后即时刷新；设置页切换显示模式后即时生效。
const cancelChanged = Events.On('codingplan:changed', onChanged)
const cancelUsageConfig = Events.On('usage-overlay:config', onUsageConfig)
</script>

<template>
  <!-- 透明背景；--wails-draggable: drag 允许拖动微调（不持久化）。
       白色文字 + text-shadow 保证在任意桌面背景上可读；圆环按用量分级着色。 -->
  <div class="overlay" style="--wails-draggable: drag">
    <div v-if="accounts.length === 0" class="row empty-row">
      <svg class="ring" viewBox="0 0 36 36">
        <circle class="track" cx="18" cy="18" :r="R" />
      </svg>
      <div class="txt">
        <div class="name">未配置账号</div>
        <div class="sub">在「用量查询」页添加 Coding Plan</div>
      </div>
    </div>

    <div v-for="a in shown()" :key="a.id" class="row">
      <div class="rings">
        <!-- 余额型（DeepSeek）：单个金额圆环 -->
        <div v-if="balanceOf(usages[a.id])" class="ring-cell">
          <svg class="ring" viewBox="0 0 36 36">
            <circle class="track" cx="18" cy="18" :r="R" />
            <circle
              class="val"
              cx="18"
              cy="18"
              :r="R"
              :stroke="balanceColor(a, usages[a.id])"
              :stroke-dasharray="C"
              :stroke-dashoffset="ringOffset(quotaPct(a, usages[a.id]))"
            />
            <text class="num" x="18" y="18.5" text-anchor="middle" dominant-baseline="central">
              {{ Math.round(quotaPct(a, usages[a.id])) }}
            </text>
          </svg>
          <span class="ring-label">余额</span>
        </div>

        <!-- 配额型：5 小时 + 周并排，各自独立圆环 -->
        <template v-else-if="quotaBuckets(usages[a.id]).length">
          <div v-for="(b, i) in quotaBuckets(usages[a.id])" :key="i" class="ring-cell">
            <svg class="ring" viewBox="0 0 36 36">
              <circle class="track" cx="18" cy="18" :r="R" />
              <circle
                class="val"
                cx="18"
                cy="18"
                :r="R"
                :stroke="quotaColor(b.quota)"
                :stroke-dasharray="C"
                :stroke-dashoffset="ringOffset(dispPct(b.quota))"
              />
              <text class="num" x="18" y="18.5" text-anchor="middle" dominant-baseline="central">
                {{ Math.round(dispPct(b.quota)) }}
              </text>
            </svg>
            <span class="ring-label">{{ b.label }}</span>
          </div>
        </template>

        <!-- 无数据：空环占位 -->
        <div v-else class="ring-cell">
          <svg class="ring" viewBox="0 0 36 36">
            <circle class="track" cx="18" cy="18" :r="R" />
          </svg>
          <span class="ring-label">—</span>
        </div>
      </div>

      <div class="txt">
        <div class="name">{{ a.name || providerMeta(a.provider).label }}</div>
        <div class="sub">{{ line2(a, usages[a.id]) }}</div>
      </div>
    </div>

    <div v-if="moreCount() > 0" class="row more">
      <span class="more-txt">还有 {{ moreCount() }} 个账号，详见主窗口</span>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  background: transparent;
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  justify-content: space-around;
  height: 100vh;
  width: 100vw;
  box-sizing: border-box;
  font-family: "Segoe UI", "Inter", -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  color: #fff;
  user-select: none;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.85), 0 0 4px rgba(0, 0, 0, 0.5);
}
.row {
  display: flex;
  align-items: center;
  gap: 9px;
  height: 52px;
  padding: 2px;
}
.rings {
  display: flex;
  align-items: flex-start;
  gap: 3px;
  flex-shrink: 0;
}
.ring-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
}
.ring {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  transform: rotate(-90deg);
}
.ring-label {
  font-size: 9px;
  line-height: 10px;
  opacity: 0.72;
}
.ring .track {
  fill: none;
  stroke: rgba(255, 255, 255, 0.18);
  stroke-width: 4.5;
}
.ring .val {
  fill: none;
  stroke-width: 4.5;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.5s ease, stroke 0.3s;
}
.ring .num {
  font-size: 10px;
  font-weight: 700;
  fill: #fff;
  transform: rotate(90deg);
  transform-origin: 18px 18px;
  font-family: "Segoe UI", "Inter", sans-serif;
}
.txt {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.name {
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.sub {
  font-size: 10px;
  opacity: 0.75;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.empty-row .sub {
  opacity: 0.6;
}
.more {
  height: 20px;
  align-items: center;
}
.more-txt {
  font-size: 10px;
  opacity: 0.65;
}
</style>
