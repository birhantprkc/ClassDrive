import { describe, expect, it } from "vitest";
import { getFilePreviewKind } from "@/utils/file-preview";
import { renderMarkdownPreview } from "@/utils/markdown-preview";

describe("file preview kind", () => {
  it("detects common image files from extension when MIME type is absent", () => {
    expect(getFilePreviewKind({ kind: "file", name: "photo.JPG" })).toBe("image");
    expect(getFilePreviewKind({ kind: "file", name: "scan.png" })).toBe("image");
    expect(getFilePreviewKind({ kind: "file", name: "cover.webp" })).toBe("image");
  });

  it("detects PDF files from extension when MIME type is absent", () => {
    expect(getFilePreviewKind({ kind: "file", name: "49梁桂瑛.pdf" })).toBe("pdf");
    expect(getFilePreviewKind({ kind: "file", name: "REPORT.PDF" })).toBe("pdf");
  });

  it("detects html files as html previews, not raw text", () => {
    expect(getFilePreviewKind({ kind: "file", name: "index.html" })).toBe("html");
    expect(getFilePreviewKind({ kind: "file", name: "page.HTM" })).toBe("html");
    expect(getFilePreviewKind({ kind: "file", name: "index.html", mimeType: "text/html; charset=utf-8" })).toBe("html");
  });

  it("detects markdown files as markdown previews", () => {
    expect(getFilePreviewKind({ kind: "file", name: "报告.md" })).toBe("markdown");
    expect(getFilePreviewKind({ kind: "file", name: "README.markdown" })).toBe("markdown");
    expect(getFilePreviewKind({ kind: "file", name: "notes.md", mimeType: "text/markdown" })).toBe("markdown");
  });

  it("keeps code and data files as plain text previews", () => {
    expect(getFilePreviewKind({ kind: "file", name: "data.csv" })).toBe("text");
    expect(getFilePreviewKind({ kind: "file", name: "config.json" })).toBe("text");
    expect(getFilePreviewKind({ kind: "file", name: "app.js" })).toBe("text");
  });
});

describe("markdown preview rendering", () => {
  it("renders headings and bold text", () => {
    const html = renderMarkdownPreview("# 标题\n\n**加粗**内容");
    expect(html).toContain("<h1");
    expect(html).toContain("标题");
    expect(html).toContain("<strong>加粗</strong>");
  });

  it("strips scripts and dangerous markup from untrusted markdown", () => {
    const html = renderMarkdownPreview(
      "# 标题\n\n<script>window.location='https://evil.example'</script>\n\n[链接](javascript:alert(1))\n\n<img src=x onerror=alert(1)>",
    );
    expect(html).not.toContain("<script");
    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("onerror");
  });

  it("forces links to open safely in a new tab", () => {
    const html = renderMarkdownPreview("[ClassDrive](https://example.com)");
    expect(html).toContain("https://example.com");
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });

  it("returns empty string for empty source", () => {
    expect(renderMarkdownPreview("   ")).toBe("");
  });
});
