import DOMPurify from "dompurify";
import { marked } from "marked";

/**
 * 将 Markdown 源码渲染为经过净化的 HTML，用于预览弹窗的 v-html。
 * 学生提交的 Markdown 属于不可信内容：链接/脚本/事件属性都会被 DOMPurify 移除，
 * 链接统一加 rel 并强制在新窗口打开，防止钓鱼与 XSS。
 */
export function renderMarkdownPreview(source: string): string {
  if (!source.trim()) {
    return "";
  }
  const rawHtml = marked.parse(source, {
    async: false,
    breaks: true,
    gfm: true,
  }) as string;
  return DOMPurify.sanitize(rawHtml, {
    ADD_ATTR: ["target"],
    FORBID_TAGS: ["style", "form", "input", "button", "textarea", "select"],
    FORBID_ATTR: ["style"],
  });
}

DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A") {
    node.setAttribute("target", "_blank");
    node.setAttribute("rel", "noopener noreferrer");
  }
});
