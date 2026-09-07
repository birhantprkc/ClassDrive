import { expect, test, type Locator, type Page } from "@playwright/test";

async function stabilizePage(page: Page) {
  await page.setViewportSize({ width: 1440, height: 1200 });
  await page.addStyleTag({
    content: `
      *,
      *::before,
      *::after {
        animation: none !important;
        transition: none !important;
        caret-color: transparent !important;
      }
    `,
  });
}

async function loginAsTeacher(page: Page) {
  await page.goto("/login");
  await page.getByLabel("账号").fill("admin");
  await page.getByTestId("teacher-login-password").fill("demo123");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/files\/library$/);
}

async function teacherRequest<T>(page: Page, url: string, init?: RequestInit): Promise<T> {
  return page.evaluate(async ({ requestUrl, requestInit }) => {
    const csrfToken = document.cookie.match(/classdrive_csrf=([^;]+)/)?.[1] ?? "";
    const headers = new Headers(requestInit?.headers);
    if (csrfToken) {
      headers.set("X-CSRF-Token", csrfToken);
    }
    const response = await fetch(requestUrl, {
      credentials: "same-origin",
      ...requestInit,
      headers,
    });
    if (!response.ok) {
      throw new Error(`${requestUrl} -> ${response.status}: ${await response.text()}`);
    }
    return response.json();
  }, { requestUrl: url, requestInit: init });
}

async function activateStudentFixture(page: Page) {
  const studentNo = "30990001";
  const displayName = "视觉学生";
  const className = "视觉班级";
  const assignmentTitle = "视觉作业";

  const createdClass = await teacherRequest<{ id: number }>(page, "/api/classes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      name: className,
    }),
  });

  const joinCodeResponse = await teacherRequest<{ joinCode: string }>(page, `/api/classes/${createdClass.id}/join-code`, {
    method: "POST",
  });

  await teacherRequest(page, "/api/students", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      classId: createdClass.id,
      studentNo,
      displayName,
    }),
  });

  await teacherRequest(page, "/api/assignments", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      classId: createdClass.id,
      title: assignmentTitle,
      description: "用于视觉回归基线的学生作业。",
      dueAt: "2030-05-01T12:00:00.000Z",
      status: "published",
    }),
  });

  await page.goto("/student/activate");
  await page.getByTestId("student-activate-join-code").fill(joinCodeResponse.joinCode);
  await page.getByTestId("student-activate-no").fill(studentNo);
  await page.getByTestId("student-activate-password").fill("student123");
  await page.getByTestId("student-activate-confirm-password").fill("student123");
  await page.getByTestId("student-activate-submit").click();
  await expect(page).toHaveURL(/\/student\/assignments$/);
}

async function createAuditPaginationFixture(page: Page) {
  for (let index = 0; index < 10; index += 1) {
    await page.evaluate(async () => {
      await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: "admin",
          password: "wrong-password",
        }),
      });
    });
  }
}

async function expectStableScreenshot(target: Locator, name: string, masks: Locator[] = []) {
  await expect(target).toHaveScreenshot(name, {
    animations: "disabled",
    caret: "hide",
    mask: masks,
  });
}

test.describe("visual baseline", () => {
  test("teacher shell pages stay visually stable", async ({ page }) => {
    await stabilizePage(page);
    await loginAsTeacher(page);

    await expectStableScreenshot(page.locator(".shell"), "teacher-files-shell.png", [
      page.locator('[data-testid^="entry-updated-"]'),
    ]);

    await page.getByTestId("sidebar-nav").getByRole("link", { name: "班级管理", exact: true }).click();
    await expect(page).toHaveURL(/\/classes$/);
    await expectStableScreenshot(page.locator(".shell"), "teacher-classes-shell.png");

    await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
    await expect(page).toHaveURL(/\/assignments$/);
    await expectStableScreenshot(page.locator(".shell"), "teacher-assignments-shell.png");

    await expect(page.getByTestId("sidebar-nav").getByRole("link", { name: "学生管理", exact: true })).toHaveCount(0);

    await page.getByTestId("sidebar-nav").getByRole("link", { name: "设置", exact: true }).click();
    await expect(page).toHaveURL(/\/settings$/);
    await expectStableScreenshot(page.locator(".shell"), "teacher-settings-shell.png");
  });

  test("audit pagination controls stay usable on mobile width", async ({ page }) => {
    await stabilizePage(page);
    await page.setViewportSize({ width: 390, height: 900 });
    await loginAsTeacher(page);
    await createAuditPaginationFixture(page);
    await page.goto("/settings/logs");
    await expect(page.getByTestId("audit-log-pagination-summary")).toContainText("第 1 / 2 页");
    await expect(page.getByTestId("audit-log-page-first")).toBeVisible();
    await expect(page.getByTestId("audit-log-page-last")).toBeVisible();
    await expect(page.getByTestId("audit-log-page-number-2")).toBeVisible();
    await expect(page.getByTestId("audit-log-page-jump-input")).toBeVisible();
    await expectStableScreenshot(page.locator(".pagination-controls"), "mobile-audit-pagination-controls.png");
  });

  test("student shell page stays visually stable", async ({ page }) => {
    await stabilizePage(page);
    await loginAsTeacher(page);
    await activateStudentFixture(page);

    await expectStableScreenshot(page.locator(".student-shell"), "student-assignments-shell.png", [
      page.locator('[data-testid^="student-assignment-row-published-"]'),
    ]);
  });
});
