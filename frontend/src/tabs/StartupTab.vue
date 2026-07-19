<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { StartupService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { StartupEntry } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models.js'
import { useToast } from '../composables/useToast'
import { formatNumber } from '../lib/format'

const { toast } = useToast()
const items = ref<StartupEntry[]>([])
const loading = ref(false)
const query = ref('')
const acting = ref<string | null>(null)

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

function actKey(s: StartupEntry) {
  return `${s.location}::${s.name}`
}

async function toggleDisable(s: StartupEntry) {
  if (acting.value !== null) return
  const nextDisable = !s.disabled
  const ans = await Dialogs.Question({
    Title: nextDisable ? '确认禁用启动项' : '确认启用启动项',
    Message: nextDisable
      ? `将禁用「${s.name}」。\n禁用可恢复，下次开机不再自动启动。确认继续吗？`
      : `将启用「${s.name}」。\n下次开机将自动启动。确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })
  if (ans !== 'Yes') return
  acting.value = actKey(s)
  try {
    const r = nextDisable
      ? await StartupService.DisableStartup(s.name, s.location)
      : await StartupService.EnableStartup(s.name, s.location)
    toast(r.message || (nextDisable ? `已禁用：${s.name}` : `已启用：${s.name}`), 'success')
    await load()
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    acting.value = null
  }
}

async function remove(s: StartupEntry) {
  if (acting.value !== null) return
  const ans = await Dialogs.Question({
    Title: '确认删除启动项',
    Message: `将彻底删除「${s.name}」（来源：${locTag(s.location)}）。\n删除不可恢复，若只是不想开机启动请改用「禁用」。确认继续吗？`,
    Buttons: [
      { Label: 'No', IsCancel: true },
      { Label: 'Yes', IsDefault: true },
    ],
  })
  if (ans !== 'Yes') return
  acting.value = actKey(s)
  try {
    const r = await StartupService.DeleteStartup(s.name, s.location)
    toast(r.message || `已删除：${s.name}`, 'success')
    await load()
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    acting.value = null
  }
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
        <div class="c-state">状态</div>
        <div class="c-act">操作</div>
      </div>
      <div class="tbody scroll">
        <div v-if="!loading && filtered().length === 0" class="empty">没有启动项</div>
        <div v-for="(s, i) in filtered()" :key="s.name + i" class="row body" :class="{ disabled: s.disabled }">
          <div class="c-name">
            <img class="app-ico" :src="s.iconDataUrl || '/appicon.png'" alt="" />
            <span class="name-text">{{ s.name }}</span>
          </div>
          <div class="c-cmd" :title="s.command">{{ s.command }}</div>
          <div class="c-loc"><span class="tag">{{ locTag(s.location) }}</span></div>
          <div class="c-state">
            <span class="state" :class="s.disabled ? 'off' : 'on'">{{ s.disabled ? '已禁用' : '已启用' }}</span>
          </div>
          <div class="c-act">
            <button
              class="act toggle"
              :disabled="acting !== null"
              @click="toggleDisable(s)"
            >{{ acting === actKey(s) ? '…' : (s.disabled ? '启用' : '禁用') }}</button>
            <button
              class="act del"
              :disabled="acting !== null"
              @click="remove(s)"
            >删除</button>
          </div>
        </div>
      </div>
    </div>
    <div class="foot">共 {{ formatNumber(items.length) }} 项</div>
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
.row { display: grid; grid-template-columns: 1.2fr 1.8fr 100px 80px 130px; align-items: center; }
.row.head { padding: 8px 12px; background: var(--header-bg); border-bottom: 1px solid var(--hairline); font-size: 11px; font-weight: 600; color: var(--text-3); }
.c-name, .c-cmd { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding-right: 8px; }
.c-name { display: flex; align-items: center; gap: 8px; min-width: 0; }
.app-ico { width: 18px; height: 18px; border-radius: 4px; flex-shrink: 0; object-fit: contain; background: var(--field-bg); }
.app-ico.placeholder { display: inline-block; background: linear-gradient(135deg, var(--field-bg), var(--hairline)); border: 1px solid var(--hairline); }
.name-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
.tbody { flex: 1; overflow-y: auto; min-height: 0; }
.row.body { padding: 9px 12px; border-bottom: 1px solid var(--hairline); font-size: 12px; }
.row.body:hover { background: var(--row-hover); }
.row.body.disabled { opacity: 0.72; }
.c-name { color: var(--text-1); }
.c-cmd { color: var(--text-3); font-size: 11px; font-family: "Cascadia Code", "Consolas", monospace; }
.c-act { display: flex; gap: 6px; justify-content: flex-end; }
.tag { padding: 2px 8px; border-radius: 99px; font-size: 10px; color: var(--brand); background: var(--brand-glow); }
.state { padding: 1px 8px; border-radius: 99px; font-size: 10px; }
.state.on { color: var(--emerald); background: rgba(16,185,129,0.14); }
.state.off { color: var(--text-4); background: var(--field-bg); }
.act { padding: 3px 10px; border: none; border-radius: 99px; font-size: 11px; font-weight: 600; color: #fff; }
.act.toggle { background: var(--brand); }
.act.del { background: var(--red); }
.act:disabled { opacity: 0.5; }
.empty { padding: 40px; text-align: center; color: var(--text-4); font-size: 12px; }
.foot { font-size: 11px; color: var(--text-3); }
</style>
