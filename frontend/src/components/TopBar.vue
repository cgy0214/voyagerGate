<script setup>
import { state, App, run, toast } from '../store'

const props = defineProps({
  showNewEnv: Boolean,
  showEnvMgr: Boolean,
  showSettings: Boolean,
})
const emit = defineEmits(['new-env', 'env-mgr', 'settings'])

async function switchEnv(name) {
  if (name === state.snapshot?.current) return
  await run(() => App.SwitchEnvironment(name))
  toast(`已切换环境: ${name}`)
}

function toggleTheme() {
  const theme = document.body.classList.contains('light') ? 'dark' : 'light'
  App.SetTheme(theme)
  document.body.classList.toggle('light', theme === 'light')
}
</script>

<template>
  <div class="topbar">
    <div class="logo"><span class="logo-badge">⚡</span> VoyagerGate</div>
    <div class="env-chips">
      <template v-for="e in (state.snapshot?.envs || [])" :key="e.name">
        <div
          class="env-chip"
          :class="{ active: e.name === state.snapshot?.current }"
          @click="switchEnv(e.name)"
        >{{ e.name === state.snapshot?.current ? '● ' : '' }}{{ e.name }}</div>
      </template>
      <div class="env-chip add" @click="emit('new-env')">＋ 新建环境</div>
    </div>
    <div class="spacer"></div>
    <div class="top-actions">
      <div class="top-btn" @click="emit('env-mgr')">
        <svg viewBox="0 0 24 24"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>
        环境
      </div>
      <div class="top-btn" @click="emit('settings')">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        设置
      </div>
      <div class="top-btn" @click="toggleTheme">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
        主题
      </div>
    </div>
  </div>
</template>
