<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { Dialogs, Events } from '@wailsio/runtime'
import { SettingsService } from '../../../bindings/github.com/Sxuan-Coder/PortCheck'
import { SetInterval } from '../../../bindings/github.com/Sxuan-Coder/PortCheck/monitorservice'
import { useSettings } from '../../composables/useSettings'
import { useTheme } from '../../composables/useTheme'
import { useToast } from '../../composables/useToast'

const { theme } = useTheme()
const { settings, load, save, setAutostart, setThemeMode, applyOverlay } = useSettings()
const { toast } = useToast()

// 悬浮窗外观预设：颜色与字号。id 与后端 settings.go 白名单一致。
const overlayColorOptions = [
  { value: 'white', label: '白色' },
  { value: 'red', label: '红色' },
  { value: 'green', label: '绿色' },
  { value: 'blue', label: '蓝色' },
  { value: 'yellow', label: '黄色' },
]
const overlayFontSizeOptions = [
  { value: 10, label: '小' },
  { value: 12, label: '中' },
  { value: 14, label: '大' },
  { value: 16, label: '特大' },
]

const autostartEnabled = ref(false)
const changingScope = ref(false)

onMounted(() => {
  load()
})

// 开关双向绑定辅助：读运行时主题（useTheme 反映界面真实状态），切换统一走 setThemeMode
const themeChecked = computed({
  get: () => theme.value === 'light',
  set: (v: boolean) => setThemeMode(v ? 'light' : 'dark'),
})

// 进程范围切换：system 需管理员权限并提权重启应用；currentUser 切回时若在管理员下仅提示。
async function onScopeChange(v: 'currentUser' | 'system') {
  if (changingScope.value || settings.value.processScope === v) return

  if (v === 'system') {
    const ans = await Dialogs.Question({
      Title: '切换到整个系统进程',
      Message: '查看 SYSTEM 等全部系统进程需要管理员权限，应用将以管理员身份自动重启。确认继续吗？',
      Buttons: [
        { Label: 'No', IsCancel: true },
        { Label: 'Yes', IsDefault: true },
      ],
    })
    if (ans !== 'Yes') {
      settings.value.processScope = 'currentUser' // 回滚下拉显示
      return
    }
    changingScope.value = true
    settings.value.processScope = 'system'
    await save()
    try {
      await SettingsService.RelaunchElevated()
      toast('正在以管理员权限重启应用…', 'info')
    } catch (e) {
      settings.value.processScope = 'currentUser'
      toast(e instanceof Error ? e.message : String(e), 'error')
    } finally {
      changingScope.value = false
    }
    return
  }

  // 切回当前用户
  settings.value.processScope = 'currentUser'
  await save()
  if (await SettingsService.IsElevated()) {
    toast('当前以管理员权限运行，如需仅查看当前用户进程，请以普通权限重新启动 PortCheck', 'info')
  }
}

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

async function onOverlayToggle() {
  await applyOverlay()
}

async function onOverlayPositionChange(e: Event) {
  settings.value.overlayPosition = (e.target as HTMLSelectElement).value as 'topLeft' | 'topRight'
  await applyOverlay()
}

// emitOverlayAppearance 把颜色/字号即时推送给悬浮窗窗口（独立 webview），并静默持久化。
async function emitOverlayAppearance() {
  Events.Emit('overlay:config', {
    color: settings.value.overlayColor,
    fontSize: settings.value.overlayFontSize,
  })
  try {
    await SettingsService.SaveSettings(settings.value)
  } catch {
    /* 忽略持久化失败 */
  }
}

async function onOverlayColorChange(e: Event) {
  settings.value.overlayColor = (e.target as HTMLSelectElement).value as typeof settings.value.overlayColor
  await emitOverlayAppearance()
}

async function onOverlayFontSizeChange(e: Event) {
  settings.value.overlayFontSize = Number((e.target as HTMLSelectElement).value)
  await emitOverlayAppearance()
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

    <!-- 进程范围 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">进程范围</span>
        <span class="setting-desc">「整个系统」可查看 SYSTEM 等全部进程，需管理员权限并重启应用</span>
      </div>
      <select
        class="setting-select"
        :value="settings.processScope"
        :disabled="changingScope"
        @change="onScopeChange(($event.target as HTMLSelectElement).value as 'currentUser' | 'system')"
      >
        <option value="currentUser">当前用户</option>
        <option value="system">整个系统</option>
      </select>
    </div>

    <!-- 性能悬浮窗 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">性能悬浮窗</span>
        <span class="setting-desc">在屏幕角落常驻显示 CPU / 内存 / 提交内存，主窗口最小化后仍保持</span>
      </div>
      <label class="switch" :class="{ on: settings.overlayEnabled }">
        <input
          type="checkbox"
          class="switch-input"
          v-model="settings.overlayEnabled"
          @change="onOverlayToggle"
        />
        <span class="switch-track">
          <span class="switch-thumb" />
        </span>
        <span class="switch-text">{{ settings.overlayEnabled ? '开' : '关' }}</span>
      </label>
    </div>

    <!-- 悬浮窗位置 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">悬浮窗位置</span>
        <span class="setting-desc">仅在悬浮窗开启时生效</span>
      </div>
      <select
        class="setting-select"
        :value="settings.overlayPosition"
        :disabled="!settings.overlayEnabled"
        @change="onOverlayPositionChange"
      >
        <option value="topRight">右上角</option>
        <option value="topLeft">左上角</option>
      </select>
    </div>

    <!-- 悬浮窗颜色 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">悬浮窗颜色</span>
        <span class="setting-desc">悬浮窗文字颜色，仅在开启时生效</span>
      </div>
      <select
        class="setting-select"
        :value="settings.overlayColor"
        :disabled="!settings.overlayEnabled"
        @change="onOverlayColorChange"
      >
        <option v-for="opt in overlayColorOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
      </select>
    </div>

    <!-- 悬浮窗字号 -->
    <div class="setting-row">
      <div class="setting-info">
        <span class="setting-label">悬浮窗字号</span>
        <span class="setting-desc">悬浮窗文字大小，仅在开启时生效</span>
      </div>
      <select
        class="setting-select"
        :value="settings.overlayFontSize"
        :disabled="!settings.overlayEnabled"
        @change="onOverlayFontSizeChange"
      >
        <option v-for="opt in overlayFontSizeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
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
