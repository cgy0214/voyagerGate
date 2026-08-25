<script setup>
import { computed, ref } from 'vue'
import { state, App, toast } from '../store'
import ConfirmModal from './modals/ConfirmModal.vue'

const version = computed(() => (state.version))
const updInfo = ref(null)
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
  const detail = await App.CheckUpdateDetail()
  if (detail?.hasUpdate) {
    updInfo.value = detail
    toast(detail.message)
  } else {
    toast(detail?.message || '已是最新版本')
  }
}

// 直接跳转到下载页，不做自动下载
function goDownload() {
  App.OpenURL(updInfo.value?.download || 'https://github.com/cgy0214/voyagerGate/releases/latest')
  updInfo.value = null
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

  <ConfirmModal
    v-if="updInfo"
    title="发现新版本"
    :message="`当前版本 ${version}，可更新至 ${updInfo.version}。\n\n${updInfo.note || ''}\n\n`"
    ok-text="去下载"
    @cancel="updInfo = null"
    @ok="goDownload"
  />
</template>
