<script setup lang="ts">
import { Window } from '@wailsio/runtime'
import AppIcon from './AppIcon.vue'
import { useTheme } from '../composables/useTheme'

const { theme, toggle } = useTheme()

async function onMinimise() {
  try {
    await Window.Minimise()
  } catch {
    /* ignore */
  }
}
async function onMaximise() {
  try {
    await Window.ToggleMaximise()
  } catch {
    /* ignore */
  }
}
async function onClose() {
  // 后端 RegisterHook 拦截为最小化到托盘
  try {
    await Window.Close()
  } catch {
    /* ignore */
  }
}
</script>

<template>
  <header class="titlebar" style="--wails-draggable: drag">
    <div class="brand">
      <span class="bolt"><AppIcon name="terminal" :size="14" /></span>
      <span class="name">PortCheck</span>
      <span class="badge">REALTIME</span>
    </div>

    <div class="controls">
      <button
        class="ctl"
        style="--wails-draggable: no-drag"
        :title="theme === 'dark' ? '切换到亮色' : '切换到暗色'"
        @click="toggle"
      >
        <AppIcon :name="theme === 'dark' ? 'moon' : 'sun'" :size="13" />
      </button>
      <button class="ctl" style="--wails-draggable: no-drag" title="最小化" @click="onMinimise">
        <AppIcon name="minimize" :size="14" />
      </button>
      <button class="ctl" style="--wails-draggable: no-drag" title="最大化/还原" @click="onMaximise">
        <AppIcon name="maximize" :size="12" />
      </button>
      <button class="ctl close" style="--wails-draggable: no-drag" title="最小化到托盘" @click="onClose">
        <AppIcon name="close" :size="14" />
      </button>
    </div>
  </header>
</template>

<style scoped>
.titlebar {
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px 0 14px;
  border-bottom: 1px solid var(--hairline);
  background: rgba(0, 0, 0, 0.18);
}
.brand {
  display: flex;
  align-items: center;
  gap: 8px;
}
.bolt {
  width: 22px;
  height: 22px;
  border-radius: var(--radius-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: linear-gradient(135deg, var(--brand-glow), transparent);
  border: 1px solid var(--brand-glow);
}
.name {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--text-2);
  text-transform: uppercase;
}
.badge {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--brand);
  background: var(--brand-glow);
  border: 1px solid var(--brand-glow);
  padding: 1px 5px;
  border-radius: var(--radius-sm);
}
.controls {
  display: flex;
  align-items: center;
}
.ctl {
  width: 40px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-3);
  transition: background 0.15s, color 0.15s;
}
.ctl:hover {
  background: var(--row-hover);
  color: var(--text-1);
}
.ctl.close:hover {
  background: var(--red);
  color: #fff;
}
</style>
