# moyu 后端 — POST API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：全部 ⏳ 待审计（inventory）。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 POST 端点：**37**
  - 认证 3 · 补丁/评论/资源 3 · Galgame 代理 6 · 分类代理（基础 5 + 回滚 4）9 · 用户 2 · 消息 1 · 管理 1 · 聊天 5 · 上传 6 · 搜索 1

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/auth/oauth/callback` | 🌐 | `authH.OAuthCallback` | ⏳ | OAuth 授权码回调 → 建立 `kun_session` |
| `POST /api/v1/auth/logout` | 🌐 | `authH.Logout` | ⏳ | 登出（读 session 销毁；无 auth 中间件）|
| `POST /api/v1/auth/me/avatar` | 🔒 | `authH.UploadAvatar` | ⏳ | 代理 OAuth 上传头像（展示层）|

## 2. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/patch/` | 🔒 | `patchH.CreatePatch` | ⏳ | 创建补丁（D12：JSON `{vndb_id}`）|
| `POST /api/v1/patch/:id/comment` | 🔒 | `patchH.CreateComment` | ⏳ | 发评论（受“评论需审核”开关影响）|
| `POST /api/v1/patch/:id/resource` | 🔒 | `patchH.CreateResource` | ⏳ | 发资源（+3 萌萌点）|

## 3. Galgame 投稿 / 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/galgame/submit` | 🔒 | `patchH.SubmitGalgame` | ⏳ | 投稿新 galgame（代理 Wiki，回传 20003/4/6/7/8/9）|
| `POST /api/v1/galgame/:gid/claim` | 🔒 | `patchH.ClaimGalgame` | ⏳ | 认领 galgame（+3 萌萌点）|
| `POST /api/v1/galgame/:gid/revert` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：回滚修订（Wiki 强制 admin/mod）|
| `POST /api/v1/galgame/:gid/prs` | 🔒 | `patchH.WikiPRSubmit` | ⏳ | 代理：提 PR |
| `POST /api/v1/galgame/:gid/links` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：加关联链接（Wiki 强制 owner/admin）|
| `POST /api/v1/galgame/:gid/aliases` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 代理：加别名 |

## 4. 分类代理 `/tag /official /engine /series`（→ Wiki）

### 4.1 创建（5）—— 任意登录用户

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/tag` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 创建标签 |
| `POST /api/v1/official` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 创建会社 |
| `POST /api/v1/engine` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 创建引擎 |
| `POST /api/v1/series/modal` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 弹窗内快速建系列 |
| `POST /api/v1/series` | 🔒 | `patchH.WikiEditProxy` | ⏳ | 创建系列 |

### 4.2 回滚（4 = 4 实体 × 1）—— Wiki 强制 admin/mod

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/tag/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏳ | |
| `POST /api/v1/official/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏳ | |
| `POST /api/v1/engine/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏳ | |
| `POST /api/v1/series/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏳ | |

## 5. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/user/image` | 🔒 | `userH.UploadImage` | ⏳ | 个人页配图（`daily_image_count` 限额 20/日）|
| `POST /api/v1/user/check-in` | 🔒 | `userH.CheckIn` | ⏳ | 每日签到（0–7 萌萌点，经 OAuth）|

## 6. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/message/` | 🔒 | `messageH.CreateMessage` | ⏳ | 创建消息 |

## 7. 管理 `/admin`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/admin/user/:id/purge` | ⚙️ | `adminH.PurgeUser` | ⏳ | 清除用户全部 moyu 侧痕迹（反 spam）；账户级 → 仅 admin |

## 8. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/chat/room` | 🔒 | `chatH.CreateRoom` | ⏳ | 建房 |
| `POST /api/v1/chat/room/join` | 🔒 | `chatH.JoinRoom` | ⏳ | 加入房间 |
| `POST /api/v1/chat/room/private` | 🔒 | `chatH.StartPrivate` | ⏳ | 发起私聊 |
| `POST /api/v1/chat/room/:link/message` | 🔒 | `chatH.CreateMessage` | ⏳ | 发消息 |
| `POST /api/v1/chat/message/:id/reaction` | 🔒 | `chatH.ToggleReaction` | ⏳ | 表情回应 |

## 9. 上传 `/upload`（组级 `auth`，D10：minio-go 预签名直传）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/upload/small/init` | 🔒 | `uploadH.InitSmall` | ⏳ | 小文件预签名初始化 |
| `POST /api/v1/upload/small/complete` | 🔒 | `uploadH.CompleteSmall` | ⏳ | 完成（SETNX 去重防双计 `daily_upload_size`）|
| `POST /api/v1/upload/multipart/init` | 🔒 | `uploadH.InitMultipart` | ⏳ | 分片初始化 |
| `POST /api/v1/upload/multipart/complete` | 🔒 | `uploadH.CompleteMultipart` | ⏳ | 分片完成 |
| `POST /api/v1/upload/multipart/abort` | 🔒 | `uploadH.AbortMultipart` | ⏳ | 分片中止 |
| `POST /api/v1/upload/image-service` | 🔒 | `uploadH.UploadImageService` | ⏳ | 图片转 image_service（截图编辑器用）|

## 10. 搜索

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/search` | 🌐 | `searchH.Search` | ⏳ | 全文搜索（Meilisearch）|
