<script setup>
import { reactive } from 'vue'
import ModalShell from './ModalShell.vue'
import { App, run, state, toast } from '../../store'

const emit = defineEmits(['close'])
const form = reactive({ name: '', addr: '', remark: '' })

async function add() {
  const name = form.name.trim()
  if (!name) { toast('请输入服务名', 'err'); return }
  await run(() => App.AddService(name, form.addr.trim(), form.remark.trim()), `已手动添加服务 ${name}`)
  state.selected = name
  emit('close')
}
</script>

<template>
  <ModalShell title="手动添加服务" @close="emit('close')">
    <div class="frow"><label>服务名</label><input v-model="form.name" placeholder="report-server"></div>
    <div class="frow"><label>本地地址</label><input v-model="form.addr" placeholder="127.0.0.1:9106（留空只登记不转发）"></div>
    <div class="frow"><label>备注</label><input v-model="form.remark" placeholder="本地单独启动的报表服务"></div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">取消</button>
      <button class="m-btn primary" @click="add">保存</button>
    </div>
  </ModalShell>
</template>
