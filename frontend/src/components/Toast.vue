<script setup>
import { state } from '../store'
</script>

<template>
  <transition name="toastfade">
    <div class="toast" :class="state.toastType === 'err' ? 'err' : 'ok'" v-if="state.toastMsg">{{ state.toastMsg }}</div>
  </transition>
</template>

<style scoped>
.toastfade-enter-active, .toastfade-leave-active { transition: opacity .18s, transform .18s; }
.toastfade-enter-from, .toastfade-leave-to { opacity: 0; transform: translateX(-50%) translateY(6px); }
</style>
