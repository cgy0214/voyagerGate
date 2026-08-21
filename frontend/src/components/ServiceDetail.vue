<script setup>
import { computed, ref } from 'vue'
import { state, App, run, rulesOf, toast } from '../store'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import ConfirmModal from './modals/ConfirmModal.vue'

const emit = defineEmits(['add-rule', 'edit-rule'])

const svc = computed(() => state.current?.services.find(s => s.name === state.selected) || null)
const rules = computed(() => svc.value ? rulesOf(svc.value.name) : [])
const ruleFilter = ref('')
const confirmDel = ref(null) // { idx, path, dest }

// 模糊搜索：按路径 / 备注过滤规则列表
const filteredRules = computed(() => {
  const q = ruleFilter.value.trim().toLowerCase()
  if (!q) return rules.value
  return rules.value.filter(r =>
    r.path.toLowerCase().includes(q) ||
    (r.remark || '').toLowerCase().includes(q)
  )
})

// 过滤后的规则行 → 原始索引（后端以原列表顺序为索引）
const ridx = r => rules.value.indexOf(r)

const decision = computed(() => {
  const s = svc.value
  if (!s) return null
  if (!state.snapshot?.running) return { txt: '代理未运行', dim: true }
  if (!s.on) return { txt: '服务已禁用 · 不参与转发', dim: true }
  if (s.online) {
    return { txt: '本地转发', dim: false }
  }
  return { txt: '降级 → 网关', dim: false, gw: true }
})

async function toggleSvc() {
  const s = svc.value
  if (!s) return
  s.on = !s.on
  await run(() => App.ToggleService(s.name))
  toast(`服务 ${s.name} 已${s.on ? '启用' : '禁用'}`)
}

async function toggleRule(idx) {
  const s = svc.value
  if (!s) return
  const r = rules.value[idx]
  if (!r) return
  r.on = !r.on
  await run(() => App.ToggleRule(s.name, idx))
  toast(`规则 ${r.path}→${r.dest} 已${r.on ? '启用' : '停用'}`)
}

// 单条规则：切换本地转发是否去除前缀（无文案提示，仅开关状态）
async function toggleRuleStrip(idx) {
  const s = svc.value
  if (!s) return
  await run(() => App.ToggleRuleStrip(s.name, idx))
}

// 批量启用 / 禁用全部规则
async function setAllRules(on) {
  const s = svc.value
  if (!s) return
  await run(() => App.SetAllRules(s.name, on))
  toast(`已${on ? '启用' : '停用'} ${s.name} 全部规则`)
}

function editRule(idx) {
  const s = svc.value
  if (!s) return
  emit('edit-rule', { svc: s.name, idx })
}
function askDeleteRule(idx) {
  const s = svc.value
  if (!s) return
  const r = rules.value[idx]
  if (!r) return
  confirmDel.value = { idx, path: r.path, dest: r.dest }
}
async function doDeleteRule() {
  const s = svc.value
  if (!s || !confirmDel.value) return
  const { idx, path } = confirmDel.value
  confirmDel.value = null
  await run(() => App.DeleteRule(s.name, idx), `已删除规则 ${path}`)
}

// 点击端口复制「本机IP:端口」到剪贴板
async function copyPort() {
  const s = svc.value
  if (!s || !s.port) return
  const ip = state.snapshot?.localIp || '127.0.0.1'
  await ClipboardSetText(`${ip}:${s.port}`)
  toast(`已复制 ${ip}:${s.port}`)
}
</script>

<template>
  <div class="col col-rule glass">
    <div class="col-head">服务详情</div>
    <div class="rule-detail" v-if="svc">
      <div class="rd-head">
        <div class="rd-title">
          <b>{{ svc.name }}</b>
        </div>
        <div class="rd-port" :class="{ clickable: !!svc.port }" :title="svc.port ? '点击复制 本机IP:端口' : '未注册本地端口'" @click="copyPort">
           <b>{{ svc.port || '未注册本地端口' }}</b>
          <svg v-if="svc.port" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        </div>
        <div class="rd-svc-toggle">
          <span class="switch" :class="{ on: svc.on }" @click="toggleSvc"><span class="knob"></span></span>
        </div>
      </div>

      <div class="rd-decision">
        <span class="decision" :class="{ dim: decision.dim, gw: decision.gw }">{{ decision.txt }}</span>
        <span class="rd-sync">心跳 <b>{{ state.snapshot?.lastSync || '--:--:--' }}</b></span>
      </div>

      <div class="rd-rules-head">
        <span class="rules-label">路由规则</span>
        <span class="rd-batch">
          <button class="icon-btn" title="批量启用全部规则" @click="setAllRules(true)">
            <svg viewBox="0 0 24 24"><path d="M18 6 7 17l-5-5"/><path d="m22 10-7.5 7.5L13 16"/></svg>
          </button>
          <button class="icon-btn" title="批量禁用全部规则" @click="setAllRules(false)">
            <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
          </button>
          <button class="icon-btn add" title="添加规则" @click="emit('add-rule')">
            <svg viewBox="0 0 24 24"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          </button>
        </span>
      </div>
      <div class="prefix-bar">
        <input class="field" v-model="ruleFilter" placeholder="🔍 搜索规则路径">
      </div>

      <template v-if="filteredRules.length">
        <div
          v-for="(r, fi) in filteredRules"
          :key="r.svc + r.path + r.dest + r.prio + fi"
          class="rule-item"
          :class="{ dim: !r.on }"
        >
          <span class="rule-path">{{ r.path }}</span><span class="rule-arrow">→</span>
          <span class="rule-dest" :class="{ local: r.dest === '本地' }">{{ r.dest }}</span>
          <span class="rule-meta">
            {{ ['优先级 ' + r.prio, r.remark, r.stripPrefix].filter(Boolean).join(' · ') }}
          </span>
          <span class="rule-row-ops">
            <label class="switch" :class="{ on: r.on }" @click="toggleRule(ridx(r))"><span class="knob"></span></label>
            <span class="strip-toggle" :class="{ on: r.stripPrefix }" @click="toggleRuleStrip(ridx(r))">⊘</span>
            <span title="编辑" @click="editRule(ridx(r))">✎</span>
            <span class="del" title="删除" @click="askDeleteRule(ridx(r))">🗑</span>
          </span>
        </div>
      </template>
      <div v-else-if="rules.length" class="empty">无匹配的规则</div>
      <div v-else class="empty">该服务暂无规则</div>
    </div>
    <div v-else class="empty">无服务，点击「+ 添加」或「连接并拉取」</div>

    <ConfirmModal
      v-if="confirmDel"
      title="删除路由规则"
      :message="`确认删除规则 ${confirmDel.path} → ${confirmDel.dest} ?`"
      ok-text="删除"
      @cancel="confirmDel = null"
      @ok="doDeleteRule"
    />
  </div>
</template>

<style scoped>
.rd-head { display:flex; align-items:center; gap:14px; }
.rd-title { display:flex; align-items:center; gap:10px; flex:1; min-width:0; }
.rd-title b { color:var(--text); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.rd-port { color:var(--sub); white-space:nowrap; display:flex; align-items:center; gap:6px; }
.rd-port b { color:var(--text); font-weight:600; }
.rd-port svg { width:13px; height:13px; stroke:var(--dim); fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.rd-port.clickable { cursor:pointer; }
.rd-port.clickable:hover b { color:var(--blue); }
.rd-port.clickable:hover svg { stroke:var(--blue); }
.rd-svc-toggle { display:flex; align-items:center; gap:8px; color:var(--dim); white-space:nowrap; }
.rd-decision { display:flex; align-items:center; gap:12px; }
.rd-decision .decision { margin-top:0; }
.rd-sync { color:var(--dim); font-size:12px; }
.rd-sync b { color:var(--sub); font-weight:600; }
.rd-rules-head { display:flex; align-items:center; gap:10px; }
.rd-rules-head .rules-label { margin-top:0; }
.rd-batch { display:flex; gap:6px; margin-left:auto; }
.rd-batch .icon-btn { display:flex; align-items:center; justify-content:center; width:24px; height:24px; border-radius:7px; border:1px solid var(--border); background:var(--glass2); color:var(--dim); cursor:pointer; transition:all .15s; }
.rd-batch .icon-btn svg { width:13px; height:13px; stroke:currentColor; fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.rd-batch .icon-btn:hover { color:var(--blue); border-color:var(--border-hi); }
.rd-batch .icon-btn.add:hover { color:var(--blue); }
.prefix-bar { display:flex; align-items:center; gap:8px; margin-bottom:2px; }
.prefix-bar .field { flex:1; }
.strip-toggle { cursor:pointer; opacity:.6; transition:all .15s; font-size:12px; }
.strip-toggle.on { color:var(--green); opacity:1; }
.strip-toggle:hover { opacity:1; transform:scale(1.15); }
</style>
