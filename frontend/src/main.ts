import { createApp } from 'vue'
import App from './App.vue'
import './assets/theme.css'
import './composables/useTheme' // 模块副作用：应用持久化主题，避免首屏闪烁

createApp(App).mount('#app')
