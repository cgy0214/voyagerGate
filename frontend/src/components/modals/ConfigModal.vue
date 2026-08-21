<script setup>
import { ref, onMounted } from 'vue'
import ModalShell from './ModalShell.vue'
import { App, run, toast } from '../../store'

const emit = defineEmits(['close'])
const yaml = ref('')

onMounted(async () => {
  yaml.value = await App.PreviewYAML()
})

async function openDir() {
  await run(() => App.OpenConfigDir())
}

async function save() {
  await run(() => App.SaveConfig(), '配置已保存 ✓')
  yaml.value = await App.PreviewYAML()
  emit('close')
}
</script>

<template>
  <ModalShell width="620px" @close="emit('close')">
    <div class="frow" style="align-items:center">
      <span style="font-weight:600">配置文件</span>
      <span style="color:var(--dim)">~/.voyagerGate/config.yaml</span>
      <button class="btn" style="margin-left:auto" @click="openDir">打开目录</button>
    </div>
    <pre class="cfg-pre">{{ yaml }}</pre>
    <div class="m-actions">
      <button class="m-btn" @click="emit('close')">关闭</button>
      <button class="m-btn primary" @click="save">保存</button>
    </div>
  </ModalShell>
</template>
