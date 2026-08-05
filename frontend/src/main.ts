import { createApp } from 'vue'
import App from './App.vue'
import PerfOverlay from './components/PerfOverlay.vue'
import './assets/theme.css'
import './composables/useTheme' // 模块副作用：应用持久化主题，避免首屏闪烁

// 通过 URL hash 区分主窗口与悬浮窗：同一 SPA bundle，避免额外构建入口。
// 悬浮窗由后端 OverlayService 以 URL "/#/overlay" 创建，加载本视图。
const isOverlay = location.hash === '#/overlay'

if (isOverlay) {
  // 悬浮窗需要真正透明：覆盖 theme.css 中 html/body 的不透明背景。
  document.documentElement.style.background = 'transparent'
  document.body.style.background = 'transparent'
  createApp(PerfOverlay).mount('#app')
} else {
  createApp(App).mount('#app')
}
