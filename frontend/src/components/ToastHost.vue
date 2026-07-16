<script setup lang="ts">
import AppIcon from './AppIcon.vue'
import { useToast } from '../composables/useToast'

const { toasts, dismiss } = useToast()

function iconFor(type: string) {
  if (type === 'success') return 'ban'
  if (type === 'error') return 'ban'
  return 'search'
}
</script>

<template>
  <div class="toast-host">
    <transition-group name="toast">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="toast"
        :class="t.type"
        @click="dismiss(t.id)"
      >
        <AppIcon :name="iconFor(t.type)" :size="13" />
        <span>{{ t.message }}</span>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 14px;
  right: 14px;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  border-radius: var(--radius-md);
  font-size: 12px;
  color: var(--text-2);
  background: var(--win-bg);
  border: 1px solid var(--hairline-strong);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(12px);
  max-width: 320px;
}
.toast.success {
  color: var(--brand-light);
  border-color: var(--brand-glow);
}
.toast.error {
  color: var(--red);
  border-color: rgba(239, 68, 68, 0.3);
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(40px);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(40px);
}
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}
</style>
