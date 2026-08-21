<script setup>
import { reactive, ref } from 'vue'
import ModalShell from './ModalShell.vue'
import { state, App, run, toast } from '../../store'

const emit = defineEmits(['close'])
const form = reactive({ name: '', type: 'Nacos', addr: '', ns: '', group: '', user: '', pass: '', dc: '', token: '', gw: '', port: 6000, copyFrom: '' })
const err = ref('')

function buildReg() {
  const base = { type: form.type, addr: form.addr }
  if (form.type === 'Nacos') { base.ns = form.ns; base.group = form.group }
  else if (form.type === 'Eureka') { base.user = form.user; base.pass = form.pass }
  else { base.dc = form.dc; base.token = form.token }
  return base
}

async function create() {
  const name = form.name.trim()
  if (!name) { err.value = '请输入环境名称'; return }
  if ((state.snapshot?.envs || []).some(e => e.name === name)) { err.value = '环境名称已存在'; return }
  const port = parseInt(form.port, 10)
  if (!port || port < 1 || port > 65535) { err.value = '代理端口无效'; return }
  err.value = ''
  await run(() => App.AddEnvironment(name, buildReg(), form.gw.trim() || '', port, form.copyFrom))
  toast(`环境 ${name} 已创建`)
  emit('close')
}
</script>

<template>
  <ModalShell title="新建环境" @close="emit('close')">
    <div class="frow"><label>名称</label><input v-model="form.name" placeholder="如 uat-测试"></div>
    <div class="frow"><label>注册中心</label>
      <select v-model="form.type">
        <option>Nacos</option><option>Eureka</option><option>Consul</option>
      </select>
    </div>

    <template v-if="form.type === 'Nacos'">
      <div class="frow"><label>地址</label><input v-model="form.addr" placeholder="127.0.0.1:8848"></div>
      <div class="frow"><label>命名空间</label><input v-model="form.ns" placeholder="dev"></div>
      <div class="frow"><label>分组</label><input v-model="form.group" placeholder="DEFAULT_GROUP"></div>
    </template>
    <template v-else-if="form.type === 'Eureka'">
      <div class="frow"><label>地址</label><input v-model="form.addr" placeholder="http://localhost:8761/eureka/"></div>
      <div class="frow"><label>用户名</label><input v-model="form.user" placeholder="用户名（可选）"></div>
      <div class="frow"><label>密码</label><input v-model="form.pass" type="password" placeholder="密码（可选）"></div>
    </template>
    <template v-else>
      <div class="frow"><label>地址</label><input v-model="form.addr" placeholder="localhost:8500"></div>
      <div class="frow"><label>数据中心</label><input v-model="form.dc" placeholder="dc1"></div>
      <div class="frow"><label>Token</label><input v-model="form.token" type="password" placeholder="Token（可选）"></div>
    </template>

    <div class="frow"><label>网关地址</label><input v-model="form.gw" placeholder="https://voyagergate.com"></div>
    <div class="frow"><label>代理端口</label><input v-model="form.port" type="number"></div>
    <div class="frow"><label>从环境复制</label>
      <select v-model="form.copyFrom">
        <option value="">不复制</option>
        <option v-for="e in (state.snapshot?.envs || [])" :key="e.name" :value="e.name">{{ e.name }}</option>
      </select>
    </div>

    <div v-if="err" style="color:var(--red);font-size:12px;margin-bottom:8px">{{ err }}</div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">取消</button>
      <button class="m-btn primary" @click="create">创建并进入</button>
    </div>
  </ModalShell>
</template>
