import { ref } from 'vue'

// 主题切换：默认暗色，持久化到 localStorage，通过 body.light-theme class 控制 CSS token。
const THEME_KEY = 'portcheck-theme'
type Theme = 'dark' | 'light'

const theme = ref<Theme>((localStorage.getItem(THEME_KEY) as Theme) || 'dark')

function apply(cls: Theme) {
  const body = document.body
  if (cls === 'light') body.classList.add('light-theme')
  else body.classList.remove('light-theme')
}

// 模块加载即应用，避免首屏闪烁（main.ts 已先 import 本模块）。
apply(theme.value)

export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem(THEME_KEY, theme.value)
    apply(theme.value)
  }
  return { theme, toggle }
}
