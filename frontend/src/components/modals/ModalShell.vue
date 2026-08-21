<script setup>
defineProps({
  title: { type: String, default: '' },
  width: { type: String, default: '560px' },
})
const emit = defineEmits(['close'])
</script>

<template>
  <div class="mask" @click.self="emit('close')">
    <div class="modal" :style="{ width }">
      <h3 v-if="title">{{ title }}</h3>
      <slot />
    </div>
  </div>
</template>
