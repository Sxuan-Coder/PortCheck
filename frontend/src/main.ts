import { createApp } from 'vue'
import App from './App.vue'
import PerfOverlay from './components/PerfOverlay.vue'
import UsageOverlay from './components/UsageOverlay.vue'
import './assets/theme.css'
import './composables/useTheme' // 模块副作用：应用持久化主题，避免首屏闪烁

// 通过 URL hash 区分主窗口与悬浮窗：同一 SPA bundle，避免额外构建入口。
// 悬浮窗由后端 OverlayService 创建：
//   - "/#/overlay"        性能悬浮窗（CPU/内存/提交 三行文本）
//   - "/#/usage-overlay"  用量悬浮窗（Coding Plan 圆环图表）
const hash = location.hash

if (hash === '#/overlay' || hash === '#/usage-overlay') {
  // 悬浮窗需要真正透明：覆盖 theme.css 中 html/body 的不透明背景。
  document.documentElement.style.background = 'transparent'
  document.body.style.background = 'transparent'
  createApp(hash === '#/overlay' ? PerfOverlay : UsageOverlay).mount('#app')
} else {
  createApp(App).mount('#app')
}
