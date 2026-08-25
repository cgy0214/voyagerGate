<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { state, App, run, toast } from '../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const typeDefaults = {
  Nacos: { type: 'Nacos', addr: '', ns: '', group: '', user: '', pass: '', dc: '', token: '' },
  Eureka: { type: 'Eureka', addr: '', ns: '', group: '', user: '', pass: '', dc: '', token: '' },
  Consul: { type: 'Consul', addr: '', ns: '', group: '', user: '', pass: '', dc: '', token: '' },
}
// 各类型独立缓存表单输入：切换类型不丢值、不自动保存，点击「连接并拉取」才保存
const cache = reactive({
  Nacos: { ...typeDefaults.Nacos },
  Eureka: { ...typeDefaults.Eureka },
  Consul: { ...typeDefaults.Consul },
})
const form = reactive({ ...cache.Nacos })
const curType = ref('Nacos')
const regBusy = ref('')   // '' | connecting | pulling
const gwBusy = ref(false)
const gwMs = ref(0)
const gwInput = ref('')
const defTarget = ref('gateway')
const defAddr = ref('')

// 从后端当前环境同步表单（环境切换时）——只写入当前类型的缓存
function syncForm() {
  const r = state.current?.reg
  if (!r) return
  const t = r.type || 'Nacos'
  const c = cache[t]
  c.type = t
  c.addr = r.addr || ''
  c.ns = r.ns || ''
  c.group = r.group || ''
  c.user = r.user || ''
  c.pass = r.pass || ''
  c.dc = r.dc || ''
  c.token = r.token || ''
  curType.value = t
  Object.assign(form, c)
}

function syncExtra() {
  defTarget.value = state.current?.defaultTarget || 'gateway'
  defAddr.value = state.current?.defaultAddr || ''
}

watch(() => state.current?.name, () => { syncForm(); syncExtra(); gwMs.value = 0; gwInput.value = state.current?.gw || '' })
onMounted(() => { syncForm(); syncExtra() })

// 未匹配服务的默认转发目标：开关切换或输入地址后自动保存
function isSelfForward(addr) {
  const envPort = state.current?.port
  const a = addr.trim().toLowerCase().replace(/^https?:\/\//, '')
  const m = a.match(/:(\d+)$/)
  if (!m) return false
  if (Number(m[1]) !== envPort) return false
  const host = a.slice(0, -m[0].length)
  return host === '' || host === 'localhost' || host === '127.0.0.1' || host === '::1' || host === '[::1]'
}
async function saveDefaultTarget() {
  if (defTarget.value === 'local' && isSelfForward(defAddr.value)) {
    toast(t('configcard.loopRefused', { addr: defAddr.value }), 'err')
    defTarget.value = 'gateway'
    defAddr.value = ''
    return
  }
  await run(() => App.SetDefaultTarget(defTarget.value, defAddr.value),
    defTarget.value === 'local'
      ? t('configcard.defaultSet', { addr: defAddr.value })
      : t('configcard.defaultGateway'))
}
function toggleDefTarget() {
  const next = defTarget.value === 'gateway' ? 'local' : 'gateway'
  defTarget.value = next
  if (next === 'gateway') {
    // 切到网关兜底：立即保存并清空地址
    defAddr.value = ''
    saveDefaultTarget()
  }
  // 切到指定地址：不立即保存，等用户输入地址后由 onDefAddrChange 保存
}
function onDefAddrChange() {
  if (defAddr.value.trim()) saveDefaultTarget()
}

// 切换类型：仅在前端缓存间切换，不自动保存、不丢已填的值
function onRegTypeChange() {
  const old = curType.value
  const t = form.type
  if (old === t) return
  cache[old] = { ...form, type: old }
  curType.value = t
  Object.assign(form, cache[t])
}

// 点击「连接并拉取」时才保存注册中心配置
async function saveReg() {
  if (!state.current) return
  await run(() => App.SaveRegistry({ ...form }))
}

// 连接并拉取：连接中… → 拉取中… → 刷新在线状态
async function connectAndPull() {
  if (!state.current) return
  await saveReg()
  regBusy.value = 'connecting'
  try {
    await App.ConnectRegistry()
    regBusy.value = 'pulling'
    const r = await App.PullServices()
    toast(r.message || t('configcard.pulled'))
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  } finally {
    regBusy.value = ''
  }
}

// 保存网关地址并检测连通性
async function saveAndCheck() {
  if (!state.current) return
  await run(() => App.SetGateway(gwInput.value.trim()))
  gwBusy.value = true
  try {
    const r = await run(() => App.CheckGateway())
    gwMs.value = r?.ms || 0
    toast(r?.message || t('configcard.checked'))
  } finally {
    gwBusy.value = false
  }
}

const regStatusClass = () => regBusy.value ? 'busy' : (state.current?.reg?.ok ? 'ok-tag' : 'fail')
const regStatusText = () => {
  if (regBusy.value === 'connecting') return t('configcard.connecting')
  if (regBusy.value === 'pulling') return t('configcard.pulling')
  return state.current?.reg?.ok
    ? t('configcard.connected', { n: state.current.services.length })
    : t('configcard.connectFail')
}
const gwStatusClass = () => gwBusy.value ? 'busy' : (state.current?.gwOk ? 'ok-tag' : (gwInput.value.trim() ? 'fail' : ''))
const gwStatusText = () => {
  if (gwBusy.value) return t('configcard.checking')
  if (state.current?.gwOk) return t('configcard.gatewayOk', { ms: gwMs.value || 0 })
  return gwInput.value.trim() ? t('configcard.gatewayFail') : ''
}
</script>

<template>
  <div class="cfg-box glass">
    <div class="cfg-row">
      <span class="cfg-label">
        <svg viewBox="0 0 24 24"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
        {{ t('configcard.registry') }}</span>
      <select class="field" v-model="form.type" @change="onRegTypeChange">
        <option>Nacos</option>
        <option>Eureka</option>
        <option>Consul</option>
      </select>

      <template v-if="form.type === 'Nacos'">
        <input class="field wide" v-model="form.addr" :placeholder="t('configcard.addr') + '（127.0.0.1:8848）'" >
        <input class="field mid" v-model="form.ns" :placeholder="t('configcard.namespace')" >
        <input class="field group-field" v-model="form.group" :placeholder="t('configcard.group')" >
      </template>
      <template v-else-if="form.type === 'Eureka'">
        <input class="field wide" v-model="form.addr" :placeholder="t('configcard.addr') + '（http://localhost:8761/eureka/）'" >
        <input class="field mid" v-model="form.user" :placeholder="t('configcard.username')" >
        <input class="field mid" v-model="form.pass" type="password" :placeholder="t('configcard.password')" >
      </template>
      <template v-else>
        <input class="field wide" v-model="form.addr" :placeholder="t('configcard.addr') + '（localhost:8500）'" >
        <input class="field mid" v-model="form.dc" :placeholder="t('configcard.datacenter')" >
        <input class="field mid" v-model="form.token" type="password" :placeholder="t('configcard.token')" >
      </template>

      <button class="btn" @click="connectAndPull">
        <svg viewBox="0 0 24 24"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        <span class="bt">{{ t('configcard.savePull') }}</span>
      </button>
      <span :class="regStatusClass()">{{ regStatusText() }}</span>
    </div>

    <div class="cfg-row" style="padding-top:9px;border-top:1px solid var(--border)">
      <span class="cfg-label">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
        {{ t('configcard.gateway') }}</span>
      <input
        class="field xwide"
        v-model="gwInput"
        :placeholder="t('configcard.gateway')"
        @keyup.enter="saveAndCheck"
      >
      <button class="btn" @click="saveAndCheck">
        <svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <span class="bt">{{ t('configcard.saveCheck') }}</span>
      </button>
      <span :class="gwStatusClass()">{{ gwStatusText() }}</span>
      <span class="cfg-sep"></span>
      <span class="switch" :class="{ on: defTarget === 'gateway' }" :title="t('configcard.selfForwardTip')" @click="toggleDefTarget"><span class="knob"></span></span>
      <input
        v-if="defTarget === 'local'"
        class="field wide"
        v-model="defAddr"
        :placeholder="t('configcard.defaultAddr')"
        @keyup.enter="onDefAddrChange"
        @change="onDefAddrChange"
      >
      <span v-else class="def-hint">{{ t('configcard.defaultTarget') }}</span>
    </div>
  </div>
</template>
