<script setup>
import { computed, ref, watch } from 'vue'
import { state } from '../store'

const fltDec = ref('全部')
const fltSvc = ref('全部')
const kw = ref('') // 日志关键字搜索

const visibleLogs = computed(() => {
  const q = kw.value.trim().toLowerCase()
  return state.logs.filter(l => {
    if (fltDec.value !== '全部' && l.dec !== fltDec.value) return false
    if (fltSvc.value !== '全部' && l.sname !== fltSvc.value) return false
    if (q) {
      const hay = [l.p, l.final, l.s, l.m, l.target].filter(Boolean).join(' ').toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

const svcOptions = computed(() => {
  return [...new Set((state.current?.services || []).map(s => s.name))]
})

// 服务列表变化时校正筛选（服务被删除则回退「全部」）
watch(svcOptions, names => {
  if (fltSvc.value !== '全部' && !names.includes(fltSvc.value)) fltSvc.value = '全部'
})

function pauseLog() {
  state.logPaused = !state.logPaused
}
function clearLog() {
  state.logs = []
}
</script>

<template>
  <div class="log glass">
    <div class="log-head">
      <span>▍ 实时日志</span>
      <span class="lbtn" :title="state.logPaused ? '继续' : '暂停'" @click="pauseLog">
        <svg v-if="!state.logPaused" viewBox="0 0 24 24"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
        <svg v-else viewBox="0 0 24 24"><polygon points="7 4 19 12 7 20 7 4"/></svg>
      </span>
      <span class="lbtn" title="清空日志" @click="clearLog">
        <svg viewBox="0 0 24 24"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
      </span>
      <input class="log-search" v-model="kw" placeholder="🔍 搜索日志">
      <span class="filters">决策
        <select v-model="fltDec">
          <option>全部</option><option>本地</option><option>网关</option>
        </select>
        服务 <select v-model="fltSvc">
          <option>全部</option>
          <option v-for="s in svcOptions" :key="s">{{ s }}</option>
        </select>
      </span>
    </div>
    <div class="log-body">
      <template v-if="visibleLogs.length">
        <div v-for="(l, i) in visibleLogs" :key="l.t + l.s + l.p + l.final + i" class="log-entry">
          <div class="log-line">
            <span class="t">{{ l.t }}</span>
            <span class="m">{{ l.m }}</span>
            <span class="p" :title="l.p">{{ l.p }}</span>
            <span class="s" :title="l.s">{{ l.s }}</span>
            <span :class="l.dec === '本地' ? 'lg' : 'lt'">
              <span class="dot" :class="l.dec === '本地' ? 'd-on' : 'd-warn'"></span>{{ l.dec }}
            </span>
            <span class="c">{{ l.c }}</span>
            <span class="ms">{{ l.ms }}ms</span>
          </div>
          <div v-if="l.target" class="log-sub" :class="l.c === 200 ? 'ok' : 'bad'"><span class="go">↳ </span>{{ l.target }}{{ l.final }}</div>
        </div>
      </template>
      <div v-else class="log-line" style="color:var(--dim)">暂无日志…</div>
    </div>
  </div>
</template>

<style scoped>
.lbtn { display:flex; align-items:center; justify-content:center; width:22px; height:22px; }
.lbtn svg { width:14px; height:14px; stroke:currentColor; fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.log-search { width:150px; padding:3px 9px; font-size:11px; border-radius:6px; border:1px solid var(--line); background:var(--card); color:var(--text); outline:none; }
.log-search:focus { border-color:var(--blue); }
</style>
