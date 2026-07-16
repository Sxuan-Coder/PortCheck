<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { PortService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { PortEntry, PortListResult } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { useToast } from '../composables/useToast'
import { classifyProcess, PROCESS_TYPE_LABELS, type ProcessType } from '../lib/classify'
import { formatNumber } from '../lib/format'

type PortRow = PortEntry & { _id: number; _type: ProcessType }

const { toast } = useToast()
const COMMON_PORTS = [3000, 5173, 8000, 8080, 4200, 3001, 8888, 9000]
const PAGE_SIZE = 100

const ports = ref<PortRow[]>([])
const stats = ref<PortListResult>({
  ports: [], tcpCount: 0, udpCount: 0, listeningCount: 0, processCount: 0, warnings: [],
})
const loading = ref(false)
const killing = ref<number | null>(null)
const query = ref('')
const debounced = ref('')
const proto = ref<'ALL' | 'TCP' | 'UDP'>('ALL')
const statef = ref('ALL')
const typeFilter = ref<ProcessType>('all')
const page = ref(1)
const lastUpdated = ref('')

let t: ReturnType<typeof setTimeout> | null = null
watch(query, (v) => {
  if (t) clearTimeout(t)
  t = setTimeout(() => (debounced.value = v.trim().toLowerCase()), 250)
})

function toggleShortcutPort(port: number) {
  query.value = query.value === String(port) ? '' : String(port)
}

const counts = computed(() => {
  const c: Record<ProcessType, number> = {
    all: 0, node: 0, java: 0, python: 0, go: 0, csharp: 0, ai: 0, other: 0,
  }
  for (const p of ports.value) {
    c.all++
    c[p._type]++
  }
  return c
})

const stateOptions = computed(() => {
  const s = new Set<string>()
  for (const p of ports.value) if (p.protocol === 'TCP' && p.state) s.add(p.state)
  return ['ALL', ...Array.from(s).sort()]
})

const filtered = computed(() => {
  const q = debounced.value
  return ports.value.filter((p) => {
    if (typeFilter.value !== 'all' && p._type !== typeFilter.value) return false
    if (proto.value !== 'ALL' && p.protocol !== proto.value) return false
    if (statef.value !== 'ALL' && p.state !== statef.value) return false
    if (!q) return true
    return [
      p.protocol, p.localAddr, String(p.localPort), p.remoteAddr,
      String(p.remotePort || ''), p.state, String(p.pid), p.process, p.path,
    ].some((x) => x.toLowerCase().includes(q))
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const displayed = computed(() => {
  const s = (page.value - 1) * PAGE_SIZE
  return filtered.value.slice(s, s + PAGE_SIZE)
})
watch(filtered, () => (page.value = 1))

function endpoint(addr: string, port: number) {
  if (!addr && !port) return '—'
  if (!port) return addr || '—'
  return `${addr}:${port}`
}
function canKill(p: PortEntry) {
  return p.pid !== 0 && p.pid !== 4
}

async function refresh() {
  if (loading.value) return
  loading.value = true
  try {
    const r = await PortService.ListPorts()
    ports.value = r.ports.map((p, i) => ({ ...p, _id: i, _type: classifyProcess(p.process, p.path) }))
    stats.value = r
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    loading.value = false
  }
}

async function kill(p: PortRow) {
  if (!canKill(p) || killing.value !== null) return
  const ans = await Dialogs.Question({
    Title: '确认结束进程',
    Message: `端口 ${p.localPort} 当前由 ${p.process || `PID ${p.pid}`} 占用。\n结束的是进程，不是单个端口。确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })
  if (ans !== 'Yes') return
  killing.value = p.pid
  try {
    const r = await PortService.KillProcess(p.pid)
    toast(r.message || `已结束 PID ${p.pid}`, 'success')
    await refresh()
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    killing.value = null
  }
}

onMounted(refresh)
onBeforeUnmount(() => {
  if (t) clearTimeout(t)
})
</script>

<template>
  <div class="tab">
    <div class="stats">
      <div class="stat acrylic-card"><span class="lbl">TCP</span><b class="mono">{{ formatNumber(stats.tcpCount) }}</b></div>
      <div class="stat acrylic-card"><span class="lbl">UDP</span><b class="mono">{{ formatNumber(stats.udpCount) }}</b></div>
      <div class="stat acrylic-card"><span class="lbl">监听</span><b class="mono">{{ formatNumber(stats.listeningCount) }}</b></div>
      <div class="stat acrylic-card"><span class="lbl">进程</span><b class="mono">{{ formatNumber(stats.processCount) }}</b></div>
    </div>

    <div class="types">
      <button
        v-for="tp in PROCESS_TYPE_LABELS"
        :key="tp.value"
        class="chip"
        :class="{ active: typeFilter === tp.value }"
        @click="typeFilter = tp.value"
      >
        {{ tp.label }}<span class="cnt">{{ formatNumber(counts[tp.value]) }}</span>
      </button>
    </div>

    <div class="common">
      <span class="lbl">常用</span>
      <button
        v-for="port in COMMON_PORTS"
        :key="port"
        class="port-chip mono"
        :class="{ active: query === String(port) }"
        @click="toggleShortcutPort(port)"
      >{{ port }}</button>
    </div>

    <div class="toolbar">
      <input v-model="query" type="search" placeholder="搜索端口、进程名、PID、路径…" />
      <select v-model="proto">
        <option value="ALL">全部协议</option>
        <option value="TCP">TCP</option>
        <option value="UDP">UDP</option>
      </select>
      <select v-model="statef">
        <option v-for="s in stateOptions" :key="s" :value="s">{{ s === 'ALL' ? '全部状态' : s }}</option>
      </select>
      <button class="refresh" :disabled="loading" @click="refresh">
        <span :class="{ spinning: loading }">⟳</span> 刷新
      </button>
    </div>

    <div class="table">
      <div class="thead row">
        <div class="c-proto">协议</div>
        <div class="c-port">端口</div>
        <div class="c-addr">本地地址</div>
        <div class="c-remote">远程地址</div>
        <div class="c-state">状态</div>
        <div class="c-proc">进程</div>
        <div class="c-pid">PID</div>
        <div class="c-act">操作</div>
      </div>
      <div class="tbody scroll">
        <div v-if="!loading && displayed.length === 0" class="empty">没有匹配的端口</div>
        <div v-for="p in displayed" :key="p._id" class="row body">
          <div class="c-proto"><span class="badge" :class="p.protocol.toLowerCase()">{{ p.protocol }}</span></div>
          <div class="c-port mono">{{ p.localPort }}</div>
          <div class="c-addr mono">{{ endpoint(p.localAddr, p.localPort) }}</div>
          <div class="c-remote mono">{{ p.protocol === 'UDP' ? '—' : endpoint(p.remoteAddr, p.remotePort) }}</div>
          <div class="c-state"><span class="state" :class="{ listening: p.state === 'LISTENING' }">{{ p.state }}</span></div>
          <div class="c-proc">
            <div class="proc-name">{{ p.process || `PID ${p.pid}` }}</div>
            <div class="proc-path" :title="p.path">{{ p.path }}</div>
          </div>
          <div class="c-pid mono">{{ p.pid }}</div>
          <div class="c-act">
            <button
              v-if="canKill(p)"
              class="kill"
              :disabled="killing !== null"
              @click="kill(p)"
            >{{ killing === p.pid ? '…' : '结束' }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="pager">
      <button :disabled="page <= 1" @click="page--">上一页</button>
      <span>第 <b>{{ page }}</b> / {{ totalPages }} 页 · 共 {{ formatNumber(filtered.length) }} 条 <template v-if="lastUpdated">· {{ lastUpdated }}</template></span>
      <button :disabled="page >= totalPages" @click="page++">下一页</button>
    </div>
  </div>
</template>

<style scoped>
.tab { display: flex; flex-direction: column; gap: 10px; height: 100%; }
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }
.stat { border-radius: var(--radius-md); padding: 10px 12px; display: flex; align-items: center; justify-content: space-between; }
.lbl { font-size: 11px; color: var(--text-3); }
.stat b { font-size: 20px; color: var(--text-1); }
.types, .common { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.types .lbl, .common .lbl { font-size: 11px; color: var(--text-3); margin-right: 4px; }
.chip { padding: 4px 10px; border-radius: 99px; background: var(--field-bg); border: 1px solid var(--hairline); color: var(--text-3); font-size: 12px; }
.chip.active { color: var(--brand); border-color: var(--brand-glow); background: var(--brand-glow); }
.cnt { margin-left: 5px; font-size: 10px; opacity: 0.7; }
.port-chip { padding: 3px 9px; border-radius: var(--radius-sm); background: var(--field-bg); border: 1px solid var(--hairline); color: var(--text-3); font-size: 12px; }
.port-chip.active { color: var(--brand); border-color: var(--brand-glow); background: var(--brand-glow); }
.toolbar { display: grid; grid-template-columns: 1fr 130px 150px 90px; gap: 8px; }
.toolbar input, .toolbar select { padding: 7px 10px; background: var(--field-bg); border: 1px solid var(--hairline); border-radius: var(--radius-sm); color: var(--text-1); font-size: 12px; outline: none; }
.toolbar input:focus, .toolbar select:focus { border-color: var(--brand); }
.refresh { background: var(--brand-glow); border: 1px solid var(--brand-glow); color: var(--brand); border-radius: var(--radius-sm); font-size: 12px; }
.refresh:disabled { opacity: 0.5; }
.spinning { display: inline-block; animation: spin 0.9s linear infinite; }
.table { flex: 1; display: flex; flex-direction: column; border: 1px solid var(--hairline); border-radius: var(--radius-lg); background: var(--panel-bg); overflow: hidden; min-height: 0; }
.row { display: grid; grid-template-columns: 70px 80px 1.6fr 1.4fr 110px 1.8fr 70px 70px; align-items: center; }
.thead { padding: 8px 12px; background: var(--header-bg); border-bottom: 1px solid var(--hairline); font-size: 11px; font-weight: 600; color: var(--text-3); }
.c-pid, .c-act, .c-port { text-align: right; padding-right: 6px; }
.c-addr, .c-remote, .c-proc { overflow: hidden; }
.tbody { flex: 1; overflow-y: auto; min-height: 0; }
.row.body { padding: 8px 12px; border-bottom: 1px solid var(--hairline); font-size: 12px; }
.row.body:hover { background: var(--row-hover); }
.badge { padding: 1px 6px; border-radius: var(--radius-sm); font-size: 10px; font-weight: 700; }
.badge.tcp { color: var(--blue); background: rgba(59,130,246,0.12); }
.badge.udp { color: var(--purple); background: rgba(139,92,246,0.14); }
.c-port { font-weight: 700; color: var(--text-1); }
.state { padding: 1px 7px; border-radius: 99px; font-size: 10px; color: var(--text-3); background: var(--field-bg); }
.state.listening { color: var(--emerald); background: rgba(16,185,129,0.14); }
.proc-name { color: var(--text-1); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.proc-path { font-size: 10px; color: var(--text-4); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kill { padding: 3px 10px; background: var(--red); color: #fff; border: none; border-radius: 99px; font-size: 11px; font-weight: 600; }
.kill:disabled { opacity: 0.5; }
.empty { padding: 40px; text-align: center; color: var(--text-4); font-size: 12px; }
.pager { display: flex; align-items: center; justify-content: center; gap: 14px; font-size: 12px; color: var(--text-3); }
.pager button { padding: 5px 12px; background: var(--field-bg); border: 1px solid var(--hairline); border-radius: var(--radius-sm); color: var(--text-2); }
.pager button:disabled { opacity: 0.4; }
.pager b { color: var(--brand); }
</style>
