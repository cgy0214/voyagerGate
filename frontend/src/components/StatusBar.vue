<script setup>
import { computed } from 'vue'
import { state, App, toast } from '../store'

const version = computed(() => (state.version))
const regText = computed(() => {
  const e = state.current
  if (!e) return '—'
  return `${e.reg?.type || ''} ${e.reg?.addr || ''}`
})
const onlineText = computed(() => {
  const e = state.current
  if (!e) return '0/0'
  const on = e.services.filter(s => s.online).length
  return `${on}/${e.services.length}`
})
const reqText = computed(() => (state.snapshot?.reqCount || 0).toLocaleString())
const msText = computed(() => `${state.snapshot?.avgMs || 0}ms`)

async function checkUpdate() {
  const msg = await App.CheckUpdate()
  toast(msg)
}
</script>

<template>
  <div class="statusbar">
    <span>注册中心 <b>{{ regText }}</b></span>
    <span>在线 <b>{{ onlineText }}</b></span>
    <span>请求 <b>{{ reqText }}</b></span>
    <span>平均耗时 <b>{{ msText }}</b></span>
    <span class="ver-link" title="检查更新" @click="checkUpdate">VoyagerGate 渡桥 <b>{{ version }}</b></span>
  </div>
</template>
