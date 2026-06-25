<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { PortService } from '../bindings/github.com/Sxuan-Coder/PortCheck'
import type { PortEntry, PortListResult } from '../bindings/github.com/Sxuan-Coder/PortCheck/models'

type Protocol = 'ALL' | 'TCP' | 'UDP'
type ProcessType = 'all' | 'node' | 'java' | 'go' | 'csharp' | 'other'
type PortRow = PortEntry & { _id: number; _type: ProcessType }

// 进程类型识别（启发式：进程名 + 路径关键词；Go/C# 编译产物无法 100% 识别，归为其他）
function classifyProcess(name: string, path: string): ProcessType {
  const base = (name || '').toLowerCase().replace(/\.(exe|com)$/, '')
  const p = (path || '').toLowerCase()
  if (['node', 'npm', 'npx', 'pnpm', 'yarn', 'bun'].includes(base) || p.includes('nodejs') || p.includes('node_modules') || p.includes('nvm')) return 'node'
  if (['java', 'javaw'].includes(base) || p.includes('\\jre') || p.includes('\\jdk') || p.includes('/jre') || p.includes('/jdk')) return 'java'
  if (base === 'go' || p.includes('\\go\\bin') || p.includes('/go/bin') || p.includes('go-build')) return 'go'
  if (base === 'dotnet' || p.includes('\\dotnet') || p.includes('/dotnet')) return 'csharp'
  return 'other'
}

const PROCESS_TYPE_LABELS: { value: ProcessType; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'node', label: 'Node.js' },
  { value: 'java', label: 'Java' },
  { value: 'go', label: 'Go' },
  { value: 'csharp', label: 'C#' },
  { value: 'other', label: '其他' },
]

// 分页：每页 100 条，避免万级数据一次性撑爆 DOM
const PAGE_SIZE = 100
const currentPage = ref(1)

const ports = ref<PortRow[]>([])
const stats = ref<PortListResult>({
  ports: [],
  tcpCount: 0,
  udpCount: 0,
  listeningCount: 0,
  processCount: 0,
  warnings: [],
})
const loading = ref(false)
const killingPid = ref<number | null>(null)
const error = ref('')
const message = ref('')
const query = ref('')
const debouncedQuery = ref('')
const protocolFilter = ref<Protocol>('ALL')
const stateFilter = ref('ALL')
const processTypeFilter = ref<ProcessType>('all')

// 搜索防抖：避免每次按键都触发全量重算 + DOM 重建
let debounceTimer: ReturnType<typeof setTimeout> | null = null
watch(query, (value) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    debouncedQuery.value = value.trim().toLowerCase()
  }, 300)
})
onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})

const filteredPorts = computed(() => {
  const keyword = debouncedQuery.value

  return ports.value.filter((item) => {
    if (processTypeFilter.value !== 'all' && item._type !== processTypeFilter.value) return false
    if (protocolFilter.value !== 'ALL' && item.protocol !== protocolFilter.value) return false
    if (stateFilter.value !== 'ALL' && item.state !== stateFilter.value) return false
    if (!keyword) return true

    return [
      item.protocol,
      item.localAddr,
      String(item.localPort),
      item.remoteAddr,
      String(item.remotePort || ''),
      item.state,
      String(item.pid),
      item.process,
      item.path,
    ].some((part) => part.toLowerCase().includes(keyword))
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredPorts.value.length / PAGE_SIZE)))

const displayedPorts = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filteredPorts.value.slice(start, start + PAGE_SIZE)
})

function goToPage(page: number) {
  currentPage.value = Math.min(Math.max(1, page), totalPages.value)
}

// 搜索/筛选/刷新后数据集变化，回到第 1 页
watch(filteredPorts, () => {
  currentPage.value = 1
})

const processTypeCounts = computed(() => {
  const counts: Record<ProcessType, number> = { all: 0, node: 0, java: 0, go: 0, csharp: 0, other: 0 }
  for (const item of ports.value) {
    counts.all++
    counts[item._type]++
  }
  return counts
})

const stateOptions = computed(() => {
  const states = new Set<string>()
  for (const item of ports.value) {
    if (item.protocol === 'TCP' && item.state) states.add(item.state)
  }
  return ['ALL', ...Array.from(states).sort()]
})

const lastUpdated = ref('')

function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN').format(value)
}

function endpoint(addr: string, port: number) {
  if (!addr && !port) return '-'
  if (!port) return addr || '-'
  return `${addr}:${port}`
}

function canKill(item: PortEntry) {
  return item.pid !== 0 && item.pid !== 4
}

async function refreshPorts(clearNotice = true) {
  if (loading.value) return

  loading.value = true
  if (clearNotice) {
    error.value = ''
    message.value = ''
  }

  try {
    const result = await PortService.ListPorts()
    ports.value = result.ports.map((port, index) => ({
      ...port,
      _id: index,
      _type: classifyProcess(port.process, port.path),
    }))
    stats.value = result
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

async function killProcess(item: PortEntry) {
  if (!canKill(item) || killingPid.value !== null) return

  const answer = await Dialogs.Question({
    Title: '确认结束进程',
    Message: `端口 ${item.localPort} 当前由 ${item.process || `PID ${item.pid}`} 占用。\n\n结束的是进程，不是单个端口。确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })

  if (answer !== 'Yes') return

  killingPid.value = item.pid
  error.value = ''
  message.value = ''

  try {
    const result = await PortService.KillProcess(item.pid)
    message.value = result.message || `已结束 PID ${item.pid}`
    await refreshPorts(false)

    const remaining = ports.value.find((port) => {
      return port.protocol === item.protocol && port.localAddr === item.localAddr && port.localPort === item.localPort
    })
    if (remaining) {
      if (remaining.pid === item.pid) {
        error.value = `PID ${item.pid} 已发送结束请求，但端口 ${item.localPort} 仍然存在，可能需要管理员权限或进程没有退出。`
      } else {
        message.value = `已结束 PID ${item.pid}，但端口 ${item.localPort} 又被 ${remaining.process || `PID ${remaining.pid}`} 占用。`
      }
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    killingPid.value = null
  }
}

onMounted(refreshPorts)
</script>

<template>
  <main class="app-shell">
    <header class="topbar reveal">
      <div class="brand">
        <p class="eyebrow">Windows Port Watcher</p>
        <h1>
          PortCheck
          <span class="live-dot" title="实时" aria-hidden="true"></span>
        </h1>
      </div>
      <button class="btn btn-primary refresh-button" type="button" :disabled="loading" @click="() => refreshPorts()">
        <span class="refresh-icon" :class="{ spinning: loading }" aria-hidden="true">⟳</span>
        {{ loading ? '刷新中' : '刷新' }}
      </button>
    </header>

    <section class="stats-grid reveal" aria-label="端口统计">
      <article class="stat-card glass tone-tcp">
        <span class="stat-label">TCP</span>
        <strong class="stat-value">{{ formatNumber(stats.tcpCount) }}</strong>
      </article>
      <article class="stat-card glass tone-udp">
        <span class="stat-label">UDP</span>
        <strong class="stat-value">{{ formatNumber(stats.udpCount) }}</strong>
      </article>
      <article class="stat-card glass tone-listen">
        <span class="stat-label">监听端口</span>
        <strong class="stat-value">{{ formatNumber(stats.listeningCount) }}</strong>
      </article>
      <article class="stat-card glass tone-proc">
        <span class="stat-label">进程</span>
        <strong class="stat-value">{{ formatNumber(stats.processCount) }}</strong>
      </article>
    </section>

    <section class="type-filter reveal" aria-label="进程类型筛选">
      <span class="type-filter-label">类型</span>
      <button
        v-for="t in PROCESS_TYPE_LABELS"
        :key="t.value"
        type="button"
        class="type-chip"
        :class="[{ active: processTypeFilter === t.value }, t.value]"
        @click="processTypeFilter = t.value"
      >
        {{ t.label }}
        <span class="type-chip-count">{{ formatNumber(processTypeCounts[t.value]) }}</span>
      </button>
    </section>

    <section class="toolbar glass reveal" aria-label="筛选端口">
      <label class="search-box">
        <span class="field-label">搜索</span>
        <input v-model="query" type="search" placeholder="端口、进程名、PID、路径" />
      </label>

      <label>
        <span class="field-label">协议</span>
        <select v-model="protocolFilter">
          <option value="ALL">全部</option>
          <option value="TCP">TCP</option>
          <option value="UDP">UDP</option>
        </select>
      </label>

      <label>
        <span class="field-label">状态</span>
        <select v-model="stateFilter">
          <option v-for="state in stateOptions" :key="state" :value="state">
            {{ state === 'ALL' ? '全部' : state }}
          </option>
        </select>
      </label>

      <p class="count-text">
        当前匹配 <strong>{{ formatNumber(filteredPorts.length) }}</strong> 条
        <span v-if="lastUpdated" class="updated">· {{ lastUpdated }}</span>
      </p>
    </section>

    <p v-if="message" class="notice success">{{ message }}</p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <p v-for="warning in stats.warnings" :key="warning" class="notice warning">{{ warning }}</p>

    <section class="panel table-panel reveal">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>协议</th>
              <th>本地端口</th>
              <th>本地地址</th>
              <th>远程地址</th>
              <th>状态</th>
              <th>进程</th>
              <th>PID</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!loading && filteredPorts.length === 0">
              <td colspan="8" class="empty">没有匹配的端口</td>
            </tr>
            <tr v-for="item in displayedPorts" :key="item._id">
              <td>
                <span class="chip protocol" :class="item.protocol.toLowerCase()">{{ item.protocol }}</span>
              </td>
              <td class="port">{{ item.localPort }}</td>
              <td class="mono">{{ endpoint(item.localAddr, item.localPort) }}</td>
              <td class="mono">{{ item.protocol === 'UDP' ? '-' : endpoint(item.remoteAddr, item.remotePort) }}</td>
              <td>
                <span class="chip state" :class="{ listening: item.state === 'LISTENING' }">{{ item.state }}</span>
              </td>
              <td>
                <div class="process-cell">
                  <strong>{{ item.process || `PID ${item.pid}` }}</strong>
                  <small v-if="item.path">{{ item.path }}</small>
                </div>
              </td>
              <td class="mono pid">{{ item.pid }}</td>
              <td>
                <button
                  class="btn btn-danger kill-button"
                  type="button"
                  :disabled="!canKill(item) || killingPid !== null"
                  @click="killProcess(item)"
                >
                  {{ killingPid === item.pid ? '处理中' : '结束' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <footer v-if="filteredPorts.length > 0" class="pagination" aria-label="分页">
        <button type="button" class="btn btn-ghost page-button" :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
          上一页
        </button>
        <span class="page-info">
          第 <strong>{{ currentPage }}</strong> / {{ totalPages }} 页
        </span>
        <button type="button" class="btn btn-ghost page-button" :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
          下一页
        </button>
      </footer>
    </section>
  </main>
</template>

<style scoped>
/* ============ 设计 Token ============ */
:global(body) {
  --blue: #007aff;
  --blue-strong: #0066d6;
  --blue-soft: rgba(0, 122, 255, 0.12);
  --red: #ff3b30;
  --red-soft: rgba(255, 59, 48, 0.12);

  --text-1: #1d2129;
  --text-2: #5b6473;
  --text-3: #98a2b3;

  --glass-bg: rgba(255, 255, 255, 0.55);
  --glass-bg-deep: rgba(255, 255, 255, 0.72);
  --glass-border: rgba(255, 255, 255, 0.7);
  --glass-hairline: rgba(15, 23, 42, 0.06);
  --blur: 22px;

  --radius-lg: 18px;
  --radius-md: 12px;
  --radius-sm: 8px;

  --shadow-sm: 0 2px 8px rgba(15, 23, 42, 0.04), 0 1px 2px rgba(15, 23, 42, 0.03);
  --shadow-md: 0 10px 30px rgba(15, 23, 42, 0.08), 0 2px 8px rgba(15, 23, 42, 0.04);
  --shadow-lg: 0 24px 60px rgba(15, 23, 42, 0.14), 0 8px 24px rgba(15, 23, 42, 0.06);

  margin: 0;
  min-width: 980px;
  color: var(--text-1);
  font-family: -apple-system, "SF Pro Display", "PingFang SC", "Segoe UI", "Microsoft YaHei", sans-serif;
  background:
    radial-gradient(circle at 14% 16%, rgba(0, 122, 255, 0.09), transparent 42%),
    radial-gradient(circle at 86% 80%, rgba(139, 92, 246, 0.09), transparent 44%),
    linear-gradient(135deg, #f6f8fb 0%, #e9edf3 50%, #dde3ec 100%);
  position: relative;
  overflow-x: auto;
}

/* 诊断：移除全屏 fixed 光晕层（主线程 paint 风暴嫌疑），背景仅保留 body 上的简单渐变 */

:global(*) {
  box-sizing: border-box;
  scrollbar-width: thin;
  scrollbar-color: rgba(0, 122, 255, 0.3) transparent;
}

/* macOS 风格滚动条（Chromium / WebView2） */
:global(::-webkit-scrollbar) {
  width: 10px;
  height: 10px;
}

:global(::-webkit-scrollbar-track) {
  background: transparent;
}

:global(::-webkit-scrollbar-thumb) {
  background: rgba(0, 122, 255, 0.28);
  border-radius: 999px;
  border: 2px solid transparent;
  background-clip: padding-box;
}

:global(::-webkit-scrollbar-thumb:hover) {
  background: rgba(0, 122, 255, 0.5);
  background-clip: padding-box;
}

:global(::-webkit-scrollbar-corner) {
  background: transparent;
}

button,
input,
select {
  font: inherit;
}

/* ============ 玻璃基础类 ============ */
.glass {
  /* 不用 backdrop-filter：合成器在大数据刷新时会被持续重模糊打满。
     用半透明实色 + 边框 + 阴影模拟玻璃质感，背景光晕仍可透出 */
  background: var(--glass-bg-deep);
  border: 1px solid var(--glass-border);
  box-shadow: var(--shadow-md);
}

/* ============ 外壳 ============ */
.app-shell {
  position: relative;
  z-index: 1;
  min-height: 100vh;
  padding: 28px 32px 36px;
}

/* ============ 入场动效（staggered） ============ */
.reveal {
  animation: fadeUp 0.7s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.stats-grid.reveal { animation-delay: 0.08s; }
.toolbar.reveal { animation-delay: 0.16s; }
.table-panel.reveal { animation-delay: 0.24s; }

@keyframes fadeUp {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ============ 顶部标题栏 ============ */
.topbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 22px;
}

.eyebrow {
  margin: 0 0 6px;
  color: var(--text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

h1 {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 36px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1;
}

.live-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #34c759;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  /* 仅 opacity + transform：合成器线程运行，主线程被阻塞时也能继续 */
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(0.8);
  }
}

/* ============ 按钮系统 ============ */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-weight: 600;
  font-size: 14px;
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease, opacity 0.18s ease;
}

.btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.btn-primary {
  min-width: 96px;
  padding: 11px 20px;
  color: #fff;
  background: linear-gradient(180deg, #2a92ff 0%, var(--blue) 100%);
  box-shadow: 0 6px 16px rgba(0, 122, 255, 0.32), inset 0 1px 0 rgba(255, 255, 255, 0.25);
}

.btn-primary:hover:not(:disabled) {
  background: linear-gradient(180deg, #3a9cff 0%, var(--blue-strong) 100%);
  transform: translateY(-1px);
  box-shadow: 0 10px 22px rgba(0, 122, 255, 0.4), inset 0 1px 0 rgba(255, 255, 255, 0.3);
}

.btn-primary:active:not(:disabled) {
  transform: translateY(0);
}

.refresh-icon {
  font-size: 16px;
  line-height: 1;
}

.refresh-icon.spinning {
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.btn-danger {
  min-width: 72px;
  padding: 7px 14px;
  color: var(--red);
  background: var(--red-soft);
  border-color: rgba(255, 59, 48, 0.2);
}

.btn-danger:hover:not(:disabled) {
  background: var(--red);
  color: #fff;
  border-color: var(--red);
  transform: translateY(-1px);
  box-shadow: 0 6px 14px rgba(255, 59, 48, 0.3);
}

.btn-ghost {
  min-width: 80px;
  padding: 8px 16px;
  color: var(--text-1);
  background: rgba(255, 255, 255, 0.5);
  border-color: var(--glass-hairline);
}

.btn-ghost:hover:not(:disabled) {
  background: var(--blue);
  color: #fff;
  border-color: var(--blue);
  transform: translateY(-1px);
  box-shadow: 0 6px 14px rgba(0, 122, 255, 0.28);
}

/* ============ 统计卡片 ============ */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.stat-card {
  position: relative;
  padding: 18px 20px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: transform 0.22s ease, box-shadow 0.22s ease;
}

.stat-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  width: 4px;
  height: 100%;
  background: var(--accent, var(--blue));
  opacity: 0.9;
}

.stat-card::after {
  content: "";
  position: absolute;
  top: -40px;
  right: -40px;
  width: 110px;
  height: 110px;
  border-radius: 50%;
  background: var(--accent, var(--blue));
  opacity: 0.08;
}

.stat-card:hover {
  transform: translateY(-3px);
  box-shadow: var(--shadow-lg);
}

.tone-tcp { --accent: #007aff; }
.tone-udp { --accent: #8b5cf6; }
.tone-listen { --accent: #34c759; }
.tone-proc { --accent: #ff9500; }

.stat-label {
  display: block;
  margin-bottom: 10px;
  color: var(--text-2);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.stat-value {
  display: block;
  font-size: 30px;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

/* ============ 进程类型筛选条 ============ */
.type-filter {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
  padding: 12px 16px;
  border-radius: var(--radius-lg);
  background: rgba(255, 255, 255, 0.5);
  border: 1px solid var(--glass-hairline);
  box-shadow: var(--shadow-sm);
}

.type-filter-label {
  margin-right: 4px;
  color: var(--text-2);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.type-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 13px;
  border: 1px solid var(--glass-hairline);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
  color: var(--text-1);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.18s ease, background 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.type-chip:hover {
  border-color: rgba(0, 122, 255, 0.35);
  transform: translateY(-1px);
}

.type-chip.active {
  background: var(--blue);
  border-color: var(--blue);
  color: #fff;
  box-shadow: 0 4px 12px rgba(0, 122, 255, 0.3);
}

.type-chip-count {
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  opacity: 0.65;
}

.type-chip.active .type-chip-count {
  opacity: 1;
}

/* ============ 工具栏 ============ */
.toolbar {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) 140px 170px auto;
  align-items: end;
  gap: 14px;
  margin-bottom: 14px;
  padding: 16px 18px;
  border-radius: var(--radius-lg);
}

.field-label {
  display: block;
  margin-bottom: 7px;
  color: var(--text-2);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

input,
select {
  width: 100%;
  height: 40px;
  border: 1px solid var(--glass-hairline);
  border-radius: var(--radius-sm);
  padding: 0 12px;
  color: var(--text-1);
  background: rgba(255, 255, 255, 0.6);
  transition: border-color 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
}

input::placeholder {
  color: var(--text-3);
}

input:hover,
select:hover {
  border-color: rgba(0, 122, 255, 0.3);
}

input:focus,
select:focus {
  outline: none;
  border-color: var(--blue);
  background: #fff;
  box-shadow: 0 0 0 4px var(--blue-soft);
}

.count-text {
  margin: 0 0 10px;
  color: var(--text-2);
  font-size: 13px;
  white-space: nowrap;
}

.count-text strong {
  color: var(--blue);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.updated {
  color: var(--text-3);
  margin-left: 4px;
}

/* ============ 提示条 ============ */
.notice {
  margin: 0 0 12px;
  border-radius: var(--radius-md);
  padding: 11px 14px;
  font-size: 14px;
  font-weight: 500;
  border: 1px solid transparent;
}

.success {
  color: #0f7a4d;
  background: rgba(52, 199, 89, 0.14);
  border-color: rgba(52, 199, 89, 0.25);
}

.error {
  color: #c0180c;
  background: rgba(255, 59, 48, 0.12);
  border-color: rgba(255, 59, 48, 0.25);
}

.warning {
  color: #8a5a00;
  background: rgba(255, 149, 0, 0.14);
  border-color: rgba(255, 149, 0, 0.25);
}

/* ============ 表表面板（玻璃容器 + 实色行） ============ */
.table-panel {
  border-radius: var(--radius-lg);
  overflow: hidden;
  /* 不用 backdrop-filter：大面积 + 频繁重绘区域，blur 会打满 GPU */
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid var(--glass-border);
  box-shadow: var(--shadow-md);
}

.table-wrap {
  overflow: auto;
  max-height: calc(100vh - 380px);
}

table {
  width: 100%;
  min-width: 1040px;
  border-collapse: collapse;
}

th,
td {
  border-bottom: 1px solid var(--glass-hairline);
  padding: 13px 16px;
  text-align: left;
  vertical-align: middle;
  font-size: 14px;
}

/* sticky 表头：半透明玻璃（行数固定，性能无虞） */
th {
  position: sticky;
  top: 0;
  z-index: 1;
  color: var(--text-2);
  background: rgba(246, 248, 251, 0.96);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

tbody tr {
  /* 行保持实色：万行场景绝不对每行施加 blur */
  background: rgba(255, 255, 255, 0.45);
  transition: background 0.14s ease;
}

tbody tr:nth-child(even) {
  background: rgba(248, 250, 252, 0.55);
}

tbody tr:hover {
  background: var(--blue-soft);
}

.mono {
  font-variant-numeric: tabular-nums;
  color: var(--text-1);
}

.port {
  color: var(--text-1);
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.pid {
  color: var(--text-2);
}

/* ============ 胶囊标签（协议/状态） ============ */
.chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 54px;
  height: 24px;
  border-radius: 999px;
  padding: 0 10px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.04em;
}

.chip.protocol.tcp {
  color: #0062cc;
  background: rgba(0, 122, 255, 0.14);
}

.chip.protocol.udp {
  color: #6d28d9;
  background: rgba(139, 92, 246, 0.16);
}

.chip.state {
  color: var(--text-2);
  background: rgba(120, 130, 150, 0.14);
}

.chip.state.listening {
  color: #0f7a4d;
  background: rgba(52, 199, 89, 0.18);
}

/* ============ 进程单元格 ============ */
.process-cell {
  max-width: 340px;
}

.process-cell strong {
  display: block;
  font-weight: 600;
  color: var(--text-1);
}

.process-cell small {
  display: block;
  overflow: hidden;
  margin-top: 2px;
  color: var(--text-3);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ============ 空状态 ============ */
.empty {
  height: 200px;
  color: var(--text-3);
  font-size: 15px;
  text-align: center;
}

/* ============ 分页栏（融入容器底部） ============ */
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 18px;
  padding: 14px 18px;
  border-top: 1px solid var(--glass-hairline);
  background: rgba(246, 248, 251, 0.75);
}

.page-info {
  color: var(--text-2);
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}

.page-info strong {
  color: var(--blue);
  font-weight: 700;
}

@media (prefers-reduced-motion: reduce) {
  .reveal,
  .live-dot,
  .refresh-icon.spinning {
    animation: none !important;
  }
}
</style>
