import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { nextTick } from "vue";
import { createMemoryHistory, createRouter } from "vue-router";
import App from "@/App.vue";
import ShellLayout from "@/layouts/ShellLayout.vue";
import { createAppRouter } from "@/router";
import { useAuthStore } from "@/stores/auth";
import { useClassesStore } from "@/stores/classes";
import { useSystemSettingsStore } from "@/stores/system-settings";
import { useStudentAuthStore } from "@/stores/student-auth";
import { useUploadStore } from "@/stores/upload";

function installFetchMock(responses: Array<{ ok: boolean; status: number; body: unknown }>) {
  const mock = vi.fn();
  for (const response of responses) {
    mock.mockResolvedValueOnce({
      ok: response.ok,
      status: response.status,
      json: async () => response.body,
    });
  }
  vi.stubGlobal("fetch", mock);
  return mock;
}

describe("ClassDrive app shell", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects unauthenticated users to login", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    installFetchMock([
      {
        ok: false,
        status: 401,
        body: {
          error: {
            code: "unauthorized",
            message: "未登录",
          },
        },
      },
    ]);

    const router = createAppRouter(pinia);
    await router.push("/files/library");
    await router.isReady();
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/login");
  });

  it("redirects the legacy student management route back to class management", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);
    authStore.user = {
      id: 1,
      username: "teacher",
      displayName: "示例老师",
    };
    authStore.initialized = true;

    const router = createAppRouter(pinia);
    await router.push("/students");
    await router.isReady();
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/classes");
  });


  it("switches between teacher and student login tabs on the shared root entry", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);
    authStore.initialized = true;
    const studentAuthStore = useStudentAuthStore(pinia);
    studentAuthStore.initialized = true;

    const router = createAppRouter(pinia);
    const wrapper = mount(App, {
      global: {
        plugins: [pinia, router],
      },
    });

    await router.push("/");
    await router.isReady();
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/");
    expect(wrapper.text()).toContain("老师登录");
    expect(wrapper.text()).toContain("学生登录");
    expect(wrapper.text()).not.toContain("老师和学生共用当前 IP+端口入口");
    expect(wrapper.get('[data-testid="app-brand-logo"]').attributes("alt")).toBe("ClassDrive");
    const loginFooter = wrapper.get('[data-testid="login-footer"]');
    expect(loginFooter.text()).toBe("Author: wuerzh | Ver: 1.9 | WX/QQ: 709868663");
    expect(loginFooter.get('[data-testid="app-author-link"]').attributes("href")).toBe("https://github.com/wuerzh/ClassDrive");
    expect(loginFooter.get('[data-testid="app-author-link"]').attributes("target")).toBe("_blank");
    expect(wrapper.find('[data-testid="teacher-login-username"]').exists()).toBe(true);
    expect(wrapper.get('[data-testid="teacher-login-username"]').element).toHaveProperty("value", "");
    expect(wrapper.find('[data-testid="student-login-join-code"]').exists()).toBe(false);
    expect(wrapper.get('[data-testid="teacher-login-password"]').attributes("type")).toBe("password");
    expect(wrapper.get('[data-testid="teacher-login-password"]').element).toHaveProperty("value", "");
    await wrapper.get('[data-testid="teacher-login-password-toggle"]').trigger("click");
    expect(wrapper.get('[data-testid="teacher-login-password"]').attributes("type")).toBe("text");

    await wrapper.get('[data-testid="login-tab-student"]').trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/");
    expect(wrapper.find('[data-testid="teacher-login-username"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="student-login-join-code"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="student-login-no"]').exists()).toBe(true);
    expect(wrapper.find('a[href="/student/activate"]').exists()).toBe(true);

    await wrapper.get('[data-testid="login-tab-teacher"]').trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/");
    expect(wrapper.find('[data-testid="teacher-login-username"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="student-login-join-code"]').exists()).toBe(false);
  });

  it("defaults to the student form on /student/login while keeping the unified login shell", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);
    authStore.initialized = true;
    const studentAuthStore = useStudentAuthStore(pinia);
    studentAuthStore.initialized = true;

    const router = createAppRouter(pinia);
    const wrapper = mount(App, {
      global: {
        plugins: [pinia, router],
      },
    });

    await router.push("/student/login");
    await router.isReady();
    await flushPromises();

    expect(router.currentRoute.value.fullPath).toBe("/student/login");
    expect(wrapper.text()).toContain("学生登录");
    expect(wrapper.find('[data-testid="student-login-join-code"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="student-login-no"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="teacher-login-username"]').exists()).toBe(false);
    expect(wrapper.find('a[href="/student/activate"]').exists()).toBe(true);
  });

  it("renders grouped shell navigation and uses the page view as the primary title", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);
    authStore.user = {
      id: 1,
      username: "teacher",
      displayName: "示例老师",
    };
    authStore.initialized = true;
    const classesStore = useClassesStore(pinia);
    classesStore.apply([
      { id: 1, name: "24家具1班", joinCode: "", joinCodeStatus: "inactive", joinCodeHint: "", registrationEnabled: false },
      { id: 2, name: "24家具2班", joinCode: "", joinCodeStatus: "inactive", joinCodeHint: "", registrationEnabled: false },
    ]);

    installFetchMock([
      {
        ok: true,
        status: 200,
        body: {
          settings: {
            uploadPanelEnabled: true,
          },
        },
      },
      {
        ok: true,
        status: 200,
        body: {
          user: authStore.user,
          items: [
            { key: "library", label: "老师资料", href: "/files/library", placeholder: false },
            { key: "public", label: "公共资料", href: "/files/public", placeholder: false },
            { key: "classes-files", label: "班级资料", href: "/files/classes/1", placeholder: false },
            { key: "classes", label: "班级管理", href: "/classes", placeholder: false },
            { key: "assignments", label: "作业管理", href: "/assignments", placeholder: false },
            { key: "students", label: "学生管理", href: "/students", placeholder: false },
            { key: "settings", label: "设置", href: "/settings", placeholder: false },
          ],
        },
      },
      {
        ok: true,
        status: 200,
        body: {
          space: "library",
          currentPath: "/",
          items: [],
        },
      },
    ]);

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: "/",
          component: ShellLayout,
          children: [
            {
              path: "files/library",
              component: { template: "<section>老师资料工作区</section>" },
              meta: { title: "老师资料" },
            },
            {
              path: "files/public",
              component: { template: "<section>公共资料工作区</section>" },
              meta: { title: "公共资料" },
            },
            {
              path: "files/classes/:classId",
              component: { template: "<section>班级资料工作区</section>" },
              meta: { title: "班级资料" },
            },
            {
              path: "classes",
              component: { template: "<section>班级管理工作区</section>" },
              meta: { title: "班级管理" },
            },
            {
              path: "assignments",
              component: { template: "<section>作业工作区</section>" },
              meta: { title: "作业管理" },
            },
            {
              path: "assignments/classes/:classId",
              component: { template: "<section>班级作业工作区</section>" },
              meta: { title: "作业管理" },
            },
            {
              path: "assignments/classes/:classId/:assignmentId",
              component: { template: "<section>作业批改工作区</section>" },
              meta: { title: "作业管理" },
            },
            {
              path: "students",
              component: { template: "<section>学生工作区</section>" },
              meta: { title: "学生管理" },
            },
            {
              path: "settings",
              component: { template: "<section>设置工作区</section>" },
              meta: { title: "设置" },
            },
          ],
        },
      ],
    });
    const wrapper = mount(ShellLayout, {
      global: {
        plugins: [pinia, router],
      },
    });

    await router.push("/files/library");
    await router.isReady();
    await flushPromises();

    const navigationText = wrapper.get('[data-testid="sidebar-nav"]').text();

    expect(navigationText).toContain("资料空间");
    expect(navigationText).toContain("班级工作流");
    expect(navigationText).toContain("系统与账号");
    expect(navigationText).toContain("老师资料");
    expect(navigationText).toContain("班级管理");
    expect(navigationText).not.toContain("学生管理");
    expect(wrapper.text()).toContain("示例老师");
    expect(wrapper.find(".sidebar__link.is-placeholder").exists()).toBe(false);
    expect(wrapper.get('[data-testid="sidebar-brand-logo"]').attributes("alt")).toBe("ClassDrive");
    expect(wrapper.find(".topbar h1").exists()).toBe(false);
    expect(wrapper.find(".topbar__label").exists()).toBe(false);
    expect(wrapper.get(".topbar").text()).not.toContain("ClassDrive");
    expect(wrapper.get('[data-testid="topbar-context"]').text()).toContain("老师资料");
    expect(wrapper.get('[data-testid="theme-toggle"]').exists()).toBe(true);
    const sidebarFooterLines = wrapper.get('[data-testid="sidebar-footer"]').findAll("span").map((line) => line.text());
    expect(sidebarFooterLines).toEqual([
      "Author: wuerzh | Ver: 1.9",
      "WX/QQ: 709868663",
    ]);
    expect(wrapper.get('[data-testid="sidebar-footer"]').get('[data-testid="app-author-link"]').attributes("href")).toBe(
      "https://github.com/wuerzh/ClassDrive",
    );
    expect(Array.from(new Set(wrapper.findAll(".sidebar__link").map((link) => link.text())))).toEqual([
      "老师资料",
      "公共资料",
      "班级资料",
      "班级管理",
      "作业管理",
      "设置",
    ]);

    await router.push("/files/classes/1");
    await flushPromises();

    const topbarContextHtml = wrapper.get(".topbar__context-block").html();
    expect(topbarContextHtml.indexOf('data-testid="topbar-context"')).toBeLessThan(
      topbarContextHtml.indexOf('data-testid="files-class-select"'),
    );
    expect(wrapper.get('[data-testid="topbar-context"]').text()).toContain("班级资料");
    expect((wrapper.get('[data-testid="files-class-select"]').element as HTMLSelectElement).value).toBe("1");
    await wrapper.get('[data-testid="files-class-select"]').setValue("2");
    await flushPromises();
    expect(router.currentRoute.value.fullPath).toBe("/files/classes/2");

    await router.push("/assignments/classes/3");
    await flushPromises();

    const assignmentNavLink = wrapper.findAll(".sidebar__link").find((link) => link.text() === "作业管理");
    expect(assignmentNavLink).toBeTruthy();
    expect(assignmentNavLink?.classes()).toContain("is-active");

    await router.push("/assignments/classes/3/9");
    await flushPromises();
    expect(assignmentNavLink?.classes()).toContain("is-active");

    await wrapper.get('[data-testid="theme-toggle"]').trigger("click");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("reacts to shared system settings changes without a page refresh", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);
    const authStore = useAuthStore(pinia);
    authStore.user = {
      id: 1,
      username: "teacher",
      displayName: "示例老师",
    };
    authStore.initialized = true;
    const systemSettingsStore = useSystemSettingsStore(pinia);
    const uploadStore = useUploadStore(pinia);
    uploadStore.totalBytes = 100;
    uploadStore.sentBytes = 50;
    uploadStore.items = [
      {
        id: "file-1",
        name: "上传中的资料.txt",
        totalBytes: 100,
        sentBytes: 50,
        status: "uploading",
      },
    ];

    installFetchMock([
      {
        ok: true,
        status: 200,
        body: {
          settings: {
            uploadPanelEnabled: true,
          },
        },
      },
    ]);

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/", component: { template: "<div>app shell</div>" } }],
    });
    await router.push("/");
    await router.isReady();

    const wrapper = mount(App, {
      global: {
        plugins: [pinia, router],
      },
    });

    await flushPromises();
    expect(wrapper.find('[data-testid="upload-panel"]').exists()).toBe(true);

    systemSettingsStore.apply({
      uploadPanelEnabled: false,
    });
    await nextTick();

    expect(wrapper.find('[data-testid="upload-panel"]').exists()).toBe(false);
  });
});
