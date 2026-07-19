import { ref } from 'vue'
import { SettingsService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { useTheme } from './useTheme'
import { useToast } from './useToast'

export interface AppSettings {
  theme: string
  refreshIntervalMs: number
  language: string
}

const settings = ref<AppSettings>({
  theme: 'dark',
  refreshIntervalMs: 1000,
  language: 'zh-CN',
})

const loaded = ref(false)

export function useSettings() {
  const { toast } = useToast()

  async function load() {
    if (loaded.value) return
    try {
      const s = await SettingsService.GetSettings()
      settings.value = {
        theme: s.theme || 'dark',
        refreshIntervalMs: s.refreshIntervalMs || 1000,
        language: s.language || 'zh-CN',
      }
      loaded.value = true

      // 同步主题到 useTheme
      const { theme } = useTheme()
      if ((theme.value as string) !== settings.value.theme) {
        // 配置文件主题与当前主题不一致时，以配置文件为准
        document.body.classList.toggle('light-theme', settings.value.theme === 'light')
      }
    } catch {
      // 保持默认值
    }
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

  return { settings, loaded, load, save, setAutostart }
}
