<script setup>
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { App } from '../../store'

const props = defineProps({
  title: { type: String, default: '确认' },
  message: { type: String, default: '' },
  okText: { type: String, default: '确认' },
  width: { type: String, default: '380px' },
  md: { type: Boolean, default: false },
  danger: { type: Boolean, default: true },
})
const emit = defineEmits(['ok', 'cancel'])

marked.setOptions({ breaks: true, gfm: true })

const html = computed(() =>
  props.md ? DOMPurify.sanitize(marked.parse(props.message || '')) : ''
)

// 弹窗内链接改为系统浏览器打开，避免 WebView 导航离开应用
function onLinkClick(e) {
  const a = e.target.closest('a')
  if (a && a.href) {
    e.preventDefault()
    App.OpenURL(a.href)
  }
}
</script>

<template>
  <div class="mask" @click.self="emit('cancel')">
    <div class="modal" :style="{ width: width, maxHeight: '85vh', display: 'flex', flexDirection: 'column' }">
      <h3>{{ title }}</h3>
      <div v-if="md" class="cm-msg md" v-html="html" @click="onLinkClick"></div>
      <p v-else class="cm-msg">{{ message }}</p>
      <div class="m-actions">
        <button class="m-btn" @click="emit('cancel')">取消</button>
        <button class="m-btn primary" :style="danger ? 'background:rgba(255,69,58,.9);border-color:rgba(255,69,58,.9);color:#fff' : ''" @click="emit('ok')">{{ okText }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cm-msg {
  font-size: 12px;
  color: var(--sub);
  line-height: 1.8;
  white-space: pre-line;
  overflow-wrap: anywhere;
  overflow-y: auto;
  max-height: 60vh;
}
.cm-msg::-webkit-scrollbar { width: 8px; }
.cm-msg::-webkit-scrollbar-thumb { background: var(--line2); border-radius: 4px; }

/* Markdown 渲染后的基础排版 */
.cm-msg.md { white-space: normal; }
.cm-msg.md :deep(h1),
.cm-msg.md :deep(h2),
.cm-msg.md :deep(h3) { font-size: 14px; color: var(--text); margin: 10px 0 6px; line-height: 1.4; }
.cm-msg.md :deep(h1) { font-size: 16px; }
.cm-msg.md :deep(p) { margin: 6px 0; }
.cm-msg.md :deep(ul),
.cm-msg.md :deep(ol) { margin: 6px 0; padding-left: 20px; }
.cm-msg.md :deep(li) { margin: 3px 0; }
.cm-msg.md :deep(a) { color: var(--blue); text-decoration: underline; cursor: pointer; }
.cm-msg.md :deep(code) {
  background: rgba(255,255,255,.06);
  padding: 1px 5px;
  border-radius: 4px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
}
.cm-msg.md :deep(pre) {
  background: rgba(255,255,255,.06);
  padding: 8px 10px;
  border-radius: 6px;
  overflow-x: auto;
}
.cm-msg.md :deep(pre code) { background: none; padding: 0; }
.cm-msg.md :deep(strong) { color: var(--text); }
.cm-msg.md :deep(blockquote) { border-left: 3px solid var(--line2); margin: 6px 0; padding-left: 10px; color: var(--dim); }
</style>
