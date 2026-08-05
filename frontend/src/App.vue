<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppTitlebar from './components/AppTitlebar.vue'
import AppSidebar from './components/AppSidebar.vue'
import MiniChart from './components/MiniChart.vue'
import QuickCommand from './components/QuickCommand.vue'
import ToastHost from './components/ToastHost.vue'
import UpdateDialog from './components/UpdateDialog.vue'
import ProcessesTab from './tabs/ProcessesTab.vue'
import PerformanceTab from './tabs/PerformanceTab.vue'
import PortsTab from './tabs/PortsTab.vue'
import ServicesTab from './tabs/ServicesTab.vue'
import StartupTab from './tabs/StartupTab.vue'
import SettingsTab from './tabs/SettingsTab.vue'
import { checkUpdate } from './composables/useUpdate'

const active = ref('processes')

function onSwitch(tab: string) {
  active.value = tab
}

// 启动后后台静默检查一次更新：延迟 1.5s，让 monitor 订阅与首屏先完成；
// 仅在确认有新版本时才会弹出 UpdateDialog，无更新或网络异常都静默不打扰用户。
onMounted(() => {
  window.setTimeout(() => {
    checkUpdate(true)
  }, 1500)
})
</script>

<template>
  <div class="shell">
    <AppSidebar :active="active" @switch="onSwitch" />

    <main class="main">
      <AppTitlebar />
      <MiniChart />

      <section class="content scroll">
        <ProcessesTab v-show="active === 'processes'" />
        <PerformanceTab v-if="active === 'performance'" />
        <PortsTab v-show="active === 'ports'" />
        <ServicesTab v-if="active === 'services'" />
        <StartupTab v-if="active === 'startup'" />
        <SettingsTab v-if="active === 'settings'" />
      </section>

      <QuickCommand />
    </main>

    <ToastHost />
    <UpdateDialog />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background: var(--bg-app);
}
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}
.content {
  flex: 1;
  overflow-y: auto;
  padding: 4px 16px 16px;
}
</style>
