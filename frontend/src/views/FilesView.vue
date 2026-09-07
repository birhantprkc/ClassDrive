<template>
  <section
    class="files-page"
    :class="{ 'files-page--dragging': isDraggingUpload }"
    data-testid="files-dropzone"
    @dragenter.prevent="handleUploadDragEnter"
    @dragover.prevent="handleUploadDragOver"
    @dragleave.prevent="handleUploadDragLeave"
    @drop.prevent="handleUploadDrop"
  >
    <section class="classes-page__board files-page__workspace" data-testid="files-workspace">
      <div class="files-toolbar">
      <div class="files-toolbar__top">
        <div class="files-context-bar" data-testid="files-context-bar">
          <nav class="files-breadcrumb" aria-label="当前路径" role="navigation">
            <button
              class="files-breadcrumb__button"
              type="button"
              data-testid="breadcrumb-root"
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
                :data-testid="`breadcrumb-${crumb.label}`"
                @click="navigateToPath(crumb.path)"
              >
                {{ crumb.label }}
              </button>
              <span v-else class="files-breadcrumb__current">
                {{ crumb.label }}
              </span>
            </template>
          </nav>
        </div>
      </div>

      <div class="files-toolbar__bottom">
        <div class="files-primary-actions files-primary-actions--left" data-testid="files-primary-actions">
          <button
            class="button button--ghost files-up-button"
            type="button"
            data-testid="files-up-button"
            :disabled="!canNavigateUp"
            aria-label="返回上一级文件夹"
            @click="navigateUp"
          >
            ↩
          </button>
          <div class="files-primary-actions__group">
            <button class="button button--ghost" type="button" data-testid="files-refresh" aria-label="刷新当前文件夹" @click="loadFiles">刷新</button>
            <button v-if="space === 'library'" class="button button--ghost" type="button" data-testid="files-share-manage" @click="shareManageDialogOpen = true">分享管理</button>
            <button class="button button--primary" type="button" data-testid="upload-material-open" @click="uploadDialogOpen = true">上传资料</button>
            <button class="button" type="button" @click="promptCreateFolder">新建文件夹</button>
            <button class="button" type="button" data-testid="create-file-button" @click="promptCreateFile">新建文件</button>
            <div class="files-controls__view" role="group" aria-label="视图切换">
              <button
                class="button"
                :class="{ 'button--primary': viewMode === 'list' }"
                type="button"
                data-testid="files-view-list"
                @click="setViewMode('list')"
              >
                列表
              </button>
              <button
                class="button"
                :class="{ 'button--primary': viewMode === 'grid' }"
                type="button"
                data-testid="files-view-grid"
                @click="setViewMode('grid')"
              >
                网格
              </button>
            </div>
            <div
              v-if="viewMode === 'grid'"
              class="files-controls__grid-size"
              data-testid="files-grid-size-controls"
              role="group"
              aria-label="网格大小"
            >
              <button
                class="button"
                :class="{ 'button--primary': gridSize === 'small' }"
                type="button"
                data-testid="files-grid-size-small"
                @click="setGridSize('small')"
              >
                小
              </button>
              <button
                class="button"
                :class="{ 'button--primary': gridSize === 'medium' }"
                type="button"
                data-testid="files-grid-size-medium"
                @click="setGridSize('medium')"
              >
                中
              </button>
              <button
                class="button"
                :class="{ 'button--primary': gridSize === 'large' }"
                type="button"
                data-testid="files-grid-size-large"
                @click="setGridSize('large')"
              >
                大
              </button>
            </div>
          </div>
          <input ref="fileInput" class="hidden-input" type="file" multiple data-testid="upload-input" @change="handleUpload" />
          <input ref="directoryInput" class="hidden-input" type="file" webkitdirectory multiple data-testid="upload-directory-input" @change="handleDirectoryUpload" />
        </div>
        <div class="files-toolbar__search-slot">
          <div class="files-secondary-controls files-secondary-controls--right" data-testid="files-secondary-controls">
            <div class="files-secondary-controls__inner">
              <div class="files-controls__options">
                <div class="files-controls__search" data-testid="files-filter-bar">
                  <input
                    v-model="searchQuery"
                    class="copy-dialog__search"
                    type="text"
                    placeholder="搜索当前资料"
                    data-testid="files-search-input"
                    @keyup.enter="applySearch"
                  />
                  <button class="button" type="button" data-testid="files-search-submit" @click="applySearch">搜索</button>
                  <button v-if="isSearching" class="button button--ghost" type="button" data-testid="files-search-clear" @click="clearSearch">清除筛选</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <p v-if="isSearching && activeSearchDescription" class="files-search-summary" data-testid="files-search-summary">
      当前过滤条件：{{ activeSearchDescription }}
    </p>
    <div v-if="errorText" class="files-error-banner" data-testid="files-error-banner">
      <p class="form-error">{{ errorText }}</p>
      <button class="button button--ghost" type="button" data-testid="files-error-retry" @click="loadFiles">重试</button>
    </div>
    <p v-if="isDraggingUpload" class="files-drop-hint" data-testid="files-drop-hint">拖拽文件或文件夹到这里上传</p>

    <div v-if="selectedItems.length" class="files-selection-toolbar" data-testid="files-selection-toolbar">
      <div class="files-selection-toolbar__summary" data-testid="files-selection-toolbar-summary">
        <span class="files-selection-toolbar__eyebrow">选择操作</span>
        <strong>{{ `已选 ${selectedItems.length} 项` }}</strong>
      </div>
      <div class="files-batch-toolbar" data-testid="batch-toolbar">
        <button class="button button--primary" type="button" data-testid="batch-action-download" @click="downloadSelected">下载所选</button>
        <button class="button button--secondary" type="button" data-testid="batch-action-copy" @click="openBatchCopyDialog">批量复制</button>
        <button class="button button--accent" type="button" data-testid="batch-action-move" @click="openBatchMoveDialog">批量移动</button>
        <button class="button button--danger" type="button" data-testid="batch-action-delete" @click="removeSelected">批量删除</button>
        <button class="button button--ghost" type="button" @click="clearSelection">取消选择</button>
      </div>
    </div>

    <PaginationControls
      :page="filePage"
      :page-size="filePageSize"
      :page-size-options="filePageSizeOptions"
      :total="totalFiles"
      :total-pages="totalFilePages"
      test-id-prefix="files"
      @update:page-size="updateFilePageSize"
      @go="goFilePage"
      @prev="goPrevFilePage"
      @next="goNextFilePage"
    />

    <table v-if="viewMode === 'list'" class="files-table" data-testid="files-table" @keydown="handleListKeyboardNavigation">
      <thead>
        <tr>
          <th>
            <input
              type="checkbox"
              data-testid="select-all-entries"
              :checked="allItemsSelected"
              :aria-label="allItemsSelected ? '取消全选' : '全选所有项目'"
              @change="toggleSelectAll"
            />
          </th>
          <th>
            <button
              class="table-sort-button"
              :class="{ 'is-active': sortMode === 'name-asc' || sortMode === 'name-desc' }"
              type="button"
              data-testid="files-sort-name"
              :aria-pressed="sortMode === 'name-asc' || sortMode === 'name-desc'"
              @click="toggleFileNameSort"
            >
              名称
              <span class="table-sort-button__mark">{{ fileSortMark("name") }}</span>
            </button>
          </th>
          <th>类型</th>
          <th>
            <button
              class="table-sort-button"
              :class="{ 'is-active': sortMode === 'updated-desc' || sortMode === 'updated-asc' }"
              type="button"
              data-testid="files-sort-updated"
              :aria-pressed="sortMode === 'updated-desc' || sortMode === 'updated-asc'"
              @click="toggleFileUpdatedSort"
            >
              更新时间
              <span class="table-sort-button__mark">{{ fileSortMark("updated") }}</span>
            </button>
          </th>
          <th>
            <button
              class="table-sort-button"
              :class="{ 'is-active': sortMode === 'size-desc' || sortMode === 'size-asc' }"
              type="button"
              data-testid="files-sort-size"
              :aria-pressed="sortMode === 'size-desc' || sortMode === 'size-asc'"
              @click="toggleFileSizeSort"
            >
              大小
              <span class="table-sort-button__mark">{{ fileSortMark("size") }}</span>
            </button>
          </th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(item, index) in displayedItems"
          :key="item.id"
          :data-item-id="item.id"
          class="files-table__row"
          role="button"
          tabindex="0"
          :aria-label="`${item.kind === 'dir' ? '文件夹' : '文件'}: ${item.name}`"
          :aria-describedby="`item-meta-${item.id}`"
          @click="preview(item)"
          @focus="handleItemFocus(index)"
          @keydown.enter.self.prevent="preview(item)"
          @keydown.space.self.prevent="preview(item)"
        >
          <td>
            <input
              :data-testid="`select-entry-${item.id}`"
              type="checkbox"
              :checked="isEntrySelected(item.id)"
              :aria-label="`选择 ${item.name}`"
              @click.stop
              @change="toggleEntrySelection(item.id, $event)"
            />
          </td>
          <td>
            <button
              v-if="item.kind === 'dir'"
              class="files-entry-link"
              type="button"
              :data-testid="`entry-open-${item.id}`"
              :aria-label="`打开文件夹 ${item.name}`"
              @click.stop="openDirectory(item)"
            >
              <span :data-testid="`entry-name-${item.id}`">{{ item.name }}</span>
            </button>
            <span v-else :data-testid="`entry-name-${item.id}`">
              {{ item.name }}
            </span>
            <p v-if="isSearching" class="files-entry-meta">{{ item.path }}</p>
            <span :id="`item-meta-${item.id}`" class="sr-only">
              {{ item.kind === 'dir' ? '文件夹' : '文件' }}，
              更新于 {{ formatFileUpdatedAt(item.updatedAt) }}，
              大小 {{ formatFileSize(item.size) }}
            </span>
          </td>
          <td>{{ item.kind === "dir" ? "文件夹" : "文件" }}</td>
          <td :data-testid="`entry-updated-${item.id}`">{{ formatFileUpdatedAt(item.updatedAt) }}</td>
          <td>{{ formatFileSize(item.size) }}</td>
          <td class="files-table__actions">
            <div class="files-entry-actions__primary" :data-testid="`row-primary-actions-${item.id}`">
              <button class="text-button" type="button" :data-testid="`preview-entry-${item.id}`" @click.stop="preview(item)">{{ item.kind === "dir" ? "进入" : "预览" }}</button>
              <button v-if="canEditFile(item)" class="text-button" type="button" :data-testid="`edit-entry-${item.id}`" @click.stop="openEditor(item)">编辑</button>
              <a
                v-if="item.kind === 'dir' && item.archiveUrl"
                class="text-button"
                :data-testid="`download-entry-${item.id}`"
                :href="item.archiveUrl"
                @click.stop
              >
                下载压缩包
              </a>
              <a
                v-else-if="item.downloadUrl"
                class="text-button"
                :data-testid="`download-entry-${item.id}`"
                :href="item.downloadUrl"
                @click.stop
              >
                下载
              </a>
            </div>
            <div class="files-entry-actions__overflow" :data-testid="`row-actions-overflow-${item.id}`">
              <button
                class="text-button files-entry-actions__more"
                type="button"
                :data-testid="`row-more-actions-${item.id}`"
                :aria-expanded="isEntryActionMenuOpen(item.id)"
                @click.stop="toggleEntryActionMenu(item.id)"
              >
                更多
              </button>
              <div
                v-show="isEntryActionMenuOpen(item.id)"
                class="files-entry-actions__secondary"
                :data-testid="`row-secondary-actions-${item.id}`"
              >
                <button class="text-button" type="button" :data-testid="`copy-${item.id}`" @click.stop="openCopyDialog(item)">复制</button>
                <button class="text-button" type="button" :data-testid="`move-${item.id}`" @click.stop="openMoveDialog(item)">移动</button>
                <button v-if="space === 'library'" class="text-button" type="button" :data-testid="`share-${item.id}`" @click.stop="openShareDialog(item)">分享</button>
                <button class="text-button" type="button" @click.stop="rename(item)">重命名</button>
                <button class="text-button text-button--danger" type="button" @click.stop="remove(item)">删除</button>
              </div>
            </div>
          </td>
        </tr>
        <tr v-if="!displayedItems.length">
          <td colspan="6" class="files-table__empty">{{ uiCopy.emptyDirectory }}</td>
        </tr>
      </tbody>
    </table>

    <div
      v-else
      class="files-grid"
      :class="filesGridSizeClass"
      data-testid="files-grid"
      @keydown="handleListKeyboardNavigation"
    >
      <article
        v-for="(item, index) in displayedItems"
        :key="item.id"
        :data-item-id="item.id"
        class="files-grid__card"
        role="button"
        tabindex="0"
        :aria-label="`打开 ${item.name}`"
        @click="preview(item)"
        @focus="handleItemFocus(index)"
        @keydown.enter.self.prevent="preview(item)"
        @keydown.space.self.prevent="preview(item)"
      >
        <label class="files-grid__select" @click.stop>
          <input
            :data-testid="`select-entry-${item.id}`"
            type="checkbox"
            :checked="isEntrySelected(item.id)"
            @click.stop
            @change="toggleEntrySelection(item.id, $event)"
          />
          <span>{{ item.kind === "dir" ? "文件夹" : "文件" }}</span>
        </label>

        <button
          v-if="resolveGridThumbnailUrl(item)"
          class="files-grid__thumbnail-button"
          type="button"
          :data-testid="`grid-thumbnail-open-${item.id}`"
          :aria-label="`预览 ${item.name}`"
          @click.stop="preview(item)"
        >
          <img
            class="files-grid__thumbnail"
            :data-testid="`grid-thumbnail-${item.id}`"
            :src="resolveGridThumbnailUrl(item) ?? ''"
            :alt="item.name"
            loading="lazy"
            decoding="async"
          />
        </button>

        <button
          v-if="item.kind === 'dir'"
          class="files-entry-link files-grid__title"
          type="button"
          :data-testid="`entry-open-${item.id}`"
          @click.stop="openDirectory(item)"
        >
          <span :data-testid="`entry-name-${item.id}`">{{ item.name }}</span>
        </button>
        <div v-else class="files-grid__title" :data-testid="`entry-name-${item.id}`">{{ item.name }}</div>

        <p v-if="isSearching" class="files-entry-meta">{{ item.path }}</p>
        <p class="muted">{{ formatFileSize(item.size) }}</p>

        <div class="files-grid__actions">
          <div class="files-entry-actions__primary" :data-testid="`card-primary-actions-${item.id}`">
            <button class="text-button" type="button" :data-testid="`preview-entry-${item.id}`" @click.stop="preview(item)">{{ item.kind === "dir" ? "进入" : "预览" }}</button>
            <button v-if="canEditFile(item)" class="text-button" type="button" :data-testid="`edit-entry-${item.id}`" @click.stop="openEditor(item)">编辑</button>
            <a
              v-if="item.kind === 'dir' && item.archiveUrl"
              class="text-button"
              :data-testid="`download-entry-${item.id}`"
              :href="item.archiveUrl"
              @click.stop
            >
              下载压缩包
            </a>
            <a
              v-else-if="item.downloadUrl"
              class="text-button"
              :data-testid="`download-entry-${item.id}`"
              :href="item.downloadUrl"
              @click.stop
            >
              下载
            </a>
          </div>
          <div class="files-entry-actions__overflow" :data-testid="`card-actions-overflow-${item.id}`">
            <button
              class="text-button files-entry-actions__more"
              type="button"
              :data-testid="`card-more-actions-${item.id}`"
              :aria-expanded="isEntryActionMenuOpen(item.id)"
              @click.stop="toggleEntryActionMenu(item.id)"
            >
              更多
            </button>
            <div
              v-show="isEntryActionMenuOpen(item.id)"
              class="files-entry-actions__secondary"
              :data-testid="`card-secondary-actions-${item.id}`"
            >
              <button class="text-button" type="button" :data-testid="`copy-${item.id}`" @click.stop="openCopyDialog(item)">复制</button>
              <button class="text-button" type="button" :data-testid="`move-${item.id}`" @click.stop="openMoveDialog(item)">移动</button>
              <button v-if="space === 'library'" class="text-button" type="button" :data-testid="`share-${item.id}`" @click.stop="openShareDialog(item)">分享</button>
              <button class="text-button" type="button" @click.stop="rename(item)">重命名</button>
              <button class="text-button text-button--danger" type="button" @click.stop="remove(item)">删除</button>
            </div>
          </div>
        </div>
      </article>

      <p v-if="!displayedItems.length" class="files-table__empty">{{ uiCopy.emptyDirectory }}</p>
    </div>

    </section>

    <div v-if="copyDialogEntry" class="copy-dialog-backdrop" @click.self="closeCopyDialog">
      <section class="copy-dialog" data-testid="copy-dialog">
        <div class="copy-dialog__header">
          <div>
            <div class="copy-dialog__eyebrow">{{ copyDialogEyebrow }}</div>
            <h3 class="copy-dialog__title">{{ copyDialogTitle }}</h3>
          </div>
          <button class="button button--ghost" type="button" @click="closeCopyDialog">关闭</button>
        </div>

        <div class="copy-dialog__summary copy-dialog__summary-card" data-testid="target-dialog-summary">
          <span class="copy-dialog__summary-label">当前操作</span>
          <strong>{{ targetDialogSummary }}</strong>
          <span>{{ targetDialogHint }}</span>
        </div>

        <div class="copy-dialog__section" data-testid="target-dialog-space-section">
          <div class="copy-dialog__section-title">目标空间</div>
          <div class="copy-dialog__space-grid">
            <label class="field">
              <span>资料空间</span>
              <select v-model="copyTargetSpace" class="copy-dialog__search" data-testid="copy-space-select">
                <option value="library">老师资料</option>
                <option value="public">公共资料</option>
                <option value="class">班级资料</option>
              </select>
            </label>

            <label v-if="copyTargetSpace === 'class'" class="field">
              <span>班级</span>
              <select v-model="copyTargetClassId" class="copy-dialog__search" data-testid="copy-class-select">
                <option
                  v-for="item in classes"
                  :key="item.id"
                  :value="item.id"
                >
                  {{ item.name }}
                </option>
              </select>
            </label>
          </div>
        </div>

        <div class="copy-dialog__destination copy-dialog__section" data-testid="target-dialog-destination-section">
          <div class="copy-dialog__section-title">目标目录</div>
          <div v-if="recentCopyTargets.length" class="copy-dialog__recent">
            <div class="copy-dialog__recent-label">最近目标目录</div>
            <ResourceList
              :items="recentTargetResources"
              item-test-id-prefix="copy-recent-row"
              link-test-id-prefix="copy-recent-target"
            >
              <template #item-actions="{ item }">
                <button
                  class="copy-dialog__recent-pin"
                  type="button"
                  :data-testid="`copy-recent-pin-${item.index ?? 0}`"
                  @click="togglePinnedRecentTarget(item.index ?? 0)"
                >
                  {{ item.pinned === true ? "取消固定" : "固定" }}
                </button>
                <button
                  v-if="item.pinned === true"
                  class="copy-dialog__recent-pin"
                  type="button"
                  :disabled="(item.index ?? 0) === 0"
                  :data-testid="`copy-recent-move-up-${item.index ?? 0}`"
                  @click="movePinnedRecentTarget(item.index ?? 0, -1)"
                >
                  上移
                </button>
                <button
                  v-if="item.pinned === true"
                  class="copy-dialog__recent-pin"
                  type="button"
                  :disabled="(item.index ?? 0) >= pinnedRecentTargetCount - 1"
                  :data-testid="`copy-recent-move-down-${item.index ?? 0}`"
                  @click="movePinnedRecentTarget(item.index ?? 0, 1)"
                >
                  下移
                </button>
              </template>
            </ResourceList>
            <button class="text-button" type="button" data-testid="copy-recent-clear" @click="clearRecentTargets">清空最近</button>
          </div>
          <div class="copy-dialog__current-target">
            <span>当前目录</span>
            <strong>{{ copyTargetPath }}</strong>
          </div>
          <nav class="files-breadcrumb" aria-label="复制目标路径">
            <button
              class="files-breadcrumb__button"
              type="button"
              data-testid="copy-destination-root"
              @click="navigateCopyTargetPath('/')"
            >
              根目录
            </button>
            <template v-for="crumb in copyBreadcrumbs" :key="crumb.path">
              <span class="files-breadcrumb__sep">/</span>
              <button
                v-if="!crumb.isCurrent"
                class="files-breadcrumb__button"
                type="button"
                :data-testid="`copy-destination-breadcrumb-${crumb.label}`"
                @click="navigateCopyTargetPath(crumb.path)"
              >
                {{ crumb.label }}
              </button>
              <span v-else class="files-breadcrumb__current">
                {{ crumb.label }}
              </span>
            </template>
          </nav>

          <div class="copy-dialog__toolbar">
            <input
              v-model="copySearch"
              class="copy-dialog__search"
              type="text"
              placeholder="搜索当前目录下的文件夹"
              data-testid="copy-search-input"
            />
            <div class="copy-dialog__create">
              <input
                v-model="copyCreateFolderName"
                class="copy-dialog__search"
                type="text"
                placeholder="新建目标目录"
                data-testid="copy-create-folder-input"
              />
              <button
                class="button"
                type="button"
                data-testid="copy-create-folder-submit"
                @click="createCopyTargetFolder"
              >
                新建
              </button>
            </div>
          </div>

          <p v-if="copyErrorText" class="form-error">{{ copyErrorText }}</p>
          <p v-else-if="copyLoading" class="muted">正在加载目标目录…</p>

          <div v-else class="copy-dialog__folder-list">
            <button
              v-for="folder in filteredCopyTargetFolders"
              :key="folder.id"
              class="copy-dialog__folder-button"
              type="button"
              :data-testid="`copy-destination-entry-${folder.id}`"
              @click="navigateCopyTargetPath(folder.path)"
            >
              {{ folder.name }}
            </button>
            <p v-if="filteredCopyTargetFolders.length === 0" class="muted">当前目标目录下没有匹配的子文件夹。</p>
          </div>
        </div>

        <div class="copy-dialog__actions">
          <button class="button" type="button" @click="closeCopyDialog">取消</button>
          <button class="button button--primary" type="button" data-testid="copy-submit" @click="submitCopy">{{ copyDialogSubmitLabel }}</button>
        </div>
      </section>
    </div>

    <div v-if="uploadDialogOpen" class="copy-dialog-backdrop" data-testid="upload-dialog-backdrop" @click.self="closeUploadDialog">
      <section class="copy-dialog files-upload-dialog" data-testid="upload-dialog">
        <div class="copy-dialog__header">
          <div>
            <div class="copy-dialog__eyebrow">上传资料</div>
            <h3 class="copy-dialog__title">选择上传方式</h3>
          </div>
          <button class="button button--ghost" type="button" data-testid="upload-dialog-close" @click="closeUploadDialog">关闭</button>
        </div>

        <p class="muted">上传开始后，可在右上角上传任务面板里收起、关闭或中止当前上传。</p>

        <label class="field">
          <span>同名处理</span>
          <select v-model="uploadConflictMode" data-testid="upload-conflict-mode">
            <option value="reject">提示失败</option>
            <option value="skip">跳过同名</option>
            <option value="replace">覆盖同名</option>
            <option value="rename">自动重命名</option>
          </select>
        </label>

        <div class="files-upload-dialog__actions">
          <button class="button button--primary files-upload-dialog__choice" type="button" data-testid="upload-file-button" @click="openUpload">
            上传文件
          </button>
          <button class="button files-upload-dialog__choice" type="button" data-testid="upload-directory-button" @click="openUploadDirectory">
            上传文件夹
          </button>
        </div>
      </section>
    </div>

    <FileEditorDialog
      :item="editorItem"
      :content="editorContent"
      :loading="editorLoading"
      :saving="editorSaving"
      :error-text="editorErrorText"
      :disabled-save="editorSaveDisabled"
      :dirty="editorDirty"
      @close="closeEditor"
      @save="saveEditor"
      @update:content="editorContent = $event"
    />

    <FilePreviewDialog
      :item="previewItem"
      :kind="previewKind"
      :loading="previewLoading"
      :error-text="previewErrorText"
      :text-content="previewTextContent"
      :markdown-content="previewMarkdownContent"
      :can-edit="previewItem ? canEditFile(previewItem) : false"
      :has-previous="previewHasPrevious"
      :has-next="previewHasNext"
      @close="closePreview"
      @edit="openEditorFromPreview"
      @previous="previewPrevious"
      @next="previewNext"
    />
    <ConfirmDialog
      :open="confirmDialogState !== null"
      :title="confirmDialogState?.title ?? ''"
      :message="confirmDialogState?.message ?? ''"
      :test-id-prefix="confirmDialogState?.prefix ?? 'confirm'"
      :confirm-label="confirmDialogState?.confirmLabel ?? '确认'"
      :confirm-tone="confirmDialogState?.confirmTone ?? 'primary'"
      @cancel="cancelConfirmDialog"
      @confirm="confirmDialog"
    />
    <TextInputDialog
      :open="textInputDialogState !== null"
      :test-id-prefix="textInputDialogState?.prefix ?? 'text-input'"
      :title="textInputDialogState?.title ?? ''"
      :label="textInputDialogState?.label ?? ''"
      :initial-value="textInputDialogState?.initialValue ?? ''"
      :placeholder="textInputDialogState?.placeholder ?? ''"
      :confirm-label="textInputDialogState?.confirmLabel ?? '确认'"
      @cancel="textInputDialogState = null"
      @confirm="confirmTextInputDialog"
    />

    <ShareCreateDialog
      :open="shareCreateDialogOpen"
      :entry="shareDialogEntry"
      @close="shareCreateDialogOpen = false"
    />

    <ShareManageDialog
      :open="shareManageDialogOpen"
      @close="shareManageDialogOpen = false"
    />
  </section>
</template>

<script setup lang="ts">
import { storeToRefs } from "pinia";
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { matchedRouteKey, onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter, type LocationQueryRaw } from "vue-router";
import {
  api,
  ApiError,
  describeFileSearchQuery,
  type ClassItem,
  type FileItem,
  type FileContentResult,
  type FileSpace,
  type UploadConflictMode,
  type UploadFileItem,
  type UploadLifecycleController,
} from "@/api/client";
import FileEditorDialog from "@/components/FileEditorDialog.vue";
import FilePreviewDialog from "@/components/FilePreviewDialog.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";
import ShareCreateDialog from "@/components/ShareCreateDialog.vue";
import ShareManageDialog from "@/components/ShareManageDialog.vue";
import PaginationControls from "@/components/PaginationControls.vue";
import ResourceList from "@/components/ResourceList.vue";
import TextInputDialog from "@/components/TextInputDialog.vue";
import { useRecentCopyTargetsStore, type RecentCopyTarget } from "@/stores/recent-copy-targets";
import { useToastStore } from "@/stores/toast";
import { useUploadStore } from "@/stores/upload";
import { canEditTextFile, getFilePreviewKind } from "@/utils/file-preview";
import { renderMarkdownPreview } from "@/utils/markdown-preview";
import { uiCopy, uploadSuccessMessage } from "@/utils/ui-copy";
import { collectDroppedUploadItems } from "@/utils/upload-drop";
import { filterRedundantDirectoryArchives } from "@/utils/upload-items";

interface BreadcrumbItem {
  label: string;
  path: string;
  isCurrent: boolean;
}

interface ConfirmDialogState {
  prefix: string;
  title: string;
  message: string;
  confirmLabel: string;
  confirmTone?: "primary" | "danger";
  onConfirm: () => void | Promise<void>;
  onCancel?: () => void;
}

interface TextInputDialogState {
  prefix: string;
  title: string;
  label: string;
  initialValue: string;
  placeholder: string;
  confirmLabel: string;
  onConfirm: (value: string) => void | Promise<void>;
}

const route = useRoute();
const router = useRouter();
const activeMatchedRoute = inject(matchedRouteKey, null);
const toastStore = useToastStore();
const uploadStore = useUploadStore();
const recentCopyTargetsStore = useRecentCopyTargetsStore();
const { items: recentCopyTargets, pinnedCount: pinnedRecentTargetCount } = storeToRefs(recentCopyTargetsStore);

type FilesSortMode = "name-asc" | "name-desc" | "updated-desc" | "updated-asc" | "size-desc" | "size-asc";
type FilesGridSize = "small" | "medium" | "large";

const defaultFilesSortMode: FilesSortMode = "name-asc";
const defaultFilesViewMode = "grid";
const defaultFilesGridSize: FilesGridSize = "medium";
const defaultFilesPageSize = 30;
const defaultClassFilesPageSize = 50;
const filePageSizeOptions = [1, 30, 50, 60, 100];

const items = ref<FileItem[]>([]);
const classes = ref<ClassItem[]>([]);
const selectedClassId = ref<number>(Number(route.params.classId ?? 1));
const currentPath = ref("/");
const errorText = ref("");
const fileInput = ref<HTMLInputElement | null>(null);
const directoryInput = ref<HTMLInputElement | null>(null);
const uploadConflictMode = ref<UploadConflictMode>("reject");
const uploadDialogOpen = ref(false);
const isDraggingUpload = ref(false);
const uploadDragDepth = ref(0);
const searchQuery = ref("");
const sortMode = ref<FilesSortMode>(defaultFilesSortMode);
const viewMode = ref<"list" | "grid">(defaultFilesViewMode);
const gridSize = ref<FilesGridSize>(defaultFilesGridSize);
const filePage = ref(1);
const filePageSize = ref(defaultFilesPageSize);
const totalFiles = ref(0);
const totalFilePages = ref(1);
const openEntryActionMenuId = ref<number | null>(null);
const shareCreateDialogOpen = ref(false);
const shareManageDialogOpen = ref(false);
const shareDialogEntry = ref<FileItem | null>(null);
const selectedEntryIds = ref<number[]>([]);
const targetDialogEntries = ref<FileItem[]>([]);
const targetDialogMode = ref<"copy" | "move">("copy");
const copyTargetSpace = ref<FileSpace>("public");
const copyTargetClassId = ref<number | null>(null);
const copyTargetPath = ref("/");
const copyTargetFolders = ref<FileItem[]>([]);
const copyLoading = ref(false);
const copyErrorText = ref("");
const copySearch = ref("");
const copyCreateFolderName = ref("");
const copyDialogEntry = computed(() => targetDialogEntries.value[0] ?? null);
const previewItem = ref<FileItem | null>(null);
const previewTextContent = ref("");
const previewMarkdownContent = ref("");
const previewLoading = ref(false);
const previewErrorText = ref("");
const editorItem = ref<FileItem | null>(null);
const editorContent = ref("");
const editorInitialContent = ref("");
const editorLoading = ref(false);
const editorSaving = ref(false);
const editorErrorText = ref("");
const confirmDialogState = ref<ConfirmDialogState | null>(null);
const textInputDialogState = ref<TextInputDialogState | null>(null);
let editorRequestToken = 0;
let routeNavigationApproved = false;
const previewTextCache = new Map<number, string>();
const selectedItems = computed(() => {
  const selectedIdSet = new Set(selectedEntryIds.value);
  return items.value.filter((item) => selectedIdSet.has(item.id));
});
const activeSearchQuery = computed(() => (typeof route.query.q === "string" ? route.query.q.trim() : ""));
const isSearching = computed(() => activeSearchQuery.value.length > 0);
const activeSearchDescription = computed(() => describeFileSearchQuery(activeSearchQuery.value));
const allItemsSelected = computed(() => items.value.length > 0 && items.value.every((item) => selectedEntryIds.value.includes(item.id)));
const copyDialogEyebrow = computed(() => (targetDialogMode.value === "move" ? "移动目标" : "复制目标"));
const canNavigateUp = computed(() => currentPath.value !== "/");
const copyDialogTitle = computed(() => {
  if (targetDialogEntries.value.length > 1) {
    return `已选择 ${targetDialogEntries.value.length} 项`;
  }
  return copyDialogEntry.value?.name ?? "";
});
const copyDialogSubmitLabel = computed(() => (targetDialogMode.value === "move" ? "确认移动" : "确认复制"));
const targetDialogSummary = computed(() => {
  const verb = targetDialogMode.value === "move" ? "移动" : "复制";
  if (targetDialogEntries.value.length > 1) {
    return `${verb} ${targetDialogEntries.value.length} 项`;
  }
  return copyDialogEntry.value ? `${verb} ${copyDialogEntry.value.name}` : "";
});
const targetDialogHint = computed(() => (
  targetDialogMode.value === "move"
    ? "移动后资源会从当前位置转移到目标目录。"
    : "复制后会在目标目录保留一份副本，适合快速分发资料。"
));
const recentTargetResources = computed(() => (
  recentCopyTargets.value.map((target, index) => ({
    id: index,
    index,
    pinned: target.pinned ?? false,
    name: target.label,
    onClick: () => {
      void applyRecentTarget(target);
    },
  }))
));
const previewKind = computed(() => (previewItem.value ? getFilePreviewKind(previewItem.value) : null));
const editorDirty = computed(() => editorContent.value !== editorInitialContent.value);
const editorSaveDisabled = computed(() => {
  if (!editorItem.value) {
    return true;
  }
  if (editorLoading.value || editorSaving.value) {
    return true;
  }
  if (editorErrorText.value) {
    return true;
  }
  return !editorDirty.value;
});

const space = computed(() => {
  if (route.path.startsWith("/files/public")) {
    return "public";
  }
  if (route.path.startsWith("/files/classes")) {
    return "class";
  }
  return "library";
});

const isClassSpace = computed(() => space.value === "class");
const breadcrumbs = computed<BreadcrumbItem[]>(() => {
  return toBreadcrumbs(currentPath.value);
});
const displayedItems = computed(() => sortFileItems(items.value, sortMode.value));
const previewableItems = computed(() => displayedItems.value.filter((item) => item.kind === "file" && getFilePreviewKind(item) !== "external"));
const previewItemIndex = computed(() => {
  const current = previewItem.value;
  return current ? previewableItems.value.findIndex((item) => item.id === current.id) : -1;
});
const previewHasPrevious = computed(() => previewItemIndex.value > 0);
const previewHasNext = computed(() => previewItemIndex.value >= 0 && previewItemIndex.value < previewableItems.value.length - 1);
const filesGridSizeClass = computed(() => `files-grid--${gridSize.value}`);
const filteredCopyTargetFolders = computed(() => {
  const keyword = copySearch.value.trim();
  if (!keyword) {
    return copyTargetFolders.value;
  }
  return copyTargetFolders.value.filter((item) => item.name.includes(keyword));
});

function normalizePath(value: string): string {
  const trimmed = value.trim();
  if (!trimmed || trimmed === "/") {
    return "/";
  }
  return `/${trimmed.replace(/^\/+|\/+$/g, "")}`;
}

function parseFilesSortMode(raw: unknown): FilesSortMode {
  return raw === "updated-desc"
    || raw === "updated-asc"
    || raw === "size-desc"
    || raw === "size-asc"
    || raw === "name-desc"
    || raw === "name-asc"
    ? raw
    : defaultFilesSortMode;
}

function parseFilesViewMode(raw: unknown): "list" | "grid" {
  return raw === "grid" || raw === "list" ? raw : defaultFilesViewMode;
}

function parseFilesGridSize(raw: unknown): FilesGridSize {
  return raw === "small" || raw === "large" || raw === "medium" ? raw : defaultFilesGridSize;
}

function parsePositiveInt(raw: unknown, fallback: number): number {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (typeof value !== "string" || value.trim() === "") {
    return fallback;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function defaultFilesPageSizeForCurrentSpace() {
  return space.value === "class" ? defaultClassFilesPageSize : defaultFilesPageSize;
}

function normalizeFilesPageSize(value: number, fallback = defaultFilesPageSizeForCurrentSpace()): number {
  return filePageSizeOptions.includes(value) ? value : fallback;
}

function buildFilesRouteQuery(overrides: Partial<{
  sort: FilesSortMode;
  view: "list" | "grid";
  gridSize: FilesGridSize;
  page: number;
  pageSize: number;
}> = {}) {
  const nextQuery: LocationQueryRaw = { ...route.query };
  const hasSortOverride = Object.prototype.hasOwnProperty.call(overrides, "sort");
  const nextSortMode = overrides.sort ?? sortMode.value;
  const nextViewMode = overrides.view ?? viewMode.value;
  const nextGridSize = overrides.gridSize ?? gridSize.value;
  const nextPage = overrides.page ?? filePage.value;
  const defaultPageSize = defaultFilesPageSizeForCurrentSpace();
  const nextPageSize = normalizeFilesPageSize(overrides.pageSize ?? filePageSize.value, defaultPageSize);
  if (nextSortMode === defaultFilesSortMode && !hasSortOverride) {
    delete nextQuery.sort;
  } else {
    nextQuery.sort = nextSortMode;
  }
  if (nextViewMode === defaultFilesViewMode) {
    delete nextQuery.view;
  } else {
    nextQuery.view = nextViewMode;
  }
  if (nextGridSize === defaultFilesGridSize) {
    delete nextQuery.gridSize;
  } else {
    nextQuery.gridSize = nextGridSize;
  }
  if (nextPage <= 1) {
    delete nextQuery.page;
  } else {
    nextQuery.page = String(nextPage);
  }
  if (nextPageSize === defaultPageSize) {
    delete nextQuery.pageSize;
  } else {
    nextQuery.pageSize = String(nextPageSize);
  }
  return nextQuery;
}

async function replaceFilesRoute(overrides: Partial<{
  sort: FilesSortMode;
  view: "list" | "grid";
  gridSize: FilesGridSize;
  page: number;
  pageSize: number;
}> = {}) {
  await router.replace({
    path: route.path,
    query: buildFilesRouteQuery(overrides),
  });
}

function parentPathOf(path: string) {
  const normalized = normalizePath(path);
  if (normalized === "/") {
    return "/";
  }
  const segments = normalized.split("/").filter(Boolean);
  if (segments.length <= 1) {
    return "/";
  }
  return `/${segments.slice(0, -1).join("/")}`;
}

function parseUpdatedAt(value: string) {
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
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

function formatFileSize(value: number) {
  if (!Number.isFinite(value) || value < 0) {
    return "—";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  if (value < 1024 * 1024 * 1024) {
    return `${(value / 1024 / 1024).toFixed(value >= 10 * 1024 * 1024 ? 0 : 1)} MB`;
  }
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

function fileSortMark(column: "name" | "updated" | "size") {
  if (column === "name") {
    if (sortMode.value === "name-desc") {
      return "↓";
    }
    return sortMode.value === "name-asc" ? "↑" : "";
  }
  if (column === "updated") {
    if (sortMode.value === "updated-asc") {
      return "↑";
    }
    return sortMode.value === "updated-desc" ? "↓" : "";
  }
  if (sortMode.value === "size-asc") {
    return "↑";
  }
  return sortMode.value === "size-desc" ? "↓" : "";
}

function compareByName(left: FileItem, right: FileItem) {
  return (left.name ?? "").localeCompare(right.name ?? "", "zh-CN");
}

function sortFileItems(source: FileItem[], mode: FilesSortMode) {
  return [...source].sort((left, right) => {
    if (mode === "updated-desc" || mode === "updated-asc") {
      const updatedDelta = mode === "updated-desc"
        ? parseUpdatedAt(right.updatedAt) - parseUpdatedAt(left.updatedAt)
        : parseUpdatedAt(left.updatedAt) - parseUpdatedAt(right.updatedAt);
      if (updatedDelta !== 0) {
        return updatedDelta;
      }
    }
    if (mode === "size-desc" || mode === "size-asc") {
      const sizeDelta = mode === "size-desc" ? right.size - left.size : left.size - right.size;
      if (sizeDelta !== 0) {
        return sizeDelta;
      }
    }
    const nameOrder = compareByName(left, right);
    return mode === "name-desc" ? -nameOrder : nameOrder;
  });
}

function toBreadcrumbs(value: string): BreadcrumbItem[] {
  const normalized = normalizePath(value);
  if (normalized === "/") {
    return [];
  }
  const segments = normalized.split("/").filter(Boolean);
  let current = "";

  return segments.map((segment, index) => {
    current = `${current}/${segment}`;
    return {
      label: segment,
      path: current,
      isCurrent: index === segments.length - 1,
    };
  });
}

function syncSelectedEntries() {
  const validIds = new Set(items.value.map((item) => item.id));
  selectedEntryIds.value = selectedEntryIds.value.filter((id) => validIds.has(id));
}

function isEntrySelected(entryId: number) {
  return selectedEntryIds.value.includes(entryId);
}

function toggleEntrySelection(entryId: number, event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  if (target.checked) {
    if (!selectedEntryIds.value.includes(entryId)) {
      selectedEntryIds.value = [...selectedEntryIds.value, entryId];
    }
    return;
  }
  selectedEntryIds.value = selectedEntryIds.value.filter((id) => id !== entryId);
}

function isEntryActionMenuOpen(entryId: number) {
  return openEntryActionMenuId.value === entryId;
}

function toggleEntryActionMenu(entryId: number) {
  openEntryActionMenuId.value = openEntryActionMenuId.value === entryId ? null : entryId;
}

function closeEntryActionMenuOnOutsideClick(event: MouseEvent) {
  if (openEntryActionMenuId.value === null) {
    return;
  }
  const target = event.target;
  if (target instanceof Element && target.closest(".files-entry-actions__overflow")) {
    return;
  }
  openEntryActionMenuId.value = null;
}

function toggleSelectAll(event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  selectedEntryIds.value = target.checked ? items.value.map((item) => item.id) : [];
}

function clearSelection() {
  selectedEntryIds.value = [];
  openEntryActionMenuId.value = null;
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

function canEditFile(item: FileItem) {
  return canEditTextFile(item);
}

function syncLoadedItem(nextItem: FileItem) {
  const existingIndex = items.value.findIndex((item) => item.id === nextItem.id);
  if (existingIndex >= 0) {
    items.value = items.value.map((item) => (item.id === nextItem.id ? nextItem : item));
  } else {
    items.value = [...items.value, nextItem];
  }
  if (previewItem.value?.id === nextItem.id) {
    previewItem.value = nextItem;
  }
  if (editorItem.value?.id === nextItem.id) {
    editorItem.value = nextItem;
  }
}

function downloadSelected(): void {
  const entries = selectedItems.value;
  if (entries.length === 0) {
    return;
  }
  if (entries.length === 1) {
    startDownloadUrl(downloadUrlForEntry(entries[0]));
    return;
  }
  startDownloadUrl(api.filesBatchArchiveUrl(entries.map((item) => item.id)));
}

function downloadUrlForEntry(item: FileItem): string | null {
  const url = item.kind === "dir" ? item.archiveUrl : item.downloadUrl;
  const trimmed = url?.trim() ?? "";
  return trimmed || null;
}

function startDownloadUrl(downloadUrl: string | null): void {
  if (!downloadUrl) {
    return;
  }
  const link = document.createElement("a");
  link.href = downloadUrl;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

async function loadFiles() {
  errorText.value = "";
  try {
    const routeSearchQuery = typeof route.query.q === "string" ? route.query.q.trim() : "";
    searchQuery.value = routeSearchQuery;
    sortMode.value = parseFilesSortMode(route.query.sort);
    viewMode.value = parseFilesViewMode(route.query.view);
    gridSize.value = parseFilesGridSize(route.query.gridSize);
    filePage.value = parsePositiveInt(route.query.page, 1);
    const defaultPageSize = defaultFilesPageSizeForCurrentSpace();
    filePageSize.value = normalizeFilesPageSize(parsePositiveInt(route.query.pageSize, defaultPageSize), defaultPageSize);

    if (isClassSpace.value) {
      await ensureClassesLoaded();
      const routeClassId = Number(route.params.classId);
      const firstClassId = classes.value[0]?.id ?? 0;
      const hasRouteClass = classes.value.some((item) => item.id === routeClassId);
      if (!hasRouteClass && firstClassId > 0) {
        selectedClassId.value = firstClassId;
        await router.replace({
          path: `/files/classes/${firstClassId}`,
          query: route.query,
        });
        return;
      }
      selectedClassId.value = hasRouteClass ? routeClassId : firstClassId;
    }

    const query = new URLSearchParams({
      space: space.value,
    });
    if (isClassSpace.value) {
      query.set("classId", String(selectedClassId.value));
    }
    if (typeof route.query.path === "string" && route.query.path) {
      query.set("path", route.query.path);
    }
    if (sortMode.value !== defaultFilesSortMode) {
      query.set("sort", sortMode.value);
    }
    query.set("page", String(filePage.value));
    query.set("pageSize", String(filePageSize.value));
    if (routeSearchQuery) {
      query.set("q", routeSearchQuery);
    }

    const filesResponse = routeSearchQuery ? await api.searchFiles(query) : await api.files(query);
    items.value = Array.isArray(filesResponse.items) ? filesResponse.items : [];
    const pagination = filesResponse.pagination;
    filePage.value = pagination?.page ?? filePage.value;
    filePageSize.value = normalizeFilesPageSize(pagination?.pageSize ?? filePageSize.value);
    totalFiles.value = pagination?.total ?? items.value.length;
    totalFilePages.value = Math.max(1, pagination?.totalPages ?? Math.ceil(Math.max(totalFiles.value, 1) / filePageSize.value));
    currentPath.value = normalizePath(filesResponse.currentPath);
    syncSelectedEntries();
  } catch (error) {
    errorText.value = resolveFilesLoadErrorText(error);
  }
}

function resolveFilesLoadErrorText(error: unknown) {
  if (error instanceof ApiError) {
    return error.message;
  }
  if (error instanceof TypeError) {
    return "连接已中断，请重试";
  }
  return "加载文件失败";
}

async function fetchTextPreviewContent(item: FileItem) {
  const cachedText = previewTextCache.get(item.id);
  if (typeof cachedText === "string") {
    return cachedText;
  }

  try {
    const response = await api.readFileContent(item.id);
    previewTextCache.set(item.id, response.content);
    return response.content;
  } catch {
    const response = await fetch(item.previewUrl, {
      credentials: "same-origin",
    });
    if (!response.ok) {
      throw new Error("加载失败");
    }
    const content = await response.text();
    previewTextCache.set(item.id, content);
    return content;
  }
}

async function applySearch() {
  const nextQuery: LocationQueryRaw = { ...route.query };
  const keyword = searchQuery.value.trim();
  if (keyword) {
    nextQuery.q = keyword;
  } else {
    delete nextQuery.q;
  }
  delete nextQuery.page;
  await router.push({
    path: route.path,
    query: nextQuery,
  });
}

async function clearSearch() {
  searchQuery.value = "";
  const nextQuery: LocationQueryRaw = { ...route.query };
  delete nextQuery.q;
  delete nextQuery.page;
  await router.push({
    path: route.path,
    query: nextQuery,
  });
}

async function applySortMode(nextSort: FilesSortMode) {
  sortMode.value = nextSort;
  await replaceFilesRoute({ sort: nextSort, page: 1 });
}

async function toggleFileNameSort() {
  await applySortMode(sortMode.value === "name-asc" ? "name-desc" : "name-asc");
}

async function toggleFileUpdatedSort() {
  await applySortMode(sortMode.value === "updated-desc" ? "updated-asc" : "updated-desc");
}

async function toggleFileSizeSort() {
  await applySortMode(sortMode.value === "size-desc" ? "size-asc" : "size-desc");
}

async function setViewMode(mode: "list" | "grid") {
  if (viewMode.value === mode && parseFilesViewMode(route.query.view) === mode) {
    return;
  }
  viewMode.value = mode;
  await replaceFilesRoute({ view: mode });
}

async function setGridSize(size: FilesGridSize) {
  if (gridSize.value === size && parseFilesGridSize(route.query.gridSize) === size) {
    return;
  }
  gridSize.value = size;
  await replaceFilesRoute({ gridSize: size, page: 1 });
}

async function updateFilePageSize(value: number) {
  await replaceFilesRoute({ pageSize: value, page: 1 });
}

async function goPrevFilePage() {
  if (filePage.value <= 1) {
    return;
  }
  await replaceFilesRoute({ page: filePage.value - 1 });
}

async function goNextFilePage() {
  if (filePage.value >= totalFilePages.value) {
    return;
  }
  await replaceFilesRoute({ page: filePage.value + 1 });
}

async function goFilePage(page: number): Promise<void> {
  const nextPage = Math.min(Math.max(1, Math.trunc(page)), totalFilePages.value);
  if (nextPage === filePage.value) {
    return;
  }
  await replaceFilesRoute({ page: nextPage });
}

async function ensureClassesLoaded() {
  if (classes.value.length > 0) {
    return;
  }
  const classesResponse = await api.classes();
  classes.value = classesResponse.classes ?? [];
  if (!copyTargetClassId.value && classes.value.length > 0) {
    copyTargetClassId.value = classes.value[0].id;
  }
}

async function loadCopyFolders() {
  if (!copyDialogEntry.value) {
    return;
  }
  if (copyTargetSpace.value === "class" && !copyTargetClassId.value) {
    copyTargetFolders.value = [];
    return;
  }

  copyLoading.value = true;
  copyErrorText.value = "";
  try {
    if (copySearch.value.trim()) {
      copyTargetFolders.value = await recursivelySearchTargetFolders(copySearch.value.trim());
    } else {
      copyTargetFolders.value = await fetchFoldersForPath(copyTargetPath.value);
    }
  } catch (error) {
    copyErrorText.value = error instanceof ApiError ? error.message : "加载目标目录失败";
    copyTargetFolders.value = [];
  } finally {
    copyLoading.value = false;
  }
}

async function fetchFoldersForPath(pathValue: string) {
  const query = new URLSearchParams({
    space: copyTargetSpace.value,
    path: pathValue,
  });
  if (copyTargetSpace.value === "class" && copyTargetClassId.value) {
    query.set("classId", String(copyTargetClassId.value));
  }
  const response = await api.files(query);
  return response.items.filter((item) => item.kind === "dir");
}

async function recursivelySearchTargetFolders(keyword: string) {
  const visited = new Set<string>();
  const matched: FileItem[] = [];
  const queue = [...copyTargetFolders.value.map((folder) => folder.path)];

  for (const folder of copyTargetFolders.value) {
    if (folder.name.includes(keyword)) {
      matched.push(folder);
    }
  }

  while (queue.length > 0 && visited.size < 50) {
    const current = queue.shift()!;
    if (visited.has(current)) {
      continue;
    }
    visited.add(current);

    let folders: FileItem[] = [];
    try {
      folders = await fetchFoldersForPath(current);
    } catch {
      continue;
    }
    for (const folder of folders) {
      if (folder.name.includes(keyword)) {
        matched.push(folder);
      }
      if (!visited.has(folder.path)) {
        queue.push(folder.path);
      }
    }
  }

  return matched;
}

function finalizeRouteContextChange() {
  closePreview();
}

function openConfirmDialog(state: ConfirmDialogState) {
  confirmDialogState.value = state;
}

async function confirmDialog() {
  const state = confirmDialogState.value;
  if (!state) {
    return;
  }
  confirmDialogState.value = null;
  try {
    await state.onConfirm();
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "操作失败");
  }
}

function cancelConfirmDialog() {
  const state = confirmDialogState.value;
  confirmDialogState.value = null;
  state?.onCancel?.();
}

async function confirmTextInputDialog(value: string) {
  const state = textInputDialogState.value;
  if (!state) {
    return;
  }
  textInputDialogState.value = null;
  await state.onConfirm(value);
}

function openTextInputDialog(state: TextInputDialogState) {
  textInputDialogState.value = state;
}

function prepareRouteContextChange(continuation?: () => void | Promise<void>) {
  if (editorItem.value) {
    if (editorSaving.value) {
      return "blocked" as const;
    }
    if (editorDirty.value && !editorLoading.value) {
      openConfirmDialog({
        prefix: "file-editor-unsaved",
        title: "确认放弃未保存内容",
        message: "当前内容尚未保存，继续后将丢失本次修改。",
        confirmLabel: "放弃修改",
        confirmTone: "danger",
        onConfirm: async () => {
          resetEditorState();
          finalizeRouteContextChange();
          if (continuation) {
            await continuation();
          }
        },
      });
      return "deferred" as const;
    }
    resetEditorState();
  }
  finalizeRouteContextChange();
  return "proceed" as const;
}

async function executeApprovedNavigation(navigate: () => Promise<unknown>) {
  routeNavigationApproved = true;
  try {
    await navigate();
  } finally {
    routeNavigationApproved = false;
  }
}

async function runRouteNavigation(navigate: () => Promise<unknown>, onCancel?: () => void) {
  const result = prepareRouteContextChange(async () => {
    await executeApprovedNavigation(navigate);
  });
  if (result === "blocked") {
    onCancel?.();
    return false;
  }
  if (result === "deferred") {
    if (confirmDialogState.value && onCancel) {
      const existingCancel = confirmDialogState.value.onCancel;
      confirmDialogState.value.onCancel = () => {
        existingCancel?.();
        onCancel();
      };
    }
    return false;
  }
  await executeApprovedNavigation(navigate);
  return true;
}

function openUpload() {
  closeUploadDialog();
  fileInput.value?.click();
}

function openUploadDirectory() {
  closeUploadDialog();
  directoryInput.value?.click();
}

function closeUploadDialog() {
  uploadDialogOpen.value = false;
}

function buildUploadItems(files: FileList): UploadFileItem[] {
  const items = Array.from(files).map((file) => {
    const relativePath = typeof file.webkitRelativePath === "string" && file.webkitRelativePath.length > 0 ? file.webkitRelativePath : undefined;
    return {
      file,
      relativePath,
    };
  });
  return filterRedundantDirectoryArchives(items);
}

async function uploadFromInput(target: HTMLInputElement, directoryUpload: boolean) {
  if (!target.files || !target.files.length) {
    return;
  }
  const files = buildUploadItems(target.files);
  await uploadItems(files, directoryUpload);
  target.value = "";
}

function createUploadLifecycleController(): UploadLifecycleController {
  return {
    beginBatch: (items) => uploadStore.beginBatch(items),
    setAbortHandler: (handler) => uploadStore.setAbortHandler(handler),
    applyAggregateProgress: (sentBytes, totalBytes) => uploadStore.applyAggregateProgress(sentBytes, totalBytes),
    markBatchDone: () => uploadStore.markBatchDone(),
    markBatchAborted: () => uploadStore.markBatchAborted(),
    markFailedAt: (sentBytes) => uploadStore.markFailedAt(sentBytes),
    isAbortRequested: () => uploadStore.abortRequested,
    sentBytes: () => uploadStore.sentBytes,
  };
}

async function uploadItems(files: UploadFileItem[], directoryUpload: boolean) {
  if (files.length === 0) {
    return;
  }

  try {
    const result = await api.uploadFiles({
      space: space.value,
      classId: isClassSpace.value ? selectedClassId.value : undefined,
      parentPath: currentPath.value,
      files,
      conflictMode: uploadConflictMode.value,
      controller: createUploadLifecycleController(),
    });
    toastStore.push("success", uploadSuccessMessage(result.summary, directoryUpload));
    await loadFiles();
  } catch (error) {
    toastStore.push("error", error instanceof Error ? error.message : "上传失败");
  } finally {
    uploadStore.finish();
  }
}

async function handleUpload(event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  await uploadFromInput(target, false);
}

async function handleDirectoryUpload(event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLInputElement)) {
    return;
  }
  await uploadFromInput(target, true);
}

function handleUploadDragEnter() {
  uploadDragDepth.value += 1;
  isDraggingUpload.value = true;
}

function handleUploadDragOver() {
  isDraggingUpload.value = true;
}

function handleUploadDragLeave() {
  uploadDragDepth.value = Math.max(0, uploadDragDepth.value - 1);
  if (uploadDragDepth.value === 0) {
    isDraggingUpload.value = false;
  }
}

function hasUploadDragData(event: DragEvent): boolean {
  const transfer = event.dataTransfer;
  if (!transfer) {
    return false;
  }
  const transferTypes = Array.from(transfer.types ?? []);
  return transferTypes.includes("Files") || (transfer.items?.length ?? 0) > 0 || (transfer.files?.length ?? 0) > 0;
}

function preventBrowserFileDrop(event: DragEvent): boolean {
  if (!hasUploadDragData(event)) {
    return false;
  }
  event.preventDefault();
  event.stopPropagation();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "copy";
  }
  return true;
}

function handleWindowUploadDragEnter(event: DragEvent) {
  if (!preventBrowserFileDrop(event)) {
    return;
  }
  uploadDragDepth.value = 1;
  isDraggingUpload.value = true;
}

function handleWindowUploadDragOver(event: DragEvent) {
  if (!preventBrowserFileDrop(event)) {
    return;
  }
  isDraggingUpload.value = true;
}

function handleWindowUploadDragLeave(event: DragEvent) {
  if (!preventBrowserFileDrop(event)) {
    return;
  }
  if (
    event.clientX <= 0
    || event.clientY <= 0
    || event.clientX >= window.innerWidth
    || event.clientY >= window.innerHeight
  ) {
    uploadDragDepth.value = 0;
    isDraggingUpload.value = false;
  }
}

async function handleWindowUploadDrop(event: DragEvent) {
  if (!preventBrowserFileDrop(event)) {
    return;
  }
  await handleUploadDrop(event);
}

async function handleUploadDrop(event: DragEvent) {
  uploadDragDepth.value = 0;
  isDraggingUpload.value = false;
  if (!event.dataTransfer) {
    return;
  }
  const files = await collectDroppedUploadItems(event.dataTransfer);
  if (files.length === 0) {
    return;
  }
  await uploadItems(files, files.some((item) => typeof item.relativePath === "string" && item.relativePath.length > 0));
}

async function promptCreateFolder() {
  openTextInputDialog({
    prefix: "create-folder",
    title: "新建文件夹",
    label: "文件夹名称",
    initialValue: "",
    placeholder: "输入新建文件夹名称",
    confirmLabel: "创建文件夹",
    onConfirm: async (name) => {
      try {
        await api.createFolder({
          space: space.value,
          classId: isClassSpace.value ? selectedClassId.value : undefined,
          parentPath: currentPath.value,
          name,
        });
        toastStore.push("success", uiCopy.folderCreatedSuccess);
        await loadFiles();
      } catch (error) {
        toastStore.push("error", error instanceof ApiError ? error.message : "创建失败");
      }
    },
  });
}

function openCreateFileDialog() {
  openTextInputDialog({
    prefix: "create-file",
    title: "新建文件",
    label: "文件名称",
    initialValue: "新建文档.md",
    placeholder: "输入新建文件名称",
    confirmLabel: "创建文件",
    onConfirm: async (name) => {
      const requestToken = advanceEditorRequestToken();
      const requestRouteFullPath = route.fullPath;
      try {
        const response = await api.createFile({
          space: space.value,
          classId: isClassSpace.value ? selectedClassId.value : undefined,
          parentPath: currentPath.value,
          name,
          content: "",
        });
        if (route.fullPath !== requestRouteFullPath) {
          return;
        }
        if (!openEditorFromContentResult(response, requestToken)) {
          return;
        }
        toastStore.push("success", uiCopy.fileCreatedSuccess);
      } catch (error) {
        if (route.fullPath !== requestRouteFullPath) {
          return;
        }
        if (!isEditorRequestTokenCurrent(requestToken)) {
          return;
        }
        toastStore.push("error", error instanceof ApiError ? error.message : "创建失败");
      }
    },
  });
}

async function promptCreateFile() {
  const result = prepareRouteContextChange(() => {
    openCreateFileDialog();
  });
  if (result !== "proceed") {
    return;
  }
  openCreateFileDialog();
}

async function rename(item: FileItem) {
  openTextInputDialog({
    prefix: "rename-entry",
    title: "重命名",
    label: "新的名称",
    initialValue: item.name,
    placeholder: "输入新的名称",
    confirmLabel: "确认重命名",
    onConfirm: async (name) => {
      if (name === item.name) {
        return;
      }
      try {
        await api.renameFile(item.id, name);
        toastStore.push("success", uiCopy.renameSuccess);
        await loadFiles();
      } catch (error) {
        toastStore.push("error", error instanceof ApiError ? error.message : "重命名失败");
      }
    },
  });
}

async function openTargetDialog(entries: FileItem[], mode: "copy" | "move") {
  if (entries.length === 0) {
    return;
  }
  targetDialogEntries.value = [...entries];
  targetDialogMode.value = mode;
  const initialTargetSpace = (space.value === "public" ? "library" : "public") as FileSpace;
  copyTargetSpace.value = initialTargetSpace;
  copyTargetClassId.value = classes.value[0]?.id ?? null;
  copyTargetPath.value = "/";
  copySearch.value = "";
  copyCreateFolderName.value = "";
  await recentCopyTargetsStore.load();
  await loadCopyFolders();
}

async function openCopyDialog(item: FileItem) {
  await openTargetDialog([item], "copy");
}

async function openMoveDialog(item: FileItem) {
  await openTargetDialog([item], "move");
}

async function openBatchCopyDialog() {
  await openTargetDialog(selectedItems.value, "copy");
}

async function openBatchMoveDialog() {
  await openTargetDialog(selectedItems.value, "move");
}

function closeCopyDialog() {
  targetDialogEntries.value = [];
  copyTargetPath.value = "/";
  copyTargetFolders.value = [];
  copyErrorText.value = "";
  copySearch.value = "";
  copyCreateFolderName.value = "";
}

function openShareDialog(item: FileItem) {
  shareDialogEntry.value = item;
  shareCreateDialogOpen.value = true;
  openEntryActionMenuId.value = null;
}

watch([copyTargetSpace, copyTargetClassId], async ([value], [previousValue]) => {
  if (!copyDialogEntry.value) {
    return;
  }
  if (value === "class") {
    try {
      await ensureClassesLoaded();
    } catch (error) {
      toastStore.push("error", error instanceof ApiError ? error.message : "加载班级失败");
    }
  }
  if (value !== previousValue) {
    copyTargetPath.value = "/";
  }
  await loadCopyFolders();
});

async function submitCopy() {
  if (!copyDialogEntry.value) {
    return;
  }
  if (copyTargetSpace.value === "class" && !copyTargetClassId.value) {
    toastStore.push("error", "请选择目标班级");
    return;
  }
  try {
    for (const entry of targetDialogEntries.value) {
      const payload = {
        entryId: entry.id,
        destinationSpace: copyTargetSpace.value,
        destinationClassId: copyTargetSpace.value === "class" ? copyTargetClassId.value ?? undefined : undefined,
        destinationParentPath: copyTargetPath.value,
      };
      if (targetDialogMode.value === "move") {
        await api.moveFile(payload);
      } else {
        await api.copyFile(payload);
      }
    }
    try {
      await recentCopyTargetsStore.remember({
        space: copyTargetSpace.value,
        classId: copyTargetSpace.value === "class" ? copyTargetClassId.value : null,
        path: copyTargetPath.value,
        label: copyTargetPath.value === "/" ? "根目录" : copyTargetPath.value.split("/").filter(Boolean).join(" / "),
      });
    } catch {
      toastStore.push("error", "保存最近目标目录失败");
    }
    clearSelection();
    closeCopyDialog();
    toastStore.push("success", targetDialogMode.value === "move" ? uiCopy.moveSuccess : uiCopy.copySuccess);
    if (targetDialogMode.value === "move") {
      await loadFiles();
    }
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : targetDialogMode.value === "move" ? "移动失败" : "复制失败");
  }
}

async function createCopyTargetFolder() {
  const name = copyCreateFolderName.value.trim();
  if (!name) {
    toastStore.push("error", "请输入目标目录名称");
    return;
  }

  try {
    const created = await api.createFolder({
      space: copyTargetSpace.value,
      classId: copyTargetSpace.value === "class" ? copyTargetClassId.value ?? undefined : undefined,
      parentPath: copyTargetPath.value,
      name,
    });
    copyCreateFolderName.value = "";
    await navigateCopyTargetPath(created.path);
  } catch (error) {
    toastStore.push("error", error instanceof ApiError ? error.message : "创建目标目录失败");
  }
}

async function remove(item: FileItem) {
  openConfirmDialog({
    prefix: "delete-entry",
    title: "确认删除",
    message: `确认删除 ${item.name} 吗？`,
    confirmLabel: "确认删除",
    confirmTone: "danger",
    onConfirm: async () => {
      try {
        await api.deleteFile(item.id);
        toastStore.push("success", uiCopy.deleteSuccess);
        await loadFiles();
      } catch (error) {
        toastStore.push("error", error instanceof ApiError ? error.message : "删除失败");
      }
    },
  });
}

async function removeSelected() {
  if (selectedItems.value.length === 0) {
    return;
  }
  openConfirmDialog({
    prefix: "delete-selected",
    title: "确认批量删除",
    message: `确认删除已选 ${selectedItems.value.length} 项吗？`,
    confirmLabel: "确认删除",
    confirmTone: "danger",
    onConfirm: async () => {
      const entriesToDelete = [...selectedItems.value];
      const failedEntries: string[] = [];
      const failedIds = new Set<number>();
      try {
        for (const item of entriesToDelete) {
          try {
            await api.deleteFile(item.id);
          } catch (error) {
            const message = error instanceof ApiError ? error.message : "删除失败";
            failedIds.add(item.id);
            failedEntries.push(`${item.name}：${message}`);
          }
        }
        const successfulIds = entriesToDelete
          .filter((item) => !failedIds.has(item.id))
          .map((item) => item.id);
        if (successfulIds.length > 0) {
          selectedEntryIds.value = selectedEntryIds.value.filter((id) => !successfulIds.includes(id));
          await loadFiles();
        }
        if (failedEntries.length > 0) {
          toastStore.push("error", `部分删除失败：${failedEntries.slice(0, 3).join("；")}`);
          return;
        }
        toastStore.push("success", uiCopy.deleteSuccess);
        clearSelection();
      } catch (error) {
        toastStore.push("error", error instanceof ApiError ? error.message : "删除失败");
      }
    },
  });
}

async function navigateToPath(path: string) {
  const nextPath = normalizePath(path);
  const nextQuery: LocationQueryRaw = { ...route.query };
  if (nextPath === "/") {
    delete nextQuery.path;
  } else {
    nextQuery.path = nextPath;
  }
  delete nextQuery.page;
  await runRouteNavigation(() =>
    router.push({
      path: route.path,
      query: nextQuery,
    }),
  );
}

async function navigateUp() {
  if (!canNavigateUp.value) {
    return;
  }
  await navigateToPath(parentPathOf(currentPath.value));
}

const copyBreadcrumbs = computed(() => toBreadcrumbs(copyTargetPath.value));

async function navigateCopyTargetPath(path: string) {
  copyTargetPath.value = normalizePath(path);
  await loadCopyFolders();
}

async function applyRecentTarget(target: RecentCopyTarget) {
  copyTargetSpace.value = target.space;
  copyTargetClassId.value = target.classId;
  copyTargetPath.value = target.path;
  copySearch.value = "";
  if (target.space === "class" && classes.value.length === 0) {
    await ensureClassesLoaded();
  }
  await loadCopyFolders();
}

async function togglePinnedRecentTarget(index: number) {
  try {
    await recentCopyTargetsStore.togglePinned(index);
  } catch {
    toastStore.push("error", "保存最近目标目录失败");
  }
}

async function movePinnedRecentTarget(index: number, direction: -1 | 1) {
  try {
    await recentCopyTargetsStore.movePinned(index, direction);
  } catch {
    toastStore.push("error", "保存最近目标目录失败");
  }
}

async function clearRecentTargets() {
  try {
    await recentCopyTargetsStore.clearUnpinned();
  } catch {
    toastStore.push("error", "保存最近目标目录失败");
  }
}

async function openDirectory(item: FileItem) {
  if (item.kind !== "dir") {
    return;
  }
  await navigateToPath(item.path);
}

function closePreview() {
  previewItem.value = null;
  previewTextContent.value = "";
  previewMarkdownContent.value = "";
  previewErrorText.value = "";
  previewLoading.value = false;
}

function advanceEditorRequestToken() {
  editorRequestToken += 1;
  return editorRequestToken;
}

function isEditorRequestTokenCurrent(token: number) {
  return token === editorRequestToken;
}

function openEditorFromContentResult(response: FileContentResult, token?: number) {
  if (token !== undefined && !isEditorRequestTokenCurrent(token)) {
    return false;
  }
  closePreview();
  editorItem.value = response.item;
  editorContent.value = response.content;
  editorInitialContent.value = response.content;
  editorLoading.value = false;
  editorSaving.value = false;
  editorErrorText.value = "";
  syncLoadedItem(response.item);
  return true;
}

function resetEditorState() {
  advanceEditorRequestToken();
  editorItem.value = null;
  editorContent.value = "";
  editorInitialContent.value = "";
  editorLoading.value = false;
  editorSaving.value = false;
  editorErrorText.value = "";
}

function closeEditor() {
  if (editorSaving.value) {
    return false;
  }
  if (editorDirty.value && !editorLoading.value) {
    openConfirmDialog({
      prefix: "file-editor-unsaved",
      title: "确认关闭编辑器",
      message: "当前内容尚未保存，关闭后将丢失本次修改。",
      confirmLabel: "放弃修改",
      confirmTone: "danger",
      onConfirm: () => {
        resetEditorState();
      },
    });
    return false;
  }
  resetEditorState();
  return true;
}

async function openEditor(item: FileItem) {
  if (!canEditFile(item)) {
    return;
  }

  const requestToken = advanceEditorRequestToken();
  closePreview();
  editorItem.value = item;
  editorContent.value = "";
  editorInitialContent.value = "";
  editorLoading.value = true;
  editorSaving.value = false;
  editorErrorText.value = "";

  try {
    const response = await api.readFileContent(item.id);
    openEditorFromContentResult(response, requestToken);
  } catch (error) {
    if (!isEditorRequestTokenCurrent(requestToken)) {
      return;
    }
    editorErrorText.value = error instanceof ApiError ? error.message : "加载编辑内容失败";
  } finally {
    if (isEditorRequestTokenCurrent(requestToken)) {
      editorLoading.value = false;
    }
  }
}

async function openEditorFromPreview() {
  const item = previewItem.value;
  if (!item) {
    return;
  }
  await openEditor(item);
}

async function previewPrevious() {
  if (!previewHasPrevious.value) {
    return;
  }
  const item = previewableItems.value[previewItemIndex.value - 1];
  if (item) {
    await preview(item);
  }
}

async function previewNext() {
  if (!previewHasNext.value) {
    return;
  }
  const item = previewableItems.value[previewItemIndex.value + 1];
  if (item) {
    await preview(item);
  }
}

async function saveEditor() {
  if (!editorItem.value || editorSaveDisabled.value) {
    return;
  }

  const requestToken = editorRequestToken;
  const itemId = editorItem.value.id;
  const content = editorContent.value;
  editorSaving.value = true;
  editorErrorText.value = "";
  try {
    const response = await api.saveFileContent(itemId, content);
    if (!openEditorFromContentResult(response, requestToken)) {
      return;
    }
    toastStore.push("success", uiCopy.saveSuccess);
    await loadFiles();
  } catch (error) {
    if (!isEditorRequestTokenCurrent(requestToken)) {
      return;
    }
    editorErrorText.value = error instanceof ApiError ? error.message : "保存失败";
  } finally {
    if (isEditorRequestTokenCurrent(requestToken)) {
      editorSaving.value = false;
    }
  }
}

async function preview(item: FileItem) {
  const continuePreview = async () => {
    if (item.kind === "dir") {
      await openDirectory(item);
      return;
    }
    const nextPreviewKind = getFilePreviewKind(item);
    if (nextPreviewKind === "external") {
      window.open(item.previewUrl, "_blank", "noopener");
      return;
    }

    previewItem.value = item;
    previewTextContent.value = "";
    previewMarkdownContent.value = "";
    previewErrorText.value = "";
    previewLoading.value = false;

    if (nextPreviewKind !== "text" && nextPreviewKind !== "markdown") {
      return;
    }

    previewLoading.value = true;
    try {
      const content = await fetchTextPreviewContent(item);
      if (nextPreviewKind === "markdown") {
        previewMarkdownContent.value = renderMarkdownPreview(content);
      } else {
        previewTextContent.value = content;
      }
    } catch {
      previewErrorText.value = uiCopy.previewLoadFailed;
    } finally {
      previewLoading.value = false;
    }
  };

  const result = prepareRouteContextChange(continuePreview);
  if (result !== "proceed") {
    return;
  }
  await continuePreview();
}

if (activeMatchedRoute) {
  onBeforeRouteUpdate((to, from) => {
    if (to.fullPath === from.fullPath) {
      return;
    }
    if (routeNavigationApproved) {
      return;
    }
    const result = prepareRouteContextChange(async () => {
      await executeApprovedNavigation(() => router.push(to.fullPath));
    });
    if (result !== "proceed") {
      return false;
    }
  });

  onBeforeRouteLeave((to) => {
    if (routeNavigationApproved) {
      return;
    }
    const result = prepareRouteContextChange(async () => {
      await executeApprovedNavigation(() => router.push(to.fullPath));
    });
    if (result !== "proceed") {
      return false;
    }
  });
}

watch(
  () => route.fullPath,
  async () => {
    await loadFiles();
  }
);

onMounted(async () => {
  document.addEventListener("click", closeEntryActionMenuOnOutsideClick);
  window.addEventListener("dragenter", handleWindowUploadDragEnter, true);
  window.addEventListener("dragover", handleWindowUploadDragOver, true);
  window.addEventListener("dragleave", handleWindowUploadDragLeave, true);
  window.addEventListener("drop", handleWindowUploadDrop, true);
  await loadFiles();
});

onBeforeUnmount(() => {
  document.removeEventListener("click", closeEntryActionMenuOnOutsideClick);
  window.removeEventListener("dragenter", handleWindowUploadDragEnter, true);
  window.removeEventListener("dragover", handleWindowUploadDragOver, true);
  window.removeEventListener("dragleave", handleWindowUploadDragLeave, true);
  window.removeEventListener("drop", handleWindowUploadDrop, true);
});

watch(copySearch, async () => {
  if (!copyDialogEntry.value) {
    return;
  }
  await loadCopyFolders();
});

watch(viewMode, () => {
  openEntryActionMenuId.value = null;
  // 视图切换后重置焦点索引
  focusedItemIndex.value = -1;
});

// 键盘导航支持
const focusedItemIndex = ref(-1);

function handleListKeyboardNavigation(event: KeyboardEvent) {
  if (displayedItems.value.length === 0) {
    return;
  }

  const key = event.key;

  // Arrow 键导航
  if (key === 'ArrowDown' || key === 'ArrowUp') {
    event.preventDefault();

    if (focusedItemIndex.value === -1) {
      // 首次导航，聚焦到第一项
      focusedItemIndex.value = 0;
    } else if (key === 'ArrowDown' && focusedItemIndex.value < displayedItems.value.length - 1) {
      focusedItemIndex.value++;
    } else if (key === 'ArrowUp' && focusedItemIndex.value > 0) {
      focusedItemIndex.value--;
    }

    // 聚焦到对应的元素
    focusItemByIndex(focusedItemIndex.value);
  }
  // Home / End 键
  else if (key === 'Home') {
    event.preventDefault();
    focusedItemIndex.value = 0;
    focusItemByIndex(0);
  }
  else if (key === 'End') {
    event.preventDefault();
    focusedItemIndex.value = displayedItems.value.length - 1;
    focusItemByIndex(focusedItemIndex.value);
  }
  // Escape 键关闭菜单
  else if (key === 'Escape') {
    if (openEntryActionMenuId.value !== null) {
      event.preventDefault();
      openEntryActionMenuId.value = null;
    }
  }
}

function focusItemByIndex(index: number) {
  if (index < 0 || index >= displayedItems.value.length) {
    return;
  }

  const item = displayedItems.value[index];
  const selector = viewMode.value === 'list'
    ? `.files-table__row[data-item-id="${item.id}"]`
    : `.files-grid__card[data-item-id="${item.id}"]`;

  const element = document.querySelector(selector) as HTMLElement;
  if (element) {
    element.focus();
    // 确保元素在视口中可见
    element.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
}

function handleItemFocus(index: number) {
  focusedItemIndex.value = index;
}

</script>

<style scoped>
.files-page__workspace {
  gap: 10px;
}

.files-page__workspace > .files-toolbar {
  margin: 0;
  padding: 0 0 10px;
  border: 0;
  border-bottom: 1px solid var(--border-soft);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.files-page__workspace > .files-table,
.files-page__workspace .files-table {
  box-shadow: none;
}

.files-toolbar {
  --files-search-slot-width: min(240px, 24vw);
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  align-items: stretch;
  justify-content: stretch;
  margin-bottom: 12px;
  width: 100%;
}

.files-toolbar__top,
.files-toolbar__bottom {
  display: grid;
  gap: 10px;
  width: 100%;
}

.files-toolbar__top {
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
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
.files-secondary-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  justify-content: flex-start;
}

.files-primary-actions__group,
.files-secondary-controls__inner,
.files-upload-options,
.files-selection-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.files-primary-actions__group {
  gap: 8px;
}

.files-table [data-testid^="entry-updated-"] {
  font-variant-numeric: tabular-nums;
}

.files-table__row {
  cursor: pointer;
}

.files-table__row:hover,
.files-table__row:focus-visible {
  background: var(--bg-subtle);
}

.files-table__row:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: -3px;
}

.files-secondary-controls__inner {
  width: 100%;
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

.files-upload-options,
.files-selection-toolbar {
  padding: 14px;
  border: 1px solid var(--border-soft);
  border-radius: 18px;
  background: var(--bg-subtle);
}

.files-selection-toolbar {
  border-color: rgba(37, 99, 235, 0.22);
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.08), rgba(15, 118, 110, 0.08)),
    var(--bg-subtle);
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.08);
}

:global(:root.dark) .files-selection-toolbar {
  border-color: rgba(96, 165, 250, 0.34);
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.18), rgba(15, 118, 110, 0.14)),
    rgba(10, 18, 32, 0.96);
  box-shadow: inset 0 0 0 1px rgba(125, 211, 252, 0.12);
}

.files-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
}

.files-controls__search,
.files-controls__options,
.files-controls__view {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.files-controls__search {
  flex: 0 1 auto;
  width: auto;
  justify-content: flex-end;
}

.files-controls__search input {
  min-width: 140px;
  width: 160px;
}

.files-controls__option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.files-primary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.files-primary-actions--left {
  justify-content: flex-start;
}

.files-up-button {
  min-width: 44px;
  padding-inline: 0;
  font-size: 1rem;
}

.files-upload-dialog__actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 18px;
}

.files-upload-dialog__choice {
  min-width: 140px;
}

.files-toolbar__option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.files-upload-options__hint,
.files-selection-toolbar__eyebrow {
  color: var(--text-muted);
  font-size: 12px;
}

.files-selection-toolbar__summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.files-batch-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.files-entry-meta {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.4;
}

.files-page--dragging {
  outline: 2px dashed var(--accent-primary);
  outline-offset: 6px;
}

.files-drop-hint {
  margin: 0 0 12px;
  color: var(--accent-primary);
  font-size: 13px;
}

.files-search-summary {
  margin: 0 0 12px;
  color: var(--text-secondary);
  font-size: 13px;
}

.files-error-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 0 12px;
}

.files-error-banner .form-error {
  margin: 0;
}

.files-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
}

.files-grid--small {
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px;
}

.files-grid--medium {
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
}

.files-grid--large {
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}

.files-grid__card {
  min-width: 0;
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  padding: 10px;
  background: var(--bg-surface);
  box-shadow: var(--shadow-soft);
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease;
}

.files-grid__card:hover,
.files-grid__card:focus-visible {
  border-color: color-mix(in srgb, var(--accent-primary) 38%, var(--border-soft));
  background: var(--bg-subtle);
}

.files-grid__card:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
}

.files-grid__select {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
  color: var(--text-muted);
  font-size: 12px;
}

.files-grid__thumbnail-button {
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

.files-grid__thumbnail-button:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
}

.files-grid__thumbnail {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
}

.files-grid__title {
  display: block;
  min-width: 0;
  max-width: 100%;
  margin-bottom: 6px;
  font-weight: 600;
  text-align: left;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.files-grid__title span {
  overflow-wrap: anywhere;
  word-break: break-word;
}

.files-table__actions,
.files-entry-actions__primary,
.files-entry-actions__secondary,
.files-grid__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.files-table__actions {
  position: relative;
  flex-direction: row;
  align-items: center;
}

.files-entry-actions__overflow {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.files-entry-actions__secondary {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 32;
  min-width: 132px;
  flex-direction: column;
  align-items: stretch;
  padding: 8px;
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  background: var(--popover-bg);
  box-shadow: var(--popover-shadow);
}

.files-entry-actions__more {
  color: var(--text-secondary);
}

.files-grid__actions {
  position: relative;
  align-items: center;
  justify-content: flex-start;
}

.files-grid__actions .files-entry-actions__secondary {
  left: 0;
  right: auto;
}

.files-breadcrumb {
  gap: 8px;
}

.files-context-bar select,
.files-toolbar__option select {
  min-width: 120px;
}

.files-upload-dialog {
  width: min(480px, 100%);
}

.copy-dialog__summary {
  margin-bottom: 12px;
  color: var(--text-secondary);
}

.copy-dialog__summary-card {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border: 1px solid var(--border-soft);
  border-radius: 16px;
  background: var(--bg-subtle);
}

.copy-dialog__summary-card strong {
  color: var(--text-primary);
}

.copy-dialog__summary-label {
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 700;
}

.copy-dialog__section {
  margin-bottom: 16px;
}

.copy-dialog__section-title {
  margin-bottom: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.copy-dialog__space-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.copy-dialog__space-grid .field {
  margin-top: 0;
}

.copy-dialog__current-target {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  background: var(--control-bg);
}

.copy-dialog__current-target span {
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 700;
}

.copy-dialog__current-target strong {
  color: var(--text-primary);
  font-size: 13px;
  word-break: break-all;
}

.files-secondary-controls--right {
  justify-self: end;
}

@media (max-width: 1100px) {
  .files-toolbar {
    --files-search-slot-width: min(240px, 100%);
  }

  .files-toolbar__top {
    grid-template-columns: 1fr;
  }

  .files-toolbar__bottom {
    grid-template-columns: minmax(0, 1fr);
    min-height: 0;
    padding-right: 0;
  }

  .files-toolbar__search-slot {
    position: static;
    justify-self: stretch;
    margin-left: 0;
    transform: none;
  }

  .files-controls__search {
    justify-content: flex-start;
    width: auto;
  }

  .files-toolbar__search-slot .files-controls__search {
    flex-wrap: wrap;
  }
}

/* 平板端优化 (768px - 1100px) */
@media (max-width: 768px) {
  .files-toolbar__top,
  .files-toolbar__bottom {
    gap: 8px;
  }

  .files-primary-actions__group {
    flex-wrap: wrap;
    gap: 6px;
  }

  .files-primary-actions__group .button {
    font-size: 14px;
    padding: 8px 12px;
  }

  .files-controls__view,
  .files-controls__grid-size {
    gap: 4px;
  }

  .files-controls__view .button,
  .files-controls__grid-size .button {
    min-width: 36px;
    padding: 6px 10px;
    font-size: 13px;
  }

  .files-breadcrumb {
    font-size: 14px;
    gap: 6px;
  }

  .files-table {
    font-size: 14px;
  }

  .files-table th,
  .files-table td {
    padding: 8px 10px;
  }

  .files-table__actions {
    font-size: 13px;
  }

  .files-grid--small {
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  }

  .files-grid--medium {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  }

  .files-grid--large {
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  }

  .files-grid__card {
    padding: 8px;
  }

  .files-selection-toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }

  .files-batch-toolbar {
    flex-wrap: wrap;
    gap: 6px;
  }

  .files-batch-toolbar .button {
    font-size: 13px;
    padding: 6px 10px;
  }
}

/* 移动端优化 (≤480px) */
@media (max-width: 480px) {
  .files-toolbar {
    --files-search-slot-width: 100%;
  }

  .files-toolbar__top,
  .files-toolbar__bottom {
    gap: 6px;
  }

  .files-breadcrumb {
    font-size: 13px;
    flex-wrap: wrap;
  }

  .files-breadcrumb__button {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .files-primary-actions__group {
    width: 100%;
  }

  .files-primary-actions__group .button {
    flex: 1 1 auto;
    min-width: 0;
    font-size: 13px;
    padding: 8px 10px;
  }

  .files-controls__view,
  .files-controls__grid-size {
    width: 100%;
  }

  .files-controls__view .button,
  .files-controls__grid-size .button {
    flex: 1;
  }

  .files-controls__search {
    width: 100%;
  }

  .files-controls__search input {
    flex: 1;
    min-width: 0;
  }

  /* 列表视图：隐藏类型和大小列 */
  .files-table {
    font-size: 13px;
  }

  .files-table th:nth-child(3),
  .files-table td:nth-child(3),
  .files-table th:nth-child(5),
  .files-table td:nth-child(5) {
    display: none;
  }

  .files-table th,
  .files-table td {
    padding: 6px 8px;
  }

  .files-table__actions {
    font-size: 12px;
  }

  .files-entry-actions__primary {
    flex-wrap: wrap;
    gap: 4px;
  }

  .files-entry-actions__primary .text-button {
    font-size: 12px;
  }

  /* 网格视图：双列布局 */
  .files-grid--small,
  .files-grid--medium {
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  }

  .files-grid--large {
    grid-template-columns: 1fr;
  }

  .files-grid__card {
    padding: 10px;
  }

  .files-grid__title {
    font-size: 14px;
  }

  /* 选择工具栏 */
  .files-selection-toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    padding: 10px;
  }

  .files-selection-toolbar__summary {
    text-align: center;
  }

  .files-batch-toolbar {
    width: 100%;
    justify-content: center;
  }

  .files-batch-toolbar .button {
    flex: 1 1 auto;
    min-width: 0;
    font-size: 12px;
    padding: 6px 8px;
  }

  /* 对话框优化 */
  .copy-dialog {
    width: 100%;
    max-width: calc(100vw - 20px);
    max-height: calc(100vh - 40px);
    overflow-y: auto;
  }

  .copy-dialog__header {
    flex-wrap: wrap;
    gap: 8px;
  }

  .copy-dialog__space-grid {
    grid-template-columns: 1fr;
  }

  .copy-dialog__toolbar {
    flex-direction: column;
    gap: 8px;
  }

  .copy-dialog__create {
    flex-direction: column;
    gap: 8px;
  }

  .copy-dialog__actions {
    flex-wrap: wrap;
    gap: 8px;
  }

  .copy-dialog__actions .button {
    flex: 1 1 auto;
  }

  /* 上传对话框 */
  .files-upload-dialog__actions {
    flex-direction: column;
  }

  .files-upload-dialog__choice {
    width: 100%;
  }
}

/* 极小屏幕优化 (≤360px) */
@media (max-width: 360px) {
  .files-breadcrumb__button {
    max-width: 80px;
  }

  .files-primary-actions__group .button {
    font-size: 12px;
    padding: 6px 8px;
  }

  .files-table {
    font-size: 12px;
  }

  .files-table th,
  .files-table td {
    padding: 4px 6px;
  }

  /* 网格视图：强制单列 */
  .files-grid--small,
  .files-grid--medium,
  .files-grid--large {
    grid-template-columns: 1fr;
  }

  .files-batch-toolbar .button {
    font-size: 11px;
    padding: 5px 6px;
  }
}

/* 屏幕阅读器专用（视觉隐藏） */
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

/* 键盘焦点增强 */
.files-table__row:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: -2px;
}

.files-grid__card:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}

button:focus-visible,
a:focus-visible,
input:focus-visible,
select:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}
</style>
