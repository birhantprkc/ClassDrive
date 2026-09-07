import { readFile } from "node:fs/promises";
import { inflateRawSync } from "node:zlib";
import { expect, test, type Page } from "@playwright/test";

function findZipEndOfCentralDirectory(buffer: Buffer): number {
  for (let index = buffer.length - 22; index >= 0; index -= 1) {
    if (buffer.readUInt32LE(index) === 0x06054b50) {
      return index;
    }
  }
  throw new Error("ZIP 中缺少中央目录结束记录");
}

async function unzipDownloadedEntries(downloadPath: string): Promise<Map<string, string>> {
  const buffer = await readFile(downloadPath);
  const endOffset = findZipEndOfCentralDirectory(buffer);
  const entryCount = buffer.readUInt16LE(endOffset + 10);
  let centralOffset = buffer.readUInt32LE(endOffset + 16);
  const entries = new Map<string, string>();

  for (let index = 0; index < entryCount; index += 1) {
    if (buffer.readUInt32LE(centralOffset) !== 0x02014b50) {
      throw new Error(`ZIP 中央目录记录非法：${centralOffset}`);
    }
    const compressionMethod = buffer.readUInt16LE(centralOffset + 10);
    const compressedSize = buffer.readUInt32LE(centralOffset + 20);
    const fileNameLength = buffer.readUInt16LE(centralOffset + 28);
    const extraLength = buffer.readUInt16LE(centralOffset + 30);
    const commentLength = buffer.readUInt16LE(centralOffset + 32);
    const localHeaderOffset = buffer.readUInt32LE(centralOffset + 42);
    const fileName = buffer.subarray(centralOffset + 46, centralOffset + 46 + fileNameLength).toString("utf8");

    const localFileNameLength = buffer.readUInt16LE(localHeaderOffset + 26);
    const localExtraLength = buffer.readUInt16LE(localHeaderOffset + 28);
    const dataOffset = localHeaderOffset + 30 + localFileNameLength + localExtraLength;
    const compressedData = buffer.subarray(dataOffset, dataOffset + compressedSize);
    const data = compressionMethod === 8 ? inflateRawSync(compressedData) : compressedData;
    entries.set(fileName, data.toString("utf8"));

    centralOffset += 46 + fileNameLength + extraLength + commentLength;
  }

  return entries;
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

async function prepareStudentFixture(
  page: Page,
  payload: { studentNo: string; displayName: string; futureTitle: string; expiredTitle: string }
) {
  const createdClass = await teacherRequest<{ id: number }>(page, "/api/classes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      name: `学生提交流程班级-${payload.studentNo}`,
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
      studentNo: payload.studentNo,
      displayName: payload.displayName,
    }),
  });

  const futureAssignment = await teacherRequest<{ id: number }>(page, "/api/assignments", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      classId: createdClass.id,
      title: payload.futureTitle,
      description: "E2E 学生提交作业",
      dueAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      status: "published",
    }),
  });

  const expiredAssignment = await teacherRequest<{ id: number }>(page, "/api/assignments", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      classId: createdClass.id,
      title: payload.expiredTitle,
      description: "E2E 已截止作业",
      dueAt: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
      status: "published",
    }),
  });

  return {
    classId: createdClass.id,
    joinCode: joinCodeResponse.joinCode,
    futureAssignmentId: futureAssignment.id,
    expiredAssignmentId: expiredAssignment.id,
  };
}

async function activateStudent(page: Page, payload: { joinCode: string; studentNo: string; password: string }) {
  await page.goto("/student/activate");
  await page.getByTestId("student-activate-join-code").fill(payload.joinCode);
  await page.getByTestId("student-activate-no").fill(payload.studentNo);
  await page.getByTestId("student-activate-password").fill(payload.password);
  await page.getByTestId("student-activate-confirm-password").fill(payload.password);
  await page.getByTestId("student-activate-submit").click();
  await expect(page).toHaveURL(/\/student\/assignments$/);
}

async function loginAsStudent(page: Page, payload: { studentNo: string; password: string }) {
  await page.goto("/student/login");
  await expect(page.getByTestId("student-login-join-code")).toHaveCount(0);
  await page.getByTestId("student-login-no").fill(payload.studentNo);
  await page.getByTestId("student-login-password").fill(payload.password);
  await page.getByTestId("student-login-submit").click();
  await expect(page).toHaveURL(/\/student\/assignments$/);
}

test("学生首登激活、登录并提交作业，老师可查看当前提交", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const studentNo = `2026${suffix.slice(-6)}`;
  const displayName = `学生-${suffix.slice(-4)}`;
  const futureTitle = `学生作业-${suffix}`;
  const expiredTitle = `已截止作业-${suffix}`;
  const password = "student123";

  const fixture = await prepareStudentFixture(page, {
    studentNo,
    displayName,
    futureTitle,
    expiredTitle,
  });

  await activateStudent(page, {
    joinCode: fixture.joinCode,
    studentNo,
    password,
  });
  await expect(page.getByText(futureTitle)).toBeVisible();
  await expect(page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`)).toBeVisible();

  await page.getByTestId("student-logout-submit").click();
  await expect(page).toHaveURL(/\/student\/login$/);

  await loginAsStudent(page, {
    studentNo,
    password,
  });
  await expect(page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`)).toBeVisible();

  await page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`).click();
  await expect(page).toHaveURL(new RegExp(`/student/assignments/${fixture.futureAssignmentId}$`));
  await expect(page.getByText("PDF、Word、Excel、PPT、TXT、HTML、Markdown、CSV、JSON、图片、音视频、压缩包")).toBeVisible();

  const firstChooserPromise = page.waitForEvent("filechooser");
  await page.getByTestId("student-submission-submit").click();
  await expect(page.getByTestId("student-submission-dialog")).toBeVisible();
  await page.getByTestId("student-submission-file-open").click();
  const firstChooser = await firstChooserPromise;
  await firstChooser.setFiles({
    name: "first.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("student first submission"),
  });
  await expect(page.getByTestId("student-submission-selection")).toContainText("first.txt");
  await page.getByTestId("student-submission-dialog-submit").click();
  await page.getByTestId("student-submission-confirm-confirm").click();
  await expect(page.getByTestId("student-assignment-current-submission")).toContainText("first.txt");
  await expect(page.getByTestId("student-submission-submit")).toContainText("继续添加");

  await page.goto("/student/assignments");
  await expect(page).toHaveURL(/\/student\/assignments$/);
  await expect(page.locator('[data-testid^="student-assignment-row-"]', { hasText: futureTitle })).toContainText("已提交");

  await page.getByTestId("student-logout-submit").click();
  await expect(page).toHaveURL(/\/student\/login$/);

  await page.goto(`/assignments/classes/${fixture.classId}/${fixture.futureAssignmentId}`);
  await expect(page.getByText("学生当前提交")).toBeVisible();
  await page.getByTestId("assignment-submission-filter").fill(displayName);
  await page.getByTestId("assignment-submission-search").click();
  await expect(page).toHaveURL(/q=/);
  await expect(page.getByTestId("assignment-submission-prev")).toBeVisible();
  await expect(page.getByTestId("assignment-submission-next")).toBeVisible();
  await expect(page.getByTestId("assignment-submission-review-mark-all")).toBeVisible();
  await expect(page.getByTestId("assignment-detail-overview")).toHaveCount(0);
  await expect(page.getByTestId("assignment-detail-status")).toBeVisible();
  const toolbarBox = await page.getByTestId("assignment-submission-toolbar").boundingBox();
  const tableBox = await page.getByTestId("assignment-submissions-table").boundingBox();
  if (!toolbarBox || !tableBox) {
    throw new Error("作业详情布局区域未渲染");
  }
  expect(toolbarBox.height).toBeLessThan(88);
  expect(tableBox.width).toBeGreaterThan(900);
  await page.getByTestId("theme-toggle").click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  const submissionRow = page.locator('[data-testid^="assignment-submission-row-"]', { hasText: displayName });
  await expect(submissionRow).toContainText(displayName);
  await expect(submissionRow).toContainText(studentNo);
  await expect(submissionRow).toContainText("first.txt");
  const detailSurface = await page.getByTestId("assignment-detail-main").evaluate((element) => {
    const toolbar = element.querySelector<HTMLElement>('[data-testid="assignment-submission-toolbar"]');
    const table = element.querySelector<HTMLElement>('[data-testid="assignment-submissions-table"]');
    const searchButton = element.querySelector<HTMLElement>('[data-testid="assignment-submission-search"]');
    const pendingBadge = element.querySelector<HTMLElement>('[data-testid^="assignment-submission-review-badge-"]');
    const openButton = element.querySelector<HTMLElement>('[data-testid^="assignment-submission-open-"]');
    const markAllButton = element.querySelector<HTMLElement>('[data-testid="assignment-submission-review-mark-all"]');
    const pendingBadgeStyle = pendingBadge ? window.getComputedStyle(pendingBadge) : null;
    const markAllButtonStyle = markAllButton ? window.getComputedStyle(markAllButton) : null;
    return {
      toolbarBackground: toolbar ? window.getComputedStyle(toolbar).backgroundColor : "",
      toolbarElButtonCount: toolbar?.querySelectorAll(".el-button").length ?? 0,
      toolbarElTagCount: toolbar?.querySelectorAll(".el-tag").length ?? 0,
      tableElButtonCount: table?.querySelectorAll(".el-button").length ?? 0,
      tableElTagCount: table?.querySelectorAll(".el-tag").length ?? 0,
      searchButtonClass: searchButton?.className ?? "",
      searchButtonBackground: searchButton ? window.getComputedStyle(searchButton).backgroundColor : "",
      pendingBadgeClass: pendingBadge?.className ?? "",
      pendingBadgeBackground: pendingBadgeStyle?.backgroundColor ?? "",
      pendingBadgeColor: pendingBadgeStyle?.color ?? "",
      openButtonClass: openButton?.className ?? "",
      openButtonBackground: openButton ? window.getComputedStyle(openButton).backgroundColor : "",
      markAllButtonBackgroundImage: markAllButtonStyle?.backgroundImage ?? "",
    };
  });
  expect(detailSurface.toolbarBackground).not.toBe("rgb(255, 255, 255)");
  expect(detailSurface.toolbarElButtonCount).toBe(0);
  expect(detailSurface.toolbarElTagCount).toBe(0);
  expect(detailSurface.tableElButtonCount).toBe(0);
  expect(detailSurface.tableElTagCount).toBe(0);
  expect(detailSurface.searchButtonClass).toContain("button");
  expect(detailSurface.searchButtonClass).not.toContain("el-button");
  expect(detailSurface.searchButtonBackground).not.toBe("rgb(236, 245, 255)");
  expect(detailSurface.pendingBadgeClass).toContain("status-pill--warning");
  expect(detailSurface.pendingBadgeClass).not.toContain("el-tag");
  expect(detailSurface.pendingBadgeBackground).not.toBe("rgb(244, 244, 245)");
  expect(detailSurface.pendingBadgeColor).toBe("rgb(253, 230, 138)");
  expect(detailSurface.openButtonClass).toContain("button--secondary");
  expect(detailSurface.openButtonClass).not.toContain("el-button");
  expect(detailSurface.openButtonBackground).not.toBe("rgb(236, 245, 255)");
  expect(detailSurface.markAllButtonBackgroundImage).not.toBe("none");
  await submissionRow.getByRole("button", { name: "查看/批改" }).click();
  await expect(page.getByTestId("assignment-submission-review-drawer")).toContainText(displayName);
  await expect(page.getByTestId("assignment-submission-file-grid")).toContainText("first.txt");
  await expect(page.getByTestId("assignment-review-drawer-prev")).toBeVisible();
  await expect(page.getByTestId("assignment-review-drawer-next")).toBeVisible();
  const drawerBox = await page.getByTestId("assignment-submission-review-drawer").boundingBox();
  if (!drawerBox) {
    throw new Error("提交详情抽屉未渲染");
  }
  expect(drawerBox.width).toBeGreaterThan(900);
  const drawerSurface = await page.getByTestId("assignment-submission-review-drawer").evaluate((element) => {
    const drawerStyle = window.getComputedStyle(element);
    const header = element.querySelector(".el-drawer__header");
    const headerStyle = header ? window.getComputedStyle(header) : null;
    const prevButton = element.querySelector<HTMLElement>('[data-testid="assignment-review-drawer-prev"]');
    const previewButton = element.querySelector<HTMLElement>('[data-testid^="assignment-submission-file-preview-"]');
    const prevButtonStyle = prevButton ? window.getComputedStyle(prevButton) : null;
    const previewButtonStyle = previewButton ? window.getComputedStyle(previewButton) : null;
    return {
      backgroundColor: drawerStyle.backgroundColor,
      headerColor: headerStyle?.color ?? "",
      prevButtonBackground: prevButtonStyle?.backgroundColor ?? "",
      previewButtonBackground: previewButtonStyle?.backgroundColor ?? "",
    };
  });
  expect(drawerSurface.backgroundColor).not.toBe("rgb(255, 255, 255)");
  expect(drawerSurface.headerColor).not.toBe("rgb(48, 49, 51)");
  expect(drawerSurface.prevButtonBackground).not.toBe("rgb(255, 255, 255)");
  expect(drawerSurface.prevButtonBackground).not.toBe("rgb(245, 247, 250)");
  expect(drawerSurface.previewButtonBackground).not.toBe("rgb(236, 245, 255)");
  const darkDrawerSurface = await page.getByTestId("assignment-submission-review-drawer").evaluate((element) => {
    const body = element.querySelector<HTMLElement>(".el-drawer__body");
    const summary = element.querySelector<HTMLElement>('[data-testid="assignment-review-drawer-summary"]');
    const reviewForm = element.querySelector<HTMLElement>(".assignment-review-drawer__form");
    const filesPanel = element.querySelector<HTMLElement>(".assignment-review-drawer__files-panel");
    const bodyStyle = body ? window.getComputedStyle(body) : null;
    const summaryStyle = summary ? window.getComputedStyle(summary) : null;
    const reviewFormStyle = reviewForm ? window.getComputedStyle(reviewForm) : null;
    const filesRect = filesPanel?.getBoundingClientRect();
    const formRect = reviewForm?.getBoundingClientRect();
    return {
      bodyBackground: bodyStyle?.backgroundColor ?? "",
      summaryBackground: summaryStyle?.backgroundColor ?? "",
      reviewFormBackground: reviewFormStyle?.backgroundColor ?? "",
      filesPanelBelowForm: filesRect && formRect ? filesRect.top >= formRect.top : false,
    };
  });
  expect(darkDrawerSurface.bodyBackground).not.toBe("rgb(248, 251, 255)");
  expect(darkDrawerSurface.summaryBackground).not.toBe("rgb(255, 255, 255)");
  expect(darkDrawerSurface.reviewFormBackground).not.toBe("rgb(255, 255, 255)");
  expect(darkDrawerSurface.filesPanelBelowForm).toBe(true);
  await page.locator('select[data-testid^="assignment-submission-review-status-"]').selectOption("reviewed");
  await page.locator('[data-testid^="assignment-submission-review-comment-"]').fill("书写清晰");
  await page
    .getByTestId("assignment-submission-review-drawer")
    .locator('button[data-testid^="assignment-submission-review-save-"]')
    .click();
  await expect(page.getByText("批改摘要已保存")).toBeVisible();
  await page.getByTestId("assignment-submission-review-close").click();
  await expect(page.getByTestId("assignment-submission-review-drawer")).toBeHidden();
  await expect(submissionRow).toContainText("已批改");
  await expect(submissionRow).toContainText("书写清晰");
  await expect(submissionRow).toContainText("示例老师");

  const archiveDownloadPromise = page.waitForEvent("download");
  await page.getByTestId("assignment-submission-download-archive").click();
  await expect(page.getByTestId("assignment-submission-download-confirm-dialog")).toContainText("确认下载提交包");
  await page.getByTestId("assignment-submission-download-confirm-confirm").click();
  const archiveDownload = await archiveDownloadPromise;
  await expect(archiveDownload.suggestedFilename()).toContain("作业提交.zip");
  const archivePath = await archiveDownload.path();
  if (!archivePath) {
    throw new Error("没有拿到作业提交包下载路径");
  }
  const archiveEntries = await unzipDownloadedEntries(archivePath);
  expect(archiveEntries.get(`${futureTitle}/${studentNo}-${displayName}/first.txt`)).toBe("student first submission");
  const manifest = archiveEntries.get("提交清单.csv") ?? "";
  expect(manifest).toContain("作业,学号,姓名,提交时间,文件数,文件路径,批改状态,教师评语");
  expect(manifest).toContain(`${futureTitle},${studentNo},${displayName},`);
  expect(manifest).toContain(",1,first.txt");
  expect(manifest).toContain(",已批改,书写清晰");
  const missingManifest = archiveEntries.get("未提交清单.csv") ?? "";
  expect(missingManifest).toContain("作业,学号,姓名,状态");
});

test("学生可在截止前覆盖提交，截止后只读", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const studentNo = `2027${suffix.slice(-6)}`;
  const displayName = `学生-${suffix.slice(-4)}`;
  const futureTitle = `覆盖作业-${suffix}`;
  const expiredTitle = `截止作业-${suffix}`;
  const password = "student456";

  const fixture = await prepareStudentFixture(page, {
    studentNo,
    displayName,
    futureTitle,
    expiredTitle,
  });

  await activateStudent(page, {
    joinCode: fixture.joinCode,
    studentNo,
    password,
  });
  await expect(page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`)).toBeVisible();

  await page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`).click();
  await expect(page).toHaveURL(new RegExp(`/student/assignments/${fixture.futureAssignmentId}$`));
  await expect(page.getByText("PDF、Word、Excel、PPT、TXT、HTML、Markdown、CSV、JSON、图片、音视频、压缩包")).toBeVisible();

  const firstChooserPromise = page.waitForEvent("filechooser");
  await page.getByTestId("student-submission-submit").click();
  await expect(page.getByTestId("student-submission-dialog")).toBeVisible();
  await page.getByTestId("student-submission-file-open").click();
  const firstChooser = await firstChooserPromise;
  await firstChooser.setFiles({
    name: "first.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("first version"),
  });
  await expect(page.getByTestId("student-submission-selection")).toContainText("first.txt");
  await page.getByTestId("student-submission-dialog-submit").click();
  await page.getByTestId("student-submission-confirm-confirm").click();
  await expect(page.getByText("first.txt")).toBeVisible();

  // 同名文件再次提交应替换旧版本
  const secondChooserPromise = page.waitForEvent("filechooser");
  await page.getByTestId("student-submission-submit").click();
  await expect(page.getByTestId("student-submission-dialog")).toBeVisible();
  await page.getByTestId("student-submission-file-open").click();
  const secondChooser = await secondChooserPromise;
  await secondChooser.setFiles({
    name: "first.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("second version"),
  });
  await expect(page.getByTestId("student-submission-selection")).toContainText("first.txt");
  await page.getByTestId("student-submission-dialog-submit").click();
  await page.getByTestId("student-submission-confirm-confirm").click();
  await expect(page.getByTestId("student-assignment-current-submission")).toContainText("first.txt");

  // 同名替换会生成新条目，轮询直到下载链接返回最新版本内容
  await expect.poll(async () => {
    const href = await page.locator('[data-testid^="student-assignment-submission-download-"]').first().getAttribute("href");
    if (!href) {
      return "";
    }
    return page.evaluate(async (url) => {
      const response = await fetch(url, { credentials: "same-origin" });
      return response.ok ? response.text() : "";
    }, href);
  }, { timeout: 10_000 }).toContain("second version");

  await page.goto("/student/assignments");
  await expect(page).toHaveURL(/\/student\/assignments$/);

  await page.getByTestId(`student-assignment-link-${fixture.expiredAssignmentId}`).click();
  await expect(page).toHaveURL(new RegExp(`/student/assignments/${fixture.expiredAssignmentId}$`));
  await expect(page.getByText("已截止，不能再提交")).toBeVisible();
  await expect(page.getByTestId("student-submission-input")).toHaveCount(0);
  await expect(page.getByTestId("student-submission-submit")).toHaveCount(0);
});

test("学生可提交 HTML 与 Markdown 并在线上预览效果", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const studentNo = `2028${suffix.slice(-6)}`;
  const displayName = `学生-${suffix.slice(-4)}`;
  const assignmentTitle = `网页作业-${suffix}`;
  const password = "student789";

  const fixture = await prepareStudentFixture(page, {
    studentNo,
    displayName,
    futureTitle: assignmentTitle,
    expiredTitle: `过期作业-${suffix}`,
  });

  await activateStudent(page, {
    joinCode: fixture.joinCode,
    studentNo,
    password,
  });
  await page.getByTestId(`student-assignment-link-${fixture.futureAssignmentId}`).click();
  await expect(page).toHaveURL(new RegExp(`/student/assignments/${fixture.futureAssignmentId}$`));

  const submitFile = async (name: string, contents: string) => {
    const chooserPromise = page.waitForEvent("filechooser");
    await page.getByTestId("student-submission-submit").click();
    await expect(page.getByTestId("student-submission-dialog")).toBeVisible();
    await page.getByTestId("student-submission-file-open").click();
    const chooser = await chooserPromise;
    await chooser.setFiles({
      name,
      mimeType: name.endsWith(".html") ? "text/html" : "text/markdown",
      buffer: Buffer.from(contents),
    });
    await expect(page.getByTestId("student-submission-selection")).toContainText(name);
    await page.getByTestId("student-submission-dialog-submit").click();
    await page.getByTestId("student-submission-confirm-confirm").click();
    await expect(page.getByTestId("student-assignment-current-submission")).toContainText(name);
  };

  await submitFile(
    "index.html",
    "<!doctype html><html><body><h1>AI 生成页面</h1><script>document.body.dataset.scripted='yes'</script></body></html>",
  );
  await submitFile("实验报告.md", "# 实验报告\n\n**AIGC 生成** 的内容");

  // 学生预览 HTML：沙箱 iframe 渲染出标题
  await page.locator('[data-testid^="student-assignment-submission-preview-"]', { hasText: "网页" }).first().click();
  await expect(page.getByTestId("file-preview-html")).toBeVisible();
  const htmlFrame = page.getByTestId("file-preview-html-frame");
  await expect(htmlFrame).toBeVisible();
  await expect(htmlFrame).toHaveAttribute("sandbox", "allow-scripts");
  const previewFrame = page.frames().find((frame) => frame.url().includes("/preview"));
  if (!previewFrame) {
    throw new Error("未找到 HTML 预览 iframe");
  }
  await expect(previewFrame.getByRole("heading", { name: "AI 生成页面" })).toBeVisible();
  await expect(previewFrame.locator("body")).toHaveAttribute("data-scripted", "yes");
  await page.getByTestId("file-preview-close").click();

  // 学生预览 Markdown：渲染为标题与加粗文本
  await page.locator('[data-testid^="student-assignment-submission-preview-"]', { hasText: "Markdown" }).first().click();
  await expect(page.getByTestId("file-preview-markdown")).toContainText("实验报告");
  await expect(page.getByTestId("file-preview-markdown").locator("h1")).toHaveText("实验报告");
  await expect(page.getByTestId("file-preview-markdown").locator("strong")).toContainText("AIGC 生成");
  await page.getByTestId("file-preview-close").click();

  // 老师端批改抽屉中同样可预览 HTML
  await page.getByTestId("student-logout-submit").click();
  await expect(page).toHaveURL(/\/student\/login$/);
  await page.goto(`/assignments/classes/${fixture.classId}/${fixture.futureAssignmentId}`);
  const submissionRow = page.locator('[data-testid^="assignment-submission-row-"]', { hasText: displayName });
  await expect(submissionRow).toContainText("index.html");
  await submissionRow.getByRole("button", { name: "查看/批改" }).click();
  await expect(page.getByTestId("assignment-submission-review-drawer")).toContainText("index.html");
  await page.locator('[data-testid^="assignment-submission-file-preview-"]', { hasText: "网页" }).first().click();
  await expect(page.getByTestId("file-preview-html")).toBeVisible();
  await expect(page.getByTestId("file-preview-html-frame")).toHaveAttribute("sandbox", "allow-scripts");
});
