<script setup lang="ts">
import { ref } from 'vue'
import { Dialogs } from '@wailsio/runtime'
import { PortService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import AppIcon from './AppIcon.vue'
import { useToast } from '../composables/useToast'
import { useTheme } from '../composables/useTheme'

const { toast } = useToast()
const { toggle } = useTheme()

const open = ref(false)
const value = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

function togglePanel() {
  open.value = !open.value
  if (open.value) setTimeout(() => inputEl.value?.focus(), 60)
}

async function execute() {
  const v = value.value.trim().toLowerCase()
  if (!v) return
  value.value = ''

  if (v.startsWith('kill ')) {
    const pid = parseInt(v.slice(5), 10)
    if (!pid || pid <= 4) {
      toast('无效的 PID（受保护的系统进程不可结束）', 'error')
      return
    }
    const ans = await Dialogs.Question({
      Title: '确认结束进程',
      Message: `速启指令请求结束 PID ${pid}。\n结束进程属于危险操作，确认继续吗？`,
      Buttons: [
        { Label: 'No', IsCancel: true },
        { Label: 'Yes', IsDefault: true },
      ],
    })
    if (ans !== 'Yes') return
    try {
      const r = await PortService.KillProcess(pid)
      toast(r.message || `已结束 PID ${pid}`, 'success')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
    return
  }

  if (v === 'theme') {
    toggle()
    toast('已切换主题', 'success')
    return
  }
  if (v === 'help') {
    toast('可用指令：kill <PID> · theme · help', 'info')
    return
  }
  toast(`未知指令: ${v}（输入 help 查看）`, 'error')
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Enter') execute()
  else if (e.key === 'Escape') (open.value = false)
}
</script>

<template>
  <div class="qc-wrap">
    <div v-if="open" class="qc-panel acrylic-blur acrylic-card">
      <div class="qc-head">
        <span class="qc-title">⚡ 快速指令</span>
        <button class="qc-x" @click="open = false"><AppIcon name="close" :size="11" /></button>
      </div>
      <p class="qc-hint">支持 kill &lt;PID&gt;、theme、help</p>
      <div class="qc-input-wrap">
        <input
          ref="inputEl"
          v-model="value"
          class="qc-input"
          placeholder="输入指令，回车执行…"
          @keydown="onKey"
        />
        <AppIcon name="terminal" :size="11" class="qc-input-ico" />
      </div>
    </div>

    <button class="qc-fab" title="速启终端" @click="togglePanel">
      <span class="pulse" />
      <AppIcon name="terminal" :size="18" />
    </button>
  </div>
</template>

<style scoped>
.qc-wrap {
  position: absolute;
  bottom: 16px;
  right: 16px;
  z-index: 60;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 12px;
}
.qc-panel {
  width: 280px;
  padding: 13px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--brand-glow);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
}
.qc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.qc-title {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--brand);
  text-transform: uppercase;
}
.qc-x {
  background: transparent;
  border: none;
  color: var(--text-3);
}
.qc-x:hover {
  color: var(--text-1);
}
.qc-hint {
  margin: 0 0 8px;
  font-size: 10px;
  color: var(--text-3);
}
.qc-input-wrap {
  position: relative;
}
.qc-input {
  width: 100%;
  padding: 7px 28px 7px 10px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--brand-light);
  font-size: 12px;
  outline: none;
}
.qc-input:focus {
  border-color: var(--brand);
}
.qc-input-ico {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--brand);
  opacity: 0.6;
}
.qc-fab {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: var(--brand-glow);
  border: 1px solid var(--brand-glow);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.3);
  transition: transform 0.2s;
}
.qc-fab:hover {
  transform: scale(1.08);
}
.pulse {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  border: 1px solid var(--brand-glow);
  opacity: 0.6;
}
.qc-fab:hover .pulse {
  animation: spin 2s linear infinite;
  transform: scale(1.3);
  transition: transform 0.5s;
}
</style>
