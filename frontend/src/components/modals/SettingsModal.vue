<script setup>
import { onMounted, ref, computed } from 'vue'
import ModalShell from './ModalShell.vue'
import ConfirmModal from './ConfirmModal.vue'
import { App, run, toast, state } from '../../store'

const emit = defineEmits(['close', 'open-config'])
const theme = ref('dark')
const upd = ref('')
const zoom = ref(100)
const probe = ref(5)
const confirmReset = ref(false)
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
  upd.value = '检查中…'
  setTimeout(async () => {
    upd.value = await App.CheckUpdate()
  }, 900)
}

function home() {
  App.OpenURL('https://github.com/cgy0214/voyagerGate')
}

async function openDir() {
  await run(() => App.OpenConfigDir())
}

async function save() {
  await run(() => App.SaveConfig(), '配置已保存 ✓')
  emit('close')
}

async function exportCfg() {
  try {
    const p = await App.ExportConfig()
    toast(`已导出: ${p}`)
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  }
}

async function importCfg() {
  await run(() => App.ImportConfig(), '配置导入成功')
}

async function resetAll() {
  confirmReset.value = false
  await run(() => App.ResetConfig(), '已还原所有配置')
}

async function saveProbe() {
  const v = parseInt(probe.value, 10)
  await run(() => App.SetProbeInterval(v), `探测间隔已设为 ${v} 秒`)
}
</script>

<template>
  <ModalShell title="⚙ 设置" @close="emit('close')">
    <div class="frow"><label>界面主题</label>
      <select class="field" :value="theme" @change="e => setTheme(e.target.value)">
        <option value="dark">深色</option>
        <option value="light">浅色</option>
      </select>
    </div>
    <div class="frow"><label>页面缩放</label>
      <div class="zoom-ctl">
        <button class="mini-btn" title="缩小" @click="changeZoom(-10)">−</button>
        <span class="zoom-val">{{ zoom }}%</span>
        <button class="mini-btn" title="放大" @click="changeZoom(10)">+</button>
      </div>
    </div>
    <div class="frow"><label>探测间隔</label>
      <input class="field" type="number" min="1" max="3600" v-model.number="probe" @change="saveProbe" @keyup.enter="saveProbe">
      <span style="color:var(--dim)">秒</span>
      <span class="probe-tip" title="仅代理运行时生效，会自动更新服务列表在线状态">?</span>
    </div>
    <div class="frow"><label>配置管理</label>
      <button class="btn" @click="exportCfg">
        <svg viewBox="0 0 24 24"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        <span class="bt">导出</span>
      </button>
      <button class="btn" @click="importCfg">
        <svg viewBox="0 0 24 24"><path d="M12 15V3"/><path d="m7 8 5-5 5 5"/><path d="M5 21h14"/></svg>
        <span class="bt">导入</span>
      </button>
      <button class="btn" title="查看环境全部配置" @click="emit('open-config')">
        <svg viewBox="0 0 24 24"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
        <span class="bt">配置查看</span>
      </button>
      <button class="btn" title="还原所有配置（删除全部环境、服务与规则）" style="color:var(--red);border-color:rgba(255,69,58,.45)" @click="confirmReset = true">
        <svg viewBox="0 0 24 24"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
        <span class="bt">还原配置</span>
      </button>
    </div>
    <div class="frow"><label>更新与主页</label>
      <button class="btn" @click="checkUpdate">
        <svg viewBox="0 0 24 24"><path d="M21 2v6h-6"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 22v-6h6"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/></svg>
        <span class="bt">检查版本</span>
      </button>
      <button class="btn" @click="home">
        <svg viewBox="0 0 24 24"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
        <span class="bt">主页</span>
      </button>
      <span :class="upd.includes('✓') ? 'ok-tag' : (upd === '检查中…' ? 'busy' : '')">{{ upd }}</span>
    </div>
    <div class="frow"><label>关于</label>
      <span style="color:var(--sub)">VoyagerGate · 渡桥 <b style="color:var(--text)">{{ version }}</b></span>
    </div>
  </ModalShell>

  <ConfirmModal
    v-if="confirmReset"
    title="还原所有配置"
    message="将删除全部环境、服务列表、路由规则与各项设置，恢复出厂默认。\n此操作不可恢复，确认继续？"
    ok-text="还原"
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
