# moyu 后端 — PUT API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.post.md](./moyu.post.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：全部 ⏳ 待审计（inventory）。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 PUT 端点：**26**
  - 补丁/评论/资源 9 · Galgame 代理 4 · 分类代理 4 · 用户 1 · 消息 1 · 管理 5 · 聊天 2

---

## 1. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/patch/:id` | 🔒 | `patchH.UpdatePatch` | ⏳ | 改补丁 |
| `PUT /api/v1/patch/:id/view` | 🌐 | `patchH.IncrementView` | ⏳ | 浏览量 +1（无鉴权）|
| `PUT /api/v1/patch/:id/favorite` | 🔒 | `patchH.ToggleFavorite` | ⏳ | 收藏 / 取消（±1 萌萌点给作者）|
| `PUT /api/v1/patch/comment/:commentId` | 🔒 | `patchH.UpdateComment` | ⏳ | 改评论 |
| `PUT /api/v1/patch/comment/:commentId/like` | 🔒 | `patchH.ToggleCommentLike` | ⏳ | 点赞 / 取消（±1 萌萌点）|
| `PUT /api/v1/patch/resource/:resourceId` | 🔒 | `patchH.UpdateResource` | ⏳ | 改资源（可触发文件替换审计）|
| `PUT /api/v1/patch/resource/:resourceId/disable` | 🔒 | `patchH.ToggleResourceDisable` | ⏳ | 禁用 / 启用该资源下载 |
| `PUT /api/v1/patch/resource/:resourceId/download` | 🌐 | `patchH.IncrementResourceDownload` | ⏳ | 下载量 +1（无鉴权）|
| `PUT /api/v1/patch/resource/:resourceId/like` | 🔒 | `patchH.ToggleResourceLike` | ⏳ | 点赞 / 取消（±1 萌萌点）|

## 2. Galgame 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/galgame/:gid` | 🔒 | `patchH.UpdateGalgame` | ⏳ | 改 galgame 元数据（代理 Wiki；Wiki 强制 creator/admin）|
| `PUT /api/v1/galgame/messages/read-state` | 🔒 | `patchH.UpdateWikiMessagesReadState` | ⏳ | 标记 Wiki 消息已读 |
| `PUT /api/v1/galgame/:gid/prs/:prid/merge` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：合并 PR |
| `PUT /api/v1/galgame/:gid/prs/:prid/decline` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：拒绝 PR |

## 3. 分类代理 `/tag /official /engine /series`（→ Wiki，Wiki 强制 admin/mod）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/tag` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 改标签 |
| `PUT /api/v1/official` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 改会社 |
| `PUT /api/v1/engine` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 改引擎 |
| `PUT /api/v1/series/:id` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 改系列 |

## 4. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/user/:id/follow` | 🔒 | `userH.Follow` | ⏳ | 关注 |

## 5. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/message/read` | 🔒 | `messageH.MarkAsRead` | ⏳ | 标记已读 |

## 6. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/admin/comment/:id` | 🛡️ | `adminH.UpdateComment` | ⏳ | 改评论 |
| `PUT /api/v1/admin/comment/:id/approve` | 🛡️ | `patchH.ApproveComment` | ⏳ | 通过待审评论（复用 PatchService 评论副作用）|
| `PUT /api/v1/admin/resource/:id` | 🛡️ | `adminH.UpdateResource` | ⏳ | 改资源 |
| `PUT /api/v1/admin/setting/comment-verify` | ⚙️ | `adminH.SetCommentVerify` | ⏳ | 写“评论需审核”开关；仅 admin |
| `PUT /api/v1/admin/setting/creator-only` | ⚙️ | `adminH.SetCreatorOnly` | ⏳ | 写“仅创作者可发布”开关；仅 admin |

## 7. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/chat/room/:link/seen` | 🔒 | `chatH.MarkSeen` | ⏳ | 标记房间已读 |
| `PUT /api/v1/chat/message/:id` | 🔒 | `chatH.UpdateMessage` | ⏳ | 改消息 |
