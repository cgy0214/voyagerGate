<script setup>
import { ref } from 'vue'
import ModalShell from './ModalShell.vue'
import ConfirmModal from './ConfirmModal.vue'
import { state, App, run, toast } from '../../store'

const emit = defineEmits(['close', 'new-env'])
const confirmDel = ref('') // 待删除环境名

async function enter(name) {
  if (name !== state.snapshot?.current) {
    await run(() => App.SwitchEnvironment(name))
    toast(`已切换环境: ${name}`)
  }
  emit('close')
}

async function exportEnv(name) {
  try {
    const p = await App.ExportEnvironment(name)
    toast(`已导出: ${p}`)
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  }
}

async function del(name) {
  if ((state.snapshot?.envs || []).length <= 1) { toast('至少保留一个环境', 'err'); return }
  confirmDel.value = name
}
async function doDel() {
  if (!confirmDel.value) return
  const name = confirmDel.value
  confirmDel.value = ''
  await run(() => App.RemoveEnvironment(name))
  toast(`已删除 ${name}`)
}

function runningOf(name) {
  return state.snapshot?.running && state.snapshot?.current === name
}
</script>

<template>
  <ModalShell title="环境管理" width="620px" @close="emit('close')">
    <table class="env-table">
      <tr>
        <th>名称</th><th>注册中心</th><th>端口</th><th>状态</th><th>操作</th>
      </tr>
      <tr v-for="e in (state.snapshot?.envs || [])" :key="e.name">
        <td><b>{{ e.name }}</b>{{ e.name === state.snapshot?.current ? ' (当前)' : '' }}</td>
        <td>{{ e.reg?.type }} {{ e.reg?.addr }}</td>
        <td>{{ e.port }}</td>
        <td>{{ runningOf(e.name) ? '代理中' : '未代理' }}</td>
        <td>
          <a @click="enter(e.name)">进入</a>
          <a @click="exportEnv(e.name)">导出</a>
          <a class="del" @click="del(e.name)">删除</a>
        </td>
      </tr>
    </table>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">关闭</button>
      <button class="m-btn primary" @click="emit('close'); emit('new-env')">＋ 新建环境</button>
    </div>

    <ConfirmModal
      v-if="confirmDel"
      title="删除环境"
      :message="`确认删除环境 ${confirmDel} ?\n该环境下的服务与规则将被一并移除。`"
      ok-text="删除"
      @cancel="confirmDel = ''"
      @ok="doDel"
    />
  </ModalShell>
</template>
