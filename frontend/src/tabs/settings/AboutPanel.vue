<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { UpdateService } from '../../../bindings/github.com/Sxuan-Coder/PortCheck'
import { checkUpdate } from '../../composables/useUpdate'

const version = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    version.value = await UpdateService.CurrentVersion()
  } catch {
    version.value = '未知'
  } finally {
    loading.value = false
  }
})

function openRepo() {
  UpdateService.OpenURL('https://github.com/Sxuan-Coder/PortCheck')
}

function openIssues() {
  UpdateService.OpenURL('https://github.com/Sxuan-Coder/PortCheck/issues')
}
</script>

<template>
  <div class="about-wrap">
    <!-- 应用信息 -->
    <div class="about-card">
      <div class="about-header">
        <span class="about-logo">P</span>
        <div>
          <div class="about-name">PortCheck</div>
          <div class="about-ver" v-if="!loading">v{{ version }}</div>
        </div>
      </div>
      <div class="about-meta">
        <div class="meta-row">
          <span class="meta-key">技术栈</span>
          <span class="meta-val">Go + Wails v3 + Vue 3</span>
        </div>
        <div class="meta-row">
          <span class="meta-key">仓库</span>
          <span class="meta-val mono">github.com/Sxuan-Coder/PortCheck</span>
        </div>
      </div>
    </div>

    <!-- 更新 -->
    <div class="about-card">
      <div class="card-title">更新</div>
      <div class="update-row">
        <span class="update-info" v-if="!loading">当前版本 v{{ version }}</span>
        <button class="btn-primary" @click="checkUpdate">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M23 4v6h-6M1 20v-6h6" />
            <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
          </svg>
          检查更新
        </button>
      </div>
    </div>

    <!-- 链接 -->
    <div class="about-card">
      <div class="card-title">链接</div>
      <div class="link-row">
        <button class="btn-link" @click="openRepo">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
          </svg>
          访问 GitHub
        </button>
        <button class="btn-link" @click="openIssues">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 8v4M12 16h.01" />
          </svg>
          反馈问题
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.about-wrap {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.about-card {
  background: var(--field-bg);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-lg);
  padding: 18px;
}
.about-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 16px;
}
.about-logo {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  color: var(--brand);
  background: linear-gradient(135deg, var(--brand-glow), transparent);
  border: 1px solid var(--brand-glow);
}
.about-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-1);
}
.about-ver {
  font-size: 12px;
  color: var(--text-3);
  margin-top: 2px;
}
.about-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.meta-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.meta-key {
  font-size: 12px;
  color: var(--text-3);
  min-width: 48px;
}
.meta-val {
  font-size: 12px;
  color: var(--text-2);
}
.card-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-3);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 12px;
}
.update-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.update-info {
  font-size: 13px;
  color: var(--text-2);
}
.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--brand);
  background: var(--brand-glow);
  color: var(--brand);
  font-size: 12px;
  font-weight: 500;
  transition: background 0.18s;
}
.btn-primary:hover {
  background: var(--brand);
  color: #fff;
}
.link-row {
  display: flex;
  gap: 10px;
}
.btn-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--hairline);
  background: transparent;
  color: var(--text-2);
  font-size: 12px;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.btn-link:hover {
  background: var(--row-hover);
  color: var(--text-1);
  border-color: var(--brand-glow);
}
</style>
