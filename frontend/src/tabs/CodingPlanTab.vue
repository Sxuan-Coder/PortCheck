<script setup lang="ts">
// Coding Plan 用量页：账号卡片墙 + 添加/编辑弹窗。
// 用量按设置项自动定时刷新（默认 5 分钟）；倒计时每 30 秒本地重算（不重新请求接口）。
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { Dialogs, Events } from '@wailsio/runtime'
import { CodingPlanService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { CodingPlanAccount, CodingPlanUsage } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import CodingPlanCard from '../components/CodingPlanCard.vue'
import AppIcon from '../components/AppIcon.vue'
import { useToast } from '../composables/useToast'
import { useSettings } from '../composables/useSettings'
import { PROVIDERS, defaultRegion, providerMeta } from '../lib/codingplans'

const { toast } = useToast()
const { settings, enableUsageOverlayIfDisabled } = useSettings()

const accounts = ref<CodingPlanAccount[]>([])
const usages = ref<Record<string, CodingPlanUsage>>({})
const loading = ref(false)
const refreshing = ref(false)
const lastRefresh = ref('')
const now = ref(Date.now())

// 自动刷新间隔可由设置页调整（默认 5 分钟），watch 即时重启定时器。
const TICK_MS = 30 * 1000
let pollTimer: number | undefined
let tickTimer: number | undefined

function schedulePoll() {
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = window.setInterval(() => refreshAll(false), settings.value.codingPlanRefreshMinutes * 60 * 1000)
}

async function load() {
  loading.value = true
  try {
    accounts.value = await CodingPlanService.ListCodingPlans()
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    loading.value = false
  }
}

// broadcastUsages 把最新用量经事件推送给用量悬浮窗（独立 webview），
// 载荷形如 { usages: Record<账号ID, 用量> }，悬浮窗直接采用、无需重新请求接口。
// 主窗口每次刷新（定时/手动/保存后单查）都广播，保证两边数据始终同步。
function broadcastUsages(map: Record<string, CodingPlanUsage>) {
  Events.Emit('codingplan:changed', { usages: map })
}

// 并发查询全部账号；单账号失败不中断其它账号，失败卡片由自身 error 态展示。
async function refreshAll(manual = false) {
  if (refreshing.value || accounts.value.length === 0) return
  refreshing.value = true
  const map: Record<string, CodingPlanUsage> = {}
  await Promise.all(
    accounts.value.map(async (a) => {
      try {
        map[a.id] = await CodingPlanService.QueryCodingPlanUsage(a.id)
      } catch {
        /* 账号级失败静默，保持其它卡片可用 */
      }
    }),
  )
  usages.value = map
  lastRefresh.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  refreshing.value = false
  broadcastUsages(map)
  if (manual) {
    const failed = accounts.value.filter((a) => map[a.id] && map[a.id].status !== 'ok').length
    toast(failed > 0 ? `已刷新，${failed} 个账号查询失败` : '已刷新全部账号用量', failed > 0 ? 'error' : 'success')
  }
}

// ── 添加 / 编辑弹窗 ──

const dialog = ref(false)
const saving = ref(false)
const form = reactive({
  id: '',
  name: '',
  provider: 'zhipu',
  region: defaultRegion('zhipu'),
  baseUrl: '',
  apiKey: '',
  alertAmount: '', // 余额预警值（元），仅余额型供应商展示；空 = 不提醒
})

function resetForm() {
  form.id = ''
  form.name = ''
  form.provider = 'zhipu'
  form.region = defaultRegion('zhipu')
  form.baseUrl = ''
  form.apiKey = ''
  form.alertAmount = ''
}

function openAdd() {
  resetForm()
  dialog.value = true
}

function openEdit(a: CodingPlanAccount) {
  const meta = providerMeta(a.provider)
  form.id = a.id
  form.name = a.name
  form.provider = a.provider
  form.region = meta.regions?.some((r) => r.value === a.baseUrl) ? a.baseUrl : defaultRegion(a.provider)
  form.baseUrl = a.baseUrl
  form.apiKey = a.apiKey
  form.alertAmount = a.alertAmount > 0 ? String(a.alertAmount) : ''
  dialog.value = true
}

// 切换供应商时重置站点选择，避免残留其它供应商的 base URL。
watch(
  () => form.provider,
  (p) => {
    form.region = defaultRegion(p)
  },
)

function formBaseURL(): string {
  const meta = providerMeta(form.provider)
  if (meta.needURL) return form.baseUrl.trim()
  return form.region || defaultRegion(form.provider)
}

async function save() {
  if (saving.value) return
  if (!form.apiKey.trim()) {
    toast('请填写 API Key', 'error')
    return
  }
  if (providerMeta(form.provider).needURL && !/^https:\/\//i.test(form.baseUrl.trim())) {
    toast('查询端点必须是 https:// 开头的完整 URL', 'error')
    return
  }
  // 余额预警值：number 输入的 v-model 会被 Vue 自动转成数字，不能用字符串方法；
  // 空 = 0（不提醒）；非余额型供应商后端会归零，这里无需校验
  const alertAmount = Number(form.alertAmount || 0)
  if (Number.isNaN(alertAmount) || alertAmount < 0) {
    toast('余额预警值必须是不小于 0 的数字', 'error')
    return
  }
  saving.value = true
  try {
    const saved = await CodingPlanService.SaveCodingPlan({
      id: form.id,
      name: form.name.trim(),
      provider: form.provider,
      baseUrl: formBaseURL(),
      apiKey: form.apiKey.trim(),
      alertAmount,
      createdAt: 0,
    })
    dialog.value = false
    await load()
    // 保存后立即查询该账号，让新卡片马上有数据
    try {
      usages.value = { ...usages.value, [saved.id]: await CodingPlanService.QueryCodingPlanUsage(saved.id) }
      lastRefresh.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    } catch {
      /* 查询失败由卡片 error 态展示 */
    }
    // 添加首个账号时默认开启用量悬浮窗（已开启则不动，可在设置中关闭）
    if (!form.id && accounts.value.length === 1) {
      const enabledNow = await enableUsageOverlayIfDisabled()
      if (enabledNow) toast('已自动开启用量悬浮窗，可在「设置 → 用量悬浮窗」调整', 'info')
    }
    toast(form.id ? '账号已更新' : '账号已添加', 'success')
    // 通知用量悬浮窗（独立 webview）立即重查并调整行数。
    Events.Emit('codingplan:changed')
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    saving.value = false
  }
}

async function remove(a: CodingPlanAccount) {
  const ans = await Dialogs.Question({
    Title: '删除用量账号',
    Message: `将删除账号「${a.name}」的配置，不再监控其用量。确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })
  if (ans !== 'Yes') return
  try {
    await CodingPlanService.DeleteCodingPlan(a.id)
    await load()
    toast('已删除账号', 'success')
    Events.Emit('codingplan:changed')
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  }
}

onMounted(async () => {
  await load()
  await refreshAll(false)
  schedulePoll()
  tickTimer = window.setInterval(() => (now.value = Date.now()), TICK_MS)
})

watch(
  () => settings.value.codingPlanRefreshMinutes,
  () => schedulePoll(),
)

onUnmounted(() => {
  if (pollTimer) window.clearInterval(pollTimer)
  if (tickTimer) window.clearInterval(tickTimer)
})
</script>

<template>
  <div class="tab">
    <div class="head">
      <div class="title">
        <h2>Coding Plan 用量</h2>
        <p>5 小时窗口 / 周用量百分比监控，支持智谱 GLM · Kimi · MiniMax · ZenMux · DeepSeek（余额）</p>
      </div>
      <div class="actions">
        <span v-if="lastRefresh" class="updated">更新于 {{ lastRefresh }}</span>
        <button class="refresh" :disabled="refreshing" @click="refreshAll(true)">
          <span :class="{ spinning: refreshing }">⟳</span> 刷新
        </button>
        <button class="add" @click="openAdd"><AppIcon name="plus" :size="13" /> 添加账号</button>
      </div>
    </div>

    <div v-if="!loading && accounts.length === 0" class="empty acrylic-card">
      <div class="empty-icon"><AppIcon name="usage" :size="26" /></div>
      <p class="empty-title">还没有监控任何 Coding Plan</p>
      <p class="empty-hint">配置 API Key 后，即可在这里查看 5 小时 / 周用量百分比与重置倒计时；DeepSeek 账户则展示余额与预警</p>
      <button class="add" @click="openAdd"><AppIcon name="plus" :size="13" /> 添加账号</button>
    </div>

    <div v-else class="cards">
      <CodingPlanCard
        v-for="a in accounts"
        :key="a.id"
        :account="a"
        :usage="usages[a.id] ?? null"
        :loading="refreshing"
        :now="now"
        @edit="openEdit(a)"
        @remove="remove(a)"
      />
    </div>

    <div v-if="accounts.length > 0" class="foot">
      共 {{ accounts.length }} 个账号 · 每 5 分钟自动刷新 · 点击右上角「刷新」立即更新
    </div>

    <!-- 添加 / 编辑弹窗 -->
    <div v-if="dialog" class="modal" @click.self="dialog = false">
      <div class="dialog acrylic-card">
        <div class="d-head">{{ form.id ? '编辑账号' : '添加账号' }}</div>

        <label class="field">
          <span>供应商</span>
          <select v-model="form.provider" :disabled="!!form.id">
            <option v-for="(m, k) in PROVIDERS" :key="k" :value="k">{{ m.label }}</option>
          </select>
        </label>

        <label v-if="providerMeta(form.provider).regions" class="field">
          <span>站点</span>
          <select v-model="form.region">
            <option v-for="r in providerMeta(form.provider).regions" :key="r.value" :value="r.value">
              {{ r.label }}
            </option>
          </select>
        </label>

        <label v-if="providerMeta(form.provider).needURL" class="field">
          <span>查询端点</span>
          <input v-model="form.baseUrl" type="text" :placeholder="providerMeta(form.provider).urlHint" />
        </label>

        <label v-if="providerMeta(form.provider).isBalance" class="field">
          <span>余额预警值（元，可选）</span>
          <input
            v-model="form.alertAmount"
            type="number"
            min="0"
            step="0.01"
            placeholder="如 10：余额低于该值时卡片变红提醒，留空则不提醒"
          />
        </label>

        <label class="field">
          <span>API Key</span>
          <input
            v-model="form.apiKey"
            type="password"
            autocomplete="off"
            spellcheck="false"
            :placeholder="providerMeta(form.provider).keyHint"
          />
        </label>

        <label class="field">
          <span>备注名（可选）</span>
          <input v-model="form.name" type="text" placeholder="默认使用供应商名，如同一家有多个账号可自定义" />
        </label>

        <div class="d-foot">
          <button class="cancel" @click="dialog = false">取消</button>
          <button class="save" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存并查询' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.title h2 {
  margin: 0;
  font-size: 15px;
  color: var(--text-1);
}
.title p {
  margin: 3px 0 0;
  font-size: 11px;
  color: var(--text-3);
}
.actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.updated {
  font-size: 11px;
  color: var(--text-4);
}
.refresh {
  padding: 7px 14px;
  background: var(--brand-glow);
  border: 1px solid var(--brand-glow);
  color: var(--brand);
  border-radius: var(--radius-sm);
  font-size: 12px;
}
.refresh:disabled {
  opacity: 0.5;
}
.spinning {
  display: inline-block;
  animation: spin 0.9s linear infinite;
}
.add {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 14px;
  background: var(--brand);
  border: 1px solid var(--brand);
  color: #fff;
  border-radius: var(--radius-sm);
  font-size: 12px;
  transition: background 0.15s;
}
.add:hover {
  background: var(--brand-light);
  border-color: var(--brand-light);
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
  gap: 12px;
  align-content: start;
}

.empty {
  border-radius: var(--radius-lg);
  padding: 48px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
}
.empty-icon {
  color: var(--text-4);
  margin-bottom: 4px;
}
.empty-title {
  margin: 0;
  font-size: 13px;
  color: var(--text-2);
}
.empty-hint {
  margin: 0 0 8px;
  font-size: 11px;
  color: var(--text-4);
  max-width: 360px;
}

.foot {
  font-size: 11px;
  color: var(--text-3);
}

/* 弹窗 */
.modal {
  position: fixed;
  inset: 0;
  z-index: 60;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
}
.dialog {
  width: 400px;
  max-width: calc(100vw - 40px);
  border-radius: var(--radius-xl);
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: var(--win-bg);
  backdrop-filter: blur(var(--blur));
  -webkit-backdrop-filter: blur(var(--blur));
}
.d-head {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-1);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 5px;
  font-size: 11px;
  color: var(--text-3);
}
.field select,
.field input {
  padding: 7px 10px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--text-1);
  font-size: 12px;
  outline: none;
}
.field select:focus,
.field input:focus {
  border-color: var(--brand);
  background: var(--field-bg-focus);
}
.field select:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.d-foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}
.cancel {
  padding: 7px 14px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--text-2);
  font-size: 12px;
}
.save {
  padding: 7px 14px;
  background: var(--brand);
  border: 1px solid var(--brand);
  border-radius: var(--radius-sm);
  color: #fff;
  font-size: 12px;
}
.save:disabled {
  opacity: 0.5;
}
</style>
