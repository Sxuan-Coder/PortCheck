import { ref } from 'vue'
import { SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { useTheme } from './useTheme'
import { useToast } from './useToast'

export interface AppSettings {
  theme: string
  refreshIntervalMs: number
  language: string
  processScope: 'currentUser' | 'system'
}

const settings = ref<AppSettings>({
  theme: 'dark',
  refreshIntervalMs: 1000,
  language: 'zh-CN',
  processScope: 'currentUser',
})

const loaded = ref(false)

export function useSettings() {
  const { toast } = useToast()
  const { set: setTheme } = useTheme()

  async function load() {
    if (loaded.value) return
    try {
      const s = await SettingsService.GetSettings()
      settings.value = {
        theme: s.theme || 'dark',
        refreshIntervalMs: s.refreshIntervalMs || 1000,
        language: s.language || 'zh-CN',
        processScope: s.processScope === 'system' ? 'system' : 'currentUser',
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

  // setThemeMode 统一所有主题切换入口（标题栏/设置页/速启指令）：
  // 同时更新运行时主题并持久化到后端 settings.json。
  async function setThemeMode(cls: 'dark' | 'light') {
    settings.value.theme = cls
    setTheme(cls)
    await save()
  }

  async function save() {
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

  return { settings, loaded, load, save, setAutostart, setThemeMode }
}
