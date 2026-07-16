<script setup lang="ts">
import AppIcon from './AppIcon.vue'

defineProps<{ active: string }>()
defineEmits<{ (e: 'switch', tab: string): void }>()

const items: { id: string; icon: string; label: string }[] = [
  { id: 'processes', icon: 'process', label: '进程管理' },
  { id: 'performance', icon: 'performance', label: '性能监视' },
  { id: 'ports', icon: 'ports', label: '端口查看' },
  { id: 'services', icon: 'services', label: '系统服务' },
  { id: 'startup', icon: 'startup', label: '开机启动' },
]
</script>

<template>
  <aside class="sidebar">
    <div class="logo"><AppIcon name="terminal" :size="18" /></div>

    <nav class="nav">
      <button
        v-for="it in items"
        :key="it.id"
        class="nav-item"
        :class="{ active: active === it.id }"
        :title="it.label"
        @click="$emit('switch', it.id)"
      >
        <span class="indicator" />
        <AppIcon :name="it.icon" :size="18" />
        <span class="tip">{{ it.label }}</span>
      </button>
    </nav>

    <div class="foot">
      <div class="ver">v2.0</div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 64px;
  flex-shrink: 0;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--hairline);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0;
  gap: 12px;
}
.logo {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: linear-gradient(135deg, var(--brand-glow), transparent);
  border: 1px solid var(--brand-glow);
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 0 8px;
  flex: 1;
}
.nav-item {
  position: relative;
  width: 100%;
  height: 44px;
  border-radius: var(--radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-3);
  transition: background 0.18s, color 0.18s;
}
.nav-item:hover {
  background: var(--row-hover);
  color: var(--text-1);
}
.nav-item.active {
  background: var(--brand-glow);
  color: var(--brand);
}
.indicator {
  position: absolute;
  left: -8px;
  width: 3px;
  height: 0;
  border-radius: 0 3px 3px 0;
  background: var(--brand);
  transition: height 0.18s;
}
.nav-item.active .indicator {
  height: 20px;
}
.tip {
  position: absolute;
  left: 54px;
  padding: 4px 8px;
  background: var(--header-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-sm);
  color: var(--text-1);
  font-size: 11px;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s;
  z-index: 30;
  backdrop-filter: blur(8px);
}
.nav-item:hover .tip {
  opacity: 1;
}
.foot {
  color: var(--text-4);
}
.ver {
  font-size: 10px;
  letter-spacing: -0.02em;
  opacity: 0.7;
}
</style>
