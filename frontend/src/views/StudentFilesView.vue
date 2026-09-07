<template>
  <section class="classes-page student-files-page">
    <section class="classes-page__board student-files-page__workspace" data-testid="student-files-workspace">
      <div class="files-toolbar">
        <div class="files-toolbar__top">
          <div class="files-context-bar" data-testid="student-files-context-bar">
            <nav class="files-breadcrumb" aria-label="当前路径">
              <button
                class="files-breadcrumb__button"
                type="button"
                data-testid="student-files-breadcrumb-root"
                @click="navigateToPath('/')"
              >
                根目录
              </button>
              <template v-for="crumb in breadcrumbs" :key="crumb.path">
                <span class="files-breadcrumb__sep">/</span>
                <button
                  v-if="!crumb.isCurrent"
                  class="files-breadcrumb__button"
                  type="button"
                  :data-testid="`student-files-breadcrumb-${crumb.label}`"
                  @click="navigateToPath(crumb.path)"
                >
                  {{ crumb.label }}
                </button>
                <span v-else class="files-breadcrumb__current">{{ crumb.label }}</span>
              </template>
            </nav>
          </div>
        </div>
        <div class="files-toolbar__bottom">
          <div class="files-primary-actions files-primary-actions--left" data-testid="student-files-primary-actions">
            <button
              class="button button--ghost files-up-button"
              type="button"
              data-testid="student-files-up-button"
              :disabled="!canNavigateUp"
              aria-label="返回上一级文件夹"
              @click="navigateUp"
            >
              ↩
            </button>
            <button
              class="button button--ghost"
              type="button"
              data-testid="student-files-refresh"
              aria-label="刷新当前资料"
              @click="loadFiles"
            >
              刷新
            </button>
            <div class="files-primary-actions__group">
              <div class="files-controls__view" role="group" aria-label="视图切换">
                <button
                  class="button"
                  :class="{ 'button--primary': viewMode === 'list' }"
                  type="button"
                  data-testid="student-files-view-list"
                  @click="setViewMode('list')"
                >
                  列表
                </button>
                <button
                  class="button"
                  :class="{ 'button--primary': viewMode === 'grid' }"
                  type="button"
                  data-testid="student-files-view-grid"
                  @click="setViewMode('grid')"
                >
                  网格
                </button>
              </div>
              <div
                v-if="viewMode === 'grid'"
                class="files-controls__grid-size"
                data-testid="student-files-grid-size-controls"
                role="group"
                aria-label="网格大小"
              >
                <button
                  class="button"
                  :class="{ 'button--primary': gridSize === 'small' }"
                  type="button"
                  data-testid="student-files-grid-size-small"
                  @click="setGridSize('small')"
                >
                  小
                </button>
                <button
                  class="button"
                  :class="{ 'button--primary': gridSize === 'medium' }"
                  type="button"
                  data-testid="student-files-grid-size-medium"
                  @click="setGridSize('medium')"
                >
                  中
                </button>
                <button
                  class="button"
                  :class="{ 'button--primary': gridSize === 'large' }"
                  type="button"
                  data-testid="student-files-grid-size-large"
                  @click="setGridSize('large')"
                >
                  大
                </button>
              </div>
            </div>
          </div>
          <div class="files-toolbar__search-slot">
            <div class="files-secondary-controls files-secondary-controls--right" data-testid="student-files-secondary-controls">
              <div class="files-secondary-controls__inner">
                <div class="files-controls__options">
                  <div class="files-controls__search" data-testid="student-files-filter-bar">
                    <input
                      v-model="searchQuery"
                      class="copy-dialog__search"
                      type="text"
                      placeholder="搜索当前资料"
                      data-testid="student-files-search-input"
                      @keyup.enter="applySearch"
                    />
                    <button class="button" type="button" data-testid="student-files-search-submit" @click="applySearch">搜索</button>
                    <button v-if="isSearching" class="button button--ghost" type="button" data-testid="student-files-search-clear" @click="clearSearch">清除筛选</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <p v-if="isSearching && activeSearchDescription" class="student-files-page__search-summary" data-testid="student-files-search-summary">
        当前过滤条件：{{ activeSearchDescription }}
      </p>

      <StatePanel v-if="loading" message="正在加载资料..." test-id="student-files-loading" />
      <StatePanel v-else-if="errorText" :message="errorText" tone="error" test-id="student-files-error" />
      <template v-else>
        <PaginationControls
          :page="filePage"
          :page-size="filePageSize"
          :page-size-options="filePageSizeOptions"
          :total="totalFiles"
          :total-pages="totalFilePages"
          test-id-prefix="student-files"
          @update:page-size="updateFilePageSize"
          @go="goFilePage"
          @prev="goPrevFilePage"
          @next="goNextFilePage"
        />
        <div v-if="viewMode === 'list'" class="student-files-page__table-scroll">
          <table class="files-table student-files-page__table" data-testid="student-files-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>类型</th>
                <th>更新时间</th>
                <th>大小</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in files"
                :key="item.id"
                class="student-files-page__row"
                role="button"
                tabindex="0"
                :aria-label="`打开 ${item.name}`"
                :data-testid="`student-file-row-${item.id}`"
                @click="openCardItem(item)"
                @keydown.enter.self.prevent="openCardItem(item)"
                @keydown.space.self.prevent="openCardItem(item)"
              >
                <td>
                  <button
                    v-if="item.kind === 'dir'"
                    class="files-entry-link"
                    type="button"
                    :data-testid="`student-file-open-${item.id}`"
                    @click.stop="openDirectory(item)"
                  >
                    {{ item.name }}
                  </button>
                  <span v-else>{{ item.name }}</span>
                </td>
                <td>{{ item.kind === "dir" ? "文件夹" : "文件" }}</td>
                <td>{{ formatFileUpdatedAt(item.updatedAt) }}</td>
                <td>{{ formatFileSize(item.size) }}</td>
                <td>
                  <div class="student-files-page__actions">
                    <button
                      v-if="item.kind === 'dir'"
                      class="text-button"
                      type="button"
                      :data-testid="`student-file-enter-${item.id}`"
                      @click.stop="openDirectory(item)"
                    >
                      进入
                    </button>
                    <button
                      v-if="item.kind === 'file'"
                      class="text-button"
                      type="button"
                      :data-testid="`student-file-preview-${item.id}`"
                      @click.stop="preview(item)"
                    >
                      预览
                    </button>
                    <a
                      v-if="item.kind === 'dir' && item.archiveUrl"
                      class="text-button"
                      :data-testid="`student-file-download-${item.id}`"
                      :href="item.archiveUrl"
                      @click.stop
                    >
                      下载压缩包
                    </a>
                    <a
                      v-else
                      class="text-button"
                      :href="item.downloadUrl"
                      :data-testid="`student-file-download-${item.id}`"
                      @click.stop
                    >
                      下载
                    </a>
                  </div>
                </td>
              </tr>
              <tr v-if="!files.length">
                <td colspan="5" class="files-table__empty">{{ uiCopy.emptyDirectory }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="student-files-page__grid" :class="studentFilesGridSizeClass" data-testid="student-files-grid">
          <article
            v-for="item in files"
            :key="item.id"
            class="student-files-page__card"
            role="button"
            tabindex="0"
            :data-testid="`student-file-card-${item.id}`"
            @click="openCardItem(item)"
            @keydown.enter.self.prevent="openCardItem(item)"
            @keydown.space.self.prevent="openCardItem(item)"
          >
            <button
              v-if="resolveGridThumbnailUrl(item)"
              class="student-files-page__thumbnail-button"
              type="button"
              :data-testid="`student-file-thumbnail-open-${item.id}`"
              :aria-label="`预览 ${item.name}`"
              @click.stop="preview(item)"
            >
              <img
                class="student-files-page__thumbnail"
                :data-testid="`student-file-thumbnail-${item.id}`"
                :src="resolveGridThumbnailUrl(item) ?? ''"
                :alt="item.name"
                loading="lazy"
                decoding="async"
              />
            </button>
            <div class="student-files-page__card-type">{{ item.kind === "dir" ? "文件夹" : "文件" }}</div>
            <button
              v-if="item.kind === 'dir'"
              class="files-entry-link student-files-page__card-title"
              type="button"
              :data-testid="`student-file-open-${item.id}`"
              @click.stop="openDirectory(item)"
            >
              {{ item.name }}
            </button>
            <button
              v-else
              class="files-entry-link student-files-page__card-title"
              type="button"
              :data-testid="`student-file-open-${item.id}`"
              @click.stop="preview(item)"
            >
              {{ item.name }}
            </button>
            <p v-if="isSearching" class="files-entry-meta">{{ item.path }}</p>
            <p class="muted">{{ item.kind === "dir" ? `文件夹 · ${formatFileSize(item.size)}` : formatFileSize(item.size) }}</p>
            <div class="student-files-page__actions">
              <button
                v-if="item.kind === 'dir'"
                class="text-button"
                type="button"
                :data-testid="`student-file-enter-${item.id}`"
                @click.stop="openDirectory(item)"
              >
                进入
              </button>
              <button
                v-if="item.kind === 'file'"
                class="text-button"
                type="button"
                :data-testid="`student-file-preview-${item.id}`"
                @click.stop="preview(item)"
              >
                预览
              </button>
              <a
                v-if="item.kind === 'dir' && item.archiveUrl"
                class="text-button"
                :data-testid="`student-file-download-${item.id}`"
                :href="item.archiveUrl"
                @click.stop
              >
                下载压缩包
              </a>
              <a
                v-else
                class="text-button"
                :href="item.downloadUrl"
                :data-testid="`student-file-download-${item.id}`"
                @click.stop
              >
                下载
              </a>
            </div>
          </article>
          <p v-if="!files.length" class="files-table__empty">{{ uiCopy.emptyDirectory }}</p>
        </div>
      </template>
    </section>
    <FilePreviewDialog
      :item="previewItem"
      :kind="previewKind"
      :loading="previewLoading"
      :error-text="previewErrorText"
      :text-content="previewTextContent"
      :markdown-content="previewMarkdownContent"
      :can-edit="false"
      :has-previous="previewHasPrevious"
      :has-next="previewHasNext"
      @close="closePreview"
      @previous="previewPrevious"
      @next="previewNext"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter, type LocationQueryRaw } from "vue-router";
import { api, ApiError, describeFileSearchQuery, type FileItem } from "@/api/client";
import FilePreviewDialog from "@/components/FilePreviewDialog.vue";
import PaginationControls from "@/components/PaginationControls.vue";
import StatePanel from "@/components/StatePanel.vue";
import { getFilePreviewKind } from "@/utils/file-preview";
import { renderMarkdownPreview } from "@/utils/markdown-preview";
import { uiCopy } from "@/utils/ui-copy";

type StudentFileSpace = "public" | "class";
type StudentFilesViewMode = "list" | "grid";
type StudentFilesGridSize = "small" | "medium" | "large";

const defaultStudentFilesViewMode: StudentFilesViewMode = "grid";
const defaultStudentFilesGridSize: StudentFilesGridSize = "medium";
const defaultStudentFilesPageSize = 30;
const defaultStudentClassFilesPageSize = 50;
const filePageSizeOptions = [1, 30, 50, 60, 100];

interface BreadcrumbItem {
  label: string;
  path: string;
  isCurrent: boolean;
}

const route = useRoute();
const router = useRouter();
const files = ref<FileItem[]>([]);
const loading = ref(false);
const errorText = ref("");
const previewItem = ref<FileItem | null>(null);
const previewTextContent = ref("");
const previewMarkdownContent = ref("");
const previewLoading = ref(false);
const previewErrorText = ref("");
const previewTextCache = ref(new Map<number, string>());
const searchQuery = ref("");
const viewMode = ref<StudentFilesViewMode>(defaultStudentFilesViewMode);
const gridSize = ref<StudentFilesGridSize>(defaultStudentFilesGridSize);
const filePage = ref(1);
const filePageSize = ref(defaultStudentFilesPageSize);
const totalFiles = ref(0);
const totalFilePages = ref(1);

const space = computed<StudentFileSpace>(() => (route.name === "student-files-class" ? "class" : "public"));
const currentPath = computed(() => currentPathFromRoute());
const breadcrumbs = computed<BreadcrumbItem[]>(() => toBreadcrumbs(currentPath.value));
const canNavigateUp = computed(() => currentPath.value !== "/");
const previewKind = computed(() => (previewItem.value ? getFilePreviewKind(previewItem.value) : null));
const previewableFiles = computed(() => files.value.filter((item) => item.kind === "file" && getFilePreviewKind(item) !== "external"));
const previewItemIndex = computed(() => {
  const current = previewItem.value;
  return current ? previewableFiles.value.findIndex((item) => item.id === current.id) : -1;
});
const previewHasPrevious = computed(() => previewItemIndex.value > 0);
const previewHasNext = computed(() => previewItemIndex.value >= 0 && previewItemIndex.value < previewableFiles.value.length - 1);
const activeSearchQuery = computed(() => (typeof route.query.q === "string" ? route.query.q.trim() : ""));
const isSearching = computed(() => activeSearchQuery.value.length > 0);
const activeSearchDescription = computed(() => describeFileSearchQuery(activeSearchQuery.value));
const studentFilesGridSizeClass = computed(() => `student-files-page__grid--${gridSize.value}`);

function currentPathFromRoute() {
  const rawPath = route.query.path;
  if (typeof rawPath !== "string") {
    return "/";
  }
  return normalizePath(rawPath);
}

function normalizePath(value: string) {
  const trimmed = value.trim();
  if (!trimmed || trimmed === "/") {
    return "/";
  }
  const segments = trimmed.split("/").filter(Boolean);
  return `/${segments.join("/")}`;
}

function toBreadcrumbs(path: string): BreadcrumbItem[] {
  const segments = normalizePath(path).split("/").filter(Boolean);
  return segments.map((label, index) => ({
    label,
    path: `/${segments.slice(0, index + 1).join("/")}`,
    isCurrent: index === segments.length - 1,
  }));
}

function formatFileUpdatedAt(value: string | undefined) {
  if (!value) {
    return "—";
  }
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) {
    return value;
  }
  return new Date(timestamp).toLocaleString("zh-CN", { hour12: false });
}

function formatFileSize(size: number) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function parseStudentFilesViewMode(raw: unknown): StudentFilesViewMode {
  return raw === "grid" || raw === "list" ? raw : defaultStudentFilesViewMode;
}

function parseStudentFilesGridSize(raw: unknown): StudentFilesGridSize {
  return raw === "small" || raw === "large" || raw === "medium" ? raw : defaultStudentFilesGridSize;
}

function parsePositiveInt(raw: unknown, fallback: number): number {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (typeof value !== "string" || value.trim() === "") {
    return fallback;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function defaultFilesPageSizeForCurrentSpace(): number {
  return space.value === "class" ? defaultStudentClassFilesPageSize : defaultStudentFilesPageSize;
}

function normalizeFilesPageSize(value: number, fallback = defaultFilesPageSizeForCurrentSpace()): number {
  return filePageSizeOptions.includes(value) ? value : fallback;
}

function buildStudentFilesRouteQuery(overrides: Partial<{
  path: string;
  view: StudentFilesViewMode;
  gridSize: StudentFilesGridSize;
  q: string;
  page: number;
  pageSize: number;
}> = {}) {
  const nextQuery: LocationQueryRaw = { ...route.query };
  if (Object.prototype.hasOwnProperty.call(overrides, "path")) {
    const nextPath = normalizePath(overrides.path ?? "/");
    if (nextPath === "/") {
      delete nextQuery.path;
    } else {
      nextQuery.path = nextPath;
    }
  }

  const nextViewMode = overrides.view ?? viewMode.value;
  if (nextViewMode === defaultStudentFilesViewMode) {
    delete nextQuery.view;
  } else {
    nextQuery.view = nextViewMode;
  }

  const nextGridSize = overrides.gridSize ?? gridSize.value;
  if (nextGridSize === defaultStudentFilesGridSize) {
    delete nextQuery.gridSize;
  } else {
    nextQuery.gridSize = nextGridSize;
  }

  const nextPage = overrides.page ?? filePage.value;
  if (nextPage <= 1) {
    delete nextQuery.page;
  } else {
    nextQuery.page = String(nextPage);
  }

  const defaultPageSize = defaultFilesPageSizeForCurrentSpace();
  const nextPageSize = normalizeFilesPageSize(overrides.pageSize ?? filePageSize.value, defaultPageSize);
  if (nextPageSize === defaultPageSize) {
    delete nextQuery.pageSize;
  } else {
    nextQuery.pageSize = String(nextPageSize);
  }

  if (Object.prototype.hasOwnProperty.call(overrides, "q")) {
    const keyword = (overrides.q ?? "").trim();
    if (keyword) {
      nextQuery.q = keyword;
    } else {
      delete nextQuery.q;
    }
  }
  return nextQuery;
}

async function navigateToPath(path: string) {
  await router.push({
    path: route.path,
    query: buildStudentFilesRouteQuery({ path, page: 1 }),
  });
}

function parentPathOf(path: string) {
  const segments = normalizePath(path).split("/").filter(Boolean);
  segments.pop();
  return segments.length ? `/${segments.join("/")}` : "/";
}

async function navigateUp() {
  if (!canNavigateUp.value) {
    return;
  }
  await navigateToPath(parentPathOf(currentPath.value));
}

async function openDirectory(item: FileItem) {
  if (item.kind !== "dir") {
    return;
  }
  await navigateToPath(item.path);
}

async function openCardItem(item: FileItem) {
  if (item.kind === "dir") {
    await openDirectory(item);
    return;
  }
  await preview(item);
}

function closePreview() {
  previewItem.value = null;
  previewTextContent.value = "";
  previewMarkdownContent.value = "";
  previewLoading.value = false;
  previewErrorText.value = "";
}

function resolveGridThumbnailUrl(item: FileItem): string | null {
  if (getFilePreviewKind(item) !== "image") {
    return null;
  }
  const previewUrl = item.previewUrl.trim();
  if (previewUrl) {
    return previewUrl;
  }
  const downloadUrl = item.downloadUrl.trim();
  return downloadUrl || null;
}

async function applySearch() {
  await router.push({
    path: route.path,
    query: buildStudentFilesRouteQuery({ q: searchQuery.value, page: 1 }),
  });
}

async function clearSearch() {
  searchQuery.value = "";
  await router.push({
    path: route.path,
    query: buildStudentFilesRouteQuery({ q: "", page: 1 }),
  });
}

async function setViewMode(mode: StudentFilesViewMode) {
  if (viewMode.value === mode && parseStudentFilesViewMode(route.query.view) === mode) {
    return;
  }
  viewMode.value = mode;
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ view: mode }),
  });
}

async function setGridSize(size: StudentFilesGridSize) {
  if (gridSize.value === size && parseStudentFilesGridSize(route.query.gridSize) === size) {
    return;
  }
  gridSize.value = size;
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ gridSize: size, page: 1 }),
  });
}

async function updateFilePageSize(value: number) {
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ pageSize: value, page: 1 }),
  });
}

async function goPrevFilePage() {
  if (filePage.value <= 1) {
    return;
  }
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ page: filePage.value - 1 }),
  });
}

async function goNextFilePage() {
  if (filePage.value >= totalFilePages.value) {
    return;
  }
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ page: filePage.value + 1 }),
  });
}

async function goFilePage(page: number): Promise<void> {
  const nextPage = Math.min(Math.max(1, Math.trunc(page)), totalFilePages.value);
  if (nextPage === filePage.value) {
    return;
  }
  await router.replace({
    path: route.path,
    query: buildStudentFilesRouteQuery({ page: nextPage }),
  });
}

async function preview(item: FileItem) {
  if (item.kind !== "file") {
    return;
  }
  const kind = getFilePreviewKind(item);
  previewItem.value = item;
  previewErrorText.value = "";
  previewTextContent.value = "";
  previewMarkdownContent.value = "";
  if (kind !== "text" && kind !== "markdown") {
    previewLoading.value = false;
    return;
  }
  const cached = previewTextCache.value.get(item.id);
  if (cached !== undefined) {
    if (kind === "markdown") {
      previewMarkdownContent.value = renderMarkdownPreview(cached);
    } else {
      previewTextContent.value = cached;
    }
    previewLoading.value = false;
    return;
  }
  previewLoading.value = true;
  try {
    const response = await fetch(item.previewUrl, { credentials: "same-origin" });
    if (!response.ok) {
      throw new Error("preview_failed");
    }
    const text = await response.text();
    previewTextCache.value.set(item.id, text);
    if (kind === "markdown") {
      previewMarkdownContent.value = renderMarkdownPreview(text);
    } else {
      previewTextContent.value = text;
    }
  } catch {
    previewErrorText.value = "预览加载失败，请下载后查看。";
  } finally {
    previewLoading.value = false;
  }
}

async function previewPrevious() {
  if (!previewHasPrevious.value) {
    return;
  }
  const item = previewableFiles.value[previewItemIndex.value - 1];
  if (item) {
    await preview(item);
  }
}

async function previewNext() {
  if (!previewHasNext.value) {
    return;
  }
  const item = previewableFiles.value[previewItemIndex.value + 1];
  if (item) {
    await preview(item);
  }
}

async function loadFiles() {
  loading.value = true;
  errorText.value = "";
  try {
    searchQuery.value = activeSearchQuery.value;
    viewMode.value = parseStudentFilesViewMode(route.query.view);
    gridSize.value = parseStudentFilesGridSize(route.query.gridSize);
    filePage.value = parsePositiveInt(route.query.page, 1);
    const defaultPageSize = defaultFilesPageSizeForCurrentSpace();
    filePageSize.value = normalizeFilesPageSize(parsePositiveInt(route.query.pageSize, defaultPageSize), defaultPageSize);
    const params = new URLSearchParams();
    params.set("space", space.value);
    params.set("path", currentPath.value);
    params.set("page", String(filePage.value));
    params.set("pageSize", String(filePageSize.value));
    if (activeSearchQuery.value) {
      params.set("q", activeSearchQuery.value);
    }
    const response = await api.studentFiles(params);
    files.value = response.items ?? [];
    const pagination = response.pagination;
    filePage.value = pagination?.page ?? filePage.value;
    filePageSize.value = normalizeFilesPageSize(pagination?.pageSize ?? filePageSize.value);
    totalFiles.value = pagination?.total ?? files.value.length;
    totalFilePages.value = Math.max(1, pagination?.totalPages ?? Math.ceil(Math.max(totalFiles.value, 1) / filePageSize.value));
  } catch (error) {
    errorText.value = error instanceof ApiError ? error.message : "加载资料失败";
    files.value = [];
    totalFiles.value = 0;
    totalFilePages.value = 1;
  } finally {
    loading.value = false;
  }
}

watch(() => route.fullPath, () => {
  void loadFiles();
}, { immediate: true });
</script>

<style scoped>
.student-files-page__workspace {
  gap: 12px;
}

.student-files-page__workspace > .files-toolbar {
  --files-search-slot-width: min(320px, 34vw);
  margin: 0;
  padding: 0 0 10px;
  border: 0;
  border-bottom: 1px solid var(--border-soft);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.files-toolbar {
  --files-search-slot-width: min(420px, 44vw);
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  align-items: stretch;
  justify-content: stretch;
  width: 100%;
  margin-bottom: 4px;
}

.files-toolbar__top,
.files-toolbar__bottom {
  display: grid;
  gap: 10px;
  width: 100%;
}

.files-toolbar__bottom {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
  position: relative;
  min-height: 36px;
  padding-right: calc(var(--files-search-slot-width) + 10px);
}

.files-context-bar,
.files-primary-actions,
.files-secondary-controls,
.files-primary-actions__group,
.files-secondary-controls__inner,
.files-controls__options,
.files-controls__search,
.files-controls__view,
.files-controls__grid-size {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.files-secondary-controls__inner,
.files-controls__options,
.files-toolbar__search-slot {
  justify-content: flex-end;
}

.files-toolbar__search-slot {
  display: flex;
  position: absolute;
  top: 50%;
  right: 0;
  width: var(--files-search-slot-width);
  max-width: 100%;
  margin-left: 0;
  transform: translateY(-50%);
  justify-content: flex-end;
}

.files-toolbar__search-slot .files-secondary-controls,
.files-toolbar__search-slot .files-secondary-controls__inner,
.files-toolbar__search-slot .files-controls__options,
.files-toolbar__search-slot .files-controls__search {
  width: 100%;
}

.files-toolbar__search-slot .files-controls__search {
  flex-wrap: nowrap;
  justify-content: flex-end;
}

.files-toolbar__search-slot .files-controls__search input {
  flex: 1 1 auto;
  min-width: 0;
}

.files-controls__search input {
  min-width: 200px;
  width: 220px;
}

.student-files-page__search-summary {
  margin: 12px 0 0;
  color: var(--text-secondary);
  font-size: 0.94rem;
}

.student-files-page__table-scroll {
  width: 100%;
  overflow-x: auto;
}

.student-files-page__table {
  min-width: 720px;
}

.student-files-page__row {
  cursor: pointer;
}

.student-files-page__row:hover,
.student-files-page__row:focus-visible {
  background: var(--bg-subtle);
}

.student-files-page__row:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: -3px;
}

.student-files-page__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.student-files-page__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
}

.student-files-page__grid--small {
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px;
}

.student-files-page__grid--medium {
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
}

.student-files-page__grid--large {
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}

.student-files-page__card {
  padding: 8px;
  border: 1px solid var(--border-soft);
  border-radius: 8px;
  background: var(--bg-surface);
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease;
}

.student-files-page__card:hover,
.student-files-page__card:focus-visible {
  border-color: color-mix(in srgb, var(--accent-primary) 38%, var(--border-soft));
  background: var(--bg-subtle);
}

.student-files-page__card:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
}

.student-files-page__thumbnail-button {
  display: block;
  width: 100%;
  aspect-ratio: 4 / 3;
  margin: 0 0 8px;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--border-soft);
  border-radius: 8px;
  background: var(--bg-subtle);
  cursor: pointer;
}

.student-files-page__thumbnail-button:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
}

.student-files-page__thumbnail {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
}

.student-files-page__card-type {
  margin-bottom: 4px;
  color: var(--text-muted);
  font-size: 0.94rem;
}

.student-files-page__card-title {
  display: block;
  margin-bottom: 4px;
  font-weight: 600;
  text-align: left;
  width: 100%;
}

@media (max-width: 720px) {
  .files-toolbar {
    --files-search-slot-width: 100%;
  }

  .files-toolbar__bottom {
    grid-template-columns: minmax(0, 1fr);
    min-height: 0;
    padding-right: 0;
  }

  .files-toolbar__search-slot,
  .files-secondary-controls,
  .files-controls__search {
    width: 100%;
    position: static;
    margin-left: 0;
    transform: none;
    justify-content: flex-start;
  }

  .files-toolbar__search-slot .files-controls__search {
    flex-wrap: wrap;
  }

  .files-controls__search input {
    width: 100%;
    min-width: 0;
  }
}
</style>
