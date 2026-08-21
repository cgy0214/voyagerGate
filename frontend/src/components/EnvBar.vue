<script setup>
import { state, App, run, toast } from '../store'

function savePort() {
  const port = parseInt(document.getElementById('portInput').value, 10)
  if (!port || port < 1 || port > 65535) {
    toast('端口无效', 'err')
    return
  }
  run(() => App.SetPort(port), `代理端口已改为 ${port}`)
}

// 复制 IP:端口（兼容 document.execCommand 降级）
function copyIp() {
  const txt = `${state.snapshot?.localIp || '127.0.0.1'}:${state.current?.port || ''}`
  try {
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(txt).then(() => toast(`已复制: ${txt}`)).catch(() => legacyCopy(txt))
    } else {
      legacyCopy(txt)
    }
  } catch {
    legacyCopy(txt)
  }
}

function legacyCopy(txt) {
  const ta = document.createElement('textarea')
  ta.value = txt
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
    toast(`已复制: ${txt}`)
  } catch {
    toast('复制失败', 'err')
  }
  ta.remove()
}

async function toggleStart() {
  if (state.snapshot?.running) {
    await run(() => App.StopProxy(), '代理已停止')
  } else {
    await run(() => App.StartProxy(), '代理已启动')
  }
}
</script>

<template>
  <div class="envbar glass">
    <span class="status-on" v-if="state.snapshot?.running"><span class="status-dot dot-green"></span>运行中</span>
    <span class="status-off" v-else><span class="status-dot dot-red"></span>已停止</span>

    <span class="kv">代理端口
      <input class="port-input" id="portInput" :value="state.current?.port ?? ''">
      <button class="mini-btn" @click="savePort">✓</button>
    </span>

    <span class="kv">本机IP <b>{{ state.snapshot?.localIp || '…' }}</b></span>
    <span class="ip-copy" title="复制 IP:端口" @click="copyIp">
      <svg viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
      <span class="ip-txt">{{ state.snapshot?.localIp || '…' }}:{{ state.current?.port ?? '' }}</span>
    </span>

    <button
      class="start-btn"
      :class="{ stopped: !state.snapshot?.running }"
      @click="toggleStart"
    >
      <svg v-if="state.snapshot?.running" viewBox="0 0 24 24"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
      <svg v-else viewBox="0 0 24 24"><polygon points="7 4 19 12 7 20 7 4"/></svg>
      {{ state.snapshot?.running ? '停止' : '启动' }}
    </button>
  </div>
</template>

<style scoped>
.ip-copy { display:flex; align-items:center; gap:5px; color:var(--dim); cursor:pointer; font-size:12px; user-select:none; transition:color .15s; }
.ip-copy svg { width:13px; height:13px; stroke:currentColor; fill:none; stroke-width:2; stroke-linecap:round; stroke-linejoin:round; }
.ip-copy:hover { color:var(--blue); }
.ip-copy .ip-txt { color:var(--sub); }
.ip-copy:hover .ip-txt { color:var(--blue); }
.start-btn svg { width:15px; height:15px; stroke:currentColor; fill:currentColor; stroke-width:0; }
</style>
