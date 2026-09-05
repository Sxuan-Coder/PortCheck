<script setup lang="ts">
// Coding Plan 用量卡片：CPU 占用风格的桌面组件。
// 5 小时窗口用环形仪表展示，周用量用进度条展示；颜色分级与 cc-switch 一致
// （<70% 绿、70-89% 橙、≥90% 红），重置倒计时格式 4h41m / 2d22h。
import AppIcon from './AppIcon.vue'
import type { CodingPlanAccount, CodingPlanQuota, CodingPlanUsage } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { providerMeta, usageLevel, LEVEL_COLOR, countdown } from '../lib/codingplans'

const props = defineProps<{
  account: CodingPlanAccount
  usage: CodingPlanUsage | null
  loading?: boolean
  now: number
}>()

defineEmits<{ (e: 'edit'): void; (e: 'remove'): void }>()

// 环形仪表几何参数（viewBox 64x64）
const R = 26
const C = 2 * Math.PI * R

const meta = () => providerMeta(props.account.provider)
const colorOf = (q: CodingPlanQuota | null | undefined) => LEVEL_COLOR[q ? usageLevel(q.usedPercent) : 'ok']
const pctText = (q: CodingPlanQuota) => `${Math.round(q.usedPercent)}%`
const barWidth = (q: CodingPlanQuota) => `${Math.min(100, Math.max(0, q.usedPercent))}%`
const ringOffset = (q: CodingPlanQuota) => C * (1 - Math.min(100, Math.max(0, q.usedPercent)) / 100)
</script>

<template>
  <div
    class="card acrylic-card"
    :class="{ expired: usage?.status === 'expired', errored: usage?.status === 'error' }"
  >
    <div class="head">
      <div class="who">
        <span class="avatar" :style="{ color: meta().color, borderColor: meta().color + '55', background: meta().color + '1a' }">
          {{ meta().short }}
        </span>
        <div class="names">
          <div class="name">{{ account.name || meta().label }}</div>
          <div class="sub">
            {{ meta().label }}
            <span v-if="usage?.planName" class="plan">{{ usage.planName }}</span>
          </div>
        </div>
      </div>
      <div class="ops">
        <button class="op" title="编辑" @click="$emit('edit')"><AppIcon name="edit" :size="13" /></button>
        <button class="op danger" title="删除" @click="$emit('remove')"><AppIcon name="trash" :size="13" /></button>
      </div>
    </div>

    <!-- 加载骨架 -->
    <div v-if="loading && !usage" class="body">
      <div class="skeleton ring-sk" />
      <div class="sk-col">
        <div class="skeleton w60" />
        <div class="skeleton w40" />
      </div>
    </div>

    <!-- 失败态：密钥失效（黄）/ 查询失败（红） -->
    <div v-else-if="usage && usage.status !== 'ok'" class="body err">
      <div class="err-dot" :class="{ warn: usage.status === 'expired' }" />
      <div class="err-text">
        <div class="err-title">{{ usage.status === 'expired' ? '密钥失效' : '查询失败' }}</div>
        <div class="err-msg" :title="usage.error">{{ usage.error }}</div>
      </div>
    </div>

    <!-- 成功态 -->
    <div v-else-if="usage" class="body">
      <div v-if="usage.fiveHour" class="five">
        <div class="gauge">
          <svg viewBox="0 0 64 64">
            <circle class="track" cx="32" cy="32" :r="R" />
            <circle
              class="val"
              cx="32"
              cy="32"
              :r="R"
              :stroke="colorOf(usage.fiveHour)"
              :stroke-dasharray="C"
              :stroke-dashoffset="ringOffset(usage.fiveHour)"
            />
          </svg>
          <span class="gauge-num mono" :style="{ color: colorOf(usage.fiveHour) }">{{ pctText(usage.fiveHour) }}</span>
        </div>
        <div class="info">
          <div class="label">5 小时窗口</div>
          <div class="reset">
            重置 <b class="mono">{{ countdown(usage.fiveHour.resetsAt, now) || '—' }}</b>
          </div>
          <div v-if="usage.fiveHour.usedLabel" class="usd mono">{{ usage.fiveHour.usedLabel }}</div>
        </div>
      </div>

      <div v-if="usage.weekly" class="week">
        <div class="week-head">
          <span class="label">周用量（7 天）</span>
          <span class="pct mono" :style="{ color: colorOf(usage.weekly) }">{{ pctText(usage.weekly) }}</span>
        </div>
        <div class="meter">
          <span :style="{ width: barWidth(usage.weekly), background: colorOf(usage.weekly) }" />
        </div>
        <div class="reset">
          重置 <b class="mono">{{ countdown(usage.weekly.resetsAt, now) || '—' }}</b>
          <span v-if="usage.weekly.usedLabel" class="usd mono">· {{ usage.weekly.usedLabel }}</span>
        </div>
      </div>
      <div v-else class="week none">该套餐无周限额</div>
    </div>

    <!-- 尚未查询 -->
    <div v-else class="body idle">尚未查询</div>
  </div>
</template>

<style scoped>
.card {
  border-radius: var(--radius-lg);
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 168px;
}
.card.expired {
  border-color: rgba(245, 158, 11, 0.4);
}
.card.errored {
  border-color: rgba(239, 68, 68, 0.35);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.who {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.avatar {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  border: 1px solid;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  font-weight: 700;
}
.names {
  min-width: 0;
}
.name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub {
  font-size: 11px;
  color: var(--text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.plan {
  margin-left: 6px;
  padding: 1px 7px;
  border-radius: 99px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  color: var(--text-2);
  font-size: 10px;
}
.ops {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}
.card:hover .ops {
  opacity: 1;
}
.op {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  background: var(--field-bg);
  color: var(--text-3);
  transition: color 0.15s, border-color 0.15s;
}
.op:hover {
  color: var(--brand);
  border-color: var(--brand-glow);
}
.op.danger:hover {
  color: var(--red);
  border-color: rgba(239, 68, 68, 0.4);
}

.body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  justify-content: center;
}

/* 5 小时环形仪表 */
.five {
  display: flex;
  align-items: center;
  gap: 14px;
}
.gauge {
  position: relative;
  width: 68px;
  height: 68px;
  flex-shrink: 0;
}
.gauge svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.gauge .track {
  fill: none;
  stroke: var(--hairline-strong);
  stroke-width: 6;
}
.gauge .val {
  fill: none;
  stroke-width: 6;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.5s ease, stroke 0.3s;
}
.gauge-num {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 700;
}
.info {
  min-width: 0;
}
.label {
  font-size: 12px;
  color: var(--text-2);
}
.reset {
  margin-top: 3px;
  font-size: 11px;
  color: var(--text-3);
}
.reset b {
  color: var(--text-2);
  font-weight: 600;
}
.usd {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-4);
}

/* 周用量进度条 */
.week {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.week-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}
.pct {
  font-weight: 700;
}
.meter {
  height: 6px;
  border-radius: 99px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  overflow: hidden;
}
.meter span {
  display: block;
  height: 100%;
  border-radius: 99px;
  transition: width 0.5s ease, background 0.3s;
}
.week.none {
  font-size: 11px;
  color: var(--text-4);
}

/* 失败态 */
.err {
  flex-direction: row;
  align-items: center;
  gap: 10px;
}
.err-dot {
  width: 8px;
  height: 8px;
  border-radius: 99px;
  background: var(--red);
  flex-shrink: 0;
}
.err-dot.warn {
  background: var(--amber);
}
.err-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-1);
}
.err-msg {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-3);
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  word-break: break-all;
}

.idle {
  font-size: 11px;
  color: var(--text-4);
}

/* 骨架屏 */
.skeleton {
  border-radius: var(--radius-sm);
  background: linear-gradient(90deg, var(--field-bg) 25%, var(--header-bg) 50%, var(--field-bg) 75%);
  background-size: 200% 100%;
  animation: sk 1.2s linear infinite;
}
.ring-sk {
  width: 68px;
  height: 68px;
  border-radius: 99px;
  flex-shrink: 0;
}
.sk-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.w60 {
  width: 60%;
  height: 12px;
}
.w40 {
  width: 40%;
  height: 10px;
}
@keyframes sk {
  to {
    background-position: -200% 0;
  }
}
</style>
