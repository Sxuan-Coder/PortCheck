<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { PortService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { useMonitor } from '../composables/useMonitor'
import { useVirtualList } from '../composables/useVirtualList'
import { useToast } from '../composables/useToast'
import { classifyProcess, PROCESS_TYPE_LABELS, type ProcessType } from '../lib/classify'
import { formatBytes, formatNumber, formatPercent } from '../lib/format'
import type { ProcessInfo } from '../lib/types'

const { state } = useMonitor()
const { toast } = useToast()

const ROW_H = 44
const query = ref('')
const debounced = ref('')
const sortBy = ref<'cpu' | 'mem'>('cpu')
const typeFilter = ref<ProcessType>('all')
const killing = ref<number | null>(null)

let t: ReturnType<typeof setTimeout> | null = null
watch(query, (v) => {
  if (t) clearTimeout(t)
  t = setTimeout(() => (debounced.value = v.trim().toLowerCase()), 250)
})

// 过滤 + 排序：在 markRaw 数组上派生新数组（不深响应）
const filtered = computed<ProcessInfo[]>(() => {
  const q = debounced.value
  const list = state.processes.filter((p) => {
    if (typeFilter.value !== 'all') {
      if (classifyProcess(p.name, p.path) !== typeFilter.value) return false
    }
    if (!q) return true
    return (
      p.name.toLowerCase().includes(q) ||
      String(p.pid).includes(q) ||
      p.path.toLowerCase().includes(q)
    )
  })
  const sorted = [...list]
  sorted.sort((a, b) => (sortBy.value === 'cpu' ? b.cpu - a.cpu : b.memBytes - a.memBytes))
  return sorted
})

const counts = computed(() => {
  const c: Record<ProcessType, number> = {
    all: 0, node: 0, java: 0, python: 0, go: 0, csharp: 0, ai: 0, other: 0,
  }
  for (const p of state.processes) {
    c.all++
    c[classifyProcess(p.name, p.path)]++
  }
  return c
})

const source = computed(() => filtered.value)
const { slice, padTop, totalHeight, onScroll, viewport } = useVirtualList(source, ROW_H)

const bodyEl = ref<HTMLDivElement | null>(null)
function measure() {
  if (bodyEl.value) viewport.value = bodyEl.value.clientHeight
}
let ro: ResizeObserver | null = null
onMounted(() => {
  measure()
  ro = new ResizeObserver(measure)
  if (bodyEl.value) ro.observe(bodyEl.value)
})
onBeforeUnmount(() => ro?.disconnect())

function isProtected(pid: number) {
  return pid === 0 || pid === 4
}

async function kill(p: ProcessInfo) {
  if (isProtected(p.pid) || killing.value !== null) return
  const ans = await Dialogs.Question({
    Title: '确认结束进程',
    Message: `将结束 ${p.name || `PID ${p.pid}`}（PID ${p.pid}）。\n结束进程属于危险操作，确认继续吗？`,
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
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    killing.value = null
  }
}

function cpuColor(v: number) {
  return v > 50 ? 'var(--red)' : 'var(--brand-light)'
}
// p.cpu 为单核基准原始值；整机基准 = 单核 ÷ 核心数（钳 100）。
function wholeCpu(p: ProcessInfo): number {
  const cores = Math.max(1, state.perf.numCores || 1)
  return Math.min(100, p.cpu / cores)
}
function singleColor(cpu: number) {
  return cpu >= 90 ? 'var(--red)' : 'var(--text-2)'
}
</script>

<template>
  <div class="tab">
    <div class="toolbar">
      <div class="search">
        <span class="ico"><slot name="search" /></span>
        <input v-model="query" type="search" placeholder="搜索进程名、PID、路径…" />
      </div>
      <div class="sort">
        <span class="label">排序</span>
        <button :class="{ on: sortBy === 'cpu' }" @click="sortBy = 'cpu'">CPU</button>
        <button :class="{ on: sortBy === 'mem' }" @click="sortBy = 'mem'">内存</button>
      </div>
    </div>

    <div class="types">
      <button
        v-for="t in PROCESS_TYPE_LABELS"
        :key="t.value"
        class="chip"
        :class="{ active: typeFilter === t.value }"
        @click="typeFilter = t.value"
      >
        {{ t.label }}<span class="cnt">{{ formatNumber(counts[t.value]) }}</span>
      </button>
    </div>

    <div class="table">
      <div class="thead">
        <div class="row head">
          <div class="c-name">名称</div>
          <div class="c-pid">PID</div>
          <div class="c-cpu">CPU 整机</div>
          <div class="c-cpu2">CPU 单核</div>
          <div class="c-mem">内存</div>
          <div class="c-act">操作</div>
        </div>
      </div>
      <div ref="bodyEl" class="tbody scroll" @scroll.passive="onScroll">
        <div :style="{ height: totalHeight + 'px', position: 'relative' }">
          <div :style="{ height: padTop + 'px' }" />
          <div
            v-for="p in slice"
            :key="p.pid"
            class="row body"
          >
            <div class="c-name">
              <img class="app-ico" :src="p.iconDataUrl || '/appicon.png'" alt="" />
              <div class="name-wrap">
                <span class="name">{{ p.name }}</span>
                <span class="path" :title="p.path">{{ p.path || '—' }}</span>
              </div>
            </div>
            <div class="c-pid mono">{{ p.pid }}</div>
            <div class="c-cpu mono" :style="{ color: cpuColor(wholeCpu(p)) }">
              {{ formatPercent(wholeCpu(p)) }}
            </div>
            <div class="c-cpu2 mono" :style="{ color: singleColor(p.cpu) }">
              {{ formatPercent(p.cpu) }}
            </div>
            <div class="c-mem mono">{{ formatBytes(p.memBytes) }}</div>
            <div class="c-act">
              <button
                v-if="!isProtected(p.pid)"
                class="kill"
                :disabled="killing !== null"
                @click="kill(p)"
              >
                {{ killing === p.pid ? '处理中' : '结束' }}
              </button>
              <span v-else class="locked">系统</span>
            </div>
          </div>
          <div v-if="filtered.length === 0" class="empty">没有匹配的进程</div>
        </div>
      </div>
    </div>

    <div class="foot">共 {{ formatNumber(state.processes.length) }} 个进程 · 显示 {{ formatNumber(filtered.length) }}</div>
  </div>
</template>

<style scoped>
.tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 10px;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.search {
  position: relative;
  flex: 1;
  max-width: 320px;
}
.search input {
  width: 100%;
  padding: 7px 10px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--text-1);
  font-size: 12px;
  outline: none;
}
.search input:focus {
  border-color: var(--brand);
  background: var(--field-bg-focus);
}
.sort {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}
.sort .label {
  color: var(--text-3);
}
.sort button {
  padding: 5px 10px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--text-3);
}
.sort button.on {
  color: var(--brand);
  border-color: var(--brand-glow);
  background: var(--brand-glow);
}
.types {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.chip {
  padding: 4px 10px;
  border-radius: 99px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  color: var(--text-3);
  font-size: 12px;
}
.chip.active {
  color: var(--brand);
  border-color: var(--brand-glow);
  background: var(--brand-glow);
}
.cnt {
  margin-left: 5px;
  font-size: 10px;
  opacity: 0.7;
}
.table {
  flex: 1;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--hairline);
  border-radius: var(--radius-lg);
  background: var(--panel-bg);
  overflow: hidden;
  min-height: 0;
}
.thead {
  flex-shrink: 0;
}
.row {
  display: grid;
  grid-template-columns: 1fr 70px 84px 84px 104px 80px;
  align-items: center;
}
.row.head {
  padding: 8px 12px;
  background: var(--header-bg);
  border-bottom: 1px solid var(--hairline);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-3);
}
.c-pid,
.c-cpu,
.c-cpu2,
.c-mem,
.c-act {
  text-align: right;
  padding-right: 4px;
}
.tbody {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}
.row.body {
  height: 44px;
  padding: 0 12px;
  border-bottom: 1px solid var(--hairline);
  font-size: 12px;
}
.row.body:hover {
  background: var(--row-hover);
}
.c-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.app-ico {
  width: 18px;
  height: 18px;
  border-radius: 4px;
  flex-shrink: 0;
  object-fit: contain;
  background: var(--field-bg);
}
.app-ico.placeholder {
  display: inline-block;
  background: linear-gradient(135deg, var(--field-bg), var(--hairline));
  border: 1px solid var(--hairline);
}
.name-wrap {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.name {
  color: var(--text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path {
  font-size: 10px;
  color: var(--text-4);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kill {
  padding: 4px 12px;
  background: var(--red);
  color: #fff;
  border: none;
  border-radius: 99px;
  font-size: 11px;
  font-weight: 600;
  opacity: 0;
  transition: opacity 0.15s;
}
.row.body:hover .kill {
  opacity: 1;
}
.kill:disabled {
  opacity: 0.5;
}
.locked {
  color: var(--text-4);
  font-size: 11px;
}
.empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-4);
  font-size: 12px;
}
.foot {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--text-3);
  padding: 2px 2px 0;
}
</style>
