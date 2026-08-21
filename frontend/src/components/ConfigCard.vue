<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { state, App, run, toast } from '../store'

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
    toast(`转发地址 ${defAddr.value} 指向本软件监听端口，会造成转发死循环，已拒绝`, 'err')
    defTarget.value = 'gateway'
    defAddr.value = ''
    return
  }
  await run(() => App.SetDefaultTarget(defTarget.value, defAddr.value), defTarget.value === 'local' ? `默认转发已设为 ${defAddr.value}` : '默认转发已设为网关兜底')
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
    toast(r.message || '拉取完成')
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
    toast(r?.message || '检测完成')
  } finally {
    gwBusy.value = false
  }
}

const regStatusClass = () => regBusy.value ? 'busy' : (state.current?.reg?.ok ? 'ok-tag' : 'fail')
const regStatusText = () => {
  if (regBusy.value === 'connecting') return '连接中…'
  if (regBusy.value === 'pulling') return '拉取中…'
  return state.current?.reg?.ok
    ? `✓ 已连接 · 拉取 ${state.current.services.length} 个服务`
    : '✗ 连接失败'
}
const gwStatusClass = () => gwBusy.value ? 'busy' : (state.current?.gwOk ? 'ok-tag' : (gwInput.value.trim() ? 'fail' : ''))
const gwStatusText = () => {
  if (gwBusy.value) return '检测中…'
  if (state.current?.gwOk) return `✓ 连通 (延迟 ${gwMs.value || 0}ms)`
  return gwInput.value.trim() ? '✗ 连接失败' : ''
}
</script>

<template>
  <div class="cfg-box glass">
    <div class="cfg-row">
      <span class="cfg-label">
        <svg viewBox="0 0 24 24"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
        注册中心</span>
      <select class="field" v-model="form.type" @change="onRegTypeChange">
        <option>Nacos</option>
        <option>Eureka</option>
        <option>Consul</option>
      </select>

      <template v-if="form.type === 'Nacos'">
        <input class="field wide" v-model="form.addr" placeholder="127.0.0.1:8848" >
        <input class="field mid" v-model="form.ns" placeholder="命名空间" >
        <input class="field group-field" v-model="form.group" placeholder="DEFAULT_GROUP" >
      </template>
      <template v-else-if="form.type === 'Eureka'">
        <input class="field wide" v-model="form.addr" placeholder="http://localhost:8761/eureka/" >
        <input class="field mid" v-model="form.user" placeholder="用户名（可选）" >
        <input class="field mid" v-model="form.pass" type="password" placeholder="密码（可选）" >
      </template>
      <template v-else>
        <input class="field wide" v-model="form.addr" placeholder="localhost:8500" >
        <input class="field mid" v-model="form.dc" placeholder="数据中心 dc1" >
        <input class="field mid" v-model="form.token" type="password" placeholder="Token（可选）" >
      </template>

      <button class="btn" @click="connectAndPull">
        <svg viewBox="0 0 24 24"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
        <span class="bt">保存并拉取</span>
      </button>
      <span :class="regStatusClass()">{{ regStatusText() }}</span>
    </div>

    <div class="cfg-row" style="padding-top:9px;border-top:1px solid var(--border)">
      <span class="cfg-label">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
        网关地址</span>
      <input
        class="field xwide"
        v-model="gwInput"
        placeholder="https://voyagergate.com"
        @keyup.enter="saveAndCheck"
      >
      <button class="btn" @click="saveAndCheck">
        <svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <span class="bt">保存并检测</span>
      </button>
      <span :class="gwStatusClass()">{{ gwStatusText() }}</span>
      <span class="cfg-sep"></span>
      <span class="switch" :class="{ on: defTarget === 'gateway' }" title="开启=未匹配服务的请求走网关兜底；关闭=转发到指定地址" @click="toggleDefTarget"><span class="knob"></span></span>
      <input
        v-if="defTarget === 'local'"
        class="field wide"
        v-model="defAddr"
        placeholder="转发到指定地址（如 127.0.0.1:60018）"
        @keyup.enter="onDefAddrChange"
        @change="onDefAddrChange"
      >
      <span v-else class="def-hint">未匹配服务 → 网关兜底</span>
    </div>
  </div>
</template>
