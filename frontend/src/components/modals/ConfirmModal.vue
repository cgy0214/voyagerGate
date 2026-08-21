<script setup>
defineProps({
  title: { type: String, default: '确认' },
  message: { type: String, default: '' },
  okText: { type: String, default: '确认' },
  danger: { type: Boolean, default: true },
})
const emit = defineEmits(['ok', 'cancel'])
</script>

<template>
  <div class="mask" @click.self="emit('cancel')">
    <div class="modal" style="width:380px">
      <h3>{{ title }}</h3>
      <p style="font-size:12px;color:var(--sub);line-height:1.8;white-space:pre-line">{{ message }}</p>
      <div class="m-actions">
        <button class="m-btn" @click="emit('cancel')">取消</button>
        <button class="m-btn primary" :style="danger ? 'background:rgba(255,69,58,.9);border-color:rgba(255,69,58,.9);color:#fff' : ''" @click="emit('ok')">{{ okText }}</button>
      </div>
    </div>
  </div>
</template>