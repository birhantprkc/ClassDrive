<template>
  <section class="classes-page">
    <header class="classes-page__header assignment-detail-header">
      <div class="assignment-detail-heading">
        <h2 class="classes-page__title" data-testid="assignment-detail-title">{{ pageTitle }}</h2>
        <template v-if="assignment && !loading && !notFound">
          <div class="assignment-detail-heading__meta" data-testid="assignment-detail-heading-meta">
            <span class="assignment-detail-heading__class" data-testid="assignment-detail-class">{{ currentClassName }}</span>
            <StatusPill
              :label="assignmentStatusLabel(assignment.status)"
              :tone="assignmentStatusTone(assignment.status)"
              data-testid="assignment-detail-status"
            />
            <span class="assignment-detail-heading__due">截止 {{ formatDateTime(assignment.dueAt) }}</span>
          </div>
          <p class="assignment-detail-heading__description" data-testid="assignment-detail-description">
            {{ assignment.description || uiCopy.emptyAssignmentDescription }}
          </p>
        </template>
        <p v-else class="muted">{{ pageDescription }}</p>
      </div>
      <div class="classes-page__header-actions">
        <button
          v-if="assignment && !loading && !notFound"
          class="button button--primary"
          type="button"
          data-testid="assignment-edit-open"
          @click="openEditDialog"
        >
          修改作业
        </button>
        <RouterLink class="button" :to="backLink" data-testid="assignment-detail-back">返回作业管理</RouterLink>
      </div>
    </header>

    <section class="classes-page__board" v-if="loading">
      <p class="muted">正在加载作业详情...</p>
    </section>

    <section class="classes-page__board" v-else-if="notFound">
      <article class="classes-card" data-testid="assignment-detail-empty">
        <div>
          <h3 class="classes-card__title">{{ uiCopy.assignmentNotFound }}</h3>
          <p class="muted">当前链接对应的作业不存在，或已不属于这个班级。</p>
        </div>
        <RouterLink class="button" :to="backLink">返回作业管理</RouterLink>
      </article>
    </section>

    <section v-if="assignment && !loading && !notFound" class="classes-page__board classes-page__detail-layout">
      <div class="classes-page__detail-main" data-testid="assignment-detail-main">
        <article class="classes-card assignment-submissions-workspace">
          <div class="assignment-submissions-workspace__header">
            <div>
              <h3 class="classes-card__title">学生当前提交</h3>
              <p class="muted">列表用于快速扫描，点击“查看/批改”后在右侧抽屉处理文件和评语。</p>
            </div>
            <StatusPill
              :label="`${submissionTotal} 条提交`"
              tone="status-pill--neutral"
              data-testid="assignment-submission-count"
            />
          </div>
            <p v-if="submissionsLoading" class="muted">正在加载学生提交...</p>
            <template v-else-if="submissions.length || submissionTotal > 0 || submissionFilter.trim()">
              <div class="classes-page__submission-toolbar" data-testid="assignment-submission-toolbar">
                <div class="classes-page__submission-toolbar-group classes-page__submission-toolbar-group--filters">
                  <input
                    v-model="submissionFilter"
                    class="copy-dialog__search classes-page__submission-search"
                    type="text"
                    placeholder="按姓名、学号或评语搜索"
                    data-testid="assignment-submission-filter"
                    @keyup.enter="applySubmissionFilters"
                  />
                  <button
                    class="button classes-page__submission-action"
                    type="button"
                    data-testid="assignment-submission-search"
                    @click="applySubmissionFilters"
                  >
                    搜索
                  </button>
                  <select
                    v-model="submissionSort"
                    class="copy-dialog__search classes-page__submission-sort"
                    data-testid="assignment-submission-sort"
                    @change="applySubmissionSort"
                  >
                    <option value="updatedAt-desc">最近更新</option>
                    <option value="updatedAt-asc">最早更新</option>
                    <option value="studentNo-asc">学号升序</option>
                    <option value="studentNo-desc">学号降序</option>
                    <option value="displayName-asc">姓名 A-Z</option>
                    <option value="displayName-desc">姓名 Z-A</option>
                  </select>
                  <button
                    class="button button--ghost classes-page__submission-action"
                    type="button"
                    data-testid="assignment-submission-refresh"
                    aria-label="刷新学生提交"
                    @click="refreshSubmissions"
                  >
                    刷新
                  </button>
                </div>
                <div
                  class="classes-page__submission-toolbar-group"
                  data-testid="assignment-submission-navigation"
                >
                  <div class="classes-page__submission-nav-group">
                    <button
                      class="button classes-page__submission-action"
                      type="button"
                      data-testid="assignment-submission-prev"
                      :disabled="activeSubmissionIndex <= 0"
                      @click="selectPrevSubmission"
                    >
                      上一条
                    </button>
                    <button
                      class="button classes-page__submission-action"
                      type="button"
                      data-testid="assignment-submission-next"
                      :disabled="activeSubmissionIndex < 0 || activeSubmissionIndex >= visibleSubmissions.length - 1"
                      @click="selectNextSubmission"
                    >
                      下一条
                    </button>
                  </div>
                </div>
                <div
                  class="classes-page__submission-toolbar-group classes-page__submission-toolbar-group--end"
                  data-testid="assignment-submission-bulk-actions"
                >
                  <button
                    class="button button--accent classes-page__submission-action"
                    type="button"
                    data-testid="assignment-detail-missing-open"
                    @click="openAssignmentMissingStatsDialog"
                  >
                    未交统计
                  </button>
                  <button
                    v-if="submissionArchiveUrl"
                    class="button"
                    type="button"
                    data-testid="assignment-submission-download-archive"
                    @click="openSubmissionArchiveDownloadConfirm"
                  >
                    下载本作业提交
                  </button>
                  <button
                    class="button button--success classes-page__submission-action"
                    type="button"
                    data-testid="assignment-submission-review-mark-all"
                    @click="markAllSubmissionsReviewed"
                  >
                    一键批改本作业
                  </button>
                </div>
              </div>
              <PaginationControls
                v-if="submissionTotal > 0"
                :page="submissionPage"
                :page-size="submissionPageSize"
                :page-size-options="submissionPageSizeOptions"
                :total="submissionTotal"
                :total-pages="submissionTotalPages"
                test-id-prefix="assignment-submission"
                @update:page-size="updateSubmissionPageSize"
                @go="goSubmissionPage"
                @prev="goPrevSubmissionPage"
                @next="goNextSubmissionPage"
              />
              <div class="assignment-submissions-table-wrap">
                <table class="files-table assignment-submissions-table" data-testid="assignment-submissions-table">
                  <thead>
                    <tr>
                      <th>学生</th>
                      <th>提交内容</th>
                      <th>更新时间</th>
                      <th>批改状态</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="submission in visibleSubmissions"
                      :key="submission.id"
                      class="assignment-submissions-table__row"
                      :class="{ 'is-active': submission.id === activeSubmissionId }"
                      :data-testid="`assignment-submission-row-${submission.id}`"
                      @click="openSubmissionDrawer(submission.id)"
                    >
                      <td>
                        <div class="assignment-submissions-table__student">
                          <strong>{{ submission.displayName }}</strong>
                          <span>学号 {{ submission.studentNo }}</span>
                        </div>
                      </td>
                      <td>
                        <div
                          class="assignment-submissions-table__content"
                          :data-testid="`assignment-submission-content-${submission.id}`"
                        >
                          <strong>{{ formatSubmissionItemsSummary(submission.items) }}</strong>
                          <div
                            class="assignment-submissions-table__content-meta"
                            :data-testid="`assignment-submission-content-meta-${submission.id}`"
                          >
                            <StatusPill
                              :label="submissionStatusText(submission.status)"
                              :tone="submissionStatusPillTone(submission.status)"
                              :data-testid="`assignment-submission-status-badge-${submission.id}`"
                            />
                            <span
                              v-if="submission.items.length"
                              class="assignment-submissions-table__item-names"
                              :data-testid="`assignment-submission-item-names-${submission.id}`"
                            >
                              {{ formatSubmissionItemNames(submission.items) }}
                            </span>
                          </div>
                        </div>
                      </td>
                      <td>
                        <div class="assignment-submissions-table__time">
                          <span>{{ formatDateTime(submission.updatedAt) }}</span>
                          <span class="assignment-submissions-table__secondary">提交 {{ formatDateTime(submission.submittedAt) }}</span>
                        </div>
                      </td>
                      <td>
                        <div class="assignment-submissions-table__review">
                          <StatusPill
                            :label="reviewStatusText(submission.reviewStatus)"
                            :tone="reviewStatusTone(submission.reviewStatus)"
                            :data-testid="`assignment-submission-review-badge-${submission.id}`"
                          />
                          <span v-if="submission.teacherCommentSummary">{{ submission.teacherCommentSummary }}</span>
                          <span v-if="submission.reviewerName" class="assignment-submissions-table__secondary">批改：{{ submission.reviewerName }}</span>
                        </div>
                      </td>
                      <td>
                        <button
                          class="button button--secondary assignment-submissions-table__open"
                          type="button"
                          :data-testid="`assignment-submission-open-${submission.id}`"
                          @click.stop="openSubmissionDrawer(submission.id)"
                        >
                          查看/批改
                        </button>
                      </td>
                    </tr>
                    <tr v-if="!visibleSubmissions.length">
                      <td colspan="5" class="files-table__empty">当前筛选下没有学生提交。</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>
            <p v-else class="muted">{{ uiCopy.emptyStudentSubmissions }}</p>
          </article>
        </div>

        <ElDrawer
          v-model="reviewDrawerOpen"
          class="assignment-review-drawer"
          data-testid="assignment-submission-review-drawer"
          size="min(1280px, 96vw)"
          :close-on-click-modal="true"
          :before-close="handleReviewDrawerBeforeClose"
          :teleported="false"
        >
          <template #header>
            <div class="assignment-review-drawer__header">
              <div>
                <h3>提交详情与批改</h3>
                <p v-if="activeSubmission" class="assignment-review-drawer__header-subtitle">
                  {{ activeSubmission.studentNo }} · {{ activeSubmission.displayName }}
                </p>
              </div>
              <div class="assignment-review-drawer__nav">
                <button
                  class="button assignment-review-drawer__nav-button"
                  type="button"
                  data-testid="assignment-review-drawer-prev"
                  :disabled="drawerNavigationLoading || !drawerHasPreviousSubmission"
                  @click="selectPrevSubmission"
                >
                  上一个学生
                </button>
                <button
                  class="button assignment-review-drawer__nav-button"
                  type="button"
                  data-testid="assignment-review-drawer-next"
                  :disabled="drawerNavigationLoading || !drawerHasNextSubmission"
                  @click="selectNextSubmission"
                >
                  下一个学生
                </button>
              </div>
            </div>
          </template>
          <template v-if="activeSubmission">
            <section
              class="assignment-review-drawer__summary assignment-review-drawer__summary--readable"
              data-testid="assignment-review-drawer-summary"
            >
              <div class="assignment-review-drawer__identity">
                <strong>{{ activeSubmission.displayName }}</strong>
                <span>学号 {{ activeSubmission.studentNo }}</span>
              </div>
              <div class="assignment-review-drawer__status-pills" data-testid="assignment-review-drawer-statuses">
                <StatusPill
                  :label="reviewStatusText(activeSubmission.reviewStatus)"
                  :tone="reviewStatusTone(activeSubmission.reviewStatus)"
                />
                <StatusPill
                  :label="submissionStatusText(activeSubmission.status)"
                  :tone="submissionStatusPillTone(activeSubmission.status)"
                />
              </div>
              <div class="assignment-review-drawer__summary-list">
                <span>
                  <strong>提交</strong>
                  {{ formatDateTime(activeSubmission.submittedAt) }}
                </span>
                <span>
                  <strong>更新</strong>
                  {{ formatDateTime(activeSubmission.updatedAt) }}
                </span>
                <span>
                  <strong>内容</strong>
                  {{ formatSubmissionItemsSummary(activeSubmission.items) }}
                </span>
              </div>
            </section>
            <div class="assignment-review-drawer__workspace" data-testid="assignment-review-drawer-main">
              <section
                v-if="submissionReviewDrafts[activeSubmission.id]"
                class="assignment-review-drawer__section assignment-review-drawer__form"
              >
                <div class="assignment-review-drawer__section-title">
                  <div>
                    <h4>状态与评语</h4>
                  </div>
                </div>
                <div class="assignment-review-drawer__review-row">
                  <label
                    class="app-field assignment-review-drawer__status-field"
                    :data-testid="`assignment-submission-review-status-field-${activeSubmission.id}`"
                  >
                    <span>批改状态</span>
                    <select
                      v-model="submissionReviewDrafts[activeSubmission.id].reviewStatus"
                      class="copy-dialog__search"
                      :data-testid="`assignment-submission-review-status-${activeSubmission.id}`"
                    >
                      <option value="pending">未批改</option>
                      <option value="reviewed">已批改</option>
                    </select>
                  </label>
                  <label class="app-field">
                    <span>评语摘要</span>
                    <input
                      v-model="submissionReviewDrafts[activeSubmission.id].teacherComment"
                      class="copy-dialog__search"
                      type="text"
                      placeholder="例如：文件齐全，继续完善排版"
                      :data-testid="`assignment-submission-review-comment-${activeSubmission.id}`"
                    />
                  </label>
                  <div class="assignment-review-drawer__actions">
                    <button
                      class="button"
                      type="button"
                      data-testid="assignment-submission-review-close"
                      @click="requestCloseReviewDrawer"
                    >
                      关闭
                    </button>
                    <button
                      class="button button--primary"
                      type="button"
                      :data-testid="`assignment-submission-review-save-${activeSubmission.id}`"
                      @click="saveSubmissionReview(activeSubmission.id)"
                    >
                      保存批改
                    </button>
                  </div>
                </div>
              </section>

              <section class="assignment-review-drawer__section assignment-review-drawer__files-panel">
                <div class="assignment-review-drawer__section-title">
                  <div>
                    <h4>提交文件</h4>
                  </div>
                  <div class="assignment-review-drawer__file-toolbar">
                    <div class="assignment-review-drawer__file-view" role="group" aria-label="提交文件视图">
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileViewMode === 'list' }"
                        type="button"
                        data-testid="assignment-submission-files-view-list"
                        @click="setSubmissionFileViewMode('list')"
                      >
                        列表
                      </button>
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileViewMode === 'grid' }"
                        type="button"
                        data-testid="assignment-submission-files-view-grid"
                        @click="setSubmissionFileViewMode('grid')"
                      >
                        网格
                      </button>
                    </div>
                    <div
                      v-if="submissionFileViewMode === 'grid'"
                      class="assignment-review-drawer__file-grid-size"
                      data-testid="assignment-submission-files-grid-size-controls"
                      role="group"
                      aria-label="提交文件网格大小"
                    >
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileGridSize === 'small' }"
                        type="button"
                        data-testid="assignment-submission-files-grid-size-small"
                        @click="setSubmissionFileGridSize('small')"
                      >
                        小
                      </button>
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileGridSize === 'medium' }"
                        type="button"
                        data-testid="assignment-submission-files-grid-size-medium"
                        @click="setSubmissionFileGridSize('medium')"
                      >
                        中
                      </button>
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileGridSize === 'large' }"
                        type="button"
                        data-testid="assignment-submission-files-grid-size-large"
                        @click="setSubmissionFileGridSize('large')"
                      >
                        大
                      </button>
                      <button
                        class="button"
                        :class="{ 'button--primary': submissionFileGridSize === 'xlarge' }"
                        type="button"
                        data-testid="assignment-submission-files-grid-size-xlarge"
                        @click="setSubmissionFileGridSize('xlarge')"
                      >
                        超大
                      </button>
                    </div>
                    <a
                      v-if="activeSubmission.items.length === 1"
                      class="button button--secondary assignment-review-drawer__file-download"
                      :href="submissionDownloadUrl(activeSubmission.items[0])"
                      data-testid="assignment-submission-download-root"
                    >
                      下载全部
                    </a>
                  </div>
                </div>
                <div
                  v-if="activeSubmission.items.length && submissionFileViewMode === 'list'"
                  class="assignment-review-drawer__file-list"
                  data-testid="assignment-submission-file-list"
                >
                  <table class="files-table assignment-review-drawer__file-table">
                    <thead>
                      <tr>
                        <th>名称</th>
                        <th>类型</th>
                        <th>大小</th>
                        <th>路径</th>
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr
                        v-for="row in activeSubmissionFileRows"
                        :key="row.item.id"
                        :data-kind="row.item.kind"
                      >
                        <td>
                          <div class="assignment-review-drawer__file-name" :style="{ paddingLeft: `${row.depth * 18}px` }">
                            <strong>{{ row.item.name }}</strong>
                          </div>
                        </td>
                        <td>{{ submissionFileTypeLabel(row.item) }}</td>
                        <td>{{ formatFileSize(row.item.size) ?? "文件" }}</td>
                        <td>
                          <span class="assignment-review-drawer__file-meta">{{ submissionFilePathLabel(row.item) }}</span>
                        </td>
                        <td>
                          <div class="assignment-review-drawer__file-actions">
                            <button
                              v-if="canPreviewSubmissionItem(row.item)"
                              class="text-button"
                              type="button"
                              :data-testid="`assignment-submission-file-preview-${row.item.id}`"
                              @click="previewSubmissionItem(row.item)"
                            >
                              预览
                            </button>
                            <a
                              class="text-button"
                              :href="submissionDownloadUrl(row.item)"
                              :data-testid="`assignment-submission-download-${row.item.id}`"
                            >
                    {{ row.item.kind === "dir" ? "下载压缩包" : "下载" }}
                            </a>
                          </div>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <SubmissionFileGrid
                  v-else-if="activeSubmission.items.length"
                  :items="activeSubmission.items"
                  :grid-size="submissionFileGridSize"
                  test-id="assignment-submission-file-grid"
                  item-test-id-prefix="assignment-submission-file"
                  preview-test-id-prefix="assignment-submission-file-preview"
                  thumbnail-test-id-prefix="assignment-submission-file-thumb"
                  download-test-id-prefix="assignment-submission-download"
                  @preview="previewSubmissionItem"
                />
                <p v-else class="muted">当前提交没有附件。</p>
              </section>
            </div>
          </template>
        </ElDrawer>
        <ConfirmDialog
          :open="reviewDrawerCloseConfirmOpen"
          title="关闭批改抽屉"
          message="批改内容还没有保存，关闭后本次修改不会保留。"
          test-id-prefix="assignment-review-close-discard"
          confirm-label="确认关闭"
          cancel-label="继续编辑"
          confirm-tone="danger"
          @confirm="confirmCloseReviewDrawer"
          @cancel="cancelCloseReviewDrawer"
        />
        <FilePreviewDialog
          :item="submissionPreviewItem"
          :kind="submissionPreviewKind"
          :loading="submissionPreviewLoading"
          :error-text="submissionPreviewErrorText"
          :text-content="submissionPreviewTextContent"
          :markdown-content="submissionPreviewMarkdownContent"
          :can-edit="false"
          :has-previous="submissionPreviewHasPrevious"
          :has-next="submissionPreviewHasNext"
          @close="closeSubmissionPreview"
          @previous="previewPreviousSubmissionFile"
          @next="previewNextSubmissionFile"
        />
    </section>

    <div v-if="assignment && assignmentMissingStatsOpen" class="copy-dialog-backdrop" @click.self="closeAssignmentMissingStatsDialog">
      <section class="copy-dialog assignment-missing-dialog" data-testid="assignment-detail-missing-dialog">
        <div class="copy-dialog__header">
          <div>
            <h3 class="copy-dialog__title">{{ `${assignment.title} · 未交名单` }}</h3>
          </div>
          <button class="button button--ghost" type="button" data-testid="assignment-detail-missing-close" @click="closeAssignmentMissingStatsDialog">
            关闭
          </button>
        </div>

        <div class="assignment-missing-dialog__body">
          <div class="assignment-missing-dialog__actions">
            <span class="assignment-missing-dialog__count" data-testid="assignment-detail-missing-count">
              {{ assignmentMissingStatsSummaryText }}
            </span>
            <button
              class="button button--secondary"
              type="button"
              data-testid="assignment-detail-missing-export"
              :disabled="!assignmentMissingStatsRows.length"
              @click="exportAssignmentMissingStats"
            >
              导出 Excel
            </button>
          </div>
          <p v-if="assignmentMissingStatsLoading" class="muted">正在统计未交名单...</p>
          <p v-else-if="!assignmentMissingStatsRows.length" class="muted">当前作业没有未交学生。</p>
          <PaginationControls
            v-if="assignmentMissingStatsRows.length"
            :page="assignmentMissingStatsPage"
            :page-size="assignmentMissingStatsPageSize"
            :page-size-options="assignmentMissingStatsPageSizeOptions"
            :total="assignmentMissingStatsRows.length"
            :total-pages="assignmentMissingStatsTotalPages"
            test-id-prefix="assignment-detail-missing"
            @update:page-size="updateAssignmentMissingStatsPageSize"
            @go="goAssignmentMissingStatsPage"
            @prev="goPrevAssignmentMissingStatsPage"
            @next="goNextAssignmentMissingStatsPage"
          />
          <div
            v-if="!assignmentMissingStatsLoading && assignmentMissingStatsRows.length"
            class="assignment-missing-dialog__table-frame"
            :style="assignmentMissingStatsTableFrameStyle"
          >
            <table
              class="files-table assignment-missing-dialog__table"
              data-testid="assignment-detail-missing-table"
            >
              <thead>
                <tr>
                  <th>学生</th>
                  <th>学号</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in paginatedAssignmentMissingStatsRows" :key="row.studentId">
                  <td>{{ row.displayName }}</td>
                  <td>{{ row.studentNo }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>

  <div
    v-if="assignment && editDialogOpen"
    class="copy-dialog-backdrop"
    data-testid="assignment-edit-backdrop"
    @click.self="closeEditDialog"
  >
      <section class="copy-dialog assignment-edit-dialog" data-testid="assignment-edit-dialog">
        <div class="copy-dialog__header">
          <div>
            <h3 class="copy-dialog__title">修改作业</h3>
          </div>
          <button class="button button--ghost" type="button" data-testid="assignment-edit-close" @click="closeEditDialog">
            关闭
          </button>
        </div>
        <SectionIntro
          label="基本信息"
          test-id="assignment-edit-intro"
        />
        <div class="assignment-edit-dialog__content">
          <div class="students-import assignment-edit-form" data-testid="assignment-edit-form">
            <label class="app-field">
              <span>作业标题</span>
              <input
                v-model="editTitle"
                class="copy-dialog__search"
                type="text"
                placeholder="作业标题"
                data-testid="assignment-edit-title"
              />
            </label>
            <label class="app-field" data-testid="assignment-edit-description-field">
              <span>作业说明</span>
              <textarea
                v-model="editDescription"
                class="students-import__input"
                placeholder="作业说明"
                data-testid="assignment-edit-description"
              />
            </label>
            <label class="app-field" data-testid="assignment-edit-due-at-field">
              <span>截止时间</span>
              <div class="app-datetime-input" data-testid="assignment-edit-due-at">
                <ElDatePicker
                  v-model="editDueAt"
                  class="app-datetime-control"
                  type="datetime"
                  format="YYYY-MM-DD HH:mm"
                  value-format="YYYY-MM-DDTHH:mm"
                  popper-class="classdrive-date-picker"
                  :editable="false"
                />
              </div>
            </label>
            <label class="app-field">
              <span>发布状态</span>
              <select
                v-model="editStatus"
                class="copy-dialog__search"
                data-testid="assignment-edit-status"
              >
                <option value="draft">未发布</option>
                <option value="published">已发布</option>
              </select>
              <p class="assignment-edit-form__help" data-testid="assignment-edit-status-help">
                未发布仅老师可见；已发布后学生端可见并可提交。
              </p>
            </label>
            <label class="app-field">
              <span>提交方式</span>
              <select
                v-model="editSubmissionMode"
                class="copy-dialog__search"
                data-testid="assignment-edit-submission-mode"
              >
                <option value="any">不限</option>
                <option value="files">只收文件</option>
                <option value="folder">只收文件夹</option>
              </select>
            </label>
            <label class="app-field">
              <span>提交格式</span>
              <select
                v-model="editSubmissionTypeCategory"
                class="copy-dialog__search"
                data-testid="assignment-edit-submission-type"
              >
                <option value="mixed">常用文件</option>
                <option value="image">图片文件</option>
                <option value="word">Word 文档</option>
                <option value="pdf">PDF 文件</option>
                <option value="archive">压缩包</option>
              </select>
            </label>
            <label class="app-field">
              <span>最少文件数</span>
              <input
                v-model.number="editMinFileCount"
                class="copy-dialog__search"
                type="number"
                min="1"
                max="500"
                step="1"
                data-testid="assignment-edit-min-file-count"
              />
            </label>
            <div class="students-form assignment-edit-form__actions">
              <button
                class="button button--primary"
                type="button"
                data-testid="assignment-save-submit"
                @click="saveAssignment"
              >
                保存修改
              </button>
              <button
                class="button"
                type="button"
                data-testid="assignment-delete-submit"
                @click="removeAssignment"
              >
                删除作业
              </button>
            </div>
          </div>

          <section
            class="students-import assignment-attachment-manager assignment-edit-dialog__attachment-manager"
            data-testid="assignment-edit-attachment-manager"
          >
            <SectionIntro
              label="附件管理"
              description="在修改作业时统一添加、下载和删除附件。"
              test-id="assignment-attachment-intro"
            />
            <div class="assignment-attachment-manager__body" data-testid="assignment-attachment-manager">
              <div class="students-form assignment-attachment-manager__actions">
                <button class="button button--secondary" type="button" data-testid="assignment-attachment-upload" @click="openAttachmentUpload">
                  添加附件
                </button>
                <input
                  ref="attachmentInput"
                  class="hidden-input"
                  type="file"
                  multiple
                  data-testid="assignment-attachment-input"
                  @change="handleAttachmentUpload"
                />
              </div>

              <ResourceList
                :items="assignmentAttachmentResources"
                :loading="attachmentsLoading"
                loading-message="正在加载附件..."
                :empty-message="uiCopy.emptyTeacherAttachments"
                test-id="assignment-attachment-list"
                item-test-id-prefix="assignment-attachment-row"
                link-test-id-prefix="assignment-attachment-download"
              >
                <template #item-actions="{ item }">
                  <button
                    class="copy-dialog__recent-pin"
                    type="button"
                    :data-testid="`assignment-attachment-delete-${item.id}`"
                    @click="removeAttachment(Number(item.id), item.name)"
                  >
                    删除
                  </button>
                </template>
              </ResourceList>
            </div>
          </section>
        </div>
      </section>
    </div>

    <ConfirmDialog
      :open="editDialogDiscardConfirmOpen"
      title="放弃作业修改"
      message="作业修改还没有保存，关闭后本次修改不会保留。"
      test-id-prefix="assignment-edit-discard"
      confirm-label="确认关闭"
      cancel-label="继续编辑"
      confirm-tone="danger"
      @cancel="cancelCloseEditDialog"
      @confirm="confirmCloseEditDialog"
    />

    <ConfirmDialog
      :open="pendingAttachmentDelete !== null"
      title="确认删除附件"
      :message="pendingAttachmentDelete ? `删除后将移除附件 ${pendingAttachmentDelete.name}。` : ''"
      test-id-prefix="assignment-attachment-delete"
      confirm-label="确认删除"
      confirm-tone="danger"
      @cancel="pendingAttachmentDelete = null"
      @confirm="confirmRemoveAttachment"
    />

    <ConfirmDialog
      :open="pendingAssignmentDelete"
      title="确认删除作业"
      message="删除后当前作业、附件和相关入口将不可恢复。"
      test-id-prefix="assignment-delete"
      confirm-label="确认删除"
      confirm-tone="danger"
      @cancel="pendingAssignmentDelete = false"
      @confirm="confirmRemoveAssignment"
    />

    <ConfirmDialog
      :open="submissionArchiveDownloadConfirmOpen"
      title="确认下载提交包"
      message="将下载当前作业全部学生提交，并包含提交清单和未提交清单。"
      test-id-prefix="assignment-submission-download-confirm"
      confirm-label="确认下载"
      @cancel="submissionArchiveDownloadConfirmOpen = false"
      @confirm="confirmSubmissionArchiveDownload"
    />

  </section>
</template>

<script setup lang="ts">
import { storeToRefs } from "pinia";
import { computed, ref, watch } from "vue";
import { ElDatePicker, ElDrawer } from "element-plus";
import { RouterLink, useRoute, useRouter, type LocationQueryRaw } from "vue-router";
import ConfirmDialog from "@/components/ConfirmDialog.vue";
import FilePreviewDialog from "@/components/FilePreviewDialog.vue";
import PaginationControls from "@/components/PaginationControls.vue";
import ResourceList from "@/components/ResourceList.vue";
import SectionIntro from "@/components/SectionIntro.vue";
import StatusPill from "@/components/StatusPill.vue";
import SubmissionFileGrid from "@/components/SubmissionFileGrid.vue";
import { api, ApiError, type AssignmentAttachmentItem } from "@/api/client";
import { useAssignmentDetailStore, type AssignmentSubmissionSort } from "@/stores/assignment-detail";
import { useClassesStore } from "@/stores/classes";
import { useToastStore } from "@/stores/toast";
import { useUploadStore } from "@/stores/upload";
import type { StatusPillTone } from "@/types/status-pill";
import { getFilePreviewKind, type FilePreviewKind } from "@/utils/file-preview";
import { renderMarkdownPreview } from "@/utils/markdown-preview";
import { exportRowsToSpreadsheet } from "@/utils/spreadsheet-export";
import { assignmentStatusLabel, assignmentStatusTone, reviewStatusLabel, submissionStatusLabel, submissionStatusTone, uiCopy } from "@/utils/ui-copy";

interface AssignmentMissingStatsRow {
  studentId: number;
  studentNo: string;
  displayName: string;
}

type SubmissionFileViewMode = "grid" | "list";
type SubmissionFileGridSize = "small" | "medium" | "large" | "xlarge";
type DrawerBoundarySelection = "first" | "last";

const route = useRoute();
const router = useRouter();
const toastStore = useToastStore();
const uploadStore = useUploadStore();
const assignmentDetailStore = useAssignmentDetailStore();
const classesStore = useClassesStore();
const defaultSubmissionSort: AssignmentSubmissionSort = "updatedAt-desc";
const defaultSubmissionPageSize = 30;
const submissionPageSizeOptions = [30, 60, 100];
const defaultAssignmentMissingStatsPageSize = 30;
const assignmentMissingStatsPageSizeOptions = [30, 60, 100];
const fullStatsPageSize = 100;
const {
  assignment,
  attachments,
  submissions,
  submissionReviewDrafts,
  submissionFilter,
  submissionSort,
  submissionPage,
  submissionPageSize,
  submissionTotal,
  submissionTotalPages,
  activeSubmissionId,
  editTitle,
  editDescription,
  editDueAt,
  editStatus,
  editSubmissionMode,
  editSubmissionTypeCategory,
  editMinFileCount,
  loading,
  notFound,
  attachmentsLoading,
  submissionsLoading,
  visibleSubmissions,
  activeSubmissionIndex,
} = storeToRefs(assignmentDetailStore);
const { classes } = storeToRefs(classesStore);

const attachmentInput = ref<HTMLInputElement | null>(null);
const pendingAttachmentDelete = ref<{ id: number; name: string } | null>(null);
const pendingAssignmentDelete = ref(false);
const editDialogOpen = ref(false);
const editDialogDiscardConfirmOpen = ref(false);
const reviewDrawerOpen = ref(false);
const reviewDrawerCloseConfirmOpen = ref(false);
const pendingReviewDrawerCloseDone = ref<(() => void) | null>(null);
const drawerNavigationLoading = ref(false);
const pendingDrawerBoundarySelection = ref<DrawerBoundarySelection | null>(null);
const submissionArchiveDownloadConfirmOpen = ref(false);
const assignmentMissingStatsOpen = ref(false);
const assignmentMissingStatsLoading = ref(false);
const assignmentMissingStatsRows = ref<AssignmentMissingStatsRow[]>([]);
const assignmentMissingStatsPage = ref(1);
const assignmentMissingStatsPageSize = ref(defaultAssignmentMissingStatsPageSize);
const assignmentMissingRosterTotal = ref(0);
const assignmentMissingSubmittedTotal = ref(0);
const submissionPreviewItem = ref<AssignmentAttachmentItem | null>(null);
const submissionPreviewTextContent = ref("");
const submissionPreviewMarkdownContent = ref("");
const submissionPreviewLoading = ref(false);
const submissionPreviewErrorText = ref("");
const submissionPreviewTextCache = ref(new Map<number, string>());
const submissionFileViewMode = ref<SubmissionFileViewMode>("grid");
const submissionFileGridSize = ref<SubmissionFileGridSize>("xlarge");

const currentClassId = computed(() => {
  const parsed = Number(route.params.classId);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
});

const currentAssignmentId = computed(() => {
  const parsed = Number(route.params.assignmentId);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
});

function parsePositiveInt(raw: unknown, fallback: number) {
  const parsed = Number(raw);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function parseSubmissionSort(raw: unknown): AssignmentSubmissionSort {
  return raw === "updatedAt-asc"
    || raw === "studentNo-asc"
    || raw === "studentNo-desc"
    || raw === "displayName-asc"
    || raw === "displayName-desc"
    || raw === "updatedAt-desc"
    ? raw
    : defaultSubmissionSort;
}

function normalizeSubmissionPageSize(value: number) {
  return submissionPageSizeOptions.includes(value) ? value : defaultSubmissionPageSize;
}

function applySubmissionStateFromRoute() {
  assignmentDetailStore.setSubmissionFilter(typeof route.query.q === "string" ? route.query.q : "");
  assignmentDetailStore.setSubmissionSort(parseSubmissionSort(route.query.sort));
  assignmentDetailStore.setSubmissionPageState(
    parsePositiveInt(route.query.page, 1),
    normalizeSubmissionPageSize(parsePositiveInt(route.query.pageSize, defaultSubmissionPageSize)),
  );
}

function buildSubmissionQuery(overrides: Partial<{
  q: string;
  sort: AssignmentSubmissionSort;
  page: number;
  pageSize: number;
}> = {}) {
  const nextKeyword = (overrides.q ?? submissionFilter.value).trim();
  const nextSort = overrides.sort ?? submissionSort.value;
  const nextPage = overrides.page ?? submissionPage.value;
  const nextPageSize = overrides.pageSize ?? submissionPageSize.value;
  const query: LocationQueryRaw = {};

  if (nextKeyword) {
    query.q = nextKeyword;
  }
  if (nextSort !== defaultSubmissionSort) {
    query.sort = nextSort;
  }
  if (nextPage > 1) {
    query.page = String(nextPage);
  }
  if (nextPageSize !== defaultSubmissionPageSize) {
    query.pageSize = String(nextPageSize);
  }
  return query;
}

async function replaceSubmissionRoute(overrides: Partial<{
  q: string;
  sort: AssignmentSubmissionSort;
  page: number;
  pageSize: number;
}> = {}) {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  await router.replace({
    path: `/assignments/classes/${currentClassId.value}/${currentAssignmentId.value}`,
    query: buildSubmissionQuery(overrides),
  });
}

function buildSubmissionRequestOptions() {
  return {
    q: submissionFilter.value.trim() || undefined,
    sort: submissionSort.value,
    page: submissionPage.value,
    pageSize: submissionPageSize.value,
  };
}

const currentClassName = computed(() => {
  const matched = classes.value.find((item) => item.id === currentClassId.value);
  return matched?.name ?? `班级 ${currentClassId.value ?? ""}`.trim();
});

const backLink = computed(() => {
  if (!currentClassId.value) {
    return "/assignments";
  }
  return `/assignments/classes/${currentClassId.value}`;
});

const pageTitle = computed(() => assignment.value?.title ?? "作业详情");

const pageDescription = computed(() => {
  if (loading.value) {
    return "正在加载当前作业的标题、说明、状态与时间信息。";
  }
  if (notFound.value) {
    return `${currentClassName.value} 下未找到对应作业，请返回列表重新选择。`;
  }
  if (!assignment.value) {
    return "当前作业详情暂时不可用。";
  }
  return `${currentClassName.value} · ${assignmentStatusLabel(assignment.value.status)}`;
});

const hasEditDialogDraftChanges = computed(() => {
  const currentAssignment = assignment.value;
  if (!currentAssignment) {
    return false;
  }
  return editTitle.value !== currentAssignment.title
    || editDescription.value !== currentAssignment.description
    || editDueAt.value !== toDateTimeLocalValue(currentAssignment.dueAt)
    || editStatus.value !== currentAssignment.status
    || editSubmissionMode.value !== currentAssignment.submissionMode
    || editSubmissionTypeCategory.value !== (currentAssignment.submissionTypeCategory ?? "mixed")
    || editMinFileCount.value !== currentAssignment.minFileCount;
});

const activeSubmission = computed(() => (
  visibleSubmissions.value.find((submission) => submission.id === activeSubmissionId.value) ?? null
));
const hasActiveSubmissionReviewDraftChanges = computed(() => {
  const submission = activeSubmission.value;
  if (!submission) {
    return false;
  }
  const draft = submissionReviewDrafts.value[submission.id];
  if (!draft) {
    return false;
  }
  return draft.reviewStatus !== submission.reviewStatus || draft.teacherComment !== submission.teacherCommentSummary;
});
const drawerHasPreviousSubmission = computed(() => (
  activeSubmissionIndex.value > 0 || (activeSubmissionIndex.value === 0 && submissionPage.value > 1)
));
const drawerHasNextSubmission = computed(() => (
  activeSubmissionIndex.value >= 0
  && (
    activeSubmissionIndex.value < visibleSubmissions.value.length - 1
    || submissionPage.value < submissionTotalPages.value
  )
));

function resolvePreviewKind(item: AssignmentAttachmentItem): FilePreviewKind {
  const kind = getFilePreviewKind(item);
  return kind !== "external" && !previewSourceUrl(item) ? "external" : kind;
}

const submissionPreviewKind = computed(() => (
  submissionPreviewItem.value ? resolvePreviewKind(submissionPreviewItem.value) : null
));

function previewSourceUrl(item: AssignmentAttachmentItem): string {
  return item.previewUrl.trim() || item.downloadUrl.trim();
}

function toSubmissionPreviewItem(item: AssignmentAttachmentItem): AssignmentAttachmentItem {
  return {
    ...item,
    previewUrl: previewSourceUrl(item),
  };
}

interface SubmissionFileRow {
  item: AssignmentAttachmentItem;
  depth: number;
}

function flattenSubmissionFileRows(items: AssignmentAttachmentItem[], depth = 0): SubmissionFileRow[] {
  const rows: SubmissionFileRow[] = [];
  for (const item of items) {
    rows.push({ item, depth });
    if (item.children?.length) {
      rows.push(...flattenSubmissionFileRows(item.children, depth + 1));
    }
  }
  return rows;
}

const activeSubmissionFileRows = computed(() => (
  activeSubmission.value ? flattenSubmissionFileRows(activeSubmission.value.items) : []
));

const activeSubmissionPreviewFiles = computed(() => (
  activeSubmissionFileRows.value
    .map((row) => row.item)
    .filter((item) => item.kind === "file")
));

const submissionPreviewIndex = computed(() => (
  submissionPreviewItem.value
    ? activeSubmissionPreviewFiles.value.findIndex((item) => item.id === submissionPreviewItem.value?.id)
    : -1
));

const submissionPreviewHasPrevious = computed(() => submissionPreviewIndex.value > 0);
const submissionPreviewHasNext = computed(() => (
  submissionPreviewIndex.value >= 0 && submissionPreviewIndex.value < activeSubmissionPreviewFiles.value.length - 1
));

const assignmentAttachmentResources = computed(() => (
  mapAttachmentResources(attachments.value, { includeSize: true })
));

const submissionArchiveUrl = computed(() => {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return "";
  }
  return api.assignmentSubmissionsArchiveUrl({
    classId: currentClassId.value,
    assignmentIds: [currentAssignmentId.value],
  });
});
const assignmentMissingStatsTotalPages = computed(() => (
  Math.max(1, Math.ceil(assignmentMissingStatsRows.value.length / assignmentMissingStatsPageSize.value))
));
const assignmentMissingStatsSummaryText = computed(() => (
  `共 ${assignmentMissingStatsRows.value.length} 名未交学生 · 全班 ${assignmentMissingRosterTotal.value} 人 · 已交 ${assignmentMissingSubmittedTotal.value} 人`
));
const paginatedAssignmentMissingStatsRows = computed(() => {
  const start = (assignmentMissingStatsPage.value - 1) * assignmentMissingStatsPageSize.value;
  return assignmentMissingStatsRows.value.slice(start, start + assignmentMissingStatsPageSize.value);
});
const assignmentMissingStatsTableFrameStyle = computed<Record<string, string>>(() => ({
  "--assignment-missing-page-size": String(assignmentMissingStatsPageSize.value),
}));

function formatDateTime(value: string) {
  if (!value) {
    return uiCopy.emptyValue;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString("zh-CN", {
    hour12: false,
  });
}

function toDateTimeLocalValue(value: string) {
  if (!value) {
    return "";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }
  const offset = parsed.getTimezoneOffset() * 60_000;
  return new Date(parsed.getTime() - offset).toISOString().slice(0, 16);
}

function reviewStatusText(value: "pending" | "reviewed") {
  return reviewStatusLabel(value);
}

function reviewStatusTone(value: "pending" | "reviewed"): StatusPillTone {
  return value === "reviewed" ? "status-pill--success" : "status-pill--warning";
}

function submissionStatusText(value: "partial" | "submitted") {
  return submissionStatusLabel(value);
}

function submissionStatusPillTone(value: "partial" | "submitted"): StatusPillTone {
  return submissionStatusTone(value);
}

function normalizeMinFileCount(value: number) {
  return Number.isInteger(value) && value > 0 && value <= 500 ? value : null;
}

function formatSubmissionItemsSummary(items: AssignmentAttachmentItem[]) {
  if (!items.length) {
    return "0 个文件";
  }
  const folderCount = items.filter((item) => item.kind === "dir").length;
  const fileCount = items.reduce((sum, item) => sum + (item.kind === "dir" ? item.fileCount ?? 0 : 1), 0);
  if (folderCount > 0) {
    return `文件夹 ${folderCount} 个 · ${fileCount} 个文件`;
  }
  return `${fileCount} 个文件`;
}

function formatSubmissionItemNames(items: AssignmentAttachmentItem[]) {
  const names = items.slice(0, 2).map((item) => item.name).join("，");
  if (items.length <= 2) {
    return names;
  }
  return `${names} 等 ${items.length} 项`;
}

function mapAttachmentResources(
  items: AssignmentAttachmentItem[],
  options: { includeSize?: boolean; openInNewTab?: boolean } = {},
) {
  return items.map((item) => ({
    id: item.id,
    name: item.name,
    href: item.kind === "dir" ? item.archiveUrl || item.downloadUrl : item.downloadUrl,
    meta: buildAttachmentMeta(item, options.includeSize === true),
    openInNewTab: options.openInNewTab ?? false,
  }));
}

function buildAttachmentMeta(item: AssignmentAttachmentItem, includeSize: boolean) {
  if (item.kind === "dir") {
    const folderText = item.folderCount ? `，${item.folderCount} 个子文件夹` : "";
    return `文件夹 · ${item.fileCount ?? 0} 个文件${folderText}`;
  }
  return includeSize && typeof item.size === "number" ? formatFileSize(item.size) : undefined;
}

function formatFileSize(value: number) {
  if (!Number.isFinite(value) || value < 0) {
    return undefined;
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

function submissionDownloadUrl(item: AssignmentAttachmentItem) {
  return item.kind === "dir" ? item.archiveUrl || item.downloadUrl : item.downloadUrl;
}

function submissionFileTypeLabel(item: AssignmentAttachmentItem) {
  return item.kind === "dir" ? "文件夹" : "文件";
}

function submissionFilePathLabel(item: AssignmentAttachmentItem) {
  const normalized = item.path.trim();
  if (!normalized || normalized === "/") {
    return "根目录";
  }
  return normalized;
}

function setSubmissionFileViewMode(mode: SubmissionFileViewMode) {
  submissionFileViewMode.value = mode;
}

function setSubmissionFileGridSize(size: SubmissionFileGridSize) {
  submissionFileGridSize.value = size;
}

function startDownload(url: string) {
  const link = document.createElement("a");
  link.href = url;
  document.body.appendChild(link);
  link.click();
  link.remove();
}

function toDueAtPayload(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return trimmed;
  }
  return parsed.toISOString();
}

async function navigateDrawerBoundaryPage(page: number, selection: DrawerBoundarySelection) {
  if (drawerNavigationLoading.value) {
    return;
  }
  drawerNavigationLoading.value = true;
  pendingDrawerBoundarySelection.value = selection;
  try {
    await replaceSubmissionRoute({ page });
  } catch (error) {
    drawerNavigationLoading.value = false;
    pendingDrawerBoundarySelection.value = null;
    toastStore.push("error", error instanceof ApiError ? error.message : "切换学生失败");
  }
}

function applyPendingDrawerBoundarySelection() {
  const selection = pendingDrawerBoundarySelection.value;
  if (!selection) {
    return;
  }
  pendingDrawerBoundarySelection.value = null;
  drawerNavigationLoading.value = false;
  const nextSubmission = selection === "first"
    ? visibleSubmissions.value[0]
    : visibleSubmissions.value[visibleSubmissions.value.length - 1];
  assignmentDetailStore.setActiveSubmission(nextSubmission?.id ?? null);
  handleDrawerActiveSubmissionChanged();
}

async function selectPrevSubmission() {
  if (drawerNavigationLoading.value || activeSubmissionIndex.value < 0) {
    return;
  }
  if (activeSubmissionIndex.value > 0) {
    assignmentDetailStore.selectPrevSubmission();
    handleDrawerActiveSubmissionChanged();
    return;
  }
  if (submissionPage.value > 1) {
    await navigateDrawerBoundaryPage(submissionPage.value - 1, "last");
  }
}

async function selectNextSubmission() {
  if (drawerNavigationLoading.value || activeSubmissionIndex.value < 0) {
    return;
  }
  if (activeSubmissionIndex.value < visibleSubmissions.value.length - 1) {
    assignmentDetailStore.selectNextSubmission();
    handleDrawerActiveSubmissionChanged();
    return;
  }
  if (submissionPage.value < submissionTotalPages.value) {
    await navigateDrawerBoundaryPage(submissionPage.value + 1, "first");
  }
}

function openSubmissionArchiveDownloadConfirm() {
  submissionArchiveDownloadConfirmOpen.value = true;
}

function confirmSubmissionArchiveDownload() {
  if (!submissionArchiveUrl.value) {
    submissionArchiveDownloadConfirmOpen.value = false;
    toastStore.push("error", "当前作业提交包暂不可下载");
    return;
  }
  submissionArchiveDownloadConfirmOpen.value = false;
  startDownload(submissionArchiveUrl.value);
  toastStore.push("success", "已开始下载本作业提交包");
}

function openSubmissionDrawer(submissionID: number) {
  assignmentDetailStore.setActiveSubmission(submissionID);
  reviewDrawerOpen.value = true;
  handleDrawerActiveSubmissionChanged();
}

function canPreviewSubmissionItem(item: AssignmentAttachmentItem) {
  return item.kind === "file";
}

async function markSubmissionReviewedOnOpen(submissionID: number) {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  const target = visibleSubmissions.value.find((submission) => submission.id === submissionID);
  if (!target || target.reviewStatus === "reviewed") {
    return;
  }
  const draft = submissionReviewDrafts.value[submissionID];
  try {
    const updated = await assignmentDetailStore.saveSubmissionReview({
      assignmentId: currentAssignmentId.value,
      classId: currentClassId.value,
      submissionId: submissionID,
      reviewStatus: "reviewed",
      teacherComment: draft?.teacherComment ?? target.teacherCommentSummary,
    });
    assignmentDetailStore.setActiveSubmission(updated.id);
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "自动标记已批改失败，可手动保存");
  }
}

function handleDrawerActiveSubmissionChanged() {
  if (!reviewDrawerOpen.value || activeSubmissionId.value === null) {
    return;
  }
  const submissionID = activeSubmissionId.value;
  void markSubmissionReviewedOnOpen(submissionID);
}

function requestCloseReviewDrawer() {
  if (hasActiveSubmissionReviewDraftChanges.value) {
    pendingReviewDrawerCloseDone.value = null;
    reviewDrawerCloseConfirmOpen.value = true;
    return;
  }
  reviewDrawerOpen.value = false;
}

function handleReviewDrawerBeforeClose(done: () => void) {
  if (hasActiveSubmissionReviewDraftChanges.value) {
    pendingReviewDrawerCloseDone.value = done;
    reviewDrawerCloseConfirmOpen.value = true;
    return;
  }
  done();
}

function confirmCloseReviewDrawer() {
  reviewDrawerCloseConfirmOpen.value = false;
  const pendingDone = pendingReviewDrawerCloseDone.value;
  pendingReviewDrawerCloseDone.value = null;
  if (pendingDone) {
    pendingDone();
    return;
  }
  reviewDrawerOpen.value = false;
}

function cancelCloseReviewDrawer() {
  reviewDrawerCloseConfirmOpen.value = false;
  pendingReviewDrawerCloseDone.value = null;
}

function closeSubmissionPreview() {
  submissionPreviewItem.value = null;
  submissionPreviewTextContent.value = "";
  submissionPreviewMarkdownContent.value = "";
  submissionPreviewLoading.value = false;
  submissionPreviewErrorText.value = "";
}

async function previewSubmissionItem(item: AssignmentAttachmentItem) {
  if (!canPreviewSubmissionItem(item)) {
    return;
  }
  const previewItem = toSubmissionPreviewItem(item);
  const kind = resolvePreviewKind(previewItem);
  submissionPreviewItem.value = previewItem;
  submissionPreviewErrorText.value = "";
  submissionPreviewTextContent.value = "";
  submissionPreviewMarkdownContent.value = "";
  if (kind !== "text" && kind !== "markdown") {
    submissionPreviewLoading.value = false;
    return;
  }
  const cached = submissionPreviewTextCache.value.get(previewItem.id);
  if (cached !== undefined) {
    if (kind === "markdown") {
      submissionPreviewMarkdownContent.value = renderMarkdownPreview(cached);
    } else {
      submissionPreviewTextContent.value = cached;
    }
    submissionPreviewLoading.value = false;
    return;
  }
  submissionPreviewLoading.value = true;
  try {
    const response = await fetch(previewItem.previewUrl, { credentials: "same-origin" });
    if (!response.ok) {
      throw new Error("preview_failed");
    }
    const text = await response.text();
    submissionPreviewTextCache.value.set(previewItem.id, text);
    if (kind === "markdown") {
      submissionPreviewMarkdownContent.value = renderMarkdownPreview(text);
    } else {
      submissionPreviewTextContent.value = text;
    }
  } catch {
    submissionPreviewErrorText.value = "预览失败，请下载后查看。";
  } finally {
    submissionPreviewLoading.value = false;
  }
}

async function previewSubmissionFileAt(index: number) {
  const nextItem = activeSubmissionPreviewFiles.value[index];
  if (!nextItem) {
    return;
  }
  await previewSubmissionItem(nextItem);
}

async function previewPreviousSubmissionFile() {
  if (!submissionPreviewHasPrevious.value) {
    return;
  }
  await previewSubmissionFileAt(submissionPreviewIndex.value - 1);
}

async function previewNextSubmissionFile() {
  if (!submissionPreviewHasNext.value) {
    return;
  }
  await previewSubmissionFileAt(submissionPreviewIndex.value + 1);
}

function openEditDialog() {
  if (!assignment.value) {
    return;
  }
  assignmentDetailStore.syncEditForm(assignment.value);
  editDialogDiscardConfirmOpen.value = false;
  editDialogOpen.value = true;
}

function closeEditDialog() {
  if (hasEditDialogDraftChanges.value) {
    editDialogDiscardConfirmOpen.value = true;
    return;
  }
  forceCloseEditDialog();
}

function forceCloseEditDialog() {
  editDialogOpen.value = false;
  editDialogDiscardConfirmOpen.value = false;
}

function confirmCloseEditDialog() {
  forceCloseEditDialog();
}

function cancelCloseEditDialog() {
  editDialogDiscardConfirmOpen.value = false;
}

async function loadClasses() {
  try {
    await classesStore.load();
  } catch {
    classesStore.clear();
  }
}

async function loadPage() {
  if (!currentClassId.value || !currentAssignmentId.value) {
    assignmentDetailStore.clear();
    assignmentDetailStore.notFound = true;
    return;
  }
  applySubmissionStateFromRoute();
  const canReuseAssignment = assignment.value?.id === currentAssignmentId.value
    && assignment.value.classId === currentClassId.value
    && !notFound.value;
  try {
    if (canReuseAssignment) {
      await assignmentDetailStore.loadSubmissions(currentClassId.value, currentAssignmentId.value, buildSubmissionRequestOptions());
    } else {
      await loadClasses();
      await assignmentDetailStore.loadPage(currentClassId.value, currentAssignmentId.value, buildSubmissionRequestOptions());
    }
    if (submissionPage.value > submissionTotalPages.value) {
      await replaceSubmissionRoute({ page: submissionTotalPages.value });
      return;
    }
    applyPendingDrawerBoundarySelection();
  } catch (error) {
    drawerNavigationLoading.value = false;
    pendingDrawerBoundarySelection.value = null;
    toastStore.push("error", error instanceof ApiError ? error.message : "加载作业详情失败");
  }
}

async function applySubmissionFilters() {
  await replaceSubmissionRoute({ q: submissionFilter.value, page: 1 });
}

async function applySubmissionSort() {
  await replaceSubmissionRoute({ sort: submissionSort.value, page: 1 });
}

async function refreshSubmissions() {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  try {
    await assignmentDetailStore.loadSubmissions(currentClassId.value, currentAssignmentId.value, buildSubmissionRequestOptions());
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "刷新学生提交失败");
  }
}

async function updateSubmissionPageSize(value: number) {
  await replaceSubmissionRoute({ pageSize: value, page: 1 });
}

async function goPrevSubmissionPage() {
  if (submissionPage.value <= 1) {
    return;
  }
  await replaceSubmissionRoute({ page: submissionPage.value - 1 });
}

async function goNextSubmissionPage() {
  if (submissionPage.value >= submissionTotalPages.value) {
    return;
  }
  await replaceSubmissionRoute({ page: submissionPage.value + 1 });
}

async function goSubmissionPage(page: number): Promise<void> {
  const nextPage = Math.min(Math.max(1, Math.trunc(page)), submissionTotalPages.value);
  if (nextPage === submissionPage.value) {
    return;
  }
  await replaceSubmissionRoute({ page: nextPage });
}

function openAttachmentUpload() {
  attachmentInput.value?.click();
}

async function saveSubmissionReview(submissionID: number) {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  const draft = submissionReviewDrafts.value[submissionID];
  if (!draft) {
    return;
  }
  try {
    const updated = await assignmentDetailStore.saveSubmissionReview({
      assignmentId: currentAssignmentId.value,
      classId: currentClassId.value,
      submissionId: submissionID,
      reviewStatus: draft.reviewStatus,
      teacherComment: draft.teacherComment,
    });
    assignmentDetailStore.setActiveSubmission(updated.id);
    toastStore.push("success", "批改摘要已保存");
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "保存批改摘要失败");
  }
}

async function openAssignmentMissingStatsDialog() {
  assignmentMissingStatsOpen.value = true;
  assignmentMissingStatsPage.value = 1;
  await loadAssignmentMissingStats();
}

function closeAssignmentMissingStatsDialog() {
  assignmentMissingStatsOpen.value = false;
}

async function loadAssignmentMissingStats() {
  if (!currentClassId.value || !currentAssignmentId.value) {
    assignmentMissingStatsRows.value = [];
    assignmentMissingRosterTotal.value = 0;
    assignmentMissingSubmittedTotal.value = 0;
    return;
  }
  assignmentMissingStatsLoading.value = true;
  try {
    const statistics = await api.assignmentStatistics({
      classId: currentClassId.value,
      assignmentIds: [currentAssignmentId.value],
    });
    assignmentMissingRosterTotal.value = statistics.rosterTotal;
    assignmentMissingSubmittedTotal.value = statistics.submittedTotal;

    assignmentMissingStatsRows.value = (statistics.rows ?? [])
      .filter((student) => student.missingCount > 0)
      .map((student) => ({
        studentId: student.studentId,
        studentNo: student.studentNo,
        displayName: student.displayName,
      }))
      .sort((a, b) => a.studentNo.localeCompare(b.studentNo));
    if (assignmentMissingStatsPage.value > assignmentMissingStatsTotalPages.value) {
      assignmentMissingStatsPage.value = assignmentMissingStatsTotalPages.value;
    }
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "统计未交名单失败");
    assignmentMissingStatsRows.value = [];
    assignmentMissingRosterTotal.value = 0;
    assignmentMissingSubmittedTotal.value = 0;
  } finally {
    assignmentMissingStatsLoading.value = false;
  }
}

function updateAssignmentMissingStatsPageSize(value: number) {
  assignmentMissingStatsPageSize.value = assignmentMissingStatsPageSizeOptions.includes(value)
    ? value
    : defaultAssignmentMissingStatsPageSize;
  assignmentMissingStatsPage.value = 1;
}

function goPrevAssignmentMissingStatsPage() {
  if (assignmentMissingStatsPage.value <= 1) {
    return;
  }
  assignmentMissingStatsPage.value -= 1;
}

function goNextAssignmentMissingStatsPage() {
  if (assignmentMissingStatsPage.value >= assignmentMissingStatsTotalPages.value) {
    return;
  }
  assignmentMissingStatsPage.value += 1;
}

function goAssignmentMissingStatsPage(page: number): void {
  const nextPage = Math.min(Math.max(1, Math.trunc(page)), assignmentMissingStatsTotalPages.value);
  if (nextPage === assignmentMissingStatsPage.value) {
    return;
  }
  assignmentMissingStatsPage.value = nextPage;
}

function exportAssignmentMissingStats() {
  if (!assignmentMissingStatsRows.value.length) {
    toastStore.push("warning", "当前没有可导出的未交统计");
    return;
  }
  exportRowsToSpreadsheet({
    fileName: `${assignment.value?.title ?? "作业"}-未交统计.xls`,
    sheetName: "未交统计",
    rows: assignmentMissingStatsRows.value,
    columns: [
      { header: "学生", value: (row) => row.displayName },
      { header: "学号", value: (row) => row.studentNo },
    ],
  });
  toastStore.push("success", "未交统计已导出");
}

async function markAllSubmissionsReviewed() {
  if (!currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  try {
    const allSubmissions: typeof submissions.value = [];
    let page = 1;
    let totalPages = 1;
    do {
      const response = await api.assignmentSubmissions(currentAssignmentId.value, {
        classId: currentClassId.value,
        sort: submissionSort.value,
        page,
        pageSize: fullStatsPageSize,
      });
      allSubmissions.push(...(response.submissions ?? []));
      totalPages = Math.max(1, response.pagination?.totalPages ?? 1);
      page += 1;
    } while (page <= totalPages);

    for (const submission of allSubmissions) {
      const draft = submissionReviewDrafts.value[submission.id];
      await assignmentDetailStore.saveSubmissionReview({
        assignmentId: currentAssignmentId.value,
        classId: currentClassId.value,
        submissionId: submission.id,
        reviewStatus: "reviewed",
        teacherComment: draft?.teacherComment ?? submission.teacherCommentSummary,
      });
    }

    await assignmentDetailStore.loadSubmissions(currentClassId.value, currentAssignmentId.value, buildSubmissionRequestOptions());
    toastStore.push("success", `已将 ${allSubmissions.length} 份提交标记为已批改`);
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "一键批改失败");
  }
}

async function saveAssignment() {
  if (!assignment.value || !currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  if (!editTitle.value.trim()) {
    toastStore.push("error", "请输入作业标题");
    return;
  }
  const minFileCount = normalizeMinFileCount(editMinFileCount.value);
  if (minFileCount === null) {
    toastStore.push("error", "最少文件数需在 1 到 500 之间");
    return;
  }
  try {
    const updated = await assignmentDetailStore.saveAssignment({
      assignmentId: currentAssignmentId.value,
      classId: currentClassId.value,
      title: editTitle.value.trim(),
      description: editDescription.value.trim(),
      dueAt: toDueAtPayload(editDueAt.value),
      status: editStatus.value,
      submissionMode: editSubmissionMode.value,
      submissionTypeCategory: editSubmissionTypeCategory.value,
      minFileCount,
    });
    if (updated) {
      assignment.value = updated;
    }
    forceCloseEditDialog();
    toastStore.push("success", "作业已更新");
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "更新作业失败");
  }
}

async function handleAttachmentUpload(event: Event) {
  const target = event.target as HTMLInputElement;
  const files = target.files ? Array.from(target.files) : [];
  if (!currentClassId.value || !currentAssignmentId.value || files.length === 0) {
    return;
  }
  const totalBytes = files.reduce((sum, file) => sum + file.size, 0);
  uploadStore.start(totalBytes);
  try {
    await assignmentDetailStore.uploadAttachments({
      assignmentId: currentAssignmentId.value,
      classId: currentClassId.value,
      files,
    });
    if (totalBytes > 0) {
      uploadStore.progress(totalBytes);
    }
    toastStore.push("success", "附件已上传");
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "上传作业附件失败");
  } finally {
    uploadStore.finish();
    target.value = "";
  }
}

function removeAttachment(fileId: number, fileName: string) {
  pendingAttachmentDelete.value = { id: fileId, name: fileName };
}

async function confirmRemoveAttachment() {
  if (!currentClassId.value || !currentAssignmentId.value || !pendingAttachmentDelete.value) {
    return;
  }
  try {
    await assignmentDetailStore.deleteAttachment(currentClassId.value, currentAssignmentId.value, pendingAttachmentDelete.value.id);
    pendingAttachmentDelete.value = null;
    toastStore.push("success", "附件已删除");
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "删除附件失败");
  }
}

function removeAssignment() {
  pendingAssignmentDelete.value = true;
}

async function confirmRemoveAssignment() {
  if (!assignment.value || !currentClassId.value || !currentAssignmentId.value) {
    return;
  }
  try {
    await assignmentDetailStore.deleteAssignment(currentClassId.value, currentAssignmentId.value);
    pendingAssignmentDelete.value = false;
    toastStore.push("success", "作业已删除");
    await router.push(backLink.value);
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "删除作业失败");
  }
}

watch([
  currentClassId,
  currentAssignmentId,
  () => route.query.q,
  () => route.query.sort,
  () => route.query.page,
  () => route.query.pageSize,
], () => {
  void loadPage();
}, { immediate: true });

watch(visibleSubmissions, (items) => {
  if (pendingDrawerBoundarySelection.value) {
    applyPendingDrawerBoundarySelection();
    return;
  }
  if (!items.length) {
    assignmentDetailStore.setActiveSubmission(null);
    return;
  }
  if (!items.some((submission) => submission.id === activeSubmissionId.value)) {
    assignmentDetailStore.setActiveSubmission(items[0]?.id ?? null);
  }
});

</script>

<style scoped>
.classes-page__detail-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.classes-page__detail-main {
  display: grid;
  gap: 12px;
}

.classes-page__header-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  justify-content: flex-end;
}

.assignment-detail-header {
  align-items: stretch;
  padding: 1rem 1.1rem;
}

.assignment-detail-heading {
  display: grid;
  gap: 7px;
  min-width: 0;
  align-content: center;
}

.assignment-detail-heading .classes-page__title {
  margin: 0;
  font-size: 1.16rem;
  line-height: 1.22;
}

.assignment-detail-heading__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  color: var(--text-secondary);
  font-size: 0.92rem;
  font-weight: 700;
}

.assignment-detail-heading__class,
.assignment-detail-heading__due {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid var(--border-soft);
  border-radius: 999px;
  background: var(--bg-subtle);
  color: var(--text-secondary);
}

.assignment-detail-heading__description {
  margin: 0;
  max-width: 74ch;
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 0.96rem;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: normal;
}

.assignment-submissions-workspace {
  display: grid;
  gap: 8px;
}

.assignment-missing-dialog {
  width: min(720px, 100%);
}

.assignment-missing-dialog__body {
  display: grid;
  gap: 10px;
}

.assignment-missing-dialog__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.assignment-missing-dialog__count {
  color: var(--text-secondary);
  font-size: 0.96rem;
  font-weight: 700;
}

.assignment-missing-dialog__table-frame {
  align-self: start;
  width: 100%;
  min-height: calc(44px * (var(--assignment-missing-page-size) + 1));
  overflow-x: auto;
  background: var(--bg-surface);
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  box-shadow: var(--shadow-soft);
}

.assignment-missing-dialog__table {
  width: 100%;
  min-width: 480px;
  height: auto;
  border: 0;
  border-radius: 0;
  box-shadow: none;
  table-layout: fixed;
}

.assignment-missing-dialog__table thead tr,
.assignment-missing-dialog__table tbody tr {
  height: 44px;
}

.assignment-missing-dialog__table th,
.assignment-missing-dialog__table td {
  box-sizing: border-box;
  height: 44px;
  max-height: 44px;
}

.assignment-submissions-workspace__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.assignment-submissions-workspace__header p {
  margin: 0;
}

.assignment-submissions-table-wrap {
  width: 100%;
  overflow-x: auto;
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  background: var(--bg-surface);
}

.assignment-submissions-table {
  min-width: 980px;
  table-layout: fixed;
  border: 0;
  border-radius: 0;
  box-shadow: none;
}

.assignment-submissions-table th {
  background: var(--bg-subtle);
  color: var(--text-muted);
  font-size: 0.94rem;
  font-weight: 800;
}

.assignment-submissions-table th:nth-child(1),
.assignment-submissions-table td:nth-child(1) {
  width: 20%;
}

.assignment-submissions-table th:nth-child(2),
.assignment-submissions-table td:nth-child(2) {
  width: 33%;
}

.assignment-submissions-table th:nth-child(3),
.assignment-submissions-table td:nth-child(3) {
  width: 21%;
}

.assignment-submissions-table th:nth-child(4),
.assignment-submissions-table td:nth-child(4) {
  width: 16%;
}

.assignment-submissions-table th:nth-child(5),
.assignment-submissions-table td:nth-child(5) {
  width: 10%;
}

.assignment-submissions-table__row {
  cursor: pointer;
  transition: background-color 150ms ease, box-shadow 150ms ease;
}

.assignment-submissions-table__row:hover {
  background: color-mix(in srgb, var(--accent-primary) 4%, transparent);
}

.assignment-submissions-table__row.is-active {
  background: color-mix(in srgb, var(--accent-primary) 7%, transparent);
  box-shadow: inset 3px 0 0 var(--accent-primary);
}

.assignment-submissions-table th,
.assignment-submissions-table td {
  padding: 6px 8px;
  vertical-align: middle;
}

.assignment-submissions-table__student,
.assignment-submissions-table__content,
.assignment-submissions-table__time,
.assignment-submissions-table__review {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.assignment-submissions-table__student strong,
.assignment-submissions-table__content strong,
.assignment-submissions-table__time span {
  overflow: hidden;
  color: var(--text-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignment-submissions-table__student span,
.assignment-submissions-table__item-names,
.assignment-submissions-table__secondary,
.assignment-submissions-table__review span:not(.status-pill),
.assignment-submissions-table__review .assignment-submissions-table__secondary {
  overflow: hidden;
  color: var(--text-muted);
  font-size: 0.96rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignment-submissions-table__content-meta {
  display: flex;
  gap: 6px;
  align-items: center;
  min-width: 0;
}

.assignment-submissions-table__item-names {
  min-width: 0;
}

.assignment-submissions-table__review {
  align-items: flex-start;
}

.classes-page__submission-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 6px 10px;
  align-items: center;
  padding: 6px 8px;
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  background: var(--modal-subtle);
}

.classes-page__submission-toolbar-group,
.classes-page__submission-review {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.classes-page__submission-toolbar-group--end {
  margin-left: auto;
  justify-content: flex-end;
}

.classes-page__submission-toolbar-group {
  min-width: 0;
}

.classes-page__submission-toolbar-group:not(:first-child) {
  padding-left: 12px;
  border-left: 1px solid var(--border-soft);
}

.classes-page__submission-toolbar-group--filters {
  flex: 0 0 auto;
  flex-wrap: nowrap;
}

.classes-page__submission-toolbar .button {
  min-height: 30px;
  padding: 6px 10px;
  border-radius: 8px;
}

.classes-page__submission-action {
  min-height: 30px;
  border-radius: 8px;
}

.classes-page__submission-toolbar .copy-dialog__search {
  min-height: 34px;
  padding: 6px 10px;
  border-radius: 9px;
}

.classes-page__submission-action:disabled,
.assignment-review-drawer__nav .button:disabled {
  cursor: not-allowed;
  opacity: 1;
  transform: none;
  box-shadow: none;
}

.classes-page__submission-nav-group,
.assignment-review-drawer__nav {
  display: inline-flex;
  gap: 6px;
  align-items: center;
}

.assignment-submissions-table__open {
  min-height: 28px;
  padding: 5px 10px;
  border-radius: 8px;
}

.classes-page__submission-toolbar .button:not(.button--primary):not(.button--success):not(.button--secondary):not(.button--accent) {
  background: var(--control-bg);
  border-color: var(--control-border);
  color: var(--text-primary);
}

.classes-page__submission-toolbar .button:disabled,
.classes-page__submission-toolbar .button:disabled:hover {
  background: var(--bg-muted);
  border-color: var(--border-soft);
  color: var(--text-muted);
}

.classes-page__submission-toolbar :deep(.el-button) {
  min-height: 32px;
  padding: 6px 10px;
  border-radius: 9px;
}

.assignment-submissions-workspace :deep(.el-button:not(.el-button--primary):not(.el-button--success):not(.el-button--warning):not(.el-button--danger)) {
  --el-button-bg-color: var(--control-bg);
  --el-button-border-color: var(--border-soft);
  --el-button-text-color: var(--text-primary);
  --el-button-hover-bg-color: color-mix(in srgb, var(--accent-primary) 8%, var(--bg-subtle));
  --el-button-hover-border-color: var(--border-strong);
  --el-button-hover-text-color: var(--text-primary);
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

.assignment-submissions-workspace :deep(.el-button--primary.is-plain) {
  --el-button-bg-color: color-mix(in srgb, var(--accent-primary) 12%, var(--bg-subtle));
  --el-button-border-color: color-mix(in srgb, var(--accent-primary) 44%, var(--border-strong));
  --el-button-text-color: var(--accent-primary);
  --el-button-hover-bg-color: var(--accent-primary);
  --el-button-hover-border-color: var(--accent-primary);
  --el-button-hover-text-color: #fff;
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

.assignment-submissions-workspace :deep(.el-button--success.is-plain) {
  --el-button-bg-color: color-mix(in srgb, var(--success) 12%, var(--bg-subtle));
  --el-button-border-color: color-mix(in srgb, var(--success) 44%, var(--border-strong));
  --el-button-text-color: var(--success);
  --el-button-hover-bg-color: var(--success);
  --el-button-hover-border-color: var(--success);
  --el-button-hover-text-color: #fff;
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

.assignment-submissions-workspace :deep(.el-button--primary:not(.is-plain)) {
  --el-button-bg-color: var(--accent-primary);
  --el-button-border-color: var(--accent-primary);
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: var(--accent-primary-hover);
  --el-button-hover-border-color: var(--accent-primary-hover);
  --el-button-hover-text-color: #fff;
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

.assignment-submissions-workspace :deep(.el-button.is-disabled),
.assignment-submissions-workspace :deep(.el-button.is-disabled:hover) {
  --el-button-disabled-bg-color: color-mix(in srgb, var(--bg-subtle) 74%, transparent);
  --el-button-disabled-border-color: var(--border-soft);
  --el-button-disabled-text-color: var(--text-muted);
  background: var(--el-button-disabled-bg-color);
  border-color: var(--el-button-disabled-border-color);
  color: var(--el-button-disabled-text-color);
  opacity: 1;
}

.assignment-submissions-workspace :deep(.el-tag) {
  --el-tag-border-radius: 999px;
  font-weight: 700;
}

.assignment-submissions-workspace :deep(.el-tag--info) {
  --el-tag-bg-color: color-mix(in srgb, var(--text-muted) 10%, var(--bg-subtle));
  --el-tag-border-color: color-mix(in srgb, var(--text-muted) 24%, var(--border-soft));
  --el-tag-text-color: var(--text-secondary);
  background: var(--el-tag-bg-color);
  border-color: var(--el-tag-border-color);
  color: var(--el-tag-text-color);
}

.assignment-submissions-workspace :deep(.el-tag--success) {
  --el-tag-bg-color: color-mix(in srgb, var(--success) 12%, var(--bg-subtle));
  --el-tag-border-color: color-mix(in srgb, var(--success) 32%, var(--border-soft));
  --el-tag-text-color: var(--success);
  background: var(--el-tag-bg-color);
  border-color: var(--el-tag-border-color);
  color: var(--el-tag-text-color);
}

.classes-page__submission-search {
  flex: 0 0 clamp(190px, 22vw, 260px);
  width: clamp(190px, 22vw, 260px);
  min-width: 180px;
}

.classes-page__submission-sort {
  flex: 0 0 114px;
  width: 114px;
}

.classes-page__submission-toolbar-label {
  color: var(--text-muted);
  font-size: 0.94rem;
  font-weight: 800;
}

:deep(.assignment-review-drawer.el-drawer) {
  --review-drawer-bg: #f8fbff;
  --review-panel-bg: #ffffff;
  --review-panel-subtle: #eef4fb;
  --review-divider: rgba(15, 23, 42, 0.1);
  background: var(--review-drawer-bg);
  color: var(--text-primary);
  font-size: 15px;
}

:global(:root.dark .assignment-review-drawer.el-drawer) {
  --review-drawer-bg: #243047;
  --review-panel-bg: #2d3d56;
  --review-panel-subtle: #334660;
  --review-divider: rgba(148, 197, 253, 0.2);
  background: var(--review-drawer-bg);
  color: var(--text-primary);
}

:deep(.assignment-review-drawer.el-drawer .el-drawer__body) {
  padding: 12px 20px 20px;
  background: var(--review-drawer-bg);
}

:global(:root.dark .assignment-review-drawer.el-drawer .el-drawer__body) {
  background: var(--review-drawer-bg);
}

:deep(.assignment-review-drawer.el-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--review-divider);
  background: var(--review-panel-bg);
  color: var(--text-primary);
}

:global(:root.dark .assignment-review-drawer.el-drawer .el-drawer__header) {
  border-bottom-color: var(--review-divider);
  background: var(--review-panel-bg);
  color: var(--text-primary);
}

:deep(.assignment-review-drawer.el-drawer .el-button:not(.el-button--primary):not(.el-button--success):not(.el-button--warning):not(.el-button--danger)) {
  --el-button-bg-color: var(--bg-subtle);
  --el-button-border-color: var(--border-soft);
  --el-button-text-color: var(--text-primary);
  --el-button-hover-bg-color: color-mix(in srgb, var(--accent-primary) 12%, var(--bg-subtle));
  --el-button-hover-border-color: var(--border-strong);
  --el-button-hover-text-color: var(--text-primary);
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

:deep(.assignment-review-drawer.el-drawer .el-button--primary.is-plain) {
  --el-button-bg-color: color-mix(in srgb, var(--accent-primary) 12%, var(--bg-subtle));
  --el-button-border-color: color-mix(in srgb, var(--accent-primary) 42%, var(--border-strong));
  --el-button-text-color: var(--text-primary);
  --el-button-hover-bg-color: var(--accent-primary);
  --el-button-hover-border-color: var(--accent-primary);
  --el-button-hover-text-color: #fff;
  background: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
  color: var(--el-button-text-color);
}

:deep(.assignment-review-drawer.el-drawer .el-button.is-disabled),
:deep(.assignment-review-drawer.el-drawer .el-button.is-disabled:hover) {
  --el-button-disabled-bg-color: color-mix(in srgb, var(--bg-subtle) 74%, transparent);
  --el-button-disabled-border-color: var(--border-soft);
  --el-button-disabled-text-color: var(--text-muted);
  background: var(--el-button-disabled-bg-color);
  border-color: var(--el-button-disabled-border-color);
  color: var(--el-button-disabled-text-color);
  opacity: 1;
}

.assignment-review-drawer__header {
  width: 100%;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
}

.assignment-review-drawer__header h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1.18rem;
  line-height: 1.2;
}

.assignment-review-drawer__header-subtitle {
  margin: 5px 0 0;
  color: var(--text-secondary);
  font-size: 0.98rem;
  font-weight: 700;
}

.assignment-review-drawer__nav-button {
  min-height: 38px;
  padding: 7px 12px;
  border-radius: 8px;
  background: var(--review-panel-subtle);
  border-color: var(--review-divider);
  color: var(--text-primary);
  font-weight: 800;
}

.assignment-review-drawer__nav-button:disabled,
.assignment-review-drawer__nav-button:disabled:hover {
  background: var(--review-panel-subtle);
  border-color: var(--review-divider);
  color: var(--text-secondary);
  opacity: 1;
}

.assignment-review-drawer__summary {
  display: grid;
  grid-template-columns: minmax(220px, auto) auto minmax(0, 1fr);
  gap: 14px;
  align-items: center;
  margin-bottom: 12px;
  padding: 12px 14px;
  border: 1px solid var(--review-divider);
  border-radius: 10px;
  background: var(--review-panel-bg);
}

.assignment-review-drawer__identity {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
  min-width: 0;
}

.assignment-review-drawer__identity strong {
  color: var(--text-primary);
  font-size: 1.2rem;
  line-height: 1.2;
}

.assignment-review-drawer__identity span:last-child {
  color: var(--text-secondary);
  font-weight: 700;
}

.assignment-review-drawer__status-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: flex-start;
}

.assignment-review-drawer__summary-list {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px 14px;
  min-width: 0;
  color: var(--text-secondary);
  font-size: 0.96rem;
}

.assignment-review-drawer__summary-list span {
  min-width: 0;
  display: inline-flex;
  gap: 6px;
  align-items: center;
  white-space: nowrap;
}

.assignment-review-drawer__summary-list strong {
  color: var(--text-primary);
  font-size: 0.96rem;
  letter-spacing: 0;
}

.assignment-review-drawer__section {
  display: grid;
  gap: 10px;
}

.assignment-review-drawer__workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.assignment-review-drawer__files-panel,
.assignment-review-drawer__form {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.assignment-review-drawer__section-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.assignment-review-drawer__section-title h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: 1rem;
}

.assignment-review-drawer__file-toolbar,
.assignment-review-drawer__file-view,
.assignment-review-drawer__file-grid-size {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.assignment-review-drawer__file-toolbar {
  justify-content: flex-end;
}

.assignment-review-drawer__file-list {
  width: 100%;
  overflow-x: auto;
  border: 1px solid var(--review-divider);
  border-radius: 10px;
  background: var(--review-panel-bg);
}

.assignment-review-drawer__file-table {
  min-width: 780px;
  border: 0;
  border-radius: 0;
}

.assignment-review-drawer__file-table td {
  vertical-align: middle;
}

.assignment-review-drawer__file-browser {
  overflow: hidden;
  border: 1px solid var(--review-divider);
  border-radius: 10px;
  background: var(--review-panel-bg);
}

.assignment-review-drawer__file-browser-head,
.assignment-review-drawer__file-row {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) minmax(150px, 220px) minmax(160px, auto);
  gap: 12px;
  align-items: center;
}

.assignment-review-drawer__file-browser-head {
  padding: 9px 12px;
  background: var(--review-panel-subtle);
  color: var(--text-muted);
  font-size: 0.94rem;
  font-weight: 800;
}

.assignment-review-drawer__file-row {
  min-height: 48px;
  padding: 9px 12px;
  border-top: 1px solid var(--review-divider);
}

.assignment-review-drawer__file-row[data-kind="dir"] {
  background: color-mix(in srgb, var(--accent-primary) 5%, var(--review-panel-bg));
}

.assignment-review-drawer__file-name {
  min-width: 0;
  display: flex;
  gap: 9px;
  align-items: center;
}

.assignment-review-drawer__file-type {
  flex: 0 0 auto;
  min-width: 42px;
  padding: 3px 7px;
  border: 1px solid var(--review-divider);
  border-radius: 999px;
  background: var(--review-panel-subtle);
  color: var(--text-muted);
  font-size: 0.9rem;
  font-weight: 800;
  text-align: center;
}

.assignment-review-drawer__file-name strong,
.assignment-review-drawer__file-meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignment-review-drawer__file-name strong {
  color: var(--text-primary);
}

.assignment-review-drawer__file-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.assignment-review-drawer__file-download {
  min-height: 32px;
  padding: 6px 10px;
}

.assignment-review-drawer__form {
  position: static;
  padding: 14px;
  border: 1px solid var(--review-divider);
  border-radius: 10px;
  background: var(--review-panel-bg);
}

.assignment-review-drawer__form .app-field {
  max-width: 720px;
}

.assignment-review-drawer__review-row {
  display: grid;
  grid-template-columns: 220px minmax(260px, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.assignment-review-drawer__review-row .app-field {
  min-width: 0;
  max-width: none;
}

.assignment-review-drawer__status-field {
  flex: 0 0 220px;
  max-width: 220px;
}

.assignment-review-drawer__review-row .copy-dialog__search {
  width: 100%;
}

.assignment-review-drawer__status-field .copy-dialog__search {
  width: 100%;
}

.assignment-review-drawer__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
}

.assignment-review-drawer__inline-preview {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--review-divider);
  border-radius: 10px;
  background: var(--review-panel-bg);
}

.assignment-review-drawer__inline-preview-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  min-width: 0;
}

.assignment-review-drawer__inline-preview-head strong {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: var(--text-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignment-review-drawer__inline-preview-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.assignment-review-drawer__inline-preview-body {
  display: grid;
  place-items: center;
  min-height: 210px;
  overflow: hidden;
  border: 1px solid var(--review-divider);
  border-radius: 8px;
  background: var(--review-panel-subtle);
}

.assignment-review-drawer__inline-preview-image-button {
  width: 100%;
  height: 100%;
  min-height: 210px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: zoom-in;
}

.assignment-review-drawer__inline-preview-image {
  width: 100%;
  max-height: 360px;
  object-fit: contain;
  display: block;
}

.assignment-review-drawer__inline-preview-frame {
  width: 100%;
  height: 360px;
  border: 0;
  background: var(--bg-surface);
}

.assignment-review-drawer__inline-preview-text {
  width: 100%;
  max-height: 320px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  color: var(--text-primary);
  font: 0.86rem/1.55 var(--font-mono, "Consolas", monospace);
  white-space: pre-wrap;
}

:deep(.el-button) {
  border-radius: 10px;
  font-weight: 700;
}

:deep(.el-tag) {
  --el-tag-border-radius: 999px;
  font-weight: 700;
}

.classes-page__composer--compact {
  margin-top: 0;
}

.assignment-edit-form {
  display: grid;
  gap: 10px;
}

.assignment-edit-dialog {
  width: min(900px, 100%);
}

.assignment-edit-dialog__content {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 0.72fr);
  gap: 10px;
  align-items: start;
}

.assignment-edit-form .students-import__input {
  width: 100%;
  min-height: 92px;
}

.assignment-edit-form__actions {
  justify-content: flex-start;
}

.assignment-edit-form__help {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.96rem;
  line-height: 1.45;
}

.assignment-attachment-manager {
  display: grid;
  gap: 8px;
  align-items: stretch;
  padding: 10px;
  border: 1px solid var(--border-soft);
  border-radius: 10px;
  background: var(--bg-subtle);
}

.assignment-attachment-manager__body {
  display: grid;
  gap: 10px;
}

.assignment-attachment-manager__actions {
  justify-content: flex-start;
}

@media (max-width: 1500px) {
  .assignment-review-drawer__workspace {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 1080px) {
  .classes-page__submission-toolbar {
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .assignment-detail-heading__description {
    white-space: normal;
  }

  .assignment-edit-dialog__content {
    grid-template-columns: 1fr;
  }

  .classes-page__submission-toolbar-group--end {
    margin-left: 0;
    justify-content: flex-start;
  }

  .classes-page__submission-toolbar-group,
  .classes-page__submission-toolbar-group--filters {
    flex: 1 1 100%;
    flex-wrap: wrap;
  }

  .classes-page__submission-toolbar-group:not(:first-child) {
    padding-left: 0;
    border-left: 0;
  }

  .classes-page__submission-search,
  .classes-page__submission-sort {
    flex: 1 1 180px;
    width: auto;
  }

  .assignment-submissions-table {
    min-width: 760px;
  }

  .assignment-review-drawer__workspace,
  .assignment-review-drawer__summary {
    grid-template-columns: 1fr;
  }

  .assignment-review-drawer__header,
  .assignment-review-drawer__section-title {
    align-items: flex-start;
    flex-direction: column;
  }

  .assignment-review-drawer__file-browser-head,
  .assignment-review-drawer__file-row {
    grid-template-columns: minmax(220px, 1fr);
  }

  .assignment-review-drawer__file-browser-head span:nth-child(2),
  .assignment-review-drawer__file-browser-head span:nth-child(3) {
    display: none;
  }

  .assignment-review-drawer__form {
    position: static;
  }

  .assignment-review-drawer__review-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .assignment-review-drawer__file-actions {
    justify-content: flex-start;
  }

  .assignment-review-drawer__actions,
  .assignment-review-drawer__inline-preview-actions {
    justify-content: flex-start;
  }

  .assignment-review-drawer__inline-preview-head {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
