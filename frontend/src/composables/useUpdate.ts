import { reactive } from 'vue'
import { Events } from '@wailsio/runtime'
import { UpdateService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { UpdateInfo } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models'
import { useToast } from './useToast'

const { toast } = useToast()

// 更新检测的响应式状态：模块级单例，驱动 UpdateDialog 显隐。
interface UpdateState {
  visible: boolean        // 弹窗是否显示
  checking: boolean       // 是否正在检测（防重入）
  info: UpdateInfo | null // 最近一次检测结果
  downloading: boolean    // 是否正在下载安装包
  progress: number        // 下载进度百分比（Content-Length 未知时为 0）
  receivedMB: number      // 已下载大小（MB，总量未知时用于展示）
  totalMB: number         // 总大小（MB，未知为 0）
}

const state = reactive<UpdateState>({
  visible: false,
  checking: false,
  info: null,
  downloading: false,
  progress: 0,
  receivedMB: 0,
  totalMB: 0,
})

// 后端 update:download 事件载荷（兼容 ev.data 包装形态）。
function onProgress(ev: any) {
  const raw = ev && ev.data ? ev.data : ev
  const p = raw && typeof raw === 'object' ? raw : null
  if (!p) return
  state.progress = Number(p.percent) || 0
  state.receivedMB = (Number(p.received) || 0) / 1024 / 1024
  state.totalMB = (Number(p.total) || 0) / 1024 / 1024
}
// 模块级单例常驻订阅（与 useMonitor 的订阅约定一致），无需取消。
Events.On('update:download', onProgress)

// 检查更新：请求 GitHub 最新 Release，与本地版本对比。
// - silent=true（启动后台静默）：不弹 toast，失败静默吞掉，仅在有新版本时弹 Vue 弹窗。
// - silent=false（手动按钮）：保留检查中/已是最新/出错 的 toast 反馈，发现新版本时弹 Vue 弹窗。
export async function checkUpdate(silent = false) {
  if (state.checking) return
  state.checking = true
  if (!silent) toast('正在检查更新…', 'info')
  try {
    const info = await UpdateService.CheckUpdate()
    if (info.hasUpdate) {
      state.info = info
      state.visible = true
    } else if (!silent) {
      if (info.latestVersion) {
        toast(`已是最新版本（${info.currentVersion}）`, 'success')
      } else {
        toast('未能获取版本信息', 'error')
      }
    }
  } catch (e) {
    if (!silent) toast(e instanceof Error ? e.message : String(e), 'error')
  } finally {
    state.checking = false
  }
}

// 关闭更新弹窗（取消）。
export function closeUpdateDialog() {
  state.visible = false
}

// 用浏览器打开 Release 页面（下载失败或无安装包资产时的回退路径）。
async function fallbackToReleasePage() {
  const url = state.info?.releaseUrl || state.info?.downloadUrl
  if (url) {
    try {
      await UpdateService.OpenURL(url)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }
}

// 确认更新：优先应用内下载一键安装包并自动启动安装程序；
// 无安装包资产或下载失败时回退到浏览器打开 Release 页。
export async function confirmUpdate() {
  const info = state.info
  if (!info) {
    state.visible = false
    return
  }
  if (state.downloading) return
  if (!info.installerUrl) {
    state.visible = false
    await fallbackToReleasePage()
    return
  }
  state.downloading = true
  state.progress = 0
  state.receivedMB = 0
  state.totalMB = 0
  try {
    await UpdateService.DownloadInstaller(info.installerUrl)
    state.visible = false
    toast('下载完成，已启动安装程序', 'success')
  } catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error')
    await fallbackToReleasePage()
    state.visible = false
  } finally {
    state.downloading = false
  }
}

export function useUpdate() {
  return { state, checkUpdate, closeUpdateDialog, confirmUpdate }
}
