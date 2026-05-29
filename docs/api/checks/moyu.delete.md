# moyu 后端 — DELETE API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：全部 ⏳ 待审计（inventory）。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 DELETE 端点：**14**
  - 补丁/评论/资源 3 · Galgame 代理 3 · 分类代理 4 · 用户 1 · 管理 2 · 聊天 1

---

## 1. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/patch/:id` | 🔒 | `patchH.DeletePatch` | ⏳ | 删补丁 |
| `DELETE /api/v1/patch/comment/:commentId` | 🔒 | `patchH.DeleteComment` | ⏳ | 删评论 |
| `DELETE /api/v1/patch/resource/:resourceId` | 🔒 | `patchH.DeleteResource` | ⏳ | 删资源（−3 萌萌点 content_removed）|

## 2. Galgame 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/galgame/:gid` | 🔒 | `patchH.DeleteGalgameDraft` | ⏳ | 删草稿投稿（代理 Wiki）|
| `DELETE /api/v1/galgame/:gid/links` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：删关联链接（Wiki 强制 owner/admin）|
| `DELETE /api/v1/galgame/:gid/aliases` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：删别名 |

## 3. 分类代理 `/tag /official /engine /series`（→ Wiki，Wiki 强制 admin/mod）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/tag/:id` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 删标签 |
| `DELETE /api/v1/official/:id` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 删会社 |
| `DELETE /api/v1/engine/:id` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 删引擎 |
| `DELETE /api/v1/series/:id` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 删系列 |

## 4. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/user/:id/follow` | 🔒 | `userH.Unfollow` | ⏳ | 取关 |

## 5. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/admin/comment/:id` | 🛡️ | `adminH.DeleteComment` | ⏳ | 删评论 |
| `DELETE /api/v1/admin/resource/:id` | 🛡️ | `adminH.DeleteResource` | ⏳ | 删资源 |

## 6. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/chat/message/:id` | 🔒 | `chatH.DeleteMessage` | ⏳ | 删消息 |
