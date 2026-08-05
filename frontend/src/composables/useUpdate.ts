import { reactive } from 'vue'
import { UpdateService } from '../../bindings/github.com/Sxuan-Coder/PortCheck'
import type { UpdateInfo } from '../../bindings/github.com/Sxuan-Coder/PortCheck/models'
import { useToast } from './useToast'

const { toast } = useToast()

// 更新检测的响应式状态：模块级单例，驱动 UpdateDialog 显隐。
interface UpdateState {
  visible: boolean        // 弹窗是否显示
  checking: boolean       // 是否正在检测（防重入）
  info: UpdateInfo | null // 最近一次检测结果
}

const state = reactive<UpdateState>({
  visible: false,
  checking: false,
  info: null,
})

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

// 确认更新：跳转 Release 页面后关闭弹窗。
export async function confirmUpdate() {
  const info = state.info
  const url = info?.releaseUrl || info?.downloadUrl
  state.visible = false
  if (url) {
    try {
      await UpdateService.OpenURL(url)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'error')
    }
  }
}

export function useUpdate() {
  return { state, checkUpdate, closeUpdateDialog, confirmUpdate }
}
