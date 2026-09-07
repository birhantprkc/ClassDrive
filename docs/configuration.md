# ClassDrive 配置文档

本文档详细说明 ClassDrive 的所有配置选项。

---

## 1. 环境变量配置

ClassDrive 支持通过环境变量配置某些行为。环境变量优先级高于默认值。

### 1.1 核心配置

#### CLASSDRIVE_PORT

**说明**: 服务器监听端口

**默认值**: `80`

**类型**: 字符串（数字）

**示例**:
```powershell
# Windows PowerShell
$env:CLASSDRIVE_PORT = "8080"

# Linux/macOS
export CLASSDRIVE_PORT="8080"
```

**注意**:
- 仅在首次启动或未在设置页面保存端口时生效
- 设置页面保存的端口优先级更高
- 1-1023 端口需要管理员权限

#### CLASSDRIVE_DEFAULT_TEACHER_PASSWORD

**说明**: 默认教师账号初始密码

**默认值**: `demo123`

**类型**: 字符串

**建议**: 生产环境务必修改

**示例**:
```powershell
$env:CLASSDRIVE_DEFAULT_TEACHER_PASSWORD = "YourStrongPassword123!"
```

**安全建议**:
- 至少 8 位
- 包含大小写字母、数字和特殊字符
- 不要使用常见密码

#### CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD

**说明**: 学生密码重置后的默认密码

**默认值**: `123456`

**类型**: 字符串

**示例**:
```powershell
$env:CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD = "Student@2026"
```

**注意**:
- 学生重置密码后首次登录必须修改此密码
- 建议设置为易记但不易猜测的密码

---

## 2. 系统设置（Web 界面配置）

登录教师端后，进入 **设置 > 系统设置** 可配置以下选项。

### 2.1 服务器端口

**路径**: 设置 > 系统设置 > 服务器端口

**默认值**: `80`

**说明**: 程序监听的 HTTP 端口

**修改后**: 需要重启程序生效

**配置持久化**: 保存在数据库中，重启后自动加载

### 2.2 上传面板开关

**路径**: 设置 > 系统设置 > 上传面板默认状态

**默认值**: 开启

**选项**:
- **开启**: 教师端文件管理页面默认展开上传面板
- **关闭**: 默认收起上传面板

**适用场景**:
- 小屏幕设备建议关闭
- 频繁上传文件建议开启

### 2.3 单账号登录控制

**路径**: 设置 > 系统设置 > 单账号登录控制

**默认值**: 关闭

**说明**:
- **开启**: 同一账号在新设备登录时，旧设备会话自动失效
- **关闭**: 同一账号可在多个设备同时登录

**适用场景**:
- 防止账号共享：开启
- 多设备协作：关闭

### 2.4 默认分享有效期

**路径**: 设置 > 系统设置 > 默认分享有效期

**默认值**: 7 天

**选项**: 1 天 / 7 天 / 30 天

**说明**: 创建资料分享时的默认有效期，可单独调整

---

## 3. 个人设置

**路径**: 设置 > 个人设置

### 3.1 显示名称

**说明**: 在系统中显示的姓名

**默认值**: 注册时填写的姓名

**用途**:
- 日志审计中显示
- 批改作业时显示

### 3.2 密码修改

**说明**: 修改当前账号密码

**要求**:
- 输入当前密码验证身份
- 新密码至少 6 位

### 3.3 紧凑列表模式

**说明**: 文件和列表的显示密度

**选项**:
- **紧凑模式**: 行高更小，适合大屏幕
- **标准模式**: 行高适中，适合中小屏幕

---

## 4. 数据库配置

ClassDrive 使用 SQLite 数据库，配置通过 DSN 字符串指定。

### 4.1 当前配置

```go
dsn := filepath.Join(dbDir, "classdrive.db") +
       "?_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
// journal_mode 是文件级持久设置，启动时单独执行一次：
db.Exec(`pragma journal_mode = wal`)
```

> **注意**：modernc.org/sqlite 驱动只识别 `_pragma=` 形式的 DSN 参数。
> `_busy_timeout=...`、`_journal_mode=...` 这类写法会被静默忽略（曾导致
> 文件夹重命名等并发写操作报 SQLITE_BUSY / 服务器内部错误）。

### 4.2 参数说明

#### busy_timeout

**说明**: 写锁等待超时时间（毫秒）

**当前值**: `5000` (5 秒)

**作用**: 多个写操作并发时，后到达的操作会等待前一个完成，超时后返回错误

**调优建议**:
- 高并发场景可适当提高到 10000
- 单用户场景可降低到 3000

#### journal_mode

**说明**: 日志模式

**当前值**: `WAL` (Write-Ahead Logging)

**优点**:
- 读写并发性能好
- 不阻塞读操作
- 崩溃恢复能力强

**注意**:
- 需要文件系统支持（大部分现代系统支持）
- 会生成 `.db-shm` 和 `.db-wal` 文件

#### _synchronous

**说明**: 同步模式

**当前值**: `NORMAL`

**选项**:
- `OFF`: 最快，但崩溃可能丢失数据
- `NORMAL`: 平衡性能与安全（推荐）
- `FULL`: 最安全，但性能较慢

---

## 5. 文件上传限制

### 5.1 学生提交文件

**最大单文件大小**: 100 MB

**总大小限制**: 无硬限制（受磁盘空间约束）

**允许的文件类型**（按作业的“提交文件类型”分类）:
- 常用文件: Office（`.doc/.docx/.xls/.xlsx/.ppt/.pptx/.rtf/.odt`）、PDF、TXT、HTML（`.html/.htm`）、Markdown（`.md/.markdown`）、CSV、JSON、XML、图片（`.jpg/.jpeg/.png/.gif/.bmp/.webp/.svg/.tif/.tiff/.avif`）、音频（`.mp3/.wav/.m4a/.ogg/.flac`）、视频（`.mp4/.webm/.mov/.avi/.mkv/.wmv`）、压缩包（`.zip/.rar/.7z/.tar/.gz/.tgz`）、WPS（`.wps/.et/.dps`）
- 图片文件: `.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.svg`, `.webp`, `.tif`, `.tiff`, `.avif`
- Word 文档: `.doc`, `.docx`, `.rtf`, `.odt`
- PDF 文件: `.pdf`
- 压缩包: `.zip`, `.rar`, `.7z`, `.tar`, `.gz`, `.tgz`

HTML 提交支持在沙箱环境中在线预览效果，Markdown 支持在线渲染预览。

**代码位置**: `internal/server/server.go` 中 `studentSubmissionAllowedExtensions` 与 `studentSubmissionTypeExtensions`

**修改方式**:
修改 `studentSubmissionMaxFileSize` 常量后重新编译：
```go
const studentSubmissionMaxFileSize = 200 * 1024 * 1024  // 改为 200 MB
```

### 5.2 教师上传文件

**单文件限制**: 使用 multipart 上传时为 32 MB

**大文件上传**: 使用 TUS 协议断点续传，理论上无大小限制

**代码位置**: `internal/server/server.go` 第 1437 行

---

## 6. 会话配置

### 6.1 会话有效期

**当前值**: 24 小时

**说明**: 登录后 24 小时内无操作，会话自动过期

**代码位置**:
```go
const sessionDuration = 24 * time.Hour
```

**修改方式**: 修改常量后重新编译

### 6.2 CSRF 保护

**机制**: 所有写操作（POST/PUT/PATCH/DELETE）需要 CSRF Token

**实现**:
- Cookie: `classdrive_csrf`
- Header: `X-CSRF-Token`

**前端自动处理**: 无需手动配置

---

## 7. 日志配置

### 7.1 日志输出

**当前行为**: 输出到标准输出（控制台）

**重定向到文件**:
```powershell
.\ClassDrive.exe > logs\classdrive.log 2>&1
```

### 7.2 日志级别

**当前实现**: 仅记录错误和关键信息

**未来扩展**: 可添加日志级别配置（DEBUG/INFO/WARN/ERROR）

---

## 8. 性能调优参数

### 8.1 数据库连接池（计划中）

当前未配置连接池，未来版本可添加：

```go
db.SetMaxOpenConns(25)       // 最大打开连接数
db.SetMaxIdleConns(5)        // 最大空闲连接数
db.SetConnMaxLifetime(5 * time.Minute)  // 连接最大生命周期
```

### 8.2 分页大小

#### 教师端列表

**默认值**: 30 条/页

**最大值**: 100 条/页

**代码位置**:
```go
const defaultTeacherListPageSize = 30
const maxTeacherListPageSize = 100
```

#### 学生端列表

**默认值**: 10 条/页（作业列表）

---

## 9. 安全配置

### 9.1 密码策略

#### 最小长度

**当前值**: 6 位

**代码位置**: `internal/server/server.go` 第 4476 行

**建议修改为**: 8 位

```go
if utf8.RuneCountInString(trimmed) < 8 {
    return "", errors.New("密码至少 8 位")
}
```

#### 复杂度要求

**当前**: 无强制复杂度要求

**建议增强**: 要求包含大小写字母、数字和特殊字符

### 9.2 bcrypt 成本

**当前值**: 10

**说明**: bcrypt 哈希的计算成本，越高越安全但越慢

**代码位置**:
```go
passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
```

**调优建议**:
- 10: 适合大多数场景（推荐）
- 12: 更高安全性，适合高敏感场景
- 8: 性能优先，不推荐

---

## 10. 功能开关

### 10.1 测试模式 CSRF 禁用

**代码位置**: `internal/server/server.go`

```go
type App struct {
    disableCSRFForTests bool
}
```

**说明**: 仅用于测试，**生产环境必须为 false**

### 10.2 班级注册码有效期

**当前值**: 90 分钟

**代码位置**:
```go
const classRegistrationDuration = 90 * time.Minute
```

**说明**: 教师开启注册码后，90 分钟内学生可注册，之后自动关闭

**修改方式**: 修改常量后重新编译

---

## 11. 前端配置

### 11.1 Vite 配置

**文件**: `frontend/vite.config.ts`

**关键配置**:
```typescript
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: fileURLToPath(new URL("../internal/server/dist", import.meta.url)),
    emptyOutDir: true,
  },
});
```

**说明**:
- `@` 别名指向 `src/` 目录
- 构建产物输出到 `internal/server/dist/`，会被 Go 程序嵌入

### 11.2 API 代理（开发环境）

**文件**: `frontend/vite.config.ts`（需要添加）

如需前端开发服务器代理 API 请求到后端，添加：

```typescript
export default defineConfig({
  // ...
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
```

---

## 12. 配置文件位置汇总

| 配置项 | 位置 | 类型 |
|--------|------|------|
| 环境变量 | 系统环境变量 | 运行时 |
| 系统设置 | 数据库 `system_settings` 表 | 持久化 |
| 个人设置 | 数据库 `teacher_preferences` 表 | 持久化 |
| 数据库配置 | 代码常量 | 编译时 |
| 文件限制 | 代码常量 | 编译时 |
| 会话配置 | 代码常量 | 编译时 |
| 前端配置 | `frontend/vite.config.ts` | 构建时 |

---

## 13. 配置优先级

1. **运行时环境变量** (最高优先级)
2. **Web 界面系统设置** (数据库持久化)
3. **代码默认值** (最低优先级)

**示例**: 端口配置优先级

```
1. 数据库中保存的端口（如果存在）
2. CLASSDRIVE_PORT 环境变量
3. defaultServerPort 常量 (80)
```

---

## 14. 配置模板

### 14.1 Windows 服务环境变量

**文件**: `classdrive.env`

```ini
CLASSDRIVE_PORT=80
CLASSDRIVE_DEFAULT_TEACHER_PASSWORD=YourStrongPassword123!
CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD=Student@2026
```

**使用方式**（NSSM）:
```powershell
nssm set ClassDrive AppEnvironmentExtra < classdrive.env
```

### 14.2 生产环境建议配置

```ini
# 端口
CLASSDRIVE_PORT=80

# 强密码
CLASSDRIVE_DEFAULT_TEACHER_PASSWORD=<随机生成的强密码>
CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD=<随机生成的强密码>
```

**安全检查清单**:
- [x] 修改默认教师密码
- [x] 修改默认学生重置密码
- [x] 开启单账号登录控制（如需要）
- [x] 定期备份数据
- [x] 限制网络访问范围（局域网）

---

## 15. 故障排查

### 15.1 配置不生效

**检查步骤**:

1. **环境变量是否正确设置**:
   ```powershell
   $env:CLASSDRIVE_PORT
   ```

2. **数据库配置是否覆盖环境变量**:
   ```sql
   sqlite3 var/db/classdrive.db
   SELECT * FROM system_settings;
   ```

3. **是否重启程序**:
   部分配置需要重启才生效

### 15.2 端口冲突

**症状**: `bind: address already in use`

**解决**:
1. 更换端口
2. 或终止占用进程

### 15.3 密码策略过于宽松

**当前**: 最小 6 位

**建议**: 修改代码提升到 8 位并要求复杂度

---

## 16. 未来配置增强

### 16.1 配置文件支持

**计划**: 支持 `config.toml` 或 `config.yaml` 配置文件

**优点**:
- 配置集中管理
- 易于版本控制
- 支持更复杂的配置项

### 16.2 Web 界面完整配置

**计划**: 所有配置项均可通过 Web 界面修改，无需编辑代码

### 16.3 配置热重载

**计划**: 部分配置修改后无需重启程序即可生效

---

**文档版本**: 1.0  
**最后更新**: 2026-06-13  
**维护者**: ClassDrive 开发团队
