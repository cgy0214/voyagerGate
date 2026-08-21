<script setup>
import { reactive } from 'vue'
import ModalShell from './ModalShell.vue'
import { state, App, run, toast, rulesOf } from '../../store'

const props = defineProps({
  edit: { type: Object, default: null }, // { svc, idx } 存在则进入编辑模式
})
const emit = defineEmits(['close'])

const editing = !!props.edit
const svcName = props.edit?.svc || state.selected

const form = reactive({ path: '/**', dest: '本地', prio: 1, remark: '', stripPrefix: '' })
if (editing) {
  const r = rulesOf(svcName)[props.edit.idx]
  if (r) {
    form.path = r.path
    form.dest = r.dest
    form.prio = r.prio
    form.remark = r.remark || ''
    form.stripPrefix = r.stripPrefix || ''
  }
} else {
  // 编辑时由上方 r.stripPrefix 覆盖，此处无需额外处理
}

async function save() {
  if (!svcName) { toast('请先选择服务', 'err'); return }
  const path = form.path.trim() || '/**'
  const prio = parseInt(form.prio, 10) || 1
  const strip = form.stripPrefix.trim()
  if (editing) {
    await run(() => App.EditRule(svcName, props.edit.idx, path, form.dest, prio, form.remark.trim(), strip), `已保存规则 ${path}`)
  } else {
    await run(() => App.AddRule(svcName, path, form.dest, prio, form.remark.trim(), strip), `已添加规则 ${path}`)
  }
  emit('close')
}
</script>

<template>
  <ModalShell :title="editing ? '编辑路由规则' : '添加路由规则'" @close="emit('close')">
    <div class="frow"><label>服务</label><span style="color:var(--text);font-weight:600">{{ svcName }}</span></div>
    <div class="frow"><label>匹配路径</label><input v-model="form.path" placeholder="/order/**"></div>
    <div class="frow"><label>目标</label>
      <select v-model="form.dest">
        <option>本地</option>
        <option>网关</option>
      </select>
    </div>
    <div class="frow"><label>优先级</label><input v-model="form.prio" type="number"></div>
    <div class="frow"><label>去前缀</label><input v-model="form.stripPrefix" placeholder="如 /gateway-server（留空则保留）"></div>
    <div class="frow"><label>备注</label><input v-model="form.remark" placeholder="描述"></div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">取消</button>
      <button class="m-btn primary" @click="save">{{ editing ? '保存' : '添加' }}</button>
    </div>
  </ModalShell>
</template>