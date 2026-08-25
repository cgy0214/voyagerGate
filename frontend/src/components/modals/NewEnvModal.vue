<script setup>
import { reactive, ref } from 'vue'
import ModalShell from './ModalShell.vue'
import { state, App, run, toast } from '../../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

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
  if (!name) { err.value = t('modals.newEnv.noName'); return }
  if ((state.snapshot?.envs || []).some(e => e.name === name)) { err.value = t('modals.newEnv.nameExists'); return }
  const port = parseInt(form.port, 10)
  if (!port || port < 1 || port > 65535) { err.value = t('modals.newEnv.invalidPort'); return }
  err.value = ''
  await run(() => App.AddEnvironment(name, buildReg(), form.gw.trim() || '', port, form.copyFrom))
  toast(t('modals.newEnv.created', { name }))
  emit('close')
}
</script>

<template>
  <ModalShell :title="t('modals.newEnv.title')" @close="emit('close')">
    <div class="frow"><label>{{ t('modals.newEnv.name') }}</label><input v-model="form.name" :placeholder="t('modals.newEnv.namePh')"></div>
    <div class="frow"><label>{{ t('modals.newEnv.registry') }}</label>
      <select v-model="form.type">
        <option>Nacos</option><option>Eureka</option><option>Consul</option>
      </select>
    </div>

    <template v-if="form.type === 'Nacos'">
      <div class="frow"><label>{{ t('modals.newEnv.address') }}</label><input v-model="form.addr" :placeholder="t('modals.newEnv.address') + ': 127.0.0.1:8848'"></div>
      <div class="frow"><label>{{ t('modals.newEnv.namespace') }}</label><input v-model="form.ns" :placeholder="t('modals.newEnv.namespace')"></div>
      <div class="frow"><label>{{ t('modals.newEnv.group') }}</label><input v-model="form.group" :placeholder="t('modals.newEnv.group')"></div>
    </template>
    <template v-else-if="form.type === 'Eureka'">
      <div class="frow"><label>{{ t('modals.newEnv.address') }}</label><input v-model="form.addr" :placeholder="t('modals.newEnv.address') + ': http://localhost:8761/eureka/'"></div>
      <div class="frow"><label>{{ t('modals.newEnv.username') }}</label><input v-model="form.user" :placeholder="t('modals.newEnv.username')"></div>
      <div class="frow"><label>{{ t('modals.newEnv.password') }}</label><input v-model="form.pass" type="password" :placeholder="t('modals.newEnv.password')"></div>
    </template>
    <template v-else>
      <div class="frow"><label>{{ t('modals.newEnv.address') }}</label><input v-model="form.addr" :placeholder="t('modals.newEnv.address') + ': localhost:8500'"></div>
      <div class="frow"><label>{{ t('modals.newEnv.datacenter') }}</label><input v-model="form.dc" :placeholder="t('modals.newEnv.datacenter')"></div>
      <div class="frow"><label>{{ t('modals.newEnv.token') }}</label><input v-model="form.token" type="password" :placeholder="t('modals.newEnv.token')"></div>
    </template>

    <div class="frow"><label>{{ t('modals.newEnv.gw') }}</label><input v-model="form.gw" :placeholder="t('modals.newEnv.gw')"></div>
    <div class="frow"><label>{{ t('modals.newEnv.proxyPort') }}</label><input v-model="form.port" type="number"></div>
    <div class="frow"><label>{{ t('modals.newEnv.copyFrom') }}</label>
      <select v-model="form.copyFrom">
        <option value="">{{ t('modals.newEnv.noCopy') }}</option>
        <option v-for="e in (state.snapshot?.envs || [])" :key="e.name" :value="e.name">{{ e.name }}</option>
      </select>
    </div>

    <div v-if="err" style="color:var(--red);font-size:12px;margin-bottom:8px">{{ err }}</div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="m-btn primary" @click="create">{{ t('modals.newEnv.createEnter') }}</button>
    </div>
  </ModalShell>
</template>
