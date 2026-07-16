import { computed, ref, type Ref } from 'vue'

// 极简虚拟列表：根据滚动条 scrollTop 与固定行高，计算可见区间，只渲染可见行 + 上下缓冲。
// 适配 1000+ 进程的 60fps 滚动场景。

export function useVirtualList<T>(source: Ref<T[]>, itemHeight: number, overscan = 8) {
  const scrollTop = ref(0)
  const viewport = ref(0) // 视口高度（px），由容器 @resize 注入

  const total = computed(() => source.value.length)
  const visibleCount = computed(() =>
    viewport.value > 0 ? Math.ceil(viewport.value / itemHeight) + overscan : 0,
  )
  const startIndex = computed(() => {
    const idx = Math.floor(scrollTop.value / itemHeight) - Math.floor(overscan / 2)
    return Math.max(0, idx)
  })
  const endIndex = computed(() =>
    Math.min(total.value, startIndex.value + visibleCount.value),
  )

  const slice = computed(() => source.value.slice(startIndex.value, endIndex.value))
  const padTop = computed(() => startIndex.value * itemHeight)
  const totalHeight = computed(() => total.value * itemHeight)

  function onScroll(e: Event) {
    scrollTop.value = (e.target as HTMLElement).scrollTop
  }

  return { slice, padTop, totalHeight, itemHeight, onScroll, viewport }
}
