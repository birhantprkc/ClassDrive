<template>
  <div :class="gridShellClass" :data-testid="testId">
    <p v-if="!fileRows.length && emptyMessage" class="muted">{{ emptyMessage }}</p>
    <div v-else class="submission-file-grid">
      <article
        v-for="row in fileRows"
        :key="row.item.id"
        class="submission-file-grid__item"
        :class="{ 'submission-file-grid__item--dir': row.item.kind === 'dir' }"
        :data-testid="resolveItemTestId(row.item.id)"
      >
        <button
          v-if="canPreview(row.item)"
          class="submission-file-grid__thumb"
          :class="{ 'submission-file-grid__thumb--image': isImagePreview(row.item) }"
          type="button"
          :data-testid="resolvePreviewTestId(row.item.id)"
          @click="$emit('preview', row.item)"
        >
          <img
            v-if="isImagePreview(row.item)"
            :src="row.item.previewUrl || row.item.downloadUrl"
            :alt="row.item.name"
            :data-testid="resolveThumbnailTestId(row.item.id)"
          />
          <span v-else>{{ previewKindLabel(row.item) }}</span>
        </button>
        <div v-else class="submission-file-grid__thumb submission-file-grid__thumb--static">
          <span>{{ row.item.kind === "dir" ? "文件夹" : "文件" }}</span>
        </div>
        <div class="submission-file-grid__body">
          <strong>{{ row.item.name }}</strong>
          <span v-if="row.pathLabel" class="muted">{{ row.pathLabel }}</span>
          <span class="muted">{{ itemMeta(row.item) }}</span>
        </div>
        <div class="submission-file-grid__actions">
          <a v-if="downloadable" class="button button--secondary" :href="downloadUrl(row.item)" :data-testid="resolveDownloadTestId(row.item.id)">
              {{ row.item.kind === "dir" ? "下载压缩包" : "下载" }}
          </a>
          <button
            v-if="deletable"
            class="button button--ghost text-button--danger"
            type="button"
            :data-testid="resolveDeleteTestId(row.item.id)"
            @click="$emit('delete', row.item)"
          >
            删除
          </button>
        </div>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { AssignmentAttachmentItem } from "@/api/client";
import { getFilePreviewKind, type FilePreviewKind } from "@/utils/file-preview";

interface SubmissionFileRow {
  item: AssignmentAttachmentItem;
  depth: number;
  pathLabel: string;
}

type SubmissionFileGridSize = "small" | "medium" | "large" | "xlarge";

const props = withDefaults(defineProps<{
  items: AssignmentAttachmentItem[];
  testId?: string;
  itemTestIdPrefix?: string;
  previewTestIdPrefix?: string;
  thumbnailTestIdPrefix?: string;
  downloadTestIdPrefix?: string;
  deleteTestIdPrefix?: string;
  emptyMessage?: string;
  gridSize?: SubmissionFileGridSize;
  deletable?: boolean;
  downloadable?: boolean;
}>(), {
  testId: "submission-file-grid",
  itemTestIdPrefix: "submission-file",
  previewTestIdPrefix: "submission-file-preview",
  thumbnailTestIdPrefix: "submission-file-thumb",
  downloadTestIdPrefix: "submission-file-download",
  deleteTestIdPrefix: "submission-file-delete",
  emptyMessage: "",
  gridSize: "medium",
  deletable: false,
  downloadable: true,
});

defineEmits<{
  preview: [item: AssignmentAttachmentItem];
  delete: [item: AssignmentAttachmentItem];
}>();

const fileRows = computed(() => flattenItems(props.items));
const gridShellClass = computed(() => [
  "submission-file-grid-shell",
  `submission-file-grid--${props.gridSize}`,
]);

function flattenItems(items: AssignmentAttachmentItem[], depth = 0): SubmissionFileRow[] {
  const rows: SubmissionFileRow[] = [];
  for (const item of items) {
    rows.push({
      item,
      depth,
      pathLabel: pathLabel(item, depth),
    });
    if (item.children?.length) {
      rows.push(...flattenItems(item.children, depth + 1));
    }
  }
  return rows;
}

function pathLabel(item: AssignmentAttachmentItem, depth: number): string {
  const normalized = item.path.trim();
  if (!normalized || normalized === "/" || normalized.endsWith(`/${item.name}`) && depth === 0) {
    return "";
  }
  return normalized;
}

function previewKind(item: AssignmentAttachmentItem): FilePreviewKind {
  return getFilePreviewKind(item);
}

function isImagePreview(item: AssignmentAttachmentItem): boolean {
  return previewKind(item) === "image";
}

function canPreview(item: AssignmentAttachmentItem): boolean {
  return props.downloadable && item.kind === "file" && previewKind(item) !== "external" && Boolean((item.previewUrl || item.downloadUrl).trim());
}

function previewKindLabel(item: AssignmentAttachmentItem): string {
  const kind = previewKind(item);
  if (kind === "pdf") {
    return "PDF";
  }
  if (kind === "html") {
    return "网页";
  }
  if (kind === "markdown") {
    return "Markdown";
  }
  if (kind === "text") {
    return "文本";
  }
  if (kind === "audio") {
    return "音频";
  }
  if (kind === "video") {
    return "视频";
  }
  return "文件";
}

function itemMeta(item: AssignmentAttachmentItem): string {
  if (item.kind === "dir") {
    const folderText = item.folderCount ? `，${item.folderCount} 个子文件夹` : "";
    return `文件夹 · ${item.fileCount ?? 0} 个文件${folderText}`;
  }
  return formatFileSize(item.size);
}

function formatFileSize(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "文件";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  const units = ["KB", "MB", "GB"];
  let size = value / 1024;
  for (const unit of units) {
    if (size < 1024 || unit === units[units.length - 1]) {
      return `${size.toFixed(size >= 10 ? 0 : 1)} ${unit}`;
    }
    size /= 1024;
  }
  return `${value} B`;
}

function downloadUrl(item: AssignmentAttachmentItem): string {
  return item.kind === "dir" ? item.archiveUrl || item.downloadUrl : item.downloadUrl;
}

function resolveItemTestId(id: number): string {
  return `${props.itemTestIdPrefix}-${id}`;
}

function resolvePreviewTestId(id: number): string {
  return `${props.previewTestIdPrefix}-${id}`;
}

function resolveThumbnailTestId(id: number): string {
  return `${props.thumbnailTestIdPrefix}-${id}`;
}

function resolveDownloadTestId(id: number): string {
  return `${props.downloadTestIdPrefix}-${id}`;
}

function resolveDeleteTestId(id: number): string {
  return `${props.deleteTestIdPrefix}-${id}`;
}
</script>

<style scoped>
.submission-file-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 10px;
}

.submission-file-grid--small .submission-file-grid {
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 8px;
}

.submission-file-grid--medium .submission-file-grid {
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
}

.submission-file-grid--large .submission-file-grid {
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}

.submission-file-grid--xlarge .submission-file-grid {
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 18px;
}

.submission-file-grid__item {
  display: grid;
  gap: 8px;
  align-content: start;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--border-soft);
  border-radius: 8px;
  background: var(--bg-subtle);
}

.submission-file-grid__item--dir {
  background: color-mix(in srgb, var(--accent-primary) 5%, var(--bg-subtle));
}

.submission-file-grid__thumb {
  display: grid;
  place-items: center;
  width: 100%;
  aspect-ratio: 4 / 3;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--border-soft);
  border-radius: 6px;
  background: var(--bg-muted);
  color: var(--text-secondary);
  font-weight: 800;
  cursor: pointer;
}

.submission-file-grid__thumb:hover {
  border-color: color-mix(in srgb, var(--accent-primary) 48%, var(--border-soft));
}

.submission-file-grid__thumb--static {
  cursor: default;
}

.submission-file-grid__thumb img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.submission-file-grid__thumb--image {
  aspect-ratio: auto;
  overflow: visible;
  background: var(--bg-surface);
}

.submission-file-grid__thumb--image img {
  width: 100%;
  height: auto;
  max-height: none;
  object-fit: contain;
  object-position: center;
}

.submission-file-grid__body {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.submission-file-grid__body strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--text-primary);
  line-height: 1.35;
}

.submission-file-grid__body .muted {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.submission-file-grid__actions {
  display: flex;
  justify-content: flex-start;
}

.submission-file-grid__actions .button {
  min-height: 30px;
  padding: 5px 9px;
}
</style>
