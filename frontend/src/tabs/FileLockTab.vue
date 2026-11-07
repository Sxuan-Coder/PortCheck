<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Dialogs, Events } from '@wailsio/runtime'
import { FileLockService, PortService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { FileLockEntry, FileLockResult } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { useToast } from '../composables/useToast'
import { formatNumber } from '../lib/format'

const { toast } = useToast()

const path = ref('')
const result = ref<FileLockResult>({ entries: [], scanned: 0, warnings: [] })
const loading = ref(false)
const killing = ref<number | null>(null)
const dragging = ref(false)

async function query(p?: string) {
  const target = (p ?? path.value).trim()
  if (!target) {
    toast('请输入文件或目录路径', 'error')
    return
  }
  path.value = target
  if (loading.value) return
  loading.value = true
  try {
    result.value = await FileLockService.FindLockers(target)
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    loading.value = false
  }
}

async function kill(e: FileLockEntry) {
  if (killing.value !== null) return
  const ans = await Dialogs.Question({
    Title: '确认结束进程',
    Message: `将结束 ${e.process}（PID ${e.pid}），以解除对文件的占用。\n结束进程属于危险操作，确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })
  if (ans !== 'Yes') return
  killing.value = e.pid
  try {
    const r = await PortService.KillProcess(e.pid)
    toast(r.message || `已结束 PID ${e.pid}`, 'success')
    await query()
  } catch (err) {
    toast(err instanceof Error ? err.message : String(err), 'error')
  } finally {
    killing.value = null
  }
}

// 拖放：主窗口已开启 EnableFileDrop，落点元素带 data-file-drop-target 时，
// runtime 会解析完整路径并以 common:WindowFilesDropped 事件回传。
let offDrop: (() => void) | null = null
function onWailsDrop(ev: any) {
  const data = ev?.data ?? ev
  const files: string[] = data?.filenames ?? []
  if (files.length > 0) {
    dragging.value = false
    void query(files[0])
  }
}
onMounted(() => {
  offDrop = Events.On('common:WindowFilesDropped', onWailsDrop) as unknown as () => void
})
onBeforeUnmount(() => offDrop?.())

function formatTime(ms: number) {
  if (!ms) return '—'
  return new Date(ms).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div
    class="tab"
    data-file-drop-target
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="dragging = false"
  >
    <div class="toolbar">
      <input
        v-model="path"
        type="text"
        spellcheck="false"
        placeholder="输入文件或目录路径，或直接拖拽文件/文件夹到窗口…"
        @keyup.enter="query()"
      />
      <button class="query" :disabled="loading" @click="query()">
        <span :class="{ spinning: loading }">⟳</span> 查询占用
      </button>
    </div>

    <div v-if="dragging" class="drop-hint">松开以查询文件占用</div>

    <div v-if="result.warnings.length" class="warnings">
      <div v-for="(w, i) in result.warnings" :key="i" class="warn-item">{{ w }}</div>
    </div>

    <div class="table">
      <div class="tbody scroll">
        <div class="thead row">
          <div class="c-proc">进程</div>
          <div class="c-pid">PID</div>
          <div class="c-type">类型</div>
          <div class="c-path">可执行路径</div>
          <div class="c-time">进程启动时间</div>
          <div class="c-act">操作</div>
        </div>
        <div v-if="!loading && result.entries.length === 0" class="empty">
          <template v-if="path">没有发现占用该路径的进程</template>
          <template v-else>输入路径或拖入文件开始查询</template>
        </div>
        <div v-for="e in result.entries" :key="e.pid" class="row body">
          <div class="c-proc">{{ e.process }}</div>
          <div class="c-pid mono">{{ e.pid }}</div>
          <div class="c-type"><span class="badge">{{ e.appType || '未知类型' }}</span></div>
          <div class="c-path mono" :title="e.exePath">{{ e.exePath || '—' }}</div>
          <div class="c-time mono">{{ formatTime(e.startTime) }}</div>
          <div class="c-act">
            <button
              class="kill"
              :disabled="killing !== null"
              @click="kill(e)"
            >{{ killing === e.pid ? '…' : '结束进程' }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="foot">
      共 {{ formatNumber(result.entries.length) }} 个占用进程
      <template v-if="result.scanned"> · 已扫描 {{ formatNumber(result.scanned) }} 个文件</template>
    </div>
  </div>
</template>

<style scoped>
.tab { display: flex; flex-direction: column; gap: 10px; height: 100%; }
.toolbar { display: grid; grid-template-columns: 1fr 110px; gap: 8px; }
.toolbar input { padding: 7px 10px; background: var(--field-bg); border: 1px solid var(--hairline); border-radius: var(--radius-sm); color: var(--text-1); font-size: 12px; outline: none; }
.toolbar input:focus { border-color: var(--brand); }
.query { background: var(--brand-glow); border: 1px solid var(--brand-glow); color: var(--brand); border-radius: var(--radius-sm); font-size: 12px; }
.query:disabled { opacity: 0.5; }
.spinning { display: inline-block; animation: spin 0.9s linear infinite; }
.drop-hint { padding: 10px; text-align: center; font-size: 12px; color: var(--brand); border: 1px dashed var(--brand-glow); border-radius: var(--radius-md); background: var(--brand-glow); }
.warnings { display: flex; flex-direction: column; gap: 4px; }
.warn-item { font-size: 11px; color: var(--amber); background: var(--field-bg); border: 1px solid var(--hairline); border-radius: var(--radius-sm); padding: 5px 9px; }
.table { flex: 1; display: flex; flex-direction: column; border: 1px solid var(--hairline); border-radius: var(--radius-lg); background: var(--panel-bg); overflow: hidden; min-height: 0; }
.row { display: grid; grid-template-columns: 1.2fr 70px 110px 1.8fr 150px 90px; align-items: center; }
.thead { position: sticky; top: 0; z-index: 1; padding: 8px 12px; background: var(--header-bg); border-bottom: 1px solid var(--hairline); font-size: 11px; font-weight: 600; color: var(--text-3); }
.c-pid, .c-time, .c-act { text-align: right; padding-right: 6px; }
.c-path { overflow: hidden; }
.tbody { flex: 1; overflow-y: auto; min-height: 0; }
.row.body { padding: 8px 12px; border-bottom: 1px solid var(--hairline); font-size: 12px; }
.row.body:hover { background: var(--row-hover); }
.c-proc { color: var(--text-1); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.badge { padding: 1px 7px; border-radius: 99px; font-size: 10px; color: var(--text-3); background: var(--field-bg); }
.c-path { font-size: 10px; color: var(--text-4); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.kill { padding: 3px 10px; background: var(--red); color: #fff; border: none; border-radius: 99px; font-size: 11px; font-weight: 600; }
.kill:disabled { opacity: 0.5; }
.empty { padding: 40px; text-align: center; color: var(--text-4); font-size: 12px; }
.foot { font-size: 11px; color: var(--text-3); }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
