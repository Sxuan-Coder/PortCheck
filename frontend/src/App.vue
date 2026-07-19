<script setup lang="ts">
import { ref } from 'vue'
import AppTitlebar from './components/AppTitlebar.vue'
import AppSidebar from './components/AppSidebar.vue'
import MiniChart from './components/MiniChart.vue'
import QuickCommand from './components/QuickCommand.vue'
import ToastHost from './components/ToastHost.vue'
import ProcessesTab from './tabs/ProcessesTab.vue'
import PerformanceTab from './tabs/PerformanceTab.vue'
import PortsTab from './tabs/PortsTab.vue'
import ServicesTab from './tabs/ServicesTab.vue'
import StartupTab from './tabs/StartupTab.vue'
import SettingsTab from './tabs/SettingsTab.vue'

const active = ref('processes')

function onSwitch(tab: string) {
  active.value = tab
}
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
