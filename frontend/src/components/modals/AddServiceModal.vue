<script setup>
import { reactive } from 'vue'
import ModalShell from './ModalShell.vue'
import { App, run, state, toast } from '../../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const emit = defineEmits(['close'])
const form = reactive({ name: '', addr: '', remark: '' })

async function add() {
  const name = form.name.trim()
  if (!name) { toast(t('modals.addSvc.noName'), 'err'); return }
  await run(() => App.AddService(name, form.addr.trim(), form.remark.trim()), t('modals.addSvc.added', { name }))
  state.selected = name
  emit('close')
}
</script>

<template>
  <ModalShell :title="t('modals.addSvc.title')" @close="emit('close')">
    <div class="frow"><label>{{ t('modals.addSvc.name') }}</label><input v-model="form.name" :placeholder="t('modals.addSvc.name')"></div>
    <div class="frow"><label>{{ t('modals.addSvc.addr') }}</label><input v-model="form.addr" :placeholder="t('modals.addSvc.addrPh')"></div>
    <div class="frow"><label>{{ t('modals.addSvc.remark') }}</label><input v-model="form.remark" :placeholder="t('modals.addSvc.remarkPh')"></div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="m-btn primary" @click="add">{{ t('common.save') }}</button>
    </div>
  </ModalShell>
</template>
