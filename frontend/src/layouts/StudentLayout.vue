<template>
  <div class="shell student-shell">
    <aside class="sidebar student-shell__sidebar">
      <div class="sidebar__brand-block">
        <img class="brand-logo" src="/logo.svg" alt="ClassDrive" data-testid="student-sidebar-brand-logo" />
        <div class="sidebar__brand-copy">
          <div class="sidebar__brand">ClassDrive</div>
          <p class="sidebar__brand-note">学生端资料与作业工作台</p>
        </div>
      </div>
      <nav class="sidebar__nav" data-testid="student-shell-nav" aria-label="学生工作流导航">
        <section class="sidebar__group">
          <header class="sidebar__group-header">
            <span class="sidebar__group-label">学习资料</span>
          </header>
          <div class="sidebar__group-items">
            <RouterLink class="sidebar__link" to="/student/files/public" data-testid="student-nav-public-files">公共资料</RouterLink>
            <RouterLink class="sidebar__link" to="/student/files/class" data-testid="student-nav-class-files">班级资料</RouterLink>
          </div>
        </section>
        <section class="sidebar__group">
          <header class="sidebar__group-header">
            <span class="sidebar__group-label">作业提交</span>
          </header>
          <div class="sidebar__group-items">
            <RouterLink class="sidebar__link" to="/student/assignments" data-testid="student-nav-assignments">我的作业</RouterLink>
          </div>
        </section>
      </nav>
      <footer class="sidebar__footer" data-testid="student-sidebar-footer">
        <span>Author: <a
            href="https://github.com/wuerzh/ClassDrive"
            target="_blank"
            rel="noopener noreferrer"
            data-testid="app-author-link"
          >wuerzh</a> | Ver: 1.8</span>
        <span>WX/QQ: 709868663</span>
      </footer>
    </aside>

    <section class="shell__content">
      <header class="topbar student-shell__topbar">
        <div class="topbar__context-block student-shell__context-block">
          <RouterLink
            v-if="topbarBackTarget"
            class="button button--ghost topbar__back-button"
            :to="topbarBackTarget"
            data-testid="student-topbar-back"
            aria-label="返回列表"
          >
            ↩
          </RouterLink>
          <div class="topbar__context" data-testid="student-topbar-context">{{ pageTitle }}</div>
        </div>
        <div class="topbar__actions student-shell__actions">
          <span class="topbar__user student-shell__user">
            {{ studentIdentity }}
          </span>
          <button
            class="button button--ghost topbar__theme-toggle"
            type="button"
            data-testid="student-theme-toggle"
            @click="toggleTheme"
          >
            {{ currentTheme === "dark" ? "浅色" : "深色" }}
          </button>
          <button class="button button--ghost" type="button" data-testid="student-logout-submit" @click="submitLogout">退出登录</button>
        </div>
      </header>
      <main class="shell__main">
        <div class="shell__workspace student-shell__workspace" data-testid="student-shell-surface">
          <RouterView />
        </div>
      </main>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useStudentAuthStore } from "@/stores/student-auth";
import { useTheme } from "@/utils/theme";

const route = useRoute();
const router = useRouter();
const studentAuthStore = useStudentAuthStore();
const { currentTheme, toggleTheme } = useTheme();
const pageTitle = computed(() => (typeof route.meta.title === "string" ? route.meta.title : "学生端"));
const topbarBackTarget = computed(() => (route.name === "student-assignment-detail" ? "/student/assignments" : ""));
const studentIdentity = computed(() => {
  const user = studentAuthStore.user;
  if (!user) {
    return "";
  }
  return `${user.studentNo} · ${user.displayName} · ${user.className}`;
});

async function submitLogout() {
  await studentAuthStore.logout();
  await router.push("/student/login");
}
</script>

<style scoped>
.student-shell__actions {
  min-width: 0;
}

.student-shell__user {
  max-width: 34rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.student-shell__workspace {
  min-width: 0;
}

@media (max-width: 1100px) {
  .student-shell {
    display: grid;
    align-content: start;
    gap: 0.55rem;
    padding: 0 0.7rem 0.7rem;
  }

  .student-shell__sidebar {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.55rem 0.7rem;
    border-radius: 16px;
    overflow: hidden;
  }

  .student-shell__sidebar .sidebar__brand-block {
    flex: 0 0 auto;
    padding: 0;
  }

  .student-shell__sidebar .sidebar__brand {
    font-size: 1.08rem;
    line-height: 1;
  }

  .student-shell__sidebar .sidebar__brand-note,
  .student-shell__sidebar .sidebar__group-header {
    display: none;
  }

  .student-shell__sidebar .sidebar__nav,
  .student-shell__sidebar .sidebar__group,
  .student-shell__sidebar .sidebar__group-items {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    min-width: 0;
  }

  .student-shell__sidebar .sidebar__nav {
    flex: 1 1 auto;
    overflow-x: auto;
    scrollbar-width: thin;
  }

  .student-shell__sidebar .sidebar__link {
    width: auto;
    padding: 0.42rem 0.62rem;
    border-radius: 10px;
    white-space: nowrap;
    font-size: 0.86rem;
  }

  .student-shell__sidebar .sidebar__link:hover {
    transform: none;
  }

  .student-shell .shell__content {
    min-width: 0;
    width: 100%;
    max-width: 100%;
    min-height: auto;
    gap: 0.55rem;
    padding: 0;
    overflow-x: hidden;
  }

  .student-shell .shell__main {
    min-width: 0;
    width: 100%;
    max-width: 100%;
    padding: 0;
  }

  .student-shell__workspace {
    width: 100%;
    max-width: 100%;
    overflow-x: hidden;
  }

  .student-shell__topbar {
    min-height: 0;
    flex-wrap: wrap;
    padding: 0.45rem 0.65rem;
    border-radius: 0 0 16px 16px;
    gap: 0.55rem;
  }

  .student-shell__context-block {
    flex: 1 1 8rem;
  }

  .student-shell__actions {
    flex: 1 1 auto;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .student-shell__user {
    flex: 1 1 13rem;
    font-size: 0.86rem;
  }

  .student-shell__actions .button {
    padding: 0.38rem 0.62rem;
    border-radius: 10px;
  }
}

@media (max-width: 960px) {
  .student-shell__topbar,
  .student-shell__actions,
  .student-shell__context-block {
    flex-direction: row;
    align-items: center;
  }

  .student-shell__user {
    max-width: none;
    white-space: nowrap;
  }
}

@media (max-width: 520px) {
  .student-shell {
    padding: 0.5rem;
  }

  .student-shell__sidebar {
    gap: 0.5rem;
  }

  .student-shell__topbar {
    align-items: flex-start;
  }

  .student-shell__context-block {
    flex: 1 1 100%;
  }

  .student-shell__actions {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: center;
    width: 100%;
    justify-content: flex-start;
  }

  .student-shell__user {
    grid-column: 1 / -1;
    max-width: 100%;
    min-width: 0;
  }
}
</style>
