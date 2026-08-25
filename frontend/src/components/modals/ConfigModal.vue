<script setup>
import { ref, onMounted } from 'vue'
import ModalShell from './ModalShell.vue'
import { App, run, toast } from '../../store'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()

const emit = defineEmits(['close'])
const yaml = ref('')

onMounted(async () => {
  yaml.value = await App.PreviewYAML()
})

async function openDir() {
  await run(() => App.OpenConfigDir())
}

async function save() {
  await run(() => App.SaveConfig(), t('modals.config.saved'))
  yaml.value = await App.PreviewYAML()
  emit('close')
}
</script>

<template>
  <ModalShell :title="t('modals.config.title')" width="620px" @close="emit('close')">
    <div class="frow" style="align-items:center">
      <span style="font-weight:600">{{ t('modals.config.file') }}</span>
      <span style="color:var(--dim)">~/.voyagerGate/config.yaml</span>
      <button class="btn" style="margin-left:auto" @click="openDir">{{ t('modals.config.openDir') }}</button>
    </div>
    <pre class="cfg-pre">{{ yaml }}</pre>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">{{ t('common.close') }}</button>
      <button class="m-btn primary" @click="save">{{ t('common.save') }}</button>
    </div>
  </ModalShell>
</template>
