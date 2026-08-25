<script setup>
import { reactive } from 'vue'
import ModalShell from './ModalShell.vue'
import { state, App, run, toast, rulesOf } from '../../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const props = defineProps({
  edit: { type: Object, default: null },
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
}

async function save() {
  if (!svcName) { toast(t('modals.addRule.noSvc'), 'err'); return }
  const path = form.path.trim() || '/**'
  const prio = parseInt(form.prio, 10) || 1
  const strip = form.stripPrefix.trim()
  if (editing) {
    await run(() => App.EditRule(svcName, props.edit.idx, path, form.dest, prio, form.remark.trim(), strip), t('modals.addRule.saved', { path }))
  } else {
    await run(() => App.AddRule(svcName, path, form.dest, prio, form.remark.trim(), strip), t('modals.addRule.added', { path }))
  }
  emit('close')
}
</script>

<template>
  <ModalShell :title="editing ? t('modals.addRule.editTitle') : t('modals.addRule.addTitle')" @close="emit('close')">
    <div class="frow"><label>{{ t('modals.addRule.service') }}</label><span style="color:var(--text);font-weight:600">{{ svcName }}</span></div>
    <div class="frow"><label>{{ t('modals.addRule.path') }}</label><input v-model="form.path" :placeholder="t('modals.addRule.path')"></div>
    <div class="frow"><label>{{ t('modals.addRule.dest') }}</label>
      <select v-model="form.dest">
        <option value="本地">{{ t('common.local') }}</option>
        <option value="网关">{{ t('common.gateway') }}</option>
      </select>
    </div>
    <div class="frow"><label>{{ t('modals.addRule.priority') }}</label><input v-model="form.prio" type="number"></div>
    <div class="frow"><label>{{ t('modals.addRule.stripPrefix') }}</label><input v-model="form.stripPrefix" :placeholder="t('modals.addRule.stripPh')"></div>
    <div class="frow"><label>{{ t('modals.addRule.remark') }}</label><input v-model="form.remark" :placeholder="t('modals.addRule.remarkPh')"></div>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="m-btn primary" @click="save">{{ editing ? t('common.save') : t('modals.addRule.add') }}</button>
    </div>
  </ModalShell>
</template>