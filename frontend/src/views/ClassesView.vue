<template>
  <section class="classes-page classes-management">
    <section class="classes-page__board classes-management__panel" data-testid="classes-management-panel">
      <div class="classes-management__toolbar">
        <div class="classes-management__filters">
          <button class="button button--primary" type="button" data-testid="class-create-open" @click="createDialogOpen = true">
            新建班级
          </button>
          <div class="classes-management__search-group">
            <input
              v-model="classKeyword"
              class="copy-dialog__search"
              type="text"
              placeholder="搜索班级名称"
              data-testid="classes-search-input"
              @keyup.enter="applyClassFilters"
            />
            <button class="button" type="button" data-testid="classes-search-submit" @click="applyClassFilters">搜索</button>
          </div>
        </div>
      </div>

      <PaginationControls
        :page="classPage"
        :page-size="classPageSize"
        :page-size-options="classPageSizeOptions"
        :total="totalClasses"
        :total-pages="totalClassPages"
        test-id-prefix="classes"
        @update:page-size="updateClassPageSize"
        @go="goClassPage"
        @prev="goPrevClassPage"
        @next="goNextClassPage"
      />

      <div class="classes-management__table-wrap">
        <table class="files-table classes-management__table" data-testid="classes-table">
          <thead>
            <tr>
              <th>
                <button
                  class="table-sort-button"
                  :class="{ 'is-active': classSort === 'name-asc' || classSort === 'name-desc' }"
                  type="button"
                  data-testid="class-sort-name"
                  @click="toggleClassNameSort"
                >
                  班级名称
                  <span class="table-sort-button__mark">{{ classSortMark("name") }}</span>
                </button>
              </th>
              <th>注册码</th>
              <th>
                <button
                  class="table-sort-button"
                  :class="{ 'is-active': classSort === 'registration-desc' || classSort === 'registration-asc' }"
                  type="button"
                  data-testid="class-sort-registration"
                  @click="toggleClassRegistrationSort"
                >
                  状态
                  <span class="table-sort-button__mark">{{ classSortMark("registration") }}</span>
                </button>
              </th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in listedClasses" :key="item.id" :data-testid="`class-row-${item.id}`">
              <td>
                <strong class="classes-management__name" :data-testid="`class-row-name-${item.id}`">{{ item.name }}</strong>
              </td>
              <td class="classes-management__code-cell" :data-testid="`class-row-status-${item.id}`">
                {{ item.joinCode || "未生成" }}
              </td>
              <td>
                <div class="classes-management__status-cell">
                  <span class="classes-management__status" :class="{ 'classes-management__status--active': item.joinCodeStatus === 'active' }">
                    {{ item.joinCodeStatus === "active" ? "开放注册" : "暂停注册" }}
                  </span>
                  <span
                    v-if="item.examLockEnabled"
                    class="classes-management__status classes-management__status--locked"
                    :data-testid="`class-row-exam-lock-${item.id}`"
                  >
                    考试锁定中
                  </span>
                </div>
              </td>
              <td class="files-table__actions">
                <div class="classes-management__actions" :data-testid="`class-row-actions-${item.id}`">
                  <button
                    class="button"
                    :class="item.joinCodeStatus === 'active' ? 'button--accent' : 'button--secondary'"
                    type="button"
                    :data-testid="`class-registration-toggle-${item.id}`"
                    @click="toggleRegistration(item)"
                  >
                    {{ item.joinCodeStatus === "active" ? "关闭注册" : "开放注册" }}
                  </button>
                  <button
                    class="button"
                    :class="item.examLockEnabled ? 'button--danger' : 'button--secondary'"
                    type="button"
                    :data-testid="`class-exam-lock-toggle-${item.id}`"
                    @click="toggleExamLock(item)"
                  >
                    {{ item.examLockEnabled ? "解除考试锁定" : "考试锁定" }}
                  </button>
                  <button class="button button--ghost" type="button" :data-testid="`class-students-${item.id}`" @click="openStudentsDrawer(item)">
                    学生管理
                  </button>
                  <button class="button button--ghost" type="button" :data-testid="`class-edit-${item.id}`" @click="openEditDialog(item)">
                    编辑
                  </button>
                  <button class="button button--ghost text-button--danger" type="button" :data-testid="`class-delete-${item.id}`" @click="openDeleteDialog(item)">
                    删除
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!listedClasses.length">
              <td colspan="4" class="files-table__empty">{{ classKeyword.trim() ? "当前筛选下没有班级。" : "当前还没有班级数据。" }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </section>

  <div v-if="studentDrawerClass" class="class-students-drawer" data-testid="class-students-drawer" role="dialog" aria-modal="true" @click.self="closeStudentsDrawer">
    <section class="class-students-drawer__panel">
      <header class="class-students-drawer__header">
        <div class="class-students-drawer__title-block">
          <div class="class-students-drawer__eyebrow">学生名单</div>
          <h2 class="class-students-drawer__title">{{ studentDrawerClass.name }}</h2>
          <div class="class-students-drawer__meta">
            <span class="classes-management__status" :class="{ 'classes-management__status--active': studentDrawerClass.joinCodeStatus === 'active' }">
              {{ classRegistrationLabel(studentDrawerClass) }}
            </span>
            <span v-if="studentDrawerClass.examLockEnabled" class="classes-management__status classes-management__status--locked">考试锁定中</span>
            <span v-if="studentDrawerClass.joinCode" class="class-students-drawer__join-code">注册码 {{ studentDrawerClass.joinCode }}</span>
          </div>
        </div>
        <button class="button button--ghost" type="button" data-testid="class-students-drawer-close" @click="closeStudentsDrawer">关闭</button>
      </header>

      <StudentRosterPanel
        :class-id="studentDrawerClass.id"
        :class-name="studentDrawerClass.name"
      />
    </section>
  </div>

  <TextInputDialog
    :open="createDialogOpen"
    test-id-prefix="class-create"
    eyebrow="班级创建"
    title="新建班级"
    label="班级名称"
    placeholder="输入班级名称"
    confirm-label="创建班级"
    @cancel="createDialogOpen = false"
    @confirm="createClass"
  />

  <TextInputDialog
    :open="editingClass !== null"
    test-id-prefix="class-edit"
    eyebrow="班级编辑"
    title="编辑班级"
    label="班级名称"
    :initial-value="editingClass?.name ?? ''"
    placeholder="输入班级名称"
    confirm-label="保存修改"
    @cancel="editingClass = null"
    @confirm="submitClassEdit"
  />

  <ConfirmDialog
    :open="pendingDeleteClass !== null"
    title="确认删除班级"
    :message="pendingDeleteClass ? `确认删除班级 ${pendingDeleteClass.name} 后，将同步删除该班级下的学生、作业、作业附件、学生提交和班级资料；此操作不可恢复。` : ''"
    test-id-prefix="class-delete"
    confirm-label="确认删除"
    confirm-tone="danger"
    @cancel="pendingDeleteClass = null"
    @confirm="confirmDeleteClass"
  />
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useRoute, useRouter, type LocationQueryRaw } from "vue-router";
import ConfirmDialog from "@/components/ConfirmDialog.vue";
import PaginationControls from "@/components/PaginationControls.vue";
import StudentRosterPanel from "@/components/StudentRosterPanel.vue";
import TextInputDialog from "@/components/TextInputDialog.vue";
import { api, ApiError, type ClassItem } from "@/api/client";
import { useClassesStore } from "@/stores/classes";
import { useToastStore } from "@/stores/toast";

type ClassSort = "name-asc" | "name-desc" | "registration-desc" | "registration-asc";

const defaultClassSort: ClassSort = "name-asc";
const defaultClassPageSize = 30;
const classPageSizeOptions = [30, 60, 100];

const route = useRoute();
const router = useRouter();
const classesStore = useClassesStore();
const toastStore = useToastStore();

const listedClasses = ref<ClassItem[]>([]);
const classKeyword = ref("");
const classSort = ref<ClassSort>(defaultClassSort);
const classPage = ref(1);
const classPageSize = ref(defaultClassPageSize);
const totalClasses = ref(0);
const totalClassPages = ref(1);
const createDialogOpen = ref(false);
const editingClass = ref<ClassItem | null>(null);
const pendingDeleteClass = ref<ClassItem | null>(null);
const studentDrawerClass = ref<ClassItem | null>(null);

function parsePositiveInt(raw: unknown, fallback: number) {
  const parsed = Number(raw);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function parseClassSort(raw: unknown): ClassSort {
  return raw === "name-desc" || raw === "registration-desc" || raw === "registration-asc" || raw === "name-asc"
    ? raw
    : defaultClassSort;
}

function normalizeClassPageSize(value: number) {
  return classPageSizeOptions.includes(value) ? value : defaultClassPageSize;
}

function applyStateFromRoute() {
  classKeyword.value = typeof route.query.q === "string" ? route.query.q : "";
  classSort.value = parseClassSort(route.query.sort);
  classPage.value = parsePositiveInt(route.query.page, 1);
  classPageSize.value = normalizeClassPageSize(parsePositiveInt(route.query.pageSize, defaultClassPageSize));
}

function buildClassesQuery(overrides: Partial<{
  q: string;
  sort: ClassSort;
  page: number;
  pageSize: number;
}> = {}) {
  const nextKeyword = (overrides.q ?? classKeyword.value).trim();
  const hasSortOverride = Object.prototype.hasOwnProperty.call(overrides, "sort");
  const nextSort = overrides.sort ?? classSort.value;
  const nextPage = overrides.page ?? classPage.value;
  const nextPageSize = overrides.pageSize ?? classPageSize.value;

  const query: LocationQueryRaw = {};
  if (nextKeyword) {
    query.q = nextKeyword;
  }
  if (nextSort !== defaultClassSort || hasSortOverride) {
    query.sort = nextSort;
  }
  if (nextPage > 1) {
    query.page = String(nextPage);
  }
  if (nextPageSize !== defaultClassPageSize) {
    query.pageSize = String(nextPageSize);
  }
  return query;
}

async function replaceClassesRoute(overrides: Partial<{
  q: string;
  sort: ClassSort;
  page: number;
  pageSize: number;
}> = {}) {
  await router.replace({ path: "/classes", query: buildClassesQuery(overrides) });
}

async function loadClassesPage() {
  try {
    applyStateFromRoute();
    const response = await api.classes({
      q: classKeyword.value.trim() || undefined,
      sort: classSort.value === defaultClassSort ? undefined : classSort.value,
      page: classPage.value > 1 ? classPage.value : undefined,
      pageSize: classPageSize.value !== defaultClassPageSize ? classPageSize.value : undefined,
    });
    listedClasses.value = response.classes ?? [];
    totalClasses.value = response.pagination?.total ?? response.classes?.length ?? 0;
    totalClassPages.value = Math.max(1, response.pagination?.totalPages ?? 1);

    if (classPage.value > totalClassPages.value) {
      await replaceClassesRoute({ page: totalClassPages.value });
      return;
    }

    classesStore.apply(response.classes ?? []);
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "加载班级失败");
  }
}

async function applyClassFilters() {
  await replaceClassesRoute({ q: classKeyword.value, page: 1 });
}

async function applyClassSort(nextSort: ClassSort) {
  classSort.value = nextSort;
  await replaceClassesRoute({ sort: nextSort, page: 1 });
}

async function toggleClassNameSort() {
  await applyClassSort(classSort.value === "name-asc" ? "name-desc" : "name-asc");
}

async function toggleClassRegistrationSort() {
  await applyClassSort(classSort.value === "registration-desc" ? "registration-asc" : "registration-desc");
}

function classSortMark(column: "name" | "registration") {
  if (column === "registration") {
    if (classSort.value === "registration-asc") {
      return "↑";
    }
    return classSort.value === "registration-desc" ? "↓" : "";
  }
  if (classSort.value === "name-desc") {
    return "↓";
  }
  return classSort.value === "name-asc" ? "↑" : "";
}

function classRegistrationLabel(item: ClassItem): string {
  return item.joinCodeStatus === "active" ? "开放注册" : "暂停注册";
}

function openStudentsDrawer(item: ClassItem): void {
  studentDrawerClass.value = item;
}

function closeStudentsDrawer(): void {
  studentDrawerClass.value = null;
}

async function updateClassPageSize(value: number) {
  await replaceClassesRoute({ pageSize: value, page: 1 });
}

async function goPrevClassPage() {
  if (classPage.value <= 1) {
    return;
  }
  await replaceClassesRoute({ page: classPage.value - 1 });
}

async function goNextClassPage() {
  if (classPage.value >= totalClassPages.value) {
    return;
  }
  await replaceClassesRoute({ page: classPage.value + 1 });
}

async function goClassPage(page: number): Promise<void> {
  const nextPage = Math.min(Math.max(1, Math.trunc(page)), totalClassPages.value);
  if (nextPage === classPage.value) {
    return;
  }
  await replaceClassesRoute({ page: nextPage });
}

async function refreshClassSources() {
  totalClassPages.value = Math.max(1, Math.ceil(Math.max(totalClasses.value, 1) / classPageSize.value));
}

async function createClass(name: string) {
  try {
    const created = await classesStore.create(name);
    createDialogOpen.value = false;
    listedClasses.value = [...listedClasses.value, created];
    totalClasses.value += 1;
    toastStore.push("success", "班级已创建");
    await refreshClassSources();
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "创建班级失败");
  }
}

function openEditDialog(item: ClassItem) {
  editingClass.value = item;
}

async function submitClassEdit(name: string) {
  if (!editingClass.value) {
    return;
  }
  try {
    const updated = await classesStore.update(editingClass.value.id, name);
    listedClasses.value = listedClasses.value.map((item) => (item.id === updated.id ? updated : item));
    editingClass.value = null;
    toastStore.push("success", "班级已更新");
    await refreshClassSources();
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "更新班级失败");
  }
}

function openDeleteDialog(item: ClassItem) {
  pendingDeleteClass.value = item;
}

async function confirmDeleteClass() {
  if (!pendingDeleteClass.value) {
    return;
  }
  try {
    await classesStore.remove(pendingDeleteClass.value.id);
    listedClasses.value = listedClasses.value.filter((item) => item.id !== pendingDeleteClass.value?.id);
    totalClasses.value = Math.max(0, totalClasses.value - 1);
    pendingDeleteClass.value = null;
    toastStore.push("success", "班级已删除");
    await refreshClassSources();
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "删除班级失败");
  }
}

async function toggleRegistration(item: ClassItem) {
  try {
    const enabled = item.joinCodeStatus !== "active";
    const result = await classesStore.updateRegistration(item.id, enabled);
    listedClasses.value = listedClasses.value.map((current) =>
      current.id === item.id
        ? {
            ...current,
            joinCode: result.joinCode,
            joinCodeHint: result.joinCodeHint,
            joinCodeStatus: result.joinCodeStatus,
            registrationEnabled: result.registrationEnabled,
          }
        : current,
    );
    if (studentDrawerClass.value?.id === item.id) {
      studentDrawerClass.value = {
        ...studentDrawerClass.value,
        joinCode: result.joinCode,
        joinCodeHint: result.joinCodeHint,
        joinCodeStatus: result.joinCodeStatus,
        registrationEnabled: result.registrationEnabled,
      };
    }
    toastStore.push(enabled ? "success" : "warning", enabled ? "已开放注册" : "已关闭注册");
    await refreshClassSources();
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "更新注册状态失败");
  }
}

async function toggleExamLock(item: ClassItem) {
  try {
    const enabled = !item.examLockEnabled;
    const updated = await classesStore.updateExamLock(item.id, enabled);
    listedClasses.value = listedClasses.value.map((current) => (current.id === item.id ? updated : current));
    if (studentDrawerClass.value?.id === item.id) {
      studentDrawerClass.value = updated;
    }
    toastStore.push(enabled ? "warning" : "success", enabled ? "已开启考试锁定，学生无法下载和预览历史提交" : "已解除考试锁定");
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "更新考试锁定失败");
  }
}

watch(() => route.fullPath, () => {
  void loadClassesPage();
}, { immediate: true });
</script>

<style scoped>
.classes-management__panel,
.classes-management__toolbar {
  display: grid;
  gap: 12px;
}

.classes-management__filters,
.classes-management__search-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.classes-management__filters {
  justify-content: flex-start;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-soft);
}

.classes-management__search-group {
  flex: 0 1 310px;
  margin-left: auto;
}

.classes-management__search-group .copy-dialog__search {
  width: min(220px, 100%);
}

.classes-management__table-wrap {
  overflow-x: auto;
  max-width: 100%;
}

.classes-management__table {
  table-layout: fixed;
  min-width: 720px;
}

.classes-management__table th:nth-child(1),
.classes-management__table td:nth-child(1) {
  width: 22%;
}

.classes-management__table th:nth-child(2),
.classes-management__table td:nth-child(2) {
  width: 15%;
}

.classes-management__table th:nth-child(3),
.classes-management__table td:nth-child(3) {
  width: 13%;
}

.classes-management__table th:nth-child(4),
.classes-management__table td:nth-child(4) {
  width: 50%;
}

.classes-management__name {
  display: inline-block;
  font-size: 1rem;
}

.classes-management__code-cell {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.classes-management__status {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(217, 119, 6, 0.2);
  background: rgba(217, 119, 6, 0.12);
  color: var(--accent-warning);
  font-size: 0.875rem;
  font-weight: 700;
}

.classes-management__status--active {
  border-color: rgba(28, 117, 99, 0.18);
  background: rgba(28, 117, 99, 0.12);
  color: var(--success);
}

.classes-management__status-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.classes-management__status--locked {
  border-color: rgba(220, 38, 38, 0.22);
  background: rgba(220, 38, 38, 0.12);
  color: var(--accent-danger, #dc2626);
}

.classes-management__actions {
  display: flex;
  flex-wrap: nowrap;
  gap: 6px;
  align-items: center;
}

.classes-management__actions .button {
  flex: 0 0 auto;
  min-height: 36px;
  padding: 5px 9px;
  border-radius: 10px;
  white-space: nowrap;
  font-size: 0.9rem;
}

.class-students-drawer {
  position: fixed;
  inset: 0;
  z-index: 4500;
  display: flex;
  justify-content: flex-end;
  background: rgba(15, 23, 42, 0.42);
  padding: 0;
  overscroll-behavior: contain;
}

.class-students-drawer__panel {
  display: grid;
  grid-template-rows: auto auto;
  align-content: start;
  gap: 16px;
  width: min(1120px, calc(100vw - 28px));
  height: 100vh;
  padding: 20px;
  overflow: auto;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-soft);
  box-shadow: var(--shadow-strong);
}

.class-students-drawer__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-soft);
}

.class-students-drawer__title-block {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.class-students-drawer__eyebrow {
  color: var(--accent-primary);
  font-size: 0.8rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.class-students-drawer__title {
  margin: 0;
  color: var(--text-primary);
  font-size: clamp(1.25rem, 2vw, 1.6rem);
  line-height: 1.25;
}

.class-students-drawer__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.class-students-drawer__join-code {
  color: var(--text-secondary);
  font-size: 0.92rem;
  font-weight: 700;
}

@media (max-width: 960px) {
  .classes-management__filters {
    align-items: center;
    flex-wrap: nowrap;
    overflow-x: auto;
    padding-bottom: 12px;
  }

  .classes-management__search-group {
    flex: 0 0 310px;
    margin-left: auto;
  }

  .classes-management__search-group .copy-dialog__search {
    width: min(220px, 100%);
  }

  .classes-management__table {
    min-width: 840px;
  }

  .classes-management__actions {
    flex-wrap: nowrap;
  }
}

@media (max-width: 720px) {
  .class-students-drawer__panel {
    width: 100vw;
    padding: 16px;
  }

  .class-students-drawer__header {
    align-items: stretch;
    flex-direction: column;
  }

  .class-students-drawer__header .button {
    align-self: flex-start;
  }
}
</style>
