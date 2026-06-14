# moyu 后端 — PUT API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.post.md](./moyu.post.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：**审计完成（2026-05-29）**。详细逐端点报告见 [`_audit/`](./_audit/)。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：对齐 = 无问题 · 已修 = 已修复 · 保持 = 有意保持 · 待审计 · 新增
鉴权：公开 · 可选登录 = OptionalAuth · 登录 · 管理 = admin/mod · 仅admin · 限流

## 统计

- 本服务 PUT 端点：**26**
  - 补丁/评论/资源 9 · Galgame 代理 4 · 分类代理 4 · 用户 1 · 消息 1 · 管理 5 · 聊天 2
- 本轮：已修复 1（`/user/:id/follow` FK 报错泄露）· 代理 6 · 其余

---

## 1. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/patch/:id` | 登录 | `patchH.UpdatePatch` | 对齐 | owner/privileged gate；仅允许重绑同 galgame_id（实测）|
| `PUT /api/v1/patch/:id/view` | 公开 | `patchH.IncrementView` | 对齐 | 无鉴权公开计数；缺失 id 匹配 0 行无错（实测）|
| `PUT /api/v1/patch/:id/favorite` | 登录 | `patchH.ToggleFavorite` | 对齐 | `{favorited}`；±1 萌萌点给作者，自收藏 guard + 幂等键（实测自反转）|
| `PUT /api/v1/patch/comment/:commentId` | 登录 | `patchH.UpdateComment` | 对齐 | owner-only；无 FE 编辑 UI（dead-but-correct）|
| `PUT /api/v1/patch/comment/:commentId/like` | 登录 | `patchH.ToggleCommentLike` | 对齐 | `{liked}`；±1 萌萌点 + 自赞 guard + 幂等键（实测自反转）|
| `PUT /api/v1/patch/resource/:resourceId` | 登录 | `patchH.UpdateResource` | 对齐 | owner/mod；文件实质变更才写 file-history + 删旧 S3（事务）|
| `PUT /api/v1/patch/resource/:resourceId/disable` | 登录 | `patchH.ToggleResourceDisable` | 对齐 | `{status}`；原子 CASE 翻转（实测自反转）|
| `PUT /api/v1/patch/resource/:resourceId/download` | 公开 | `patchH.IncrementResourceDownload` | 对齐 | 无鉴权；同事务 resource+patch 各 +1（无重复）|
| `PUT /api/v1/patch/resource/:resourceId/like` | 登录 | `patchH.ToggleResourceLike` | 对齐 | `{liked}`；±1 萌萌点 + 自赞 guard + 幂等键（实测自反转）|

## 2. Galgame 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/galgame/:gid` | 登录 | `patchH.UpdateGalgame` | 对齐 | 代理 Wiki（Wiki 强制 creator/admin）|
| `PUT /api/v1/galgame/messages/read-state` | 登录 | `patchH.UpdateWikiMessagesReadState` | 对齐 | forward-only GREATEST（实测往返，DB 回 0）|
| `PUT /api/v1/galgame/:gid/prs/:prid/merge` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `PUT /api/v1/galgame/:gid/prs/:prid/decline` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 3. 分类代理 `/tag /official /engine /series`（→ Wiki，Wiki 强制 admin/mod）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/tag` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `PUT /api/v1/official` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `PUT /api/v1/engine` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `PUT /api/v1/series/:id` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 4. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/user/:id/follow` | 登录 | `userH.Follow` | 已修 | 被关注者无本地 user 行时原泄露 Postgres FK 串（SQLSTATE 23503）→ 识别 FK 冲突返回 `用户不存在`；自关注 guard + 已关注 guard 正常 |

## 5. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/message/read` | 登录 | `messageH.MarkAsRead` | 对齐 | `{type}`（或 all）；仅本人未读行（实测）|

## 6. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/admin/comment/:id` | 管理 | `adminH.UpdateComment` | 对齐 | （低）DB 错误→400、改不存在 id 静默成功（可选硬化）|
| `PUT /api/v1/admin/comment/:id/approve` | 管理 | `patchH.ApproveComment` | 对齐 | 幂等（status==0 early-return）；重复 approve 不重复通知（createDedupMessage）|
| `PUT /api/v1/admin/resource/:id` | 管理 | `adminH.UpdateResource` | 对齐 | 仅改 note（有意）；（低）同 UpdateComment 错误映射 |
| `PUT /api/v1/admin/setting/comment-verify` | 仅admin | `adminH.SetCommentVerify` | 对齐 | admin-only；upsert site_setting |
| `PUT /api/v1/admin/setting/creator-only` | 仅admin | `adminH.SetCreatorOnly` | 对齐 | admin-only；实测自反转（开→读→关→读）|

## 7. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PUT /api/v1/chat/room/:link/seen` | 登录 | `chatH.MarkSeen` | 对齐 | 成员鉴权 + 房内 id 过滤 + OnConflict 幂等；无 FE 调用方（已读回执未接，dead-but-correct）|
| `PUT /api/v1/chat/message/:id` | 登录 | `chatH.UpdateMessage` | 对齐 | 仅发送者可编辑；同事务写编辑历史；无 IDOR |
