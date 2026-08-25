<script setup>
import { computed, ref } from 'vue'
import { state, App, toast } from '../store'
import ConfirmModal from './modals/ConfirmModal.vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

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
    toast(t('statusbar.hasUpdate'))
  } else {
    toast(t('statusbar.latest'))
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
    <span>{{ t('statusbar.registry') }} <b>{{ regText }}</b></span>
    <span>{{ t('statusbar.online') }} <b>{{ onlineText }}</b></span>
    <span>{{ t('statusbar.requests') }} <b>{{ reqText }}</b></span>
    <span>{{ t('statusbar.avgMs') }} <b>{{ msText }}</b></span>
    <span class="ver-link" :title="t('statusbar.checkUpdate')" @click="checkUpdate">VoyagerGate 渡桥 <b>{{ version }}</b></span>
  </div>

  <ConfirmModal
    v-if="updInfo"
    width="560px"
    md
    :title="t('statusbar.hasUpdate')"
    :message="t('statusbar.downloadMsg', { ver: version, newVer: updInfo.version, note: updInfo.note || '' })"
    :ok-text="t('statusbar.goDownload')"
    @cancel="updInfo = null"
    @ok="goDownload"
  />
</template>
