<script setup>
import { ref } from 'vue'
import ModalShell from './ModalShell.vue'
import ConfirmModal from './ConfirmModal.vue'
import { state, App, run, toast } from '../../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const emit = defineEmits(['close', 'new-env'])
const confirmDel = ref('') // 待删除环境名

async function enter(name) {
  if (name !== state.snapshot?.current) {
    await run(() => App.SwitchEnvironment(name))
    toast(t('modals.envMgr.switched', { name }))
  }
  emit('close')
}

async function exportEnv(name) {
  try {
    const p = await App.ExportEnvironment(name)
    toast(t('modals.envMgr.exported', { path: p }))
  } catch (e) {
    toast(String(e && e.message ? e.message : e), 'err')
  }
}

async function del(name) {
  if ((state.snapshot?.envs || []).length <= 1) { toast(t('modals.envMgr.atLeastOne'), 'err'); return }
  confirmDel.value = name
}
async function doDel() {
  if (!confirmDel.value) return
  const name = confirmDel.value
  confirmDel.value = ''
  await run(() => App.RemoveEnvironment(name))
  toast(t('modals.envMgr.deleted', { name }))
}

const renameTarget = ref('')
const renameName = ref('')
function startRename(name) {
  renameTarget.value = name
  renameName.value = name
}
function cancelRename() {
  renameTarget.value = ''
}
async function doRename() {
  const oldN = renameTarget.value
  const newN = renameName.value.trim()
  if (!newN) { toast(t('modals.envMgr.renameEmpty'), 'err'); return }
  await run(() => App.RenameEnvironment(oldN, newN), t('modals.envMgr.renameSaved', { name: newN }))
  renameTarget.value = ''
}

function runningOf(name) {
  return state.snapshot?.running && state.snapshot?.current === name
}
</script>

<template>
  <ModalShell :title="t('modals.envMgr.title')" width="820px" @close="emit('close')">
    <table class="env-table">
      <thead>
        <tr>
          <th>{{ t('modals.envMgr.colName') }}</th><th>{{ t('modals.envMgr.colReg') }}</th><th>{{ t('modals.envMgr.colPort') }}</th><th>{{ t('modals.envMgr.colStatus') }}</th><th>{{ t('modals.envMgr.colOps') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="e in (state.snapshot?.envs || [])" :key="e.name">
        <td>
          <template v-if="renameTarget === e.name">
            <input class="field" v-model="renameName" style="width:150px" @keyup.enter="doRename" @keyup.esc="cancelRename">
            <a @click="doRename">{{ t('common.save') }}</a>
            <a @click="cancelRename">{{ t('common.cancel') }}</a>
          </template>
          <template v-else>
            <b>{{ e.name }}</b>{{ e.name === state.snapshot?.current ? ` (${t('common.current')})` : '' }}
            <a class="rename" @click="startRename(e.name)">{{ t('modals.envMgr.rename') }}</a>
          </template>
        </td>
        <td>{{ e.reg?.type }} {{ e.reg?.addr }}</td>
        <td>{{ e.port }}</td>
        <td>{{ runningOf(e.name) ? t('modals.envMgr.acting') : t('modals.envMgr.idle') }}</td>
        <td>
          <a @click="enter(e.name)">{{ t('modals.envMgr.enter') }}</a>
          <a @click="exportEnv(e.name)">{{ t('modals.envMgr.export') }}</a>
          <a class="del" @click="del(e.name)">{{ t('modals.envMgr.delete') }}</a>
        </td>
      </tr>
      </tbody>
    </table>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">{{ t('modals.envMgr.close') }}</button>
      <button class="m-btn primary" @click="emit('close'); emit('new-env')">＋ {{ t('modals.envMgr.newEnv') }}</button>
    </div>

    <ConfirmModal
      v-if="confirmDel"
      :title="t('modals.envMgr.delTitle')"
      :message="t('modals.envMgr.delMsg', { name: confirmDel })"
      :ok-text="t('common.delete')"
      @cancel="confirmDel = ''"
      @ok="doDel"
    />
  </ModalShell>
</template>
