<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import AppIcon from './AppIcon.vue'
import { useUpdate } from '../composables/useUpdate'

const { state, closeUpdateDialog, confirmUpdate } = useUpdate()

// 将 Release 说明按空行分段为段落，保留段内换行；空则返回空数组。
const noteParas = computed<string[]>(() => {
  const raw = state.info?.notes?.trim()
  if (!raw) return []
  return raw.split(/\n\s*\n/).map((p) => p.trim()).filter(Boolean)
})

// ESC 关闭弹窗（仅 visible 时绑定，避免全局常驻监听）。
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && state.visible) closeUpdateDialog()
}
watch(
  () => state.visible,
  (v) => {
    if (v) window.addEventListener('keydown', onKeydown)
    else window.removeEventListener('keydown', onKeydown)
  },
)
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

// 点击遮罩空白处关闭；点击卡片阻止冒泡。
function onMaskClick() {
  closeUpdateDialog()
}
</script>

<template>
  <transition name="upd">
    <div v-if="state.visible" class="upd-mask" @click="onMaskClick">
      <div class="upd-dialog acrylic-blur" @click.stop>
        <!-- 头部 -->
        <div class="upd-head">
          <div class="upd-icon">
            <AppIcon name="download" :size="18" />
          </div>
          <div class="upd-title">发现新版本</div>
          <button class="upd-x" title="关闭" @click="closeUpdateDialog">
            <AppIcon name="close" :size="13" />
          </button>
        </div>

        <!-- 版本行 -->
        <div class="upd-versions" v-if="state.info">
          <span class="ver-cur">v{{ state.info.currentVersion }}</span>
          <span class="ver-arrow">→</span>
          <span class="ver-new">v{{ state.info.latestVersion }}</span>
        </div>

        <!-- 更新说明 -->
        <div class="upd-notes scroll">
          <template v-if="noteParas.length">
            <p v-for="(p, i) in noteParas" :key="i" class="note-p">{{ p }}</p>
          </template>
          <p v-else class="note-empty">详见 GitHub Release 页面。</p>
        </div>

        <!-- 按钮区 -->
        <div class="upd-actions">
          <button class="btn-ghost" @click="closeUpdateDialog">稍后</button>
          <button class="btn-primary" @click="confirmUpdate">
            <AppIcon name="download" :size="13" />
            前往下载
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
/* 遮罩：fixed 全屏，z-index 高于 ToastHost(200) */
.upd-mask {
  position: fixed;
  inset: 0;
  z-index: 300;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}
/* 卡片 */
.upd-dialog {
  width: 420px;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 64px);
  display: flex;
  flex-direction: column;
  padding: 20px 22px 18px;
  border-radius: var(--radius-xl);
  border: 1px solid var(--hairline-strong);
  background: var(--win-bg);
  box-shadow: var(--shadow-2xl);
}
/* 头部 */
.upd-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.upd-icon {
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: linear-gradient(135deg, var(--brand-glow), transparent);
  border: 1px solid var(--brand-glow);
}
.upd-title {
  flex: 1;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-1);
}
.upd-x {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: transparent;
  border: none;
  color: var(--text-3);
  transition: background 0.15s, color 0.15s;
}
.upd-x:hover {
  color: var(--text-1);
  background: var(--row-hover);
}
/* 版本行 */
.upd-versions {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  margin-bottom: 12px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-md);
  font-size: 13px;
}
.ver-cur {
  color: var(--text-3);
  font-family: "Cascadia Code", "Consolas", "SF Mono", monospace;
}
.ver-arrow {
  color: var(--text-4);
}
.ver-new {
  color: var(--brand-light);
  font-weight: 600;
  font-family: "Cascadia Code", "Consolas", "SF Mono", monospace;
}
/* 更新说明 */
.upd-notes {
  flex: 1;
  overflow-y: auto;
  max-height: 240px;
  padding: 12px;
  margin-bottom: 16px;
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-md);
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--text-2);
  user-select: text;
}
.note-p {
  margin: 0 0 10px;
  white-space: pre-wrap;
  word-break: break-word;
}
.note-p:last-child {
  margin-bottom: 0;
}
.note-empty {
  margin: 0;
  color: var(--text-3);
  font-style: italic;
}
/* 按钮区 */
.upd-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.btn-ghost {
  padding: 8px 16px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--hairline);
  background: transparent;
  color: var(--text-2);
  font-size: 12.5px;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.btn-ghost:hover {
  background: var(--row-hover);
  color: var(--text-1);
  border-color: var(--hairline-strong);
}
.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--brand);
  background: var(--brand-glow);
  color: var(--brand);
  font-size: 12.5px;
  font-weight: 500;
  transition: background 0.18s, color 0.18s;
}
.btn-primary:hover {
  background: var(--brand);
  color: #fff;
}
/* 淡入淡出 */
.upd-enter-from {
  opacity: 0;
}
.upd-enter-from .upd-dialog {
  transform: scale(0.94);
  opacity: 0;
}
.upd-leave-to {
  opacity: 0;
}
.upd-leave-to .upd-dialog {
  transform: scale(0.96);
  opacity: 0;
}
.upd-enter-active,
.upd-leave-active {
  transition: opacity 0.2s ease;
}
.upd-enter-active .upd-dialog,
.upd-leave-active .upd-dialog {
  transition: transform 0.2s ease, opacity 0.2s ease;
}
</style>
