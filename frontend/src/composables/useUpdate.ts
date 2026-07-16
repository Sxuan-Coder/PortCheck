import { Dialogs } from '@wailsio/runtime'
import { UpdateService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import { useToast } from './useToast'

const { toast } = useToast()
let checking = false

// 检查更新：请求 GitHub 最新 Release，与本地版本对比。
// 有新版本时弹窗询问是否跳转下载页；已是最新则 Toast 提示。
export async function checkUpdate() {
  if (checking) return
  checking = true
  toast('正在检查更新…', 'info')
  try {
    const info = await UpdateService.CheckUpdate()
    if (info.hasUpdate) {
      const ans = await Dialogs.Question({
        Title: '发现新版本',
        Message: `PortCheck ${info.latestVersion} 已发布（当前 ${info.currentVersion}）。\n是否前往 GitHub 下载？`,
        Buttons: [
          { Label: '稍后', IsCancel: true },
          { Label: '前往下载', IsDefault: true },
        ],
      })
      if (ans === '前往下载') {
        const url = info.releaseUrl || info.downloadUrl
        if (url) await UpdateService.OpenURL(url)
      }
    } else if (info.latestVersion) {
      toast(`已是最新版本（${info.currentVersion}）`, 'success')
    } else {
      toast('未能获取版本信息', 'error')
    }
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    checking = false
  }
}
