# ClassDrive 项目协作说明

面向课堂/机房的局域网文件与作业工作台。教师端发资料、收作业、批改统计;学生端看资料、交作业。单二进制部署,前端构建产物通过 `go:embed` 嵌入后端可执行文件。

## 技术栈

- **后端**:Go 1.26,纯 Go SQLite(`modernc.org/sqlite`,无需 CGO)。入口 `cmd/classdrive/main.go`,几乎全部逻辑集中在单文件 `internal/server/server.go`(逾万行)。
- **前端**:Vue 3 + TypeScript + Pinia + Vue Router + Element Plus,Vite 构建,Vitest 单测,Playwright 视觉/E2E。源码在 `frontend/src`。
- **嵌入**:`internal/server/dist/` 是 `vite build` 的输出,由 `//go:embed dist` 嵌入二进制。改了前端必须重新 `build` 才会进 exe。

## 构建与验证(命令都从仓库根目录跑)

| 目的 | 命令 |
|---|---|
| 全量验证(发布前) | `npm run verify:full` |
| 前端类型检查 | `npm run typecheck` |
| 前端单测 | `npm run test` |
| 前端构建 | `npm run build` |
| 后端测试 | `go test ./...` |
| 后端编译 | `go build -o classdrive.exe ./cmd/classdrive` |

`scripts/verify-full.mjs` 定义了规范顺序:前端 typecheck → 前端 test → 前端 build → 后端 test → 后端 build。改动收尾时按需跑对应步骤,发布前跑 `verify:full`。

### 验证的坑(务必遵守)

- **类型检查只用 `npm run typecheck`**(底层 `vue-tsc --noEmit -p tsconfig.app.json`)。不要用 `vue-tsc -b`:它会在 `src/`、`tests/` 旁吐出一堆 `.js` 编译产物污染工作区,还会把 `tests/` 里的预存类型错误一并报出来,造成"我改坏了"的假象。
- `npm run typecheck` 只检查 app 源码、`--noEmit`,既不产物污染、也不掺 tests 错误,是判断"我的改动是否引入类型错误"的唯一可信信号。
- 若误用了会 emit 的命令,收尾时清理 `src/`、`tests/` 下新生成的 `.js` 和 `tsconfig.tsbuildinfo`(它们不在 .gitignore 里,容易混进提交)。

## 架构约定

- **后端单文件**:`server.go` 内按功能分块(路由分发 `handleXxx`、业务 `app.xxx`、审计、迁移)。加功能优先照抄最相近的现有样板,保持风格一致。
- **数据库迁移**:无独立迁移文件。建表 SQL 内联在 `server.go`;给已有表加列用 `ensureColumns(...)` 模式(运行时 `alter table ... add column`),在 `New()` 初始化时调用。加列时记得同步:建表 SQL、`ensureXxxColumn`、相关 SELECT/Scan、`buildXxxSummary` 等所有读写路径。
- **操作审计**:大多按路由路径自动记录(`recordOperationLog` + `operationLogSummary` 的 fallback 描述),新接口通常无需额外审计代码,只在需要更精确描述时去 `operationLogSummary` 的对应 `case` 里加分支。
- **下载/预览**:学生提交文件的预览 URL 为空,前端预览会 fallback 到下载接口——所以拦截下载接口即同时拦住预览,不必两处都改。

## 版本号

程序内显示的版本号在 3 个文件的 `Ver: x.x` 字样:`frontend/src/components/SidebarNav.vue`、`frontend/src/layouts/StudentLayout.vue`、`frontend/src/views/LoginView.vue`。改版本号三处一起改。`RELEASE_CHECKLIST.md` 是某次发布的历史快照(带固定版本号和日期),不是模板,不要随版本号改动。

## 文档

- `CHANGELOG.md`:正式变更日志,新功能在 `[Unreleased]` 段记录(Added / Technical Details / Security)。
- `README.md`:对外功能介绍,功能有增减时同步"功能总览"。
- `docs/`:体系化文档(架构、部署、配置等),改动涉及对应主题时才更新。
- 不要在根目录堆放一次性的过程文档(某次修复总结、构建快照等),这类内容应进 `CHANGELOG.md` 或留在 git 历史里。
