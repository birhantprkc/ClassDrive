<template>
  <aside class="sidebar">
    <div class="sidebar__brand-block">
      <img class="brand-logo" src="/logo.svg" alt="ClassDrive" data-testid="sidebar-brand-logo" />
      <div class="sidebar__brand-copy">
        <div class="sidebar__brand">ClassDrive</div>
        <p class="sidebar__brand-note">老师端文件与班级工作台</p>
      </div>
    </div>
    <nav class="sidebar__nav" data-testid="sidebar-nav" aria-label="老师工作流导航">
      <section v-for="group in shellStore.navGroups" :key="group.key" class="sidebar__group">
        <header class="sidebar__group-header">
          <span class="sidebar__group-label">{{ group.label }}</span>
        </header>
        <div class="sidebar__group-items">
          <RouterLink
            v-for="item in group.items"
            :key="item.key"
            :to="item.href"
            class="sidebar__link"
            :class="{ 'is-placeholder': item.placeholder, 'is-active': isShellItemActive(item) }"
          >
            {{ item.label }}
          </RouterLink>
        </div>
      </section>
    </nav>
    <footer class="sidebar__footer" data-testid="sidebar-footer">
      <span>Author: <a
          href="https://github.com/wuerzh/ClassDrive"
          target="_blank"
          rel="noopener noreferrer"
          data-testid="app-author-link"
        >wuerzh</a> | Ver: 1.8</span>
      <span>WX/QQ: 709868663</span>
    </footer>
  </aside>
</template>

<script setup lang="ts">
import { RouterLink, useRoute } from "vue-router";
import type { ShellItem } from "@/api/client";
import { useShellStore } from "@/stores/shell";

const shellStore = useShellStore();
const route = useRoute();

function isShellItemActive(item: ShellItem): boolean {
  const path = route.path;
  if (item.key === "assignments") {
    return path === "/assignments" || path.startsWith("/assignments/");
  }
  if (item.key === "settings") {
    return path === "/settings" || path.startsWith("/settings/");
  }
  if (item.key === "classes") {
    return path === "/classes" || path.startsWith("/classes/");
  }
  if (item.key === "classes-files") {
    return path === "/files/classes" || path.startsWith("/files/classes/");
  }
  return path === item.href || path.startsWith(`${item.href}/`);
}
</script>
