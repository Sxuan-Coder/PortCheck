import { ref } from 'vue'
import { SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { Apply as ApplyOverlay } from '../../bindings/github.com/Sxuan-Coder/PortCheck/overlayservice'
import { useTheme } from './useTheme'
import { useToast } from './useToast'

export interface AppSettings {
  theme: string
  refreshIntervalMs: number
  language: string
  processScope: 'currentUser' | 'system'
  overlayEnabled: boolean
  overlayPosition: 'topLeft' | 'topRight'
  overlayColor: 'white' | 'red' | 'green' | 'blue' | 'yellow'
  overlayFontSize: number
}

const settings = ref<AppSettings>({
  theme: 'dark',
  refreshIntervalMs: 1000,
  language: 'zh-CN',
  processScope: 'currentUser',
  overlayEnabled: false,
  overlayPosition: 'topRight',
  overlayColor: 'white',
  overlayFontSize: 12,
})

// loaded 标记后端配置是否已成功加载到内存。save/applyOverlay 前必须为 true，
// 否则会用上面的默认值覆盖磁盘里的真实配置（特别是被 AppTitlebar/QuickCommand 的
// setThemeMode → save 在设置页挂载前误触时）。
// 注意：dev 模式 HMR 可能重载本模块，loaded 不能跨重载持久，故每次 onMounted
// 都应调 load()（内部用 loaded 去重，正常运行只读一次后端）。
const loaded = ref(false)

export function useSettings() {
  const { toast } = useToast()
  const { set: setTheme } = useTheme()

  // load 从后端读取持久化配置填充内存。force=true 时绕过 loaded 缓存强制重读，
  // 用于 dev 热更新后同步磁盘最新值。
  async function load(force = false) {
    if (loaded.value && !force) return
    try {
      const s = await SettingsService.GetSettings()
      const overlayColors = ['white', 'red', 'green', 'blue', 'yellow'] as const
      settings.value = {
        theme: s.theme || 'dark',
        refreshIntervalMs: s.refreshIntervalMs || 1000,
        language: s.language || 'zh-CN',
        processScope: s.processScope === 'system' ? 'system' : 'currentUser',
        overlayEnabled: !!s.overlayEnabled,
        overlayPosition: s.overlayPosition === 'topLeft' ? 'topLeft' : 'topRight',
        overlayColor: (overlayColors as readonly string[]).includes(s.overlayColor)
          ? (s.overlayColor as AppSettings['overlayColor'])
          : 'white',
        overlayFontSize: Number.isFinite(s.overlayFontSize) && s.overlayFontSize >= 10 && s.overlayFontSize <= 18
          ? s.overlayFontSize
          : 12,
      }
      loaded.value = true

      // 以运行时主题为准：localStorage 始终记录用户最后一次切换（标题栏切换只写它）。
      // 后端 settings.json 仅作持久化镜像，若不同步则静默对齐，避免反向覆盖导致主题跳变。
      const { theme } = useTheme()
      if ((theme.value as string) !== settings.value.theme) {
        settings.value.theme = theme.value as string
        try {
          await SettingsService.SaveSettings(settings.value)
        } catch {
          /* 忽略同步失败 */
        }
      }
    } catch {
      // 保持默认值
    }
  }

  // ensureLoaded 保证 save/applyOverlay 在加载完成后再执行，
  // 避免设置页未挂载时被外部（标题栏主题切换）误触 save 而用默认值覆盖磁盘。
  async function ensureLoaded() {
    if (!loaded.value) await load()
  }

  // setThemeMode 统一所有主题切换入口（标题栏/设置页/速启指令）：
  // 同时更新运行时主题并持久化到后端 settings.json。
  async function setThemeMode(cls: 'dark' | 'light') {
    settings.value.theme = cls
    setTheme(cls)
    await save()
  }

  async function save() {
    await ensureLoaded()
    try {
      await SettingsService.SaveSettings(settings.value)
      toast('设置已保存', 'success')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  async function setAutostart(enabled: boolean) {
    try {
      await SettingsService.SetAutostart(enabled)
      toast(enabled ? '已开启开机自启' : '已关闭开机自启', 'success')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  // applyOverlay 同步悬浮窗运行时状态并静默持久化（不弹"设置已保存" toast，避免打扰）。
  // 由设置页开关 / 位置下拉调用，立即生效。
  async function applyOverlay() {
    await ensureLoaded()
    try {
      await ApplyOverlay(settings.value.overlayEnabled, settings.value.overlayPosition)
      await SettingsService.SaveSettings(settings.value)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }

  return { settings, loaded, load, ensureLoaded, save, setAutostart, setThemeMode, applyOverlay }
}
