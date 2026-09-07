import { expect, test, type Locator, type Page } from "@playwright/test";

async function loginAs(page: Page, username: string, password: string) {
  await page.goto("/login");
  await page.getByLabel("账号").fill(username);
  await page.getByTestId("teacher-login-password").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/files\/library$/);
  // 本套用例基于列表视图编写
  await page.getByTestId("files-view-list").click();
}

async function loginAsTeacher(page: Page) {
  await loginAs(page, "admin", "demo123");
}

async function clickOverflowAction(row: Locator, name: string) {
  await row.getByRole("button", { name: "更多" }).click();
  await row.getByRole("button", { name, exact: true }).click();
}

async function clickByTestId(page: Page, testId: string) {
  await page.getByTestId(testId).evaluate((element) => {
    (element instanceof HTMLElement ? element : null)?.click();
  });
}

async function expectDateTimePickerOpensFromClick(page: Page, testId: string) {
  const picker = page.getByTestId(testId);
  await expect(picker).toBeVisible();
  await expect(picker.locator("input")).toHaveAttribute("readonly", "");
  await expect(picker.locator("input")).toHaveAttribute("type", "text");
  await picker.click();
  await expect(page.locator(".classdrive-date-picker")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator(".classdrive-date-picker")).toBeHidden();
}

async function logoutTeacher(page: Page) {
  await page.getByRole("button", { name: "退出" }).click();
  await expect(page).toHaveURL(/\/login$/);
}

async function expectFilesSearchAtWorkspaceEnd(page: Page, path: string) {
  await page.goto(path);
  await expect(page.getByTestId("files-secondary-controls")).toBeVisible();

  const workspaceBox = await page.locator(".shell__workspace").boundingBox();
  const searchBox = await page.getByTestId("files-secondary-controls").boundingBox();
  expect(workspaceBox).not.toBeNull();
  expect(searchBox).not.toBeNull();

  if (!workspaceBox || !searchBox) {
    return;
  }

  const rightGap = Math.abs((workspaceBox.x + workspaceBox.width) - (searchBox.x + searchBox.width));
  expect(rightGap).toBeLessThanOrEqual(32);
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

async function uploadFileWithBrowserFetch(
  page: Page,
  payload: { space: string; classId?: number; parentPath: string; name: string; contents: string; conflictMode?: string }
) {
  await page.evaluate(async ({ space, classId, parentPath, name, contents, conflictMode }) => {
    const formData = new FormData();
    formData.append("space", space);
    formData.append("parentPath", parentPath);
    if (classId) {
      formData.append("classId", String(classId));
    }
    if (conflictMode) {
      formData.append("conflictMode", conflictMode);
    }
    formData.append("files", new File([contents], name, { type: "text/plain" }));
    formData.append("relativePaths", "");

    const csrfToken = document.cookie.match(/classdrive_csrf=([^;]+)/)?.[1] ?? "";
    const headers: Record<string, string> = {};
    if (csrfToken) {
      headers["X-CSRF-Token"] = csrfToken;
    }
    const response = await fetch("/api/files/upload", {
      method: "POST",
      body: formData,
      credentials: "same-origin",
      headers,
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }, payload);
}

async function activateStudentFromLogin(
  page: Page,
  payload: { joinCode: string; studentNo: string; password: string },
) {
  await page.goto("/student/activate");
  await page.getByTestId("student-activate-join-code").fill(payload.joinCode);
  await page.getByTestId("student-activate-no").fill(payload.studentNo);
  await page.getByTestId("student-activate-password").fill(payload.password);
  await page.getByTestId("student-activate-confirm-password").fill(payload.password);
  await page.getByTestId("student-activate-submit").click();
  await expect(page).toHaveURL(/\/student\/assignments$/);
}

async function uploadDirectoryWithBrowserFetch(
  page: Page,
  payload: {
    space: string;
    classId?: number;
    parentPath: string;
    conflictMode?: string;
    entries: Array<{ name: string; relativePath: string; contents: string }>;
  },
) {
  await page.evaluate(async ({ space, classId, parentPath, conflictMode, entries }) => {
    const formData = new FormData();
    formData.append("space", space);
    formData.append("parentPath", parentPath);
    if (classId) {
      formData.append("classId", String(classId));
    }
    if (conflictMode) {
      formData.append("conflictMode", conflictMode);
    }

    for (const entry of entries) {
      formData.append("files", new File([entry.contents], entry.name, { type: "text/plain" }));
      formData.append("relativePaths", entry.relativePath);
    }

    const csrfToken = document.cookie.match(/classdrive_csrf=([^;]+)/)?.[1] ?? "";
    const headers: Record<string, string> = {};
    if (csrfToken) {
      headers["X-CSRF-Token"] = csrfToken;
    }
    const response = await fetch("/api/files/upload", {
      method: "POST",
      body: formData,
      credentials: "same-origin",
      headers,
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  }, payload);
}

const zipCrcTable = (() => {
  const table = new Uint32Array(256);
  for (let index = 0; index < 256; index += 1) {
    let value = index;
    for (let bit = 0; bit < 8; bit += 1) {
      value = (value & 1) === 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
    }
    table[index] = value >>> 0;
  }
  return table;
})();

function crc32(buffer: Buffer): number {
  let value = 0xffffffff;
  for (const byte of buffer) {
    value = zipCrcTable[(value ^ byte) & 0xff] ^ (value >>> 8);
  }
  return (value ^ 0xffffffff) >>> 0;
}

function zipDateParts(date: Date) {
  const year = Math.min(Math.max(date.getFullYear(), 1980), 2107);
  return {
    time: ((date.getHours() & 0x1f) << 11) | ((date.getMinutes() & 0x3f) << 5) | ((Math.floor(date.getSeconds() / 2)) & 0x1f),
    date: (((year - 1980) & 0x7f) << 9) | (((date.getMonth() + 1) & 0xf) << 5) | (date.getDate() & 0x1f),
  };
}

function buildStoredZip(entries: Array<{ name: string; body: string }>): Buffer {
  const fileParts: Buffer[] = [];
  const centralParts: Buffer[] = [];
  let offset = 0;
  const now = zipDateParts(new Date());

  for (const entry of entries) {
    const nameBuffer = Buffer.from(entry.name, "utf8");
    const dataBuffer = Buffer.from(entry.body, "utf8");
    const checksum = crc32(dataBuffer);

    const localHeader = Buffer.alloc(30);
    localHeader.writeUInt32LE(0x04034b50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(0, 6);
    localHeader.writeUInt16LE(0, 8);
    localHeader.writeUInt16LE(now.time, 10);
    localHeader.writeUInt16LE(now.date, 12);
    localHeader.writeUInt32LE(checksum, 14);
    localHeader.writeUInt32LE(dataBuffer.length, 18);
    localHeader.writeUInt32LE(dataBuffer.length, 22);
    localHeader.writeUInt16LE(nameBuffer.length, 26);
    localHeader.writeUInt16LE(0, 28);
    fileParts.push(localHeader, nameBuffer, dataBuffer);

    const centralHeader = Buffer.alloc(46);
    centralHeader.writeUInt32LE(0x02014b50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(0, 8);
    centralHeader.writeUInt16LE(0, 10);
    centralHeader.writeUInt16LE(now.time, 12);
    centralHeader.writeUInt16LE(now.date, 14);
    centralHeader.writeUInt32LE(checksum, 16);
    centralHeader.writeUInt32LE(dataBuffer.length, 20);
    centralHeader.writeUInt32LE(dataBuffer.length, 24);
    centralHeader.writeUInt16LE(nameBuffer.length, 28);
    centralHeader.writeUInt16LE(0, 30);
    centralHeader.writeUInt16LE(0, 32);
    centralHeader.writeUInt16LE(0, 34);
    centralHeader.writeUInt16LE(0, 36);
    centralHeader.writeUInt32LE(0, 38);
    centralHeader.writeUInt32LE(offset, 42);
    centralParts.push(centralHeader, nameBuffer);

    offset += localHeader.length + nameBuffer.length + dataBuffer.length;
  }

  const centralSize = centralParts.reduce((sum, item) => sum + item.length, 0);
  const endRecord = Buffer.alloc(22);
  endRecord.writeUInt32LE(0x06054b50, 0);
  endRecord.writeUInt16LE(0, 4);
  endRecord.writeUInt16LE(0, 6);
  endRecord.writeUInt16LE(entries.length, 8);
  endRecord.writeUInt16LE(entries.length, 10);
  endRecord.writeUInt32LE(centralSize, 12);
  endRecord.writeUInt32LE(offset, 16);
  endRecord.writeUInt16LE(0, 20);

  return Buffer.concat([...fileParts, ...centralParts, endRecord]);
}

function xmlEscape(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

function spreadsheetColumnName(index: number) {
  let value = index + 1;
  let result = "";
  while (value > 0) {
    value -= 1;
    result = String.fromCharCode(65 + (value % 26)) + result;
    value = Math.floor(value / 26);
  }
  return result;
}

function buildInlineWorksheetXml(rows: string[][]) {
  const body = rows.map((row, rowIndex) => {
    const cells = row.map((cell, columnIndex) =>
      `<c r="${spreadsheetColumnName(columnIndex)}${rowIndex + 1}" t="inlineStr"><is><t>${xmlEscape(cell)}</t></is></c>`
    ).join("");
    return `<row r="${rowIndex + 1}">${cells}</row>`;
  }).join("");
  return `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>${body}</sheetData></worksheet>`;
}

function buildStudentImportWorkbook(rows: string[][]): Buffer {
  return buildStoredZip([
    {
      name: "[Content_Types].xml",
      body: `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
    },
    {
      name: "_rels/.rels",
      body: `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
    },
    {
      name: "xl/workbook.xml",
      body: `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`,
    },
    {
      name: "xl/_rels/workbook.xml.rels",
      body: `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
    },
    {
      name: "xl/worksheets/sheet1.xml",
      body: buildInlineWorksheetXml(rows),
    },
  ]);
}

function formatDateTimeLocal(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

test("老师文件工作台主路径可用", async ({ page }) => {
  await loginAsTeacher(page);
  await expect(page.getByTestId("topbar-context")).toContainText("老师资料");

  const libraryFileRow = page.locator("tr", { hasText: "教学安排.txt" });
  const downloadPromise = page.waitForEvent("download");
  await libraryFileRow.getByRole("link", { name: "下载" }).click();
  const download = await downloadPromise;
  await expect(download.suggestedFilename()).toContain("教学安排.txt");

  const libraryFolderRow = page.locator("tr", { hasText: "课程资料" });
  await clickOverflowAction(libraryFolderRow, "复制");
  await expect(page.getByTestId("copy-dialog")).toBeVisible();
  await page.getByTestId("copy-submit").click();
  await expect(page.getByText("复制成功")).toBeVisible();

  await libraryFolderRow.getByRole("button", { name: "课程资料" }).click();
  await expect(page).toHaveURL(/path=\/%E8%AF%BE%E7%A8%8B%E8%B5%84%E6%96%99/);

  await page.getByRole("link", { name: "公共资料" }).click();
  await expect(page).toHaveURL(/\/files\/public$/);
  await page.getByTestId("files-view-list").click();

  const publicFolderRow = page.locator("tr", { hasText: "课程资料" });
  await expect(publicFolderRow).toBeVisible();

  await clickOverflowAction(publicFolderRow, "删除");
  await page.getByTestId("delete-entry-confirm").click();
  await expect(page.getByText("已删除")).toBeVisible();
  await expect(publicFolderRow).toHaveCount(0);
});

test("三个资料页搜索入口固定在工作区右上角", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 900 });
  await loginAsTeacher(page);

  await expectFilesSearchAtWorkspaceEnd(page, "/files/library");
  await expectFilesSearchAtWorkspaceEnd(page, "/files/public");
  await expectFilesSearchAtWorkspaceEnd(page, "/files/classes/1");
});

test("老师可在工作台内预览文本文件", async ({ page }) => {
  await loginAsTeacher(page);

  const textFileRow = page.locator("tr", { hasText: "教学安排.txt" });
  await textFileRow.getByRole("button", { name: "预览" }).click();

  await expect(page.getByTestId("file-preview-dialog")).toBeVisible();
  await expect(page.getByTestId("file-preview-text")).toContainText("本周完成课程导入与资料整理。");
  await page.getByTestId("file-preview-close").click();
  await expect(page.getByTestId("file-preview-dialog")).toHaveCount(0);
});

test("老师可在线编辑文本文件并保存到现有预览链路", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const fileName = `editor-${suffix}.md`;
  const initialContent = "# 初稿\n第一行";
  const updatedContent = "# 修订稿\n第二行";

  await page.getByTestId("create-file-button").click();
  await page.getByTestId("create-file-input").fill(fileName);
  await page.getByTestId("create-file-confirm").click();

  await expect(page.getByTestId("file-editor-dialog")).toBeVisible();
  await expect(page.getByTestId("file-editor-title")).toHaveText(fileName);
  await expect(page.getByTestId("file-editor-textarea")).toHaveValue("");

  await page.getByTestId("file-editor-textarea").fill(initialContent);
  await page.getByTestId("file-editor-toggle-preview").click();
  await expect(page.getByTestId("file-editor-preview")).toContainText("初稿");
  await expect(page.getByTestId("file-editor-preview")).toContainText("第一行");

  await page.getByTestId("file-editor-toggle-edit").click();
  await page.getByTestId("file-editor-textarea").fill(updatedContent);

  await page.getByTestId("file-editor-close").click();
  await page.getByTestId("file-editor-unsaved-cancel").click();
  await expect(page.getByTestId("file-editor-dialog")).toBeVisible();

  await page.getByTestId("file-editor-save").click();
  await expect(page.getByText("已保存")).toBeVisible();
  await expect(page.getByTestId("file-editor-textarea")).toHaveValue(updatedContent);

  await page.getByTestId("file-editor-close").click();
  const fileRow = page.locator("tr", { hasText: fileName });
  await expect(fileRow).toBeVisible();
  await fileRow.getByRole("button", { name: "预览" }).click();
  await expect(page.getByTestId("file-preview-dialog")).toBeVisible();
  await expect(page.getByTestId("file-preview-markdown")).toContainText("修订稿");
  await expect(page.getByTestId("file-preview-markdown")).toContainText("第二行");
  await page.getByTestId("file-preview-close").click();
});

test("老师账号与系统设置具备 owner staff 权限边界", async ({ page }) => {
  await loginAsTeacher(page);

  await page.goto("/settings/system");
  await expect(page).toHaveURL(/\/settings\/system$/);
  await expect(page.getByTestId("system-access-url")).toBeVisible();
  await expect(page.getByTestId("system-port-note")).toContainText("保存后立即生效");

  const suffix = Date.now().toString();
  const username = `staff${suffix.slice(-6)}`;
  const displayName = `助教老师-${suffix.slice(-4)}`;

  const createdTeacher = await teacherRequest<{ teacher: { id: number } }>(page, "/api/teachers", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      username,
      displayName,
      password: "staff123",
      role: "staff",
    }),
  });

  await page.goto("/settings/teachers");
  await expect(page.getByTestId("teacher-role-help")).toHaveText("普通老师管教学，系统负责人管系统和账号。");
  await page.getByTestId(`teacher-detail-open-${createdTeacher.teacher.id}`).click();
  await expect(page).toHaveURL(/\/settings\/teachers$/);
  await expect(page.getByTestId("teacher-detail-dialog")).toBeVisible();
  await expect(page.getByTestId("teacher-detail-summary-panel")).toContainText(username);
  await expect(page.getByTestId("teacher-users-list-panel")).toBeVisible();
  await page.getByTestId("teacher-detail-close").click();
  await expect(page.getByTestId("teacher-detail-dialog")).toHaveCount(0);

  await logoutTeacher(page);
  await loginAs(page, username, "staff123");

  await page.goto("/settings");
  await expect(page.getByTestId("settings-nav-system")).toHaveCount(0);
  await expect(page.getByTestId("settings-nav-teachers")).toHaveCount(0);

  await page.goto("/settings/system");
  await expect(page).toHaveURL(/\/settings\/profile$/);

  await page.goto("/settings/teachers");
  await expect(page).toHaveURL(/\/settings\/profile$/);
});

test("老师上传过程可观测且可中止", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const fileName = `abort-upload-${suffix}.bin`;

  await page.evaluate(() => {
    const originalFetch = window.fetch.bind(window);
    let delayed = false;
    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      const requestUrl = typeof input === "string" ? input : input instanceof Request ? input.url : String(input);
      const requestMethod = (init?.method ?? (input instanceof Request ? input.method : "GET")).toUpperCase();
      if (!delayed && requestMethod === "PATCH" && requestUrl.includes("/api/files/upload/sessions/")) {
        delayed = true;
        await new Promise((resolve) => window.setTimeout(resolve, 1500));
      }
      return originalFetch(input, init);
    };
  });

  await page.getByTestId("upload-input").setInputFiles({
    name: fileName,
    mimeType: "application/octet-stream",
    buffer: Buffer.alloc(1024 * 1024, "a"),
  });

  await expect(page.getByTestId("upload-panel")).toBeVisible();
  await expect(page.getByTestId("upload-panel")).toContainText(fileName);
  await expect(page.getByTestId("upload-total-progress")).toBeVisible();
  await clickByTestId(page, "upload-abort");
  await expect(page.locator("tr", { hasText: fileName })).toHaveCount(0);
});

test("老师端主要页面逐页可达且班级资料可切换到空班级", async ({ page }) => {
  await loginAsTeacher(page);

  await page.getByRole("link", { name: "班级资料" }).click();
  await expect(page).toHaveURL(/\/files\/classes\/1$/);
  await expect(page.locator(".files-toolbar")).toBeVisible();
  await expect(page.getByTestId("files-class-select")).toContainText("一年级一班");

  await page.getByTestId("files-class-select").selectOption("2");
  await expect(page).toHaveURL(/\/files\/classes\/2$/);
  await expect(page.locator(".files-toolbar")).toBeVisible();
  await expect(page.getByTestId("files-class-select")).toContainText("一年级二班");
  await expect(page.getByText("当前目录暂无文件。")).toBeVisible();

  await page.getByRole("link", { name: "老师资料" }).click();
  await expect(page.locator(".files-page")).toBeVisible();

  await page.getByRole("link", { name: "公共资料" }).click();
  await expect(page.locator(".files-page")).toBeVisible();

  await page.getByRole("link", { name: "班级管理" }).click();
  await expect(page.locator(".classes-page")).toBeVisible();

  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page.locator(".classes-page")).toBeVisible();

  await expect(page.getByTestId("sidebar-nav").getByRole("link", { name: "学生管理", exact: true })).toHaveCount(0);
  await page.goto("/students");
  await expect(page).toHaveURL(/\/classes$/);
  await expect(page.locator(".classes-page")).toBeVisible();

  await page.getByRole("link", { name: "设置", exact: true }).click();
  await expect(page.locator(".settings-page")).toBeVisible();
});

test("老师可以上传、重命名并下载文件", async ({ page }) => {
  await loginAsTeacher(page);

  await page.getByTestId("upload-input").setInputFiles({
    name: "课堂记录.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("classdrive upload body"),
  });
  await expect(page.getByText("文件上传成功")).toBeVisible();

  const uploadedRow = page.locator("tr", { hasText: "课堂记录.txt" });
  await expect(uploadedRow).toBeVisible();

  await clickOverflowAction(uploadedRow, "重命名");
  await page.getByTestId("rename-entry-input").fill("课堂记录-v2.txt");
  await page.getByTestId("rename-entry-confirm").click();
  await expect(page.getByText("已重命名")).toBeVisible();

  const renamedRow = page.locator("tr", { hasText: "课堂记录-v2.txt" });
  await expect(renamedRow).toBeVisible();

  const downloadPromise = page.waitForEvent("download");
  await renamedRow.getByRole("link", { name: "下载" }).click();
  const download = await downloadPromise;
  await expect(download.suggestedFilename()).toContain("课堂记录-v2.txt");
});

test("老师可以重命名含子项的文件夹", async ({ page }) => {
  await loginAsTeacher(page);

  // 新建文件夹并上传一个子文件
  await page.getByRole("button", { name: "新建文件夹", exact: true }).click();
  await page.getByTestId("create-folder-input").fill("待重命名文件夹");
  await page.getByTestId("create-folder-confirm").click();
  await expect(page.getByText("文件夹已创建")).toBeVisible();

  const folderRow = page.locator("tr", { hasText: "待重命名文件夹" });
  await expect(folderRow).toBeVisible();
  await folderRow.getByRole("button", { name: "待重命名文件夹" }).click();
  await expect(page.getByTestId("files-context-bar")).toContainText("待重命名文件夹");

  await page.getByTestId("upload-input").setInputFiles({
    name: "子文件.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("folder child body"),
  });
  await expect(page.getByText("文件上传成功")).toBeVisible();
  await expect(page.locator("tr", { hasText: "子文件.txt" })).toBeVisible();

  // 返回上一级并重命名文件夹（回归：曾因 SQLITE_BUSY 报 500）
  await page.getByRole("button", { name: "返回上一级文件夹" }).click();
  await expect(page.getByTestId("files-context-bar")).not.toContainText("待重命名文件夹");
  const parentFolderRow = page.locator("tr", { hasText: "待重命名文件夹" });
  await expect(parentFolderRow).toBeVisible();
  await clickOverflowAction(parentFolderRow, "重命名");
  await page.getByTestId("rename-entry-input").fill("已重命名文件夹");
  await page.getByTestId("rename-entry-confirm").click();
  await expect(page.getByTestId("toast-stack").getByText("已重命名")).toBeVisible();

  // 进入重命名后的文件夹，子文件应完好且可下载
  const renamedFolderRow = page.locator("tr", { hasText: "已重命名文件夹" });
  await expect(renamedFolderRow).toBeVisible();
  await renamedFolderRow.getByRole("button", { name: "已重命名文件夹" }).click();
  await expect(page.getByTestId("files-context-bar")).toContainText("已重命名文件夹");
  const childRow = page.locator("tr", { hasText: "子文件.txt" });
  await expect(childRow).toBeVisible();
  const childDownload = page.waitForEvent("download");
  await childRow.getByRole("link", { name: "下载" }).click();
  const childFile = await childDownload;
  await expect(childFile.suggestedFilename()).toContain("子文件.txt");
});

test("老师可按冲突策略自动重命名同名上传", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const fileName = `conflict-${suffix}.txt`;

  await page.getByTestId("upload-input").setInputFiles({
    name: fileName,
    mimeType: "text/plain",
    buffer: Buffer.from("first version"),
  });
  await expect(page.getByText("文件上传成功")).toBeVisible();

  await page.getByTestId("upload-material-open").click();
  await page.getByTestId("upload-conflict-mode").selectOption("rename");
  await page.getByTestId("upload-input").setInputFiles({
    name: fileName,
    mimeType: "text/plain",
    buffer: Buffer.from("second version"),
  });
  await expect(page.getByText("重命名 1 项")).toBeVisible();

  await expect(page.locator("tr", { hasText: fileName })).toBeVisible();
  await expect(page.locator("tr", { hasText: `conflict-${suffix} (1).txt` })).toBeVisible();
});

test("老师上传目录后可看到保留的目录结构", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const rootFolder = `dir-upload-${suffix}`;
  const nestedFolder = `作业-${suffix}`;
  const rootFile = `guide-${suffix}.txt`;
  const nestedFile = `answer-${suffix}.txt`;

  await uploadDirectoryWithBrowserFetch(page, {
    space: "library",
    parentPath: "/",
    entries: [
      {
        name: rootFile,
        relativePath: `${rootFolder}/${rootFile}`,
        contents: "guide body",
      },
      {
        name: nestedFile,
        relativePath: `${rootFolder}/${nestedFolder}/${nestedFile}`,
        contents: "answer body",
      },
    ],
  });

  await page.goto("/files/library?view=list");
  const rootFolderRow = page.locator("tr", { hasText: rootFolder });
  await expect(rootFolderRow).toBeVisible();
  await rootFolderRow.getByRole("button", { name: rootFolder }).click();

  await expect(page.locator("tr", { hasText: rootFile })).toBeVisible();
  const nestedFolderRow = page.locator("tr", { hasText: nestedFolder });
  await expect(nestedFolderRow).toBeVisible();
  await nestedFolderRow.getByRole("button", { name: nestedFolder }).click();
  await expect(page.locator("tr", { hasText: nestedFile })).toBeVisible();
});

test("老师可拖拽目录上传并保留目录结构", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const rootFolder = `drag-dir-${suffix}`;
  const nestedFolder = `资料-${suffix}`;
  const rootFile = `guide-${suffix}.txt`;
  const nestedFile = `answer-${suffix}.txt`;

  await page.evaluate(
    ({ rootFolder, nestedFolder, rootFile, nestedFile }) => {
      const target = document.querySelector('[data-testid="files-dropzone"]');
      if (!(target instanceof HTMLElement)) {
        throw new Error("drop target not found");
      }

      const guideFile = new File(["guide body"], rootFile, { type: "text/plain" });
      const answerFile = new File(["answer body"], nestedFile, { type: "text/plain" });
      const nestedDirectory = {
        isFile: false,
        isDirectory: true,
        name: nestedFolder,
        createReader() {
          let done = false;
          return {
            readEntries(callback: (entries: unknown[]) => void) {
              if (done) {
                callback([]);
                return;
              }
              done = true;
              callback([
                {
                  isFile: true,
                  isDirectory: false,
                  name: nestedFile,
                  file(fileCallback: (file: File) => void) {
                    fileCallback(answerFile);
                  },
                },
              ]);
            },
          };
        },
      };

      const rootDirectory = {
        isFile: false,
        isDirectory: true,
        name: rootFolder,
        createReader() {
          let done = false;
          return {
            readEntries(callback: (entries: unknown[]) => void) {
              if (done) {
                callback([]);
                return;
              }
              done = true;
              callback([
                {
                  isFile: true,
                  isDirectory: false,
                  name: rootFile,
                  file(fileCallback: (file: File) => void) {
                    fileCallback(guideFile);
                  },
                },
                nestedDirectory,
              ]);
            },
          };
        },
      };

      const dataTransfer = {
        items: [
          {
            webkitGetAsEntry: () => rootDirectory,
          },
        ],
        files: [],
      };

      const dragEnterEvent = new Event("dragenter", { bubbles: true, cancelable: true });
      Object.defineProperty(dragEnterEvent, "dataTransfer", {
        value: dataTransfer,
      });
      target.dispatchEvent(dragEnterEvent);
      (window as typeof window & { __dragDropTransfer?: unknown }).__dragDropTransfer = dataTransfer;
    },
    { rootFolder, nestedFolder, rootFile, nestedFile },
  );
  await expect(page.getByTestId("files-drop-hint")).toBeVisible();
  await page.evaluate(() => {
    const target = document.querySelector('[data-testid="files-dropzone"]');
    const dataTransfer = (window as typeof window & { __dragDropTransfer?: unknown }).__dragDropTransfer;
    if (!(target instanceof HTMLElement) || !dataTransfer) {
      throw new Error("drop target or dataTransfer missing");
    }
    const dropEvent = new Event("drop", { bubbles: true, cancelable: true });
    Object.defineProperty(dropEvent, "dataTransfer", {
      value: dataTransfer,
    });
    target.dispatchEvent(dropEvent);
  });
  await expect(page.getByText("文件夹上传成功")).toBeVisible();

  const rootFolderRow = page.locator("tr", { hasText: rootFolder });
  await expect(rootFolderRow).toBeVisible();
  await rootFolderRow.getByRole("button", { name: rootFolder }).click();
  await expect(page.locator("tr", { hasText: rootFile })).toBeVisible();

  const nestedFolderRow = page.locator("tr", { hasText: nestedFolder });
  await expect(nestedFolderRow).toBeVisible();
  await nestedFolderRow.getByRole("button", { name: nestedFolder }).click();
  await expect(page.locator("tr", { hasText: nestedFile })).toBeVisible();
});

test("老师可以复制到指定班级子目录", async ({ page }) => {
  await loginAsTeacher(page);

  const sourceRow = page.locator("tr", { hasText: "教学安排.txt" });
  await clickOverflowAction(sourceRow, "复制");
  await expect(page.getByTestId("copy-dialog")).toBeVisible();

  await page.getByTestId("copy-space-select").selectOption("class");
  await page.getByTestId("copy-class-select").selectOption("1");
  await page.getByTestId("copy-create-folder-input").fill("课件归档");
  await page.getByTestId("copy-create-folder-submit").click();
  await clickByTestId(page, "copy-submit");
  await expect(page.getByText("复制成功")).toBeVisible();

  await page.getByRole("link", { name: "班级资料" }).click();
  await expect(page).toHaveURL(/\/files\/classes\/1$/);
  await page.getByTestId("files-view-list").click();

  const targetFolderRow = page.locator("tr", { hasText: "课件归档" });
  await expect(targetFolderRow).toBeVisible();
  await targetFolderRow.getByRole("button", { name: "课件归档" }).click();

  await expect(page).toHaveURL(/path=\/%E8%AF%BE%E4%BB%B6%E5%BD%92%E6%A1%A3/);
  await expect(page.locator("tr", { hasText: "教学安排.txt" })).toBeVisible();
});

test("老师可批量移动文件并下载目录压缩包", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const folderName = `batch-dir-${suffix}`;
  const fileName = `batch-file-${suffix}.txt`;
  const targetFolderName = `batch-target-${suffix}`;

  await page.getByRole("button", { name: "新建文件夹" }).click();
  await page.getByTestId("create-folder-input").fill(folderName);
  await page.getByTestId("create-folder-confirm").click();
  await expect(page.getByText("文件夹已创建")).toBeVisible();

  await page.getByTestId("upload-input").setInputFiles({
    name: fileName,
    mimeType: "text/plain",
    buffer: Buffer.from("batch move file body"),
  });
  await expect(page.getByText("文件上传成功")).toBeVisible();

  const fileRow = page.locator("tr", { hasText: fileName });
  const folderRow = page.locator("tr", { hasText: folderName });
  await expect(fileRow).toBeVisible();
  await expect(folderRow).toBeVisible();

  await fileRow.locator('input[type="checkbox"]').check();
  await folderRow.locator('input[type="checkbox"]').check();
  await expect(page.getByTestId("files-selection-toolbar-summary")).toContainText("已选 2 项");

  await page.getByTestId("batch-action-move").click();
  await expect(page.getByTestId("copy-dialog")).toBeVisible();
  await expect(page.getByText("移动目标")).toBeVisible();
  await page.getByTestId("copy-create-folder-input").fill(targetFolderName);
  await page.getByTestId("copy-create-folder-submit").click();
  await clickByTestId(page, "copy-submit");
  await expect(page.getByText("移动成功")).toBeVisible();

  await expect(page.locator("tr", { hasText: fileName })).toHaveCount(0);
  await expect(page.locator("tr", { hasText: folderName })).toHaveCount(0);

  await page.getByRole("link", { name: "公共资料" }).click();
  await page.getByTestId("files-view-list").click();
  const targetFolderRow = page.locator("tr", { hasText: targetFolderName });
  await expect(targetFolderRow).toBeVisible();
  await targetFolderRow.getByRole("button", { name: targetFolderName }).click();

  await expect(page.locator("tr", { hasText: fileName })).toBeVisible();
  const movedFolderRow = page.locator("tr", { hasText: folderName });
  await expect(movedFolderRow).toBeVisible();

  const downloadPromise = page.waitForEvent("download");
  await movedFolderRow.getByRole("link", { name: "下载压缩包" }).click();
  const download = await downloadPromise;
  await expect(download.suggestedFilename()).toContain(`${folderName}.zip`);
});

test("老师可搜索文件、切换排序并切到网格视图，并保留当前状态", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const smallFileName = `search-sort-small-${suffix}.txt`;
  const largeFileName = `search-sort-large-${suffix}.txt`;

  await page.getByTestId("upload-input").setInputFiles([
    {
      name: smallFileName,
      mimeType: "text/plain",
      buffer: Buffer.from("small"),
    },
    {
      name: largeFileName,
      mimeType: "text/plain",
      buffer: Buffer.from("this file is definitely larger than the small one"),
    },
  ]);
  await expect(page.getByText("文件上传成功")).toBeVisible();
  await expect(page.locator("tr", { hasText: smallFileName })).toBeVisible();
  await expect(page.locator("tr", { hasText: largeFileName })).toBeVisible();

  await page.getByTestId("files-search-input").fill("教学安排");
  await page.getByTestId("files-search-submit").click();
  await expect(page.locator("tr", { hasText: "教学安排.txt" })).toBeVisible();

  await page.getByTestId("files-search-clear").click();
  await expect(page.locator("tr", { hasText: smallFileName })).toBeVisible();
  await page.getByTestId("files-sort-size").click();
  const sortedRows = await page.locator("tbody tr").evaluateAll((rows) =>
    rows
      .map((row) => row.textContent?.trim() ?? "")
      .filter((text) => text.includes("search-sort-")),
  );
  expect(sortedRows.slice(0, 2)[0]).toContain(largeFileName);
  expect(sortedRows.slice(0, 2)[1]).toContain(smallFileName);

  await page.getByTestId("files-view-grid").click();
  await expect(page).toHaveURL(/sort=size-desc/);
  await expect(page.getByTestId("files-grid")).toBeVisible();
  await expect(page.getByTestId("files-grid")).toContainText(largeFileName);

  await page.getByTestId("files-page-size-select").selectOption("1");
  await expect(page).toHaveURL(/pageSize=1/);
  await expect(page).not.toHaveURL(/page=2/);
  await expect(page.getByTestId("files-pagination-summary")).toContainText("第 1 /");
  await page.getByTestId("files-page-next").click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page).toHaveURL(/pageSize=1/);
  await expect(page).toHaveURL(/sort=size-desc/);

  await page.reload();
  await expect(page.getByTestId("files-grid")).toBeVisible();
  await expect(page).toHaveURL(/sort=size-desc/);
  await expect(page.getByTestId("files-page-size-select")).toHaveValue("1");
  await expect(page.getByTestId("files-pagination-summary")).toContainText("第 2 /");
});

test("老师可在班级学生抽屉使用服务端搜索分页", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const className = `学生分页班级-${suffix}`;
  const createdClass = await teacherRequest<{ id: number }>(page, "/api/classes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      name: className,
    }),
  });

  const matchingStudents = Array.from({ length: 31 }, (_, index) => {
    const order = String(index + 1).padStart(2, "0");
    return {
      studentNo: `41${suffix.slice(-4)}${order}`,
      displayName: `筛选学生-${order}-${suffix.slice(-2)}`,
    };
  });
  const unrelatedStudent = { studentNo: `41${suffix.slice(-4)}9`, displayName: `无关学生-${suffix.slice(-2)}` };

  for (const student of [...matchingStudents, unrelatedStudent]) {
    await teacherRequest(page, "/api/students", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        classId: createdClass.id,
        studentNo: student.studentNo,
        displayName: student.displayName,
      }),
    });
  }

  await page.goto("/classes");
  const classRow = page.locator('tr[data-testid^="class-row-"]', { hasText: className });
  await classRow.locator('[data-testid^="class-students-"]').click();
  await expect(page.getByTestId("class-students-drawer")).toBeVisible();
  await expect(page.getByTestId("class-students-drawer")).toContainText(className);
  await expect(page.getByTestId("students-class-select")).toHaveCount(0);

  await page.getByTestId("student-search-input").fill("筛选学生");
  await page.getByTestId("student-sort-name").click();
  await expect(page.getByTestId("student-pagination-summary")).toContainText("第 1 / 2 页");
  await page.getByTestId("student-page-next").click();

  await expect(page.getByTestId("student-pagination-summary")).toContainText("第 2 / 2 页");
  await expect(page.getByTestId("class-students-drawer")).toContainText(matchingStudents[30].displayName);
});

test("老师可在作业页使用服务端搜索分页，并在刷新后保留状态", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const createdClass = await teacherRequest<{ id: number }>(page, "/api/classes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      name: `作业分页班级-${suffix}`,
    }),
  });

  const matchingAssignments = Array.from({ length: 31 }, (_, index) => {
    const order = String(index + 1).padStart(2, "0");
    return {
      title: `筛选作业-${order}-${suffix.slice(-2)}`,
      description: "用于作业分页状态回归。",
      dueAt: new Date(Date.parse("2030-05-01T00:00:00.000Z") + index * 24 * 60 * 60 * 1000).toISOString(),
    };
  });

  for (const assignment of [
    ...matchingAssignments,
    {
      title: `无关作业-${suffix.slice(-2)}`,
      description: "不会命中搜索。",
      dueAt: "2030-05-01T00:00:00.000Z",
    },
  ]) {
    await teacherRequest(page, "/api/assignments", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        classId: createdClass.id,
        title: assignment.title,
        description: assignment.description,
        dueAt: assignment.dueAt,
        status: "published",
      }),
    });
  }

  await page.goto(`/assignments/classes/${createdClass.id}`);
  await page.getByTestId("assignment-search-input").fill("筛选作业");
  await page.getByTestId("assignment-sort-due").click();
  await expect(page.getByTestId("assignment-pagination-summary")).toContainText("第 1 / 2 页");
  await page.getByTestId("assignment-page-next").click();

  await expect(page).toHaveURL(/q=%E7%AD%9B%E9%80%89%E4%BD%9C%E4%B8%9A/);
  await expect(page).toHaveURL(/sort=dueAt-asc/);
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByTestId("assignment-pagination-summary")).toContainText("第 2 / 2 页");
  await expect(page.locator('[data-testid^="assignment-row-"]')).toContainText(matchingAssignments[30].title);

  await page.reload();
  await expect(page.getByTestId("assignment-search-input")).toHaveValue("筛选作业");
  await expect(page.getByTestId("assignment-sort-due")).toHaveClass(/is-active/);
  await expect(page.getByTestId("assignment-page-size-select")).toHaveValue("30");
  await expect(page.getByTestId("assignment-pagination-summary")).toContainText("第 2 / 2 页");
  await expect(page.locator('[data-testid^="assignment-row-"]')).toContainText(matchingAssignments[30].title);
});

test("老师可以在班级管理页创建班级并生成注册码", async ({ page }) => {
  await page.goto("/login");

  await page.getByLabel("账号").fill("admin");
  await page.getByTestId("teacher-login-password").fill("demo123");
  await page.getByRole("button", { name: "登录" }).click();

  await page.getByRole("link", { name: "班级管理" }).click();
  await expect(page).toHaveURL(/\/classes$/);

  await page.getByTestId("class-create-open").click();
  await page.getByTestId("class-create-input").fill("四年级一班");
  await page.getByTestId("class-create-confirm").click();
  await expect(page.getByText("班级已创建")).toBeVisible();

  const createdRow = page.locator('tr[data-testid^="class-row-"]', { hasText: "四年级一班" });
  await expect(createdRow).toBeVisible();

  await createdRow.locator('[data-testid^="class-registration-toggle-"]').click();
  await expect(page.getByText("已开放注册")).toBeVisible();
  await expect(createdRow.locator('[data-testid^="class-row-status-"]')).toContainText(/\d{4}/);
});

test("老师可在班级资料新建目录上传并复制文件到公共资料", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const folderName = `smoke-folder-${suffix}`;
  const fileName = `smoke-file-${suffix}.txt`;

  await page.getByRole("link", { name: "班级资料" }).click();
  await expect(page).toHaveURL(/\/files\/classes\/1$/);
  await page.getByTestId("files-view-list").click();

  await page.getByRole("button", { name: "新建文件夹" }).click();
  await page.getByTestId("create-folder-input").fill(folderName);
  await page.getByTestId("create-folder-confirm").click();
  await expect(page.getByText("文件夹已创建")).toBeVisible();

  const classFolderRow = page.locator("tr", { hasText: folderName });
  await expect(classFolderRow).toBeVisible();
  await classFolderRow.getByRole("button", { name: folderName }).click();
  await expect(page).toHaveURL(new RegExp(`path=/${encodeURIComponent(folderName)}`));

  await uploadFileWithBrowserFetch(page, {
    space: "class",
    classId: 1,
    parentPath: `/${folderName}`,
    name: fileName,
    contents: "class space smoke upload",
  });
  await page.goto(`/files/classes/1?path=/${encodeURIComponent(folderName)}&view=list`);
  await expect(page).toHaveURL(new RegExp(`path=/${encodeURIComponent(folderName)}`));

  const uploadedRow = page.locator("tr", { hasText: fileName });
  await expect(uploadedRow).toBeVisible({ timeout: 10_000 });
  await clickOverflowAction(uploadedRow, "复制");

  await expect(page.getByTestId("copy-dialog")).toBeVisible();
  await page.getByTestId("copy-space-select").selectOption("public");
  await clickByTestId(page, "copy-submit");
  await expect(page.getByText("复制成功")).toBeVisible();

  await page.getByRole("link", { name: "公共资料" }).click();
  await expect(page).toHaveURL(/\/files\/public$/);
  await page.getByTestId("files-view-list").click();
  await expect(page.locator("tr", { hasText: fileName })).toBeVisible();
});

test("老师创建班级后上传文件且注册码状态保持一致", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const className = `五年级一班-${suffix}`;
  const fileName = `class-proof-${suffix}.txt`;

  await page.getByRole("link", { name: "班级管理" }).click();
  await page.getByTestId("class-create-open").click();
  await page.getByTestId("class-create-input").fill(className);
  await page.getByTestId("class-create-confirm").click();
  await expect(page.getByText("班级已创建")).toBeVisible();

  const classRow = page.locator('tr[data-testid^="class-row-"]', { hasText: className });
  await expect(classRow).toBeVisible();
  await classRow.locator('[data-testid^="class-registration-toggle-"]').click();
  await expect(page.getByText("已开放注册")).toBeVisible();

  const codeText = await classRow.locator('[data-testid^="class-row-status-"]').innerText();
  await expect(codeText).toMatch(/^\d{4}$/);

  const classesResponse = await teacherRequest<{ classes: Array<{ id: number; name: string }> }>(page, "/api/classes");
  const createdClass = classesResponse.classes.find((item) => item.name === className);
  expect(createdClass).toBeDefined();
  await page.goto(`/files/classes/${createdClass!.id}?view=list`);
  await expect(page).toHaveURL(/\/files\/classes\/\d+(\?.*)?$/);

  const classId = createdClass!.id;
  await uploadFileWithBrowserFetch(page, {
    space: "class",
    classId,
    parentPath: "/",
    name: fileName,
    contents: "class join code consistency",
  });
  await page.goto(`/files/classes/${classId}?view=list`);
  await expect(page.locator("tr", { hasText: fileName })).toBeVisible();

  await page.getByRole("link", { name: "班级管理" }).click();
  await expect(page).toHaveURL(/\/classes$/);
  await expect(page.locator('tr[data-testid^="class-row-"]', { hasText: className })).toContainText(codeText);
});

test("老师可通过 Excel 导入学生并进入作业管理入口", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const studentNo = `2026${suffix.slice(-4)}`;
  const studentName = `导入学生-${suffix.slice(-4)}`;

  await page.getByRole("link", { name: "班级管理" }).click();
  await expect(page).toHaveURL(/\/classes$/);

  const firstClassRow = page.locator('tr[data-testid^="class-row-"]').first();
  await firstClassRow.locator('[data-testid^="class-students-"]').click();
  await expect(page.getByTestId("class-students-drawer")).toBeVisible();

  await page.getByTestId("student-import-open").click();
  await expect(page.getByTestId("student-template-xlsx")).toBeVisible();
  await expect(page.getByTestId("student-template-csv")).toHaveCount(0);
  await expect(page.getByTestId("student-import-input")).toHaveCount(0);
  await page.getByTestId("student-import-file-input").setInputFiles({
    name: "students.xlsx",
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    buffer: buildStudentImportWorkbook([
      ["studentNo", "displayName"],
      [studentNo, studentName],
    ]),
  });
  await page.getByTestId("student-import-file-submit").click();
  await expect(page.getByText("学生已批量导入")).toBeVisible();
  await expect(page.getByTestId("class-students-drawer").locator('tr[data-testid^="student-row-"]', { hasText: studentName })).toBeVisible();

  await page.getByTestId("class-students-drawer-close").click();
  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page).toHaveURL(/\/assignments$/);
  await expect(page.getByTestId("assignments-table")).toBeVisible();
  await expect(page.getByTestId("assignment-sort-title")).toContainText("作业列表");
});

test("老师可在作业管理页新建作业并看到列表更新", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const assignmentTitle = `作业-${suffix}`;

  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page).toHaveURL(/\/assignments$/);

  await page.getByTestId("assignment-create-open").click();
  await page.getByTestId("assignment-create-title").fill(assignmentTitle);
  await page.getByTestId("assignment-create-description").fill("完成课后练习并提交。");
  await expectDateTimePickerOpensFromClick(page, "assignment-create-due-at");
  await page.getByTestId("assignment-create-status").selectOption("published");
  await page.getByTestId("assignment-create-submit").click();

  await expect(page.getByText("作业已创建")).toBeVisible();
  await expect(page.locator('[data-testid^="assignment-row-"]', { hasText: assignmentTitle })).toBeVisible();
});

test("作业管理列表操作按钮同行展示且不挤出行内区域", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 820 });
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const assignmentTitle = `操作列作业-${suffix}`;

  await teacherRequest(page, "/api/assignments", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      classId: 1,
      title: assignmentTitle,
      description: "用于校验作业列表操作列按钮排布。",
      dueAt: "2030-05-01T12:00:00.000Z",
      status: "draft",
    }),
  });

  await page.goto("/assignments");
  const assignmentRow = page.locator('[data-testid^="assignment-row-"]', { hasText: assignmentTitle });
  await expect(assignmentRow).toBeVisible();

  const actionButtons = assignmentRow.locator(".assignments-page__row-actions .button");
  await expect(actionButtons).toHaveCount(3);
  const buttonBoxes = await actionButtons.evaluateAll((elements) =>
    elements.map((element) => {
      const box = element.getBoundingClientRect();
      return {
        top: Math.round(box.top),
        left: Math.round(box.left),
        right: Math.round(box.right),
      };
    }),
  );
  expect(Math.max(...buttonBoxes.map((box) => box.top)) - Math.min(...buttonBoxes.map((box) => box.top))).toBeLessThanOrEqual(3);

  const rowBox = await assignmentRow.boundingBox();
  const actionBox = await assignmentRow.locator(".assignments-page__row-actions").boundingBox();
  expect(rowBox).not.toBeNull();
  expect(actionBox).not.toBeNull();
  if (rowBox && actionBox) {
    expect(actionBox.x + actionBox.width).toBeLessThanOrEqual(rowBox.x + rowBox.width + 1);
  }
});

test("老师可进入作业详情页查看附件入口", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const assignmentTitle = `详情作业-${suffix}`;
  const assignmentDescription = "完成详情页冒烟检查。";

  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page).toHaveURL(/\/assignments$/);

  await page.getByTestId("assignment-create-open").click();
  await page.getByTestId("assignment-create-title").fill(assignmentTitle);
  await page.getByTestId("assignment-create-description").fill(assignmentDescription);
  await expectDateTimePickerOpensFromClick(page, "assignment-create-due-at");
  await page.getByTestId("assignment-create-status").selectOption("published");
  await page.getByTestId("assignment-create-submit").click();

  const assignmentRow = page.locator('[data-testid^="assignment-row-"]', { hasText: assignmentTitle });
  await expect(assignmentRow).toBeVisible();
  await assignmentRow.getByRole("link", { name: "详情/批改" }).click();

  await expect(page).toHaveURL(/\/assignments\/classes\/1\/\d+$/);
  await expect(page.getByTestId("assignment-detail-title")).toHaveText(assignmentTitle);
  await expect(page.getByText(assignmentDescription)).toBeVisible();
  await expect(page.getByTestId("assignment-detail-overview")).toHaveCount(0);
  await expect(page.getByTestId("assignment-detail-status")).toContainText("已发布");
  await expect(page.getByTestId("assignment-detail-class")).toContainText("一年级一班");
});

test("学生端左侧工作区可查看公共资料与班级资料", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const studentNo = `2030${suffix.slice(-6)}`;
  const password = "student789";
  const publicFileName = `公共资料-${suffix}.txt`;
  const classFileName = `班级资料-${suffix}.txt`;

  const createdClass = await teacherRequest<{ id: number }>(page, "/api/classes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      name: `学生资料班级-${suffix.slice(-4)}`,
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
      displayName: `资料学生-${suffix.slice(-4)}`,
    }),
  });
  await uploadFileWithBrowserFetch(page, {
    space: "public",
    parentPath: "/",
    name: publicFileName,
    contents: "公共资料正文",
  });
  await uploadFileWithBrowserFetch(page, {
    space: "class",
    classId: createdClass.id,
    parentPath: "/",
    name: classFileName,
    contents: "班级资料正文",
  });

  await activateStudentFromLogin(page, {
    joinCode: joinCodeResponse.joinCode,
    studentNo,
    password,
  });

  await expect(page.getByTestId("student-shell-nav")).toContainText("我的作业");
  await expect(page.getByTestId("student-shell-nav")).toContainText("公共资料");
  await expect(page.getByTestId("student-shell-nav")).toContainText("班级资料");

  await page.getByTestId("student-nav-public-files").click();
  await expect(page).toHaveURL(/\/student\/files\/public$/);
  await page.getByTestId("student-files-view-list").click();
  await expect(page.getByTestId("student-files-table")).toContainText(publicFileName);
  await page.locator("tr", { hasText: publicFileName }).getByRole("button", { name: "预览" }).click();
  await expect(page.getByTestId("file-preview-dialog")).toContainText(publicFileName);
  await expect(page.getByTestId("file-preview-text")).toContainText("公共资料正文");
  await page.getByTestId("file-preview-close").click();
  await expect(page.getByTestId("upload-material-open")).toHaveCount(0);
  await expect(page.getByTestId("create-file-button")).toHaveCount(0);

  await page.getByTestId("student-nav-class-files").click();
  await expect(page).toHaveURL(/\/student\/files\/class$/);
  await page.getByTestId("student-files-view-list").click();
  await expect(page.getByTestId("student-files-table")).toContainText(classFileName);
  await page.locator("tr", { hasText: classFileName }).getByRole("button", { name: "预览" }).click();
  await expect(page.getByTestId("file-preview-dialog")).toContainText(classFileName);
  await expect(page.getByTestId("file-preview-text")).toContainText("班级资料正文");
  await page.getByTestId("file-preview-close").click();
  await expect(page.getByTestId("upload-material-open")).toHaveCount(0);
  await expect(page.getByTestId("create-file-button")).toHaveCount(0);
});

test("老师可在作业详情页编辑并删除作业", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const assignmentTitle = `编辑作业-${suffix}`;
  const updatedTitle = `${assignmentTitle}-修订`;

  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page).toHaveURL(/\/assignments$/);

  await page.getByTestId("assignment-create-open").click();
  await page.getByTestId("assignment-create-title").fill(assignmentTitle);
  await page.getByTestId("assignment-create-description").fill("初始作业说明。");
  await expectDateTimePickerOpensFromClick(page, "assignment-create-due-at");
  await page.getByTestId("assignment-create-submit").click();
  await expect(page.getByText("作业已创建")).toBeVisible();

  const assignmentRow = page.locator('[data-testid^="assignment-row-"]', { hasText: assignmentTitle });
  await expect(assignmentRow).toBeVisible();
  await assignmentRow.getByRole("link", { name: "详情/批改" }).click();

  await expect(page).toHaveURL(/\/assignments\/classes\/1\/\d+$/);
  await page.getByTestId("assignment-edit-open").click();
  await expect(page.getByTestId("assignment-edit-dialog")).toBeVisible();
  await page.getByTestId("assignment-edit-title").fill(updatedTitle);
  await page.getByTestId("assignment-edit-description").fill("更新后的作业说明。");
  await expectDateTimePickerOpensFromClick(page, "assignment-edit-due-at");
  await page.getByTestId("assignment-edit-status").selectOption("published");
  await page.getByTestId("assignment-save-submit").click();

  await expect(page.getByText("作业已更新")).toBeVisible();
  await expect(page.getByTestId("assignment-detail-title")).toHaveText(updatedTitle);
  await expect(page.getByText("更新后的作业说明。")).toBeVisible();
  await expect(page.getByTestId("assignment-detail-status")).toContainText("已发布");

  await page.getByTestId("assignment-edit-open").click();
  await expect(page.getByTestId("assignment-edit-dialog")).toBeVisible();
  await page.getByTestId("assignment-delete-submit").click();
  await page.getByTestId("assignment-delete-confirm").click();

  await expect(page.getByText("作业已删除")).toBeVisible();
  await expect(page).toHaveURL(/\/assignments\/classes\/1$/);
  await expect(page.locator('[data-testid^="assignment-row-"]', { hasText: updatedTitle })).toHaveCount(0);
});

test("老师可在作业详情页上传并删除附件", async ({ page }) => {
  await loginAsTeacher(page);

  const suffix = Date.now().toString();
  const assignmentTitle = `附件作业-${suffix}`;
  const attachmentName = `attachment-${suffix}.txt`;

  await page.getByTestId("sidebar-nav").getByRole("link", { name: "作业管理", exact: true }).click();
  await expect(page).toHaveURL(/\/assignments$/);

  await page.getByTestId("assignment-create-open").click();
  await page.getByTestId("assignment-create-title").fill(assignmentTitle);
  await page.getByTestId("assignment-create-submit").click();
  await expect(page.getByText("作业已创建")).toBeVisible();

  const assignmentRow = page.locator('[data-testid^="assignment-row-"]', { hasText: assignmentTitle });
  await expect(assignmentRow).toBeVisible();
  await assignmentRow.getByRole("link", { name: "详情/批改" }).click();

  await expect(page).toHaveURL(/\/assignments\/classes\/1\/\d+$/);
  await page.getByTestId("assignment-edit-open").click();
  await expect(page.getByTestId("assignment-edit-attachment-manager")).toBeVisible();
  await page.getByTestId("assignment-attachment-input").setInputFiles({
    name: attachmentName,
    mimeType: "text/plain",
    buffer: Buffer.from("assignment attachment smoke"),
  });

  await expect(page.getByText("附件已上传")).toBeVisible();
  await expect(page.getByText(attachmentName)).toBeVisible();

  await page.locator('[data-testid^="assignment-attachment-delete-"]').first().click();
  await page.getByTestId("assignment-attachment-delete-confirm").click();

  await expect(page.getByText("附件已删除")).toBeVisible();
  await expect(page.getByText(attachmentName)).toHaveCount(0);
});
