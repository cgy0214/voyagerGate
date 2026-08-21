<script setup>
import { computed, ref } from 'vue'
import { state, App, run, toast } from '../store'
import ConfirmModal from './modals/ConfirmModal.vue'

const emit = defineEmits(['add-svc'])
const confirmDel = ref(0) // 待删除的手动服务数量
const svcFilter = ref('') // 服务搜索关键字

// 排序：在线 > 手动添加 > 注册中心插入；再按服务名模糊搜索
const filteredServices = computed(() => {
  const q = svcFilter.value.trim().toLowerCase()
  const list = (state.current?.services || []).slice()
  list.sort((a, b) => {
    if (a.online !== b.online) return a.online ? -1 : 1
    const sa = a.src === 'man' ? 0 : 1
    const sb = b.src === 'man' ? 0 : 1
    return sa - sb
  })
  if (!q) return list
  return list.filter(s => s.name.toLowerCase().includes(q))
})

function select(name) {
  state.selected = name
}

async function toggle(s) {
  s.on = !s.on
  await run(() => App.ToggleService(s.name))
  toast(`服务 ${s.name} 已${s.on ? '启用' : '禁用'}`)
}

async function setAll(on) {
  await run(() => App.SetAllServices(on), `已${on ? '启用' : '停用'}全部服务`)
}

function askDeleteManual() {
  const n = (state.current?.services || []).filter(s => s.src === 'man').length
  if (!n) { toast('没有手动添加的服务', 'err'); return }
  confirmDel.value = n
}
async function doDeleteManual() {
  const n = confirmDel.value
  confirmDel.value = 0
  await run(() => App.DeleteManualServices(), `已删除 ${n} 个手动服务`)
}
</script>

<template>
  <div class="col col-svc glass">
    <div class="col-head">服务列表
      <input class="svc-search" v-model="svcFilter" placeholder="🔍 搜索服务名">
      <span class="head-btn" title="添加服务" @click="emit('add-svc')">
        <svg viewBox="0 0 24 24"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
      </span>
      <span class="head-btn" title="批量启用全部服务" @click="setAll(true)">
        <svg viewBox="0 0 24 24"><path d="M18 6 7 17l-5-5"/><path d="m22 10-7.5 7.5L13 16"/></svg>
      </span>
      <span class="head-btn" title="批量禁用全部服务" @click="setAll(false)">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
      </span>
      <span class="head-btn del" title="删除手动服务" @click="askDeleteManual">
        <svg viewBox="0 0 24 24"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
      </span>
    </div>
    <div class="svc-table">
      <template v-if="filteredServices.length">
        <div
          v-for="s in filteredServices"
          :key="s.name"
          class="svc-row"
          :class="{ sel: s.name === state.selected }"
          @click="select(s.name)"
        >
          <span class="toggle" :class="{ off: !s.on }" @click.stop="toggle(s)">{{ s.on ? '●' : '○' }}</span>
          <span class="src" :title="s.src === 'reg' ? '注册中心拉取' : '手动添加'">
            <svg v-if="s.src === 'reg'" viewBox="0 0 24 24"><path d="M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z"/><path d="M12 3v8"/><path d="M8 7l4 4 4-4"/></svg>
            <svg v-else viewBox="0 0 24 24"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
          </span>
          <span class="svc-name" :class="{ dim: !s.on }">{{ s.name }}{{ s.on ? '' : ' (已禁用)' }}</span>
          <span class="st" :class="s.online ? 'on' : 'off'">
            <span class="dot" :class="s.online ? 'd-on' : 'd-off'"></span>{{ s.online ? '在线' : '离线' }}
          </span>
        </div>
      </template>
      <div v-else-if="(state.current?.services || []).length" class="empty">无匹配的服务</div>
      <div v-else class="empty">暂无服务，点击「+ 添加」或「保存并拉取」</div>
    </div>

    <ConfirmModal
      v-if="confirmDel"
      title="删除手动服务"
      :message="`确认删除 ${confirmDel} 个手动添加的服务及其路由规则？注册中心拉取的服务不受影响。`"
      ok-text="删除"
      @cancel="confirmDel = 0"
      @ok="doDeleteManual"
    />
  </div>
</template>

<style scoped>
.svc-search { flex:1; min-width:60px; padding:4px 9px; font-size:11px; border-radius:7px; border:1px solid var(--line); background:var(--card); color:var(--text); outline:none; }
.svc-search:focus { border-color:var(--blue); }
.head-btn { display:flex; align-items:center; justify-content:center; width:25px; height:25px; border-radius:7px; color:var(--dim); cursor:pointer; border:1px solid var(--border); background:var(--glass2); transition:all .15s; flex-shrink:0; }
.head-btn svg { width:14px; height:14px; stroke:currentColor; fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.head-btn:hover { color:var(--blue); border-color:var(--border-hi); background:var(--glass-hi); }
.head-btn.del:hover { color:var(--red); }
</style>