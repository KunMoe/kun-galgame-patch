# 用户自助资料管理

返回 [README](./README.md)

这一组端点是给"已登录用户用自己的 access_token 修改自己资料"用的。kungal/moyu 等下游服务有两种典型用法：

1. **跳转模式**（推荐）：站点的"修改头像/简介"按钮直接跳转到 OAuth 前端的 profile 页面，让用户在 OAuth 完成修改。
2. **代理模式**：站点保留自己的"修改资料"端点，但内部把请求转发到下面这些 OAuth 端点。要求请求带的是终端用户 JWT（**不是** OAuth Client Basic Auth）。

| 端点 | 方法 | 用途 |
|------|------|------|
| `/auth/me` | GET | 读自己完整资料（含 moemoepoint） |
| `/auth/me` | PATCH | 改 name / avatar / avatar_image_hash / bio |
| `/auth/me/avatar` | POST | 上传头像文件，一步写入 |

---

## GET /auth/me

获取当前登录用户的完整资料。与 `/oauth/userinfo` 的区别：`/auth/me` 是面向 OAuth 自己前端的内部端点，无 scope 过滤、字段更全（含 moemoepoint）。下游服务若用得着也可以调。

**请求头**：`Authorization: Bearer <access_token>`

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "uuid": "550e8400-e29b-...",
    "name": "kun",
    "email": "kun@kungal.com",
    "avatar": "https://...",
    "bio": "...",
    "moemoepoint": 1234,
    "status": 0,
    "roles": ["user", "admin"],
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

---

## PATCH /auth/me

修改当前登录用户的展示字段。所有字段都可选，不传的字段保持不变。

**请求头**：`Authorization: Bearer <access_token>`

**请求体**：

```json
{
  "name": "newname",
  "avatar": "https://...",
  "avatar_image_hash": "abc123...",
  "bio": "新简介"
}
```

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| name | string? | 2..17 字符；全局唯一 | 用户名 |
| avatar | string? | ≤255 字符 | 头像 URL（legacy；image_service 普及前继续用） |
| avatar_image_hash | string? | ≤64 字符 | 头像的 image_service 哈希；前端 resolveAvatarUrl 优先用此字段 |
| bio | string? | ≤107 字符 | 个人简介 |

字段都用指针类型语义：**没传 = 不动；传了 = 设为该值**（包括传空字符串 = 清空）。

**成功响应**：返回更新后的完整 `UserResponse`（同 GET /auth/me 的 `data` shape）。

**错误响应**：

| HTTP | code | 触发条件 |
|------|------|----------|
| 400  | 1    | JSON 格式错误 |
| 400  | 7    | 字段约束未通过（name 长度、bio 长度等） |
| 400  | 10007 | name 与其他用户重复 |
| 401  | 10001/10002/10003 | 未提供 / 无效 / 过期 token |

**修改 email 不在这里** —— email 必须走 `/auth/email/send-code` + `/auth/email`（带验证码的两步流程，防止账号被劫持）。

**修改 password 也不在这里** —— password 必须走 `/auth/password`（需要旧密码或重置 token）。

**举例**：仅改头像 hash（image_service 上传完毕之后）：

```bash
curl -X PATCH https://oauth.kungal.com/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"avatar_image_hash":"abc123def456..."}'
```

---

## POST /auth/me/avatar

> 🆕 **2026-05-23 新增**：一次性"上传头像图片 → 写入用户记录"端点，**避免下游 kungal / moyu 自己维护 image_service client**。

直接接收图片二进制（multipart），OAuth 内部转发到 image_service，并在响应返回前把拿到的 hash 写入当前用户的 `avatar_image_hash`。**调用方拿到响应时数据库已经更新**，无需再调 `PATCH /auth/me`。

**请求头**：`Authorization: Bearer <access_token>`

**请求 body**：`multipart/form-data`

| 字段 | 必填 | 说明 |
|------|------|------|
| file | ✓ | 图片文件，MIME 必须 `image/*`；建议 ≤ 4 MiB（fiber 默认 body 上限） |

**成功响应**：直接透传 image_service 的上传结果。

```json
{
  "code": 0,
  "data": {
    "hash": "abc123def456...",
    "url": "https://image.kungal.com/i/abc123...",
    "variant_urls": {
      "256": "https://image.kungal.com/i/abc123...-256.webp",
      "100": "https://image.kungal.com/i/abc123...-100.webp"
    },
    "width": 512,
    "height": 512,
    "size_bytes": 38241,
    "deduplicated": false
  }
}
```

`variant_urls` 提供 256 / 100 像素的预生成缩略图，前端列表 / 评论场景直接用 100，主页 / 个人页用 256，原图（`url`）一般不需要展示。`deduplicated=true` 表示同 hash 文件以前传过，image_service 复用了已有对象，没有额外存储成本。

**错误响应**：

| HTTP | code | 触发条件 |
|------|------|----------|
| 400  | 8    | `file` 字段缺失或不是合法 multipart |
| 401  | 10001/10002/10003 | 未提供 / 无效 / 过期 token |
| 404  | 10005 | 用户记录不存在（一般 token 还有效就不会触发） |
| 500  | 1    | image_service 不可达 / 配额耗尽 / 审核拒绝；详见 OAuth 服务端日志 |

**和现有方式的关系**：

| 方式 | 谁调 image_service | 几次请求 | 适合 |
|------|------|------|------|
| `POST /auth/me/avatar`（**推荐**） | OAuth 内部 | 1 次 | 标准 web / 移动端"用户改头像" |
| `PATCH /auth/me { avatar_image_hash }` | **下游自己** | 2 次（先上传到 image_service 拿 hash，再 PATCH） | 下游已有 image_service client（投稿 / 截图等场景），头像和别的图片走同一上传管线 |

两种方式可以并存，下游想用哪种都行。`avatar` 和 `avatar_image_hash` 仍是独立字段（参见 PATCH /auth/me 节）。

**配额归属**：图片走的是 OAuth 自己的 image_service client，配额从 OAuth 这一侧扣，**下游 kungal / moyu 不需要为头像单独申请 image_service client**。

**CORS**：浏览器直传需要 OAuth 在 CORS 配置里允许下游 origin 上 `POST` + `Authorization` header，目前 `*.kungal.com` 已包含；新增子域接入前请确认。后端代理模式（kungal/moyu 后端接 multipart 再转发）天然不受 CORS 影响。

**举例**：

```bash
curl -X POST https://oauth.kungal.com/api/v1/auth/me/avatar \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@avatar.png"
```

浏览器版本：

```ts
const fd = new FormData()
fd.append('file', file)  // <input type="file"> 的 File 对象
const r = await fetch('https://oauth.kungal.com/api/v1/auth/me/avatar', {
  method: 'POST',
  headers: { Authorization: `Bearer ${accessToken}` },
  body: fd  // 注意：不要手动设 Content-Type，让浏览器自动带 boundary
})
const { data } = await r.json()
// data.hash 已经被 OAuth 写入了用户记录，下一次 GET /auth/me 就能看到新头像
```

---

完整错误码表见 [04-tokens-and-errors.md](./04-tokens-and-errors.md#错误码速查)。
