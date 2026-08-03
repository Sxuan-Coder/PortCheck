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
  function set(cls: Theme) {
    theme.value = cls
    localStorage.setItem(THEME_KEY, cls)
    apply(cls)
  }
  function toggle() {
    set(theme.value === 'dark' ? 'light' : 'dark')
  }
  return { theme, set, toggle }
}
