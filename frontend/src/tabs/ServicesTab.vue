<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ServicesService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { ServiceEntry } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { useToast } from '../composables/useToast'
import { formatNumber } from '../lib/format'

const { toast } = useToast()
const services = ref<ServiceEntry[]>([])
const loading = ref(false)
const query = ref('')

async function load() {
  loading.value = true
  try {
    const list = await ServicesService.ListServices()
    services.value = [...list].sort((a, b) => a.name.localeCompare(b.name))
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    loading.value = false
  }
}

const filtered = () => {
  const q = query.value.trim().toLowerCase()
  if (!q) return services.value
  return services.value.filter(
    (s) => s.name.toLowerCase().includes(q) || s.displayName.toLowerCase().includes(q) || s.state.includes(q),
  )
}

onMounted(load)
</script>

<template>
  <div class="tab">
    <div class="head">
      <input v-model="query" type="search" placeholder="搜索服务名、显示名、状态…" />
      <button class="refresh" :disabled="loading" @click="load"><span :class="{ spinning: loading }">⟳</span> 刷新</button>
    </div>
    <div class="table">
      <div class="row head">
        <div class="c-name">服务名称</div>
        <div class="c-desc">显示名称</div>
        <div class="c-state">状态</div>
        <div class="c-type">类型</div>
      </div>
      <div class="tbody scroll">
        <div v-if="!loading && filtered().length === 0" class="empty">没有匹配的服务</div>
        <div v-for="(s, i) in filtered()" :key="s.name + i" class="row body">
          <div class="c-name">{{ s.name }}</div>
          <div class="c-desc">{{ s.displayName || '—' }}</div>
          <div class="c-state">
            <span class="state" :class="s.state === '运行中' ? 'on' : s.state === '已停止' ? 'off' : ''">{{ s.state }}</span>
          </div>
          <div class="c-type">{{ s.startType }}</div>
        </div>
      </div>
    </div>
    <div class="foot">共 {{ formatNumber(services.length) }} 个服务（只读视图）</div>
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
.row { display: grid; grid-template-columns: 1.4fr 1.6fr 110px 100px; align-items: center; }
.row.head { padding: 8px 12px; background: var(--header-bg); border-bottom: 1px solid var(--hairline); font-size: 11px; font-weight: 600; color: var(--text-3); }
.c-name, .c-desc { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding-right: 8px; }
.tbody { flex: 1; overflow-y: auto; min-height: 0; }
.row.body { padding: 8px 12px; border-bottom: 1px solid var(--hairline); font-size: 12px; }
.row.body:hover { background: var(--row-hover); }
.c-name { color: var(--text-1); }
.c-desc { color: var(--text-3); font-size: 11px; }
.c-type { color: var(--text-3); font-size: 11px; }
.state { padding: 1px 8px; border-radius: 99px; font-size: 10px; background: var(--field-bg); color: var(--text-3); }
.state.on { color: var(--emerald); background: rgba(16,185,129,0.14); }
.state.off { color: var(--text-4); }
.empty { padding: 40px; text-align: center; color: var(--text-4); font-size: 12px; }
.foot { font-size: 11px; color: var(--text-3); }
</style>
