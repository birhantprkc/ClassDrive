<template>
  <div
    v-if="item"
    ref="dialogBackdropRef"
    class="preview-dialog-backdrop"
    :class="{ 'preview-dialog-backdrop--maximized': isMaximized }"
    role="dialog"
    aria-modal="true"
    aria-labelledby="file-preview-title"
    data-testid="file-preview-dialog"
    @click.self="emit('close')"
  >
    <section
      class="preview-dialog"
      :class="{ 'preview-dialog--maximized': isMaximized }"
      data-testid="file-preview-surface"
    >
      <header class="preview-dialog__header">
        <div>
          <div class="preview-dialog__eyebrow">文件预览</div>
          <h3 id="file-preview-title" class="preview-dialog__title">{{ item.name }}</h3>
        </div>
        <div class="preview-dialog__actions">
          <button
            class="button button--ghost preview-dialog__nav-button"
            type="button"
            data-testid="file-preview-previous"
            :disabled="!hasPrevious"
            @click="$emit('previous')"
          >
            上一个文件
          </button>
          <button
            class="button button--ghost preview-dialog__nav-button"
            type="button"
            data-testid="file-preview-next"
            :disabled="!hasNext"
            @click="$emit('next')"
          >
            下一个文件
          </button>
          <button
            v-if="canEdit"
            class="button button--primary"
            type="button"
            data-testid="file-preview-edit"
            @click="$emit('edit')"
          >
            编辑
          </button>
          <button
            class="button button--ghost preview-dialog__maximize-button"
            type="button"
            data-testid="file-preview-maximize"
            :aria-pressed="isMaximized"
            @click="toggleMaximized"
          >
            {{ isMaximized ? "还原" : "最大化" }}
          </button>
          <button class="button button--ghost" type="button" data-testid="file-preview-close" @click="$emit('close')">关闭</button>
        </div>
      </header>

      <div class="preview-dialog__body" :class="{ 'preview-dialog__body--image': kind === 'image' }">
        <p v-if="loading" class="muted">正在加载预览...</p>
        <p v-else-if="errorText" class="form-error" data-testid="file-preview-error">{{ errorText }}</p>
        <img v-else-if="kind === 'image'" class="preview-dialog__image" data-testid="file-preview-image" :src="item.previewUrl" :alt="item.name" />
        <iframe
          v-else-if="kind === 'pdf'"
          class="preview-dialog__frame"
          data-testid="file-preview-pdf"
          :src="item.previewUrl"
          title="PDF 预览"
        ></iframe>
        <audio v-else-if="kind === 'audio'" class="preview-dialog__media" data-testid="file-preview-audio" controls :src="item.previewUrl"></audio>
        <video v-else-if="kind === 'video'" class="preview-dialog__media preview-dialog__video" data-testid="file-preview-video" controls :src="item.previewUrl"></video>
        <pre v-else-if="kind === 'text'" class="preview-dialog__text" data-testid="file-preview-text">{{ textContent }}</pre>
        <div v-else-if="kind === 'html'" class="preview-dialog__html-shell" data-testid="file-preview-html">
          <p class="preview-dialog__sandbox-hint">沙箱预览：网页脚本在受限环境中运行，无法访问本系统数据。</p>
          <iframe
            class="preview-dialog__frame"
            data-testid="file-preview-html-frame"
            :src="item.previewUrl"
            sandbox="allow-scripts"
            referrerpolicy="no-referrer"
            title="HTML 沙箱预览"
          ></iframe>
        </div>
        <div v-else-if="kind === 'markdown'" class="preview-dialog__markdown" data-testid="file-preview-markdown" v-html="markdownContent"></div>
        <div v-else class="preview-dialog__unsupported" data-testid="file-preview-external">
          <p>该文件不支持预览，可下载后查看。</p>
          <a class="button button--primary" :href="item.downloadUrl" target="_blank" rel="noreferrer">下载文件</a>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useFocusTrap } from "@/composables/useFocusTrap";
import type { FilePreviewKind } from "@/utils/file-preview";

interface PreviewDialogItem {
  id: number;
  name: string;
  kind: "file" | "dir";
  previewUrl: string;
  downloadUrl: string;
  mimeType?: string;
}

const props = withDefaults(defineProps<{
  item: PreviewDialogItem | null;
  kind: FilePreviewKind | null;
  loading: boolean;
  errorText: string;
  textContent: string;
  markdownContent?: string;
  canEdit: boolean;
  hasPrevious?: boolean;
  hasNext?: boolean;
}>(), {
  hasPrevious: false,
  hasNext: false,
  markdownContent: "",
});

const emit = defineEmits<{
  close: [];
  edit: [];
  previous: [];
  next: [];
}>();

const dialogBackdropRef = ref<HTMLElement | null>(null);
const isMaximized = ref(false);

const isOpen = computed(() => props.item !== null);

useFocusTrap(dialogBackdropRef, () => emit("close"), isOpen);

function toggleMaximized(): void {
  isMaximized.value = !isMaximized.value;
}

watch(
  () => props.item,
  (nextItem) => {
    if (!nextItem) {
      isMaximized.value = false;
    }
  }
);
</script>

<style scoped>
.preview-dialog-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.66);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  z-index: 5060;
}

.preview-dialog-backdrop--maximized {
  align-items: stretch;
  justify-content: stretch;
  padding: 10px;
}

.preview-dialog {
  width: min(980px, 100%);
  max-height: calc(100vh - 48px);
  background: var(--modal-surface);
  border: 1px solid var(--border-soft);
  border-radius: 18px;
  box-shadow: var(--shadow-strong);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.preview-dialog--maximized {
  width: 100%;
  height: calc(100vh - 20px);
  max-height: none;
  border-radius: 10px;
}

.preview-dialog__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-soft);
  background: var(--modal-surface);
}

.preview-dialog__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-dialog__nav-button {
  min-height: 34px;
  padding: 6px 10px;
  border-radius: 8px;
}

.preview-dialog__nav-button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.preview-dialog__maximize-button {
  min-height: 34px;
  padding: 6px 12px;
  border-radius: 8px;
}

.preview-dialog__eyebrow {
  color: var(--text-muted);
  font-size: 12px;
  margin-bottom: 4px;
}

.preview-dialog__title {
  margin: 0;
  font-size: 18px;
  color: var(--text-primary);
}

.preview-dialog__body {
  flex: 1;
  min-height: 62vh;
  padding: 20px;
  overflow: auto;
  background: var(--modal-surface);
}

.preview-dialog--maximized .preview-dialog__body {
  min-height: 0;
}

.preview-dialog__body--image {
  display: grid;
  place-items: center;
  overflow: hidden;
}

.preview-dialog__image {
  display: block;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: calc(100vh - 180px);
  object-fit: contain;
  object-position: center;
}

.preview-dialog--maximized .preview-dialog__image {
  max-height: calc(100vh - 106px);
}

.preview-dialog__frame,
.preview-dialog__video {
  width: 100%;
}

.preview-dialog__frame {
  min-height: 70vh;
  border: 0;
}

.preview-dialog--maximized .preview-dialog__frame,
.preview-dialog--maximized .preview-dialog__video {
  height: 100%;
  min-height: 0;
}

.preview-dialog__html-shell {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  min-height: 70vh;
}

.preview-dialog__html-shell .preview-dialog__frame {
  flex: 1;
  min-height: 60vh;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  background: #ffffff;
}

.preview-dialog__sandbox-hint {
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--border-soft);
  border-radius: 8px;
  background: var(--control-bg);
  color: var(--text-muted);
  font-size: 12px;
}

.preview-dialog__markdown {
  margin: 0;
  min-height: 62vh;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  padding: 18px 22px;
  background: var(--modal-surface);
  color: var(--text-primary);
  line-height: 1.7;
  word-break: break-word;
}

.preview-dialog--maximized .preview-dialog__markdown {
  min-height: 100%;
}

.preview-dialog__markdown :deep(h1),
.preview-dialog__markdown :deep(h2),
.preview-dialog__markdown :deep(h3) {
  margin: 0.9em 0 0.5em;
  line-height: 1.3;
}

.preview-dialog__markdown :deep(h1:first-child),
.preview-dialog__markdown :deep(h2:first-child),
.preview-dialog__markdown :deep(h3:first-child) {
  margin-top: 0;
}

.preview-dialog__markdown :deep(p),
.preview-dialog__markdown :deep(ul),
.preview-dialog__markdown :deep(ol) {
  margin: 0.6em 0;
}

.preview-dialog__markdown :deep(pre),
.preview-dialog__markdown :deep(code) {
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
}

.preview-dialog__markdown :deep(pre) {
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--control-bg);
  overflow: auto;
}

.preview-dialog__markdown :deep(code) {
  background: var(--control-bg);
  border-radius: 4px;
  padding: 2px 5px;
}

.preview-dialog__markdown :deep(pre code) {
  background: none;
  padding: 0;
}

.preview-dialog__markdown :deep(a) {
  color: var(--accent, #2563eb);
}

.preview-dialog__markdown :deep(img) {
  max-width: 100%;
}

.preview-dialog__markdown :deep(blockquote) {
  margin: 0.8em 0;
  padding: 4px 14px;
  border-left: 3px solid var(--border-soft);
  color: var(--text-secondary);
}

.preview-dialog__markdown :deep(table) {
  border-collapse: collapse;
  margin: 0.8em 0;
}

.preview-dialog__markdown :deep(th),
.preview-dialog__markdown :deep(td) {
  border: 1px solid var(--border-soft);
  padding: 6px 12px;
}

.preview-dialog__media {
  width: 100%;
}

.preview-dialog__text {
  margin: 0;
  min-height: 62vh;
  border: 1px solid var(--control-border);
  border-radius: 8px;
  padding: 14px 16px;
  background: var(--control-bg);
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.6;
}

.preview-dialog--maximized .preview-dialog__text {
  min-height: 100%;
}

:global(:root.dark .preview-dialog-backdrop) {
  background: rgba(2, 6, 23, 0.78);
}

:global(:root.dark .preview-dialog) {
  border-color: rgba(125, 211, 252, 0.2);
}

.preview-dialog__unsupported {
  min-height: 42vh;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 14px;
  color: var(--text-secondary);
  text-align: center;
}

.preview-dialog__unsupported p {
  margin: 0;
}
</style>
