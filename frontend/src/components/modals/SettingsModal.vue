<script setup>
import { onMounted, ref, computed } from 'vue'
import ModalShell from './ModalShell.vue'
import ConfirmModal from './ConfirmModal.vue'
import { App, run, toast, state } from '../../store'
import { useI18n } from 'vue-i18n'
import { setLocale } from '../../i18n'
const { t, locale } = useI18n()

const emit = defineEmits(['close', 'open-config'])
const theme = ref('dark')
const upd = ref('')
const zoom = ref(100)
const probe = ref(5)
const confirmReset = ref(false)
const updInfo = ref(null)   // 有新版时的详细结果（hasUpdate / note / download）
const version = computed(() => state.version)

onMounted(() => {
  theme.value = document.body.classList.contains('light') ? 'light' : 'dark'
  zoom.value = Number(localStorage.getItem('vg.zoom') || 100)
  probe.value = state.current?.probe || 5
})

function setTheme(v) {
  theme.value = v
  App.SetTheme(v)
  document.body.classList.toggle('light', v === 'light')
}

function setLang(v) {
  setLocale(v)
  App.SetLang(v)
}

function setZoom(v) {
  zoom.value = v
  document.body.style.zoom = v + '%'
  document.body.style.setProperty('--zoom', v / 100)
  localStorage.setItem('vg.zoom', String(v))
}

function changeZoom(d) {
  setZoom(Math.max(50, Math.min(200, zoom.value + d)))
}

async function checkUpdate() {
  upd.value = t('common.checking')
  updInfo.value = null
  setTimeout(async () => {
    const detail = await App.CheckUpdateDetail()
    upd.value = detail?.message || t('modals.settings.checked')
    if (detail?.hasUpdate) {
      updInfo.value = detail
    }
  }, 900)
}

// 直接跳转到下载页，不做自动下载
function goDownload() {
  App.OpenURL(updInfo.value?.download || 'https://github.com/cgy0214/voyagerGate/releases/latest')
  updInfo.value = null
}

function home() {
  App.OpenURL('https://github.com/cgy0214/voyagerGate')
}

async function openDir() {
  await run(() => App.OpenConfigDir())
}

async function save() {
  await run(() => App.SaveConfig(), t('modals.settings.saved'))
  emit('close')
}

async function exportCfg() {
  try {
    const p = await App.ExportConfig()
    toast(t('modals.settings.exported', { path: p }))
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  }
}

async function importCfg() {
  await run(() => App.ImportConfig(), t('modals.settings.imported'))
}

async function resetAll() {
  confirmReset.value = false
  await run(() => App.ResetConfig(), t('modals.settings.resetted'))
}

async function saveProbe() {
  const v = parseInt(probe.value, 10)
  await run(() => App.SetProbeInterval(v), t('modals.settings.probeSet', { n: v }))
}

const allowLan = computed(() => state.snapshot?.allowLan !== false)

async function toggleAllowLan() {
  const next = !allowLan.value
  try {
    await App.SetAllowLAN(next)
    if (state.snapshot) state.snapshot.allowLan = next
    toast(next ? t('modals.settings.lanOn') : t('modals.settings.lanOff'))
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  }
}
</script>

<template>
  <ModalShell :title="t('modals.settings.title')" @close="emit('close')">
    <div class="frow"><label>{{ t('modals.settings.theme') }}</label>
      <select class="field" :value="theme" @change="e => setTheme(e.target.value)">
        <option value="dark">{{ t('modals.settings.dark') }}</option>
        <option value="light">{{ t('modals.settings.light') }}</option>
      </select>
    </div>
    <div class="frow"><label>{{ t('modals.settings.lang') }}</label>
      <select class="field" :value="locale" @change="e => setLang(e.target.value)">
        <option value="zh">中文</option>
        <option value="en">English</option>
      </select>
    </div>
    <div class="frow"><label>{{ t('modals.settings.zoom') }}</label>
      <div class="zoom-ctl">
        <button class="mini-btn" :title="t('modals.settings.zoomOut')" @click="changeZoom(-10)">−</button>
        <span class="zoom-val">{{ zoom }}%</span>
        <button class="mini-btn" :title="t('modals.settings.zoomIn')" @click="changeZoom(10)">+</button>
      </div>
    </div>
    <div class="frow"><label>{{ t('modals.settings.probe') }}</label>
      <input class="field" type="number" min="1" max="3600" v-model.number="probe" @change="saveProbe" @keyup.enter="saveProbe">
      <span style="color:var(--dim)">{{ t('common.seconds') }}</span>
      <span class="probe-tip" :title="t('modals.settings.probeTip')">?</span>
    </div>
    <div class="frow"><label>{{ t('modals.settings.lan') }}</label>
      <span class="switch" :class="{ on: allowLan }" :title="t('modals.settings.lanTip')" @click="toggleAllowLan"><span class="knob"></span></span>
      <span style="color:var(--dim)">{{ allowLan ? t('modals.settings.lanOnText') : t('modals.settings.lanOffText') }}</span>
    </div>
    <div class="frow"><label>{{ t('modals.settings.config') }}</label>
      <button class="btn" @click="exportCfg">
        <svg viewBox="0 0 24 24"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        <span class="bt">{{ t('common.export') }}</span>
      </button>
      <button class="btn" @click="importCfg">
        <svg viewBox="0 0 24 24"><path d="M12 15V3"/><path d="m7 8 5-5 5 5"/><path d="M5 21h14"/></svg>
        <span class="bt">{{ t('common.import') }}</span>
      </button>
      <button class="btn" :title="t('modals.settings.viewConfigTip')" @click="emit('open-config')">
        <svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
        <span class="bt">{{ t('modals.settings.viewConfig') }}</span>
      </button>
      <button class="btn" :title="t('modals.settings.resetTip')" style="color:var(--red);border-color:rgba(255,69,58,.45)" @click="confirmReset = true">
        <svg viewBox="0 0 24 24"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
        <span class="bt">{{ t('modals.settings.reset') }}</span>
      </button>
    </div>
    <div class="frow"><label>{{ t('modals.settings.update') }}</label>
      <button class="btn" @click="checkUpdate">
        <svg viewBox="0 0 24 24"><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/></svg>
        <span class="bt">{{ t('modals.settings.checkVer') }}</span>
      </button>
      <button v-if="updInfo" class="btn primary" @click="goDownload">
        <svg viewBox="0 0 24 24"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        <span class="bt">{{ t('modals.settings.goDownload') }}</span>
      </button>
      <button class="btn" @click="home">
        <svg viewBox="0 0 24 24"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
        <span class="bt">{{ t('modals.settings.home') }}</span>
      </button>
      <span :class="upd.includes('✓') ? 'ok-tag' : (upd === t('common.checking') ? 'busy' : '')">{{ upd }}</span>
    </div>
    <div class="frow"><label>{{ t('modals.settings.about') }}</label>
      <span style="color:var(--sub)">VoyagerGate · 渡桥 <b style="color:var(--text)">{{ version }}</b></span>
    </div>
  </ModalShell>

  <ConfirmModal
    v-if="confirmReset"
    :title="t('modals.settings.resetTitle')"
    :message="t('modals.settings.resetMsg')"
    :ok-text="t('modals.settings.reset')"
    @cancel="confirmReset = false"
    @ok="resetAll"
  />
</template>

<style scoped>
.zoom-ctl { display:flex; align-items:center; gap:8px; }
.zoom-val { min-width:46px; text-align:center; color:var(--text); font-weight:600; }
.probe-tip { width:16px; height:16px; display:inline-flex; align-items:center; justify-content:center; border-radius:50%; border:1px dashed var(--line2); color:var(--dim); font-size:11px; line-height:1; cursor:help; user-select:none; transition:all .15s; }
.probe-tip:hover { color:var(--blue); border-color:var(--blue); }
</style>
