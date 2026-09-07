# ClassDrive API 文档

本文档记录 ClassDrive 的主要 API 接口。

## 基础信息

- **Base URL**: `http://localhost:8080/api`（开发环境）
- **认证方式**: Session Cookie（教师端、学生端）或分享会话 Cookie（访客端）
- **Content-Type**: `application/json`

---

## 老师资料分享 API

### 1. 获取分享列表

获取当前教师创建的所有分享记录。

```http
GET /api/library-shares
```

**认证**: 需要教师登录

**响应示例**:
```json
{
  "shares": [
    {
      "id": 1,
      "token": "abc123def456",
      "entryId": 100,
      "entryName": "课程资料.pdf",
      "entryKind": "file",
      "permission": "download",
      "requiresAccessCode": true,
      "expiresAt": "2026-06-20T23:59:59Z",
      "disabled": false,
      "status": "active",
      "createdAt": "2026-06-13T08:00:00Z",
      "updatedAt": "2026-06-13T08:00:00Z",
      "lastAccessedAt": "2026-06-13T10:30:00Z",
      "accessCount": 5
    }
  ]
}
```

**字段说明**:
- `token`: 分享唯一标识，用于构建分享链接
- `entryKind`: 分享对象类型，`file` 或 `dir`
- `permission`: 访问权限，`view`（仅查看）或 `download`（允许下载）
- `requiresAccessCode`: 是否需要安全码
- `status`: 分享状态
  - `active`: 活跃（未过期、未停用、源文件存在）
  - `expired`: 已过期
  - `disabled`: 已停用
  - `invalid`: 源文件已删除

---

### 2. 创建分享

为老师资料空间中的文件或文件夹创建分享。

```http
POST /api/library-shares
Content-Type: application/json
```

**认证**: 需要教师登录

**请求体**:
```json
{
  "entryId": 100,
  "permission": "download",
  "requireAccessCode": true,
  "expiresAt": "2026-06-20T23:59:59Z"
}
```

**字段说明**:
- `entryId` (必填): 文件或文件夹 ID，必须属于 library 空间
- `permission` (必填): `view` 或 `download`
- `requireAccessCode` (可选): 是否需要安全码，默认 `false`
- `expiresAt` (可选): 过期时间（ISO 8601 格式），留空表示永久有效

**响应示例**:
```json
{
  "share": {
    "id": 1,
    "token": "abc123def456",
    "entryId": 100,
    "entryName": "课程资料.pdf",
    "entryKind": "file",
    "permission": "download",
    "requiresAccessCode": true,
    "expiresAt": "2026-06-20T23:59:59Z",
    "disabled": false,
    "status": "active",
    "createdAt": "2026-06-13T08:00:00Z",
    "updatedAt": "2026-06-13T08:00:00Z",
    "lastAccessedAt": null,
    "accessCount": 0
  },
  "accessCode": "ABC123"
}
```

**注意**: `accessCode` 仅在创建时返回一次，后续无法查询，请妥善保存。

---

### 3. 更新分享

修改分享的权限、有效期或停用状态。

```http
PATCH /api/library-shares/:id
Content-Type: application/json
```

**认证**: 需要教师登录

**请求体**（所有字段可选）:
```json
{
  "permission": "view",
  "expiresAt": "2026-12-31T23:59:59Z",
  "disabled": false
}
```

**响应**: 更新后的分享对象（同创建分享响应，但不含 `accessCode`）

---

### 4. 删除分享

删除分享记录，分享链接立即失效。

```http
DELETE /api/library-shares/:id
```

**认证**: 需要教师登录

**响应**: 204 No Content

---

### 5. 重置安全码

为分享生成新的安全码，旧安全码立即失效。

```http
POST /api/library-shares/:id/reset-code
```

**认证**: 需要教师登录

**响应示例**:
```json
{
  "share": { /* 分享对象 */ },
  "accessCode": "XYZ789"
}
```

---

## 访客端分享访问 API

### 1. 获取分享信息

获取分享的基本信息（无需安全码）。

```http
GET /api/share/:token
```

**认证**: 无需登录

**响应示例**:
```json
{
  "info": {
    "entryId": 100,
    "entryName": "课程资料.pdf",
    "entryKind": "file",
    "permission": "download",
    "requiresAccessCode": true,
    "expiresAt": "2026-06-20T23:59:59Z",
    "status": "active"
  }
}
```

---

### 2. 验证安全码

验证访问安全码，通过后在 Cookie 中设置分享会话。

```http
POST /api/share/:token/verify
Content-Type: application/json
```

**请求体**:
```json
{
  "accessCode": "ABC123"
}
```

**响应**: 200 OK（验证成功）或 401 Unauthorized（安全码错误）

**Set-Cookie**: `share_session_<token>=<session_value>; HttpOnly; SameSite=Lax`

---

### 3. 浏览目录

浏览分享的文件夹内容（仅目录分享可用）。

```http
GET /api/share/:token/items?path=/子目录
```

**认证**: 需要分享会话（已验证安全码或无需安全码的分享）

**查询参数**:
- `path` (可选): 子目录路径，默认为根目录

**响应示例**:
```json
{
  "path": "/子目录",
  "items": [
    {
      "id": 101,
      "name": "文档.pdf",
      "kind": "file",
      "size": 2048,
      "mimeType": "application/pdf",
      "updatedAt": "2026-06-10T00:00:00Z",
      "previewUrl": "/api/share/abc123def456/files/101/preview",
      "downloadUrl": "/api/share/abc123def456/files/101/download",
      "archiveUrl": ""
    },
    {
      "id": 102,
      "name": "子文件夹",
      "kind": "dir",
      "size": 0,
      "mimeType": "",
      "updatedAt": "2026-06-11T00:00:00Z",
      "previewUrl": "",
      "downloadUrl": "",
      "archiveUrl": "/api/share/abc123def456/files/102/archive"
    }
  ]
}
```

---

### 4. 预览文件

在浏览器中预览文件（支持图片、PDF、文本、音视频、Markdown 等；HTML 网页在沙箱环境中渲染）。

```http
GET /api/share/:token/files/:id/preview
```

**认证**: 需要分享会话

**响应**: 文件内容（Content-Type 根据文件类型设置）

**安全**: HTML/SVG 等可执行内容的内联预览响应会附带
`Content-Security-Policy: sandbox` 与 `X-Content-Type-Options: nosniff`，
即使在新标签页直接打开也无法以同源身份运行脚本。

---

### 5. 下载文件

下载文件（需要 `download` 权限）。

```http
GET /api/share/:token/files/:id/download
```

**认证**: 需要分享会话 + `download` 权限

**响应**: 文件内容（Content-Disposition: attachment）

---

### 6. 下载文件夹压缩包

将文件夹及其内容打包为 ZIP 下载（需要 `download` 权限）。

```http
GET /api/share/:token/files/:id/archive
```

**认证**: 需要分享会话 + `download` 权限

**响应**: ZIP 压缩包

---

## 系统设置 API

### 更新系统设置

```http
PATCH /api/settings/system
Content-Type: application/json
```

**认证**: 需要教师登录（owner 角色）

**请求体示例**（新增字段）:
```json
{
  "defaultShareExpiresDays": 7
}
```

**字段说明**:
- `defaultShareExpiresDays`: 默认分享有效期（天），范围 1-3650，默认 7

---

## 错误响应

所有 API 在出错时返回 JSON 格式的错误信息：

```json
{
  "error": "错误描述信息"
}
```

常见 HTTP 状态码：
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 未登录或会话过期
- `403 Forbidden`: 权限不足
- `404 Not Found`: 资源不存在
- `500 Internal Server Error`: 服务器内部错误

---

## 安全注意事项

1. **分享 token**: 使用 16 字节随机生成，base64url 编码，保证唯一性
2. **安全码**: 使用 bcrypt (cost 10) 加密存储，永不明文返回（仅创建/重置时返回一次）
3. **分享会话**: 通过 HttpOnly Cookie 管理，防止 XSS 攻击
4. **权限校验**: 
   - 仅 library 空间可创建分享
   - 访客无法访问分享范围外的文件（路径遍历防护）
   - `view` 权限禁止下载和打包操作
5. **状态检查**: 每次访问时检查分享是否过期、停用或源文件已删除
