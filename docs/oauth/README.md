# 鲲 Galgame OAuth 文档

基础路径：`/api/v1`

| 环境 | Base URL |
|------|----------|
| 开发 | `http://127.0.0.1:9277/api/v1` |
| 生产 | `https://oauth.kungal.com/api/v1` |

## 文档索引

### API 参考（按主题）

| # | 文件 | 内容 |
|---|------|------|
| 01 | [oauth-endpoints.md](./01-oauth-endpoints.md) | OAuth 2.0 协议端点：`/oauth/token`、`/oauth/authorize`、`/oauth/userinfo`、`/oauth/revoke` |
| 02 | [user-profile.md](./02-user-profile.md) | 用户自助：`GET/PATCH /auth/me` + `POST /auth/me/avatar`（含头像上传） |
| 03 | [cross-service.md](./03-cross-service.md) | 服务到服务：`/users/batch`、`/users/search`（OAuth Client Basic Auth） |
| 04 | [tokens-and-errors.md](./04-tokens-and-errors.md) | JWT Access Token claims + 完整错误码速查（OAuth 15xxx / 认证 10xxx / 通用） |

### 完整接入指南

| 文件 | 内容 |
|------|------|
| [oauth-integration-guide.md](./oauth-integration-guide.md) | 端到端 OAuth 接入走查：注册 client、PKCE、token 轮换、并发刷新、跨域 / 跨站坑、安全注意事项 |
| [nuxt-integration-prompt.md](./nuxt-integration-prompt.md) | Nuxt 3/4 项目的快速上手指南（含 SSR 回调处理代码） |

---

## 响应格式

```json
{
  "code": 0,        // 0 = 成功，非零 = 错误码
  "message": "成功",
  "data": { ... }   // 成功时有数据；失败时一般为 null 或缺省
}
```

> ⚠️ **业务级 401 走 HTTP 200**：`/auth/me` / `/auth/*` 等需要 token 的端点在 token 缺失 / 失效 / 过期时返回 **HTTP 200** + `{ code: 10001 | 10002 | 10003, message }`，而**不是** HTTP 401。下游客户端不能只靠 HTTP status 判断未授权，必须同时检查 `code`。完整列表见 [04-tokens-and-errors.md §认证错误](./04-tokens-and-errors.md#认证错误-10xxx)。

## 认证

OAuth 一共有三种鉴权方式，按场景区分：

| 方式 | 用在哪 | 谁有 |
|------|------|------|
| **Bearer Token**（用户 JWT） | `/auth/*` 用户自助 + `/oauth/userinfo` | 已登录的终端用户 |
| **OAuth Client Basic Auth** | `/users/batch`、`/users/search`（跨服务） | 已注册的 OAuth Client（kungal / moyu / wiki 等下游后端） |
| **Admin JWT**（Bearer + role=admin） | `/admin/*`（不在本文档范围） | OAuth 后台管理员 |

终端用户 JWT 通过完整的 OAuth Authorization Code + PKCE 流程拿到（详见 [oauth-integration-guide.md](./oauth-integration-guide.md)）。Client Basic Auth 的 client_id / client_secret 在 OAuth 后台创建 Client 时生成。

---

## 变更摘要

> 🆕 **2026-05-23**：新增 [POST /auth/me/avatar](./02-user-profile.md#post-authmeavatar) 端点。一次性的"上传头像图片 → 写库" multipart 端点，**避免下游 kungal / moyu 自己维护 image_service client**。配额从 OAuth 一侧扣；老的两步法（`PATCH /auth/me { avatar_image_hash }`）继续保留。

> 📦 **文档拆分（2026-05-23）**：原 `api-reference.md` 拆为 4 个主题文件（01-04）。所有内容保留，按"OAuth 协议 / 用户自助 / 跨服务 / Token 与错误"四块组织。完整 OAuth 接入指南仍是单独的 [oauth-integration-guide.md](./oauth-integration-guide.md)。
