import { describe, expect, it } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import StudentLayout from "@/layouts/StudentLayout.vue";
import { useStudentAuthStore } from "@/stores/student-auth";

describe("StudentLayout", () => {
  it("renders a student shell aligned with teacher navigation and workspace", async () => {
    const pinia = createPinia();
    setActivePinia(pinia);

    const studentAuthStore = useStudentAuthStore(pinia);
    studentAuthStore.user = {
      id: 9,
      classId: 1,
      studentNo: "20260001",
      displayName: "张小明",
      className: "一年级一班",
    };

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: "/student",
          component: StudentLayout,
          children: [
            {
              path: "assignments",
              component: { template: "<div>作业内容</div>" },
              meta: { title: "我的作业" },
            },
            {
              path: "assignments/:assignmentId",
              component: { template: "<div>作业详情</div>" },
              meta: { title: "作业详情" },
            },
            {
              path: "files/public",
              name: "student-files-public",
              component: { template: "<div>公共资料</div>" },
              meta: { title: "公共资料" },
            },
            {
              path: "files/class",
              name: "student-files-class",
              component: { template: "<div>班级资料</div>" },
              meta: { title: "班级资料" },
            },
          ],
        },
      ],
    });

    await router.push("/student/assignments");
    await router.isReady();

    const wrapper = mount(StudentLayout, {
      global: {
        plugins: [pinia, router],
      },
    });

    await flushPromises();

    expect(wrapper.get('[data-testid="student-shell-nav"]').exists()).toBe(true);
    expect(wrapper.get('[data-testid="student-shell-surface"]').exists()).toBe(true);
    expect(wrapper.get('[data-testid="student-sidebar-brand-logo"]').attributes("alt")).toBe("ClassDrive");
    expect(wrapper.get('[data-testid="student-topbar-context"]').text()).toContain("我的作业");
    expect(wrapper.text()).not.toContain("学生工作区");
    expect(wrapper.get(".student-shell__user").text()).toBe("20260001 · 张小明 · 一年级一班");
    expect(wrapper.find('a[href="/student/assignments"]').exists()).toBe(true);
    expect(wrapper.find('a[href="/student/files/public"]').exists()).toBe(true);
    expect(wrapper.find('a[href="/student/files/class"]').exists()).toBe(true);
    expect(wrapper.get('[data-testid="student-theme-toggle"]').exists()).toBe(true);
    const sidebarFooterLines = wrapper.get('[data-testid="student-sidebar-footer"]').findAll("span").map((line) => line.text());
    expect(sidebarFooterLines).toEqual([
      "Author: wuerzh | Ver: 1.8",
      "WX/QQ: 709868663",
    ]);
    expect(wrapper.get('[data-testid="student-sidebar-footer"]').get('[data-testid="app-author-link"]').attributes("href")).toBe(
      "https://github.com/wuerzh/ClassDrive",
    );
  });
});
