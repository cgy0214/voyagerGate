<script setup>
import { onMounted, reactive, ref } from 'vue'
import TopBar from './components/TopBar.vue'
import EnvBar from './components/EnvBar.vue'
import ConfigCard from './components/ConfigCard.vue'
import ServiceList from './components/ServiceList.vue'
import ServiceDetail from './components/ServiceDetail.vue'
import LogPanel from './components/LogPanel.vue'
import StatusBar from './components/StatusBar.vue'
import Toast from './components/Toast.vue'
import NewEnvModal from './components/modals/NewEnvModal.vue'
import EnvManagerModal from './components/modals/EnvManagerModal.vue'
import SettingsModal from './components/modals/SettingsModal.vue'
import AddServiceModal from './components/modals/AddServiceModal.vue'
import AddRuleModal from './components/modals/AddRuleModal.vue'
import ConfigModal from './components/modals/ConfigModal.vue'
import { init, state } from './store'

const show = reactive({ newEnv: false, envMgr: false, settings: false, addSvc: false, addRule: false, config: false })
const ruleEdit = ref(null)
const openAddRule = () => { ruleEdit.value = null; show.addRule = true }
const openEditRule = (e) => { ruleEdit.value = e; show.addRule = true }

onMounted(async () => {
  await init()
  document.body.classList.toggle('light', state.snapshot?.theme === 'light')
  const z = Number(localStorage.getItem('vg.zoom') || 100)
  if (z && z !== 100) {
    document.body.style.zoom = z + '%'
    document.body.style.setProperty('--zoom', z / 100)
  }
})
</script>

<template>
  <div class="app">
    <TopBar
      @new-env="show.newEnv = true"
      @env-mgr="show.envMgr = true"
      @settings="show.settings = true"
    />
    <EnvBar />
    <div class="main">
      <ConfigCard />
      <div class="content">
        <ServiceList @add-svc="show.addSvc = true" />
        <ServiceDetail @add-rule="openAddRule" @edit-rule="openEditRule" />
      </div>
      <LogPanel />
    </div>
    <StatusBar />

    <NewEnvModal v-if="show.newEnv" @close="show.newEnv = false" />
    <EnvManagerModal v-if="show.envMgr" @close="show.envMgr = false" @new-env="show.envMgr = false; show.newEnv = true" />
    <SettingsModal v-if="show.settings" @close="show.settings = false" @open-config="show.settings = false; show.config = true" />
    <AddServiceModal v-if="show.addSvc" @close="show.addSvc = false" />
    <AddRuleModal v-if="show.addRule" :edit="ruleEdit" @close="show.addRule = false" />
    <ConfigModal v-if="show.config" @close="show.config = false" />
    <Toast />
  </div>
</template>
