<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { SetInterval } from '../../../bindings/github.com/Sxuan-Coder/PortCheck/monitorservice'
import { useSettings } from '../../composables/useSettings'
import { useTheme } from '../../composables/useTheme'

const { theme, toggle } = useTheme()
const { settings, load, save, setAutostart } = useSettings()

const autostartEnabled = ref(false)

onMounted(() => {
  load()
})

// 开关双向绑定辅助
const themeChecked = computed({
  get: () => settings.value.theme === 'light',
  set: (v: boolean) => {
    settings.value.theme = v ? 'light' : 'dark'
    toggle() // 同步切换
    save()
  },
})

const intervalOptions = [
  { value: 500, label: '0.5s' },
  { value: 1000, label: '1s' },
  { value: 2000, label: '2s' },
  { value: 5000, label: '5s' },
]

async function onIntervalChange(newVal: number) {
  settings.value.refreshIntervalMs = newVal
  await save()
  SetInterval(newVal)
}

async function onAutostartChange(v: boolean) {
  autostartEnabled.value = v
  await setAutostart(v)
}
</script>

<template>
  <div class="settings-list">
    <!-- 主题模式 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">主题模式</span>
        <span class="setting-desc">选择界面外观风格</span>
      </div>
      <label class="switch" :class="{ on: themeChecked }">
        <input type="checkbox" v-model="themeChecked" class="switch-input" />
        <span class="switch-track">
          <span class="switch-thumb" />
        </span>
        <span class="switch-text">{{ themeChecked ? '亮色' : '暗色' }}</span>
      </label>
    </div>

    <!-- 开机自启 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">开机自启</span>
        <span class="setting-desc">开机时自动启动 PortCheck</span>
      </div>
      <label class="switch" :class="{ on: autostartEnabled }">
        <input
          type="checkbox"
          class="switch-input"
          v-model="autostartEnabled"
          @change="onAutostartChange(autostartEnabled)"
        />
        <span class="switch-track">
          <span class="switch-thumb" />
        </span>
        <span class="switch-text">{{ autostartEnabled ? '开' : '关' }}</span>
      </label>
    </div>

    <!-- 进程刷新间隔 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">进程刷新间隔</span>
        <span class="setting-desc">影响进程列表、CPU 曲线的更新频率</span>
      </div>
      <select
        class="setting-select"
        :value="settings.refreshIntervalMs"
        @change="onIntervalChange(Number(($event.target as HTMLSelectElement).value))"
      >
        <option v-for="opt in intervalOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </select>
    </div>

    <!-- 语言 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">语言</span>
        <span class="setting-desc">界面显示语言</span>
      </div>
      <select class="setting-select" disabled>
        <option value="zh-CN">简体中文</option>
      </select>
    </div>
  </div>
</template>

<style scoped>
.settings-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  transition: background 0.15s;
}
.setting-row:hover {
  background: var(--row-hover);
}
.setting-row + .setting-row {
  border-top: 1px solid var(--hairline);
}
.setting-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.setting-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-1);
}
.setting-desc {
  font-size: 11px;
  color: var(--text-3);
}

/* Switch */
.switch {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}
.switch-input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.switch-track {
  position: relative;
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: var(--hairline-strong);
  transition: background 0.2s;
}
.switch.on .switch-track {
  background: var(--brand);
}
.switch-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0,0,0,0.2);
}
.switch.on .switch-thumb {
  transform: translateX(16px);
}
.switch-text {
  font-size: 12px;
  color: var(--text-3);
  min-width: 2em;
}
.switch.on .switch-text {
  color: var(--text-1);
}

/* Select */
.setting-select {
  padding: 5px 28px 5px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--hairline);
  background: var(--field-bg);
  color: var(--text-1);
  font-size: 12px;
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg width='10' height='6' viewBox='0 0 10 6' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M1 1l4 4 4-4' stroke='%239ca3af' stroke-width='1.5' fill='none'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 8px center;
  min-width: 100px;
  text-align: left;
}
.setting-select:focus {
  outline: none;
  border-color: var(--brand);
}
.setting-select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
