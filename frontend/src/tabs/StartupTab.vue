<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { StartupService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { StartupEntry } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { useToast } from '../composables/useToast'
import { formatNumber } from '../lib/format'

const { toast } = useToast()
const items = ref<StartupEntry[]>([])
const loading = ref(false)
const query = ref('')

async function load() {
  loading.value = true
  try {
    items.value = await StartupService.ListStartup()
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    loading.value = false
  }
}

const filtered = () => {
  const q = query.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter(
    (s) => s.name.toLowerCase().includes(q) || s.command.toLowerCase().includes(q) || s.location.toLowerCase().includes(q),
  )
}

const locTag = (loc: string) => {
  if (loc === 'HKCU') return '当前用户'
  if (loc === 'HKLM') return '所有用户'
  return '启动文件夹'
}

onMounted(load)
</script>

<template>
  <div class="tab">
    <div class="head">
      <input v-model="query" type="search" placeholder="搜索启动项名称、命令、位置…" />
      <button class="refresh" :disabled="loading" @click="load"><span :class="{ spinning: loading }">⟳</span> 刷新</button>
    </div>
    <div class="table">
      <div class="row head">
        <div class="c-name">名称</div>
        <div class="c-cmd">命令 / 路径</div>
        <div class="c-loc">来源</div>
      </div>
      <div class="tbody scroll">
        <div v-if="!loading && filtered().length === 0" class="empty">没有启动项</div>
        <div v-for="(s, i) in filtered()" :key="s.name + i" class="row body">
          <div class="c-name">{{ s.name }}</div>
          <div class="c-cmd" :title="s.command">{{ s.command }}</div>
          <div class="c-loc"><span class="tag">{{ locTag(s.location) }}</span></div>
        </div>
      </div>
    </div>
    <div class="foot">共 {{ formatNumber(items.length) }} 项（只读视图）</div>
  </div>
</template>

<style scoped>
.tab { display: flex; flex-direction: column; gap: 10px; height: 100%; }
.head { display: flex; gap: 8px; }
.head input { flex: 1; padding: 7px 10px; background: var(--field-bg); border: 1px solid var(--hairline); border-radius: var(--radius-sm); color: var(--text-1); font-size: 12px; outline: none; }
.head input:focus { border-color: var(--brand); }
.refresh { padding: 7px 14px; background: var(--brand-glow); border: 1px solid var(--brand-glow); color: var(--brand); border-radius: var(--radius-sm); font-size: 12px; }
.refresh:disabled { opacity: 0.5; }
.spinning { display: inline-block; animation: spin 0.9s linear infinite; }
.table { flex: 1; display: flex; flex-direction: column; border: 1px solid var(--hairline); border-radius: var(--radius-lg); background: var(--panel-bg); overflow: hidden; min-height: 0; }
.row { display: grid; grid-template-columns: 1fr 2.2fr 110px; align-items: center; }
.row.head { padding: 8px 12px; background: var(--header-bg); border-bottom: 1px solid var(--hairline); font-size: 11px; font-weight: 600; color: var(--text-3); }
.c-name, .c-cmd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding-right: 8px; }
.tbody { flex: 1; overflow-y: auto; min-height: 0; }
.row.body { padding: 9px 12px; border-bottom: 1px solid var(--hairline); font-size: 12px; }
.row.body:hover { background: var(--row-hover); }
.c-name { color: var(--text-1); }
.c-cmd { color: var(--text-3); font-size: 11px; font-family: "Cascadia Code", "Consolas", monospace; }
.tag { padding: 2px 8px; border-radius: 99px; font-size: 10px; color: var(--brand); background: var(--brand-glow); }
.empty { padding: 40px; text-align: center; color: var(--text-4); font-size: 12px; }
.foot { font-size: 11px; color: var(--text-3); }
</style>
