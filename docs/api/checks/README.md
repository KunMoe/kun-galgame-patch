# API 字段对齐审计

> 目的：逐方法记录 moyu 后端**全部 API 端点** + FE↔BE 字段对齐审计状态。仿照
> [`kun-oauth-admin/docs/api/checks/`](../../../../kun-oauth-admin/docs/api/checks/README.md)。
>
> 当前进度：**清单完成（2026-05-29）/ 审计待开始** —— 全部 **159 端点**
> （GET 80 / POST 37 / PUT 26 / DELETE 14 / PATCH 2）已逐条录入，状态默认 ⏳（待审计）。
>
> 路由唯一来源：`apps/api/internal/app/router.go`（`RegisterRoutes`）。该文件之外**无**任何
> 路由注册（仅 `app.Use` 挂中间件），无 `/health`、无静态文件服务。

## moyu 是下游站点（代理透传占比高）

moyu 自身**不对外提供任何 s2s（🔑 OAuth Client Basic Auth）入站端点** —— 它只作为
**消费方**调用上游。大量端点是对上游服务的**代理透传**，真正的资源鉴权由上游强制，
moyu 侧仅挂 `auth`/`optionalAuth` 以便转发用户的 OAuth access_token：

| 上游 | 被代理的端点 | 鉴权归属 |
|---|---|---|
| **Galgame Wiki Service** | `/galgame/*`（投稿/编辑/修订/PR/links/aliases）、`/tag /official /engine /series` 全部 CRUD + 修订 | Wiki 强制 creator/admin/mod；moyu 转发 token，逐字回传 Wiki 的 code+message |
| **KUN OAuth** | `PATCH /auth/me`、`POST /auth/me/avatar`（展示层）、`GET /user/moemoepoint/log`（自助流水） | OAuth 是身份 + 萌萌点的唯一真源 |
| **image_service** | `POST /upload/image-service` | 图片落 image_service，回 hash + variant URL |

> 审计这些代理端点时，重点不是 Wiki/OAuth 的内部逻辑，而是 moyu 的：转发是否丢字段 /
> 鉴权挂载是否正确（该 `auth` 的没漏挂）/ 响应重写（如 `WikiTaxonomyDetailProxy`）是否对齐。

## 端点矩阵（按服务 × 方法）

| 服务 | 二进制 | Base URL | GET | POST | PUT | DELETE | PATCH | 小计 |
|---|---|---|---|---|---|---|---|---|
| moyu | `cmd/server` | `/api/v1` | [80](./moyu.get.md) | [37](./moyu.post.md) | [26](./moyu.put.md) | [14](./moyu.delete.md) | [2](./moyu.patch.md) | **159** |

### 功能域分布（便于切片审计）

| 域 | GET | POST | PUT | DELETE | PATCH | 小计 |
|---|---|---|---|---|---|---|
| 认证 `/auth` | 1 | 2 | — | — | 1 | 4 |
| 补丁/评论/资源 `/patch` | 8 | 3 | 9 | 3 | — | 23 |
| Galgame 投稿/编辑代理 `/galgame` | 11 | 6 | 4 | 3 | 1 | 25 |
| 分类代理 `/tag /official /engine /series`（含修订） | 20 | 9 | 4 | 4 | — | 37 |
| 用户 `/user` | 11 | 2 | 1 | 1 | — | 15 |
| 消息 `/message` | 3 | 1 | 1 | — | — | 5 |
| 管理 `/admin` | 11 | 1 | 5 | 2 | — | 19 |
| 公共 `/home /galgame /comment /resource /ranking` | 8 | — | — | — | — | 8 |
| 聊天 `/chat` | 3 | 5 | 2 | 1 | — | 11 |
| 上传 `/upload` | — | 6 | — | — | — | 6 |
| 搜索/外部/关于 | 4 | 1 | — | — | — | 5 |
| **合计** | **80** | **37** | **26** | **14** | **2** | **159** |

## 图例 — 审计状态

- ✅ 已审计，对齐无问题
- 🔧 已审计，发现问题并修复
- ⏭️ 已审计，有意保持当前行为
- ⏳ 待审计（inventory 默认 —— **目前全部为此状态**）
- 🆕 本轮新增端点

## 图例 — 鉴权

| 标记 | 中间件 | 含义 |
|---|---|---|
| 🌐 | （无） | 完全公开 |
| 🔐 | `OptionalAuth` | 可选鉴权；带 session 附加内容（如 viewer 的 like/收藏态），匿名只看公共部分 |
| 🔒 | `Auth` | 必须登录 |
| 🛡️ | `Auth` + `RequireRole("admin","moderator")` | admin 或 moderator（`/admin` 组级中间件）|
| ⚙️ | 组级 🛡️ + 路由级 `RequireRole("admin")` | **仅 admin**（在 moderator 门之上再叠加 admin 门）|
| ⏱️ | `+ RateLimit` | 叠加限流中间件 |

> moyu 的 `Auth`/`OptionalAuth` 基于 `kun_session`（Redis 会话，httpOnly cookie），
> 校验会话并按需后台刷新上游 OAuth token；非纯 JWT 验签。详见
> `internal/middleware/auth.go`。角色映射（`docs/user-migration/02-data-mapping.md §7`）：
> legacy role 4 → `admin`，role 3 → `moderator`。

## 配套清单

- [moyu.get.md](./moyu.get.md) — GET（80）
- [moyu.post.md](./moyu.post.md) — POST（37）
- [moyu.put.md](./moyu.put.md) — PUT（26）
- [moyu.delete.md](./moyu.delete.md) — DELETE（14）
- [moyu.patch.md](./moyu.patch.md) — PATCH（2）
