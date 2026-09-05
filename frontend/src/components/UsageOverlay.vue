<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Events } from '@wailsio/runtime'
import { CodingPlanService, SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { SetUsageRows } from '../../bindings/github.com/Sxuan-Coder/PortCheck/overlayservice'
import type { CodingPlanAccount, CodingPlanQuota, CodingPlanUsage } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { usageLevel, LEVEL_COLOR, countdown } from '../lib/codingplans'

// 用量悬浮窗：Coding Plan 圆环图表视图，透明背景置顶。
// 每个账号一行：左侧圆环显示 5 小时窗口百分比（已用/剩余由设置项 usageOverlayMode
// 决定，圆环颜色始终按「已用程度」分级），右侧账号名 + 「重置倒计时 · 周百分比」。
// 数据自取：启动查询一次 + 每 5 分钟轮询；主窗口增删账号时通过 codingplan:changed
// 事件即时刷新。窗口高度由后端按行数调整（SetUsageRows）。

const REFRESH_MS = 5 * 60 * 1000
const TICK_MS = 30 * 1000
const MAX_ACCOUNTS = 3

const accounts = ref<CodingPlanAccount[]>([])
const usages = ref<Record<string, CodingPlanUsage>>({})
const now = ref(Date.now())
// 百分比展示模式：used=已用（默认）/ remaining=剩余；启动读一次，设置页通过事件实时推送。
const mode = ref<'used' | 'remaining'>('used')

let refreshTimer: number | undefined
let tickTimer: number | undefined

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

// 圆环展示 5 小时桶；套餐只有周桶时退化展示周桶，两者皆无则空环。
function quotaOf(u: CodingPlanUsage | null | undefined): CodingPlanQuota | null {
  if (!u || u.status !== 'ok') return null
  return u.fiveHour ?? u.weekly
}
// 按展示模式换算百分比：remaining 模式显示 100-已用。
const dispPct = (q: CodingPlanQuota | null) =>
  q ? (mode.value === 'remaining' ? 100 - q.usedPercent : q.usedPercent) : 0
// 颜色始终按已用程度分级（剩余模式下用量高仍是红色警示）。
const ringColor = (q: CodingPlanQuota | null) =>
  q ? LEVEL_COLOR[usageLevel(q.usedPercent)] : 'rgba(255,255,255,0.25)'
const ringOffset = (q: CodingPlanQuota | null) => C * (1 - Math.min(100, Math.max(0, dispPct(q))) / 100)
const ringText = (q: CodingPlanQuota | null) => (q ? `${Math.round(dispPct(q))}` : '—')

// 第二行文案：常态显示「重置 倒计时 · 周 X%」；无 5h 桶时退化显示周桶倒计时。
// 剩余模式下周期数字同样换算为剩余并加「剩」前缀，避免与已用混淆。
const line2 = (u: CodingPlanUsage | null | undefined) => {
  if (!u) return '加载中…'
  if (u.status === 'expired') return '密钥失效'
  if (u.status === 'error') return '查询失败'
  const weekly = (wk: CodingPlanQuota | null) => {
    if (!wk) return '无周限额'
    const v = mode.value === 'remaining' ? 100 - wk.usedPercent : wk.usedPercent
    return mode.value === 'remaining' ? `周剩 ${Math.round(v)}%` : `周 ${Math.round(v)}%`
  }
  if (u.fiveHour) {
    const cd = countdown(u.fiveHour.resetsAt, now.value)
    return `${cd ? `重置 ${cd} · ` : ''}${weekly(u.weekly)}`
  }
  if (u.weekly) {
    const cd = countdown(u.weekly.resetsAt, now.value)
    return cd ? `周重置 ${cd}` : '仅周配额'
  }
  return '无配额数据'
}

function onChanged() {
  refresh()
}

// 设置页切换显示模式时实时推送（载荷兼容 ev.data 包装形态）。
function onUsageConfig(ev: any) {
  const raw = ev && ev.data ? ev.data : ev
  const cfg = (raw && typeof raw === 'object' ? raw : {}) as { mode?: string }
  if (cfg.mode === 'used' || cfg.mode === 'remaining') mode.value = cfg.mode
}

onMounted(async () => {
  try {
    const s = await SettingsService.GetSettings()
    if (s.usageOverlayMode === 'remaining') mode.value = 'remaining'
  } catch {
    /* 保持默认 */
  }
  refresh()
  refreshTimer = window.setInterval(refresh, REFRESH_MS)
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
      <svg class="ring" viewBox="0 0 36 36">
        <circle class="track" cx="18" cy="18" :r="R" />
        <circle
          class="val"
          cx="18"
          cy="18"
          :r="R"
          :stroke="ringColor(quotaOf(usages[a.id]))"
          :stroke-dasharray="C"
          :stroke-dashoffset="ringOffset(quotaOf(usages[a.id]))"
        />
        <text class="num" x="18" y="18.5" text-anchor="middle" dominant-baseline="central">
          {{ ringText(quotaOf(usages[a.id])) }}
        </text>
      </svg>
      <div class="txt">
        <div class="name">{{ a.name }}</div>
        <div class="sub">{{ line2(usages[a.id]) }}</div>
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
  height: 48px;
  padding: 4px 2px;
}
.ring {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  transform: rotate(-90deg);
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
