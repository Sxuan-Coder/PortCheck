<script setup lang="ts">
import { ref } from 'vue'
import SystemSettingsPanel from './settings/SystemSettingsPanel.vue'
import AboutPanel from './settings/AboutPanel.vue'

type SubTab = 'system' | 'about'

const activeSub = ref<SubTab>('system')

const subTabs: { id: SubTab; label: string }[] = [
  { id: 'system', label: '系统设置' },
  { id: 'about', label: '关于软件' },
]
</script>

<template>
  <div class="settings-shell">
    <div class="settings-header">
      <button
        v-for="tab in subTabs"
        :key="tab.id"
        class="sub-tab"
        :class="{ active: activeSub === tab.id }"
        @click="activeSub = tab.id"
      >
        {{ tab.label }}
        <span class="sub-indicator" />
      </button>
    </div>

    <div class="settings-body scroll">
      <SystemSettingsPanel v-if="activeSub === 'system'" />
      <AboutPanel v-if="activeSub === 'about'" />
    </div>
  </div>
</template>

<style scoped>
.settings-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.settings-header {
  display: flex;
  gap: 4px;
  padding: 0 0 10px;
  border-bottom: 1px solid var(--hairline);
  margin-bottom: 14px;
}
.sub-tab {
  position: relative;
  padding: 6px 14px;
  border: none;
  background: transparent;
  color: var(--text-3);
  font-size: 12px;
  font-weight: 500;
  transition: color 0.15s;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
}
.sub-tab:hover {
  color: var(--text-1);
}
.sub-tab.active {
  color: var(--brand);
}
.sub-indicator {
  position: absolute;
  bottom: -1px;
  left: 50%;
  transform: translateX(-50%);
  width: 0;
  height: 2px;
  border-radius: 2px 2px 0 0;
  background: var(--brand);
  transition: width 0.2s;
}
.sub-tab.active .sub-indicator {
  width: 60%;
}
.settings-body {
  flex: 1;
  overflow-y: auto;
}
</style>
