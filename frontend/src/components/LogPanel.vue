<script setup>
import { computed, ref, watch } from 'vue'
import { state } from '../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const fltDec = ref('all')
const fltSvc = ref('all')
const kw = ref('') // 日志关键字搜索

// 后端决策文字（本地/网关）映射到稳定的过滤 key
function decKey(dec) {
  if (dec === '本地' || dec === 'Local') return 'local'
  if (dec === '网关' || dec === 'Gateway') return 'gateway'
  return ''
}

const visibleLogs = computed(() => {
  const q = kw.value.trim().toLowerCase()
  return state.logs.filter(l => {
    if (fltDec.value !== 'all' && decKey(l.dec) !== fltDec.value) return false
    if (fltSvc.value !== 'all' && l.sname !== fltSvc.value) return false
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
  if (fltSvc.value !== 'all' && !names.includes(fltSvc.value)) fltSvc.value = 'all'
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
      <span>▍ {{ t('log.title') }}</span>
      <span class="lbtn" :title="state.logPaused ? t('log.resume') : t('log.pause')" @click="pauseLog">
        <svg v-if="!state.logPaused" viewBox="0 0 24 24"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
        <svg v-else viewBox="0 0 24 24"><polygon points="7 4 19 12 7 20 7 4"/></svg>
      </span>
      <span class="lbtn" :title="t('log.clear')" @click="clearLog">
        <svg viewBox="0 0 24 24"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
      </span>
      <input class="log-search" v-model="kw" :placeholder="t('log.search')">
      <span class="filters">{{ t('log.decision') }}
        <select v-model="fltDec">
          <option value="all">{{ t('common.all') }}</option><option value="local">{{ t('common.local') }}</option><option value="gateway">{{ t('common.gateway') }}</option>
        </select>
        {{ t('log.service') }} <select v-model="fltSvc">
          <option value="all">{{ t('common.all') }}</option>
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
      <div v-else class="log-line" style="color:var(--dim)">{{ t('log.noLogs') }}</div>
    </div>
  </div>
</template>

<style scoped>
.lbtn { display:flex; align-items:center; justify-content:center; width:22px; height:22px; }
.lbtn svg { width:14px; height:14px; stroke:currentColor; fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.log-search { width:150px; padding:3px 9px; font-size:11px; border-radius:6px; border:1px solid var(--line); background:var(--card); color:var(--text); outline:none; }
.log-search:focus { border-color:var(--blue); }
</style>
