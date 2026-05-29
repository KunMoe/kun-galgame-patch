# moyu 后端 — POST API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：**审计完成（2026-05-29）**。详细逐端点报告见 [`_audit/`](./_audit/)。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 POST 端点：**37 → 36**（`POST /message/` 因安全问题删除）
  - 认证 3 · 补丁/评论/资源 3 · Galgame 代理 6 · 分类代理（基础 5 + 回滚 4）9 · 用户 2 · 消息 1→0 · 管理 1 · 聊天 5 · 上传 6 · 搜索 1

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/auth/oauth/callback` | 🌐 | `authH.OAuthCallback` | ✅ | 校验/换码错误路径实测；成功路径需真实 OAuth code（静态核对，banned→10014→/account-banned）|
| `POST /api/v1/auth/logout` | 🌐 | `authH.Logout` | ✅ | 未实测（会销毁审计会话）；静态核对：revoke refresh_token + 销毁 session + 清 cookie |
| `POST /api/v1/auth/me/avatar` | 🔒 | `authH.UploadAvatar` | ✅ | 代理 OAuth multipart（raw body 透传，boundary 存活）；无文件→OAuth 原样回 400 实测 |

## 2. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/patch/` | 🔒 | `patchH.CreatePatch` | ✅ | `{vndb_id}`→`{id}`；缺失走 44001 CTA；+3 萌萌点幂等(`moyu:patch_create:<id>`)|
| `POST /api/v1/patch/:id/comment` | 🔒 | `patchH.CreateComment` | 🔧 | **CRITICAL**：DTO `galgame_id` 标 `required`，校验早于路径注入→评论恒 422 不可用。改 `omitempty`（实测创建成功，已回滚测试数据）|
| `POST /api/v1/patch/:id/resource` | 🔒 | `patchH.CreateResource` | ✅ | s3 分支校验 key 前缀；+3 萌萌点(`moyu:resource_publish:<id>`)、贡献者、通知收藏者（去重）|

## 3. Galgame 投稿 / 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/galgame/submit` | 🔒 | `patchH.SubmitGalgame` | ✅ | 代理 Wiki，回传 20003/4/6/7/8/9 |
| `POST /api/v1/galgame/:gid/claim` | 🔒 | `patchH.ClaimGalgame` | ✅ | +3 萌萌点；RegisterClaimedGalgame 已存在则 early-return，无重复奖励（实测）|
| `POST /api/v1/galgame/:gid/revert` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理（Wiki 强制 admin/mod）|
| `POST /api/v1/galgame/:gid/prs` | 🔒 | `patchH.WikiPRSubmit` | ✅ | JSON / multipart(`data`+`file` banner) 均支持，10MB 上限 |
| `POST /api/v1/galgame/:gid/links` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理（Wiki 强制 owner/admin）|
| `POST /api/v1/galgame/:gid/aliases` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

## 4. 分类代理 `/tag /official /engine /series`（→ Wiki）

### 4.1 创建（5）—— 任意登录用户

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/tag` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/official` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/engine` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/series/modal` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理（literal 先于 `:id`）|
| `POST /api/v1/series` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

### 4.2 回滚（4 = 4 实体 × 1）—— Wiki 强制 admin/mod

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/tag/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/official/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/engine/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `POST /api/v1/series/:id/revert` | 🔒 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

## 5. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/user/image` | 🔒 | `userH.UploadImage` | ✅ | 文件上传未实测；`daily_image_count` 限 20/日；被 `/upload/image-service` 取代，无 FE 调用方 |
| `POST /api/v1/user/check-in` | 🔒 | `userH.CheckIn` | ✅ | `{moemoepoint}`；DB flag 先置位再异步发奖（幂等键 `moyu:checkin:<uid>:<date>`，replay-safe）|

## 6. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| ~~`POST /api/v1/message/`~~ | ~~🔒~~ | ~~`messageH.CreateMessage`~~ | 🔧 | **已删除**：任意登录用户可向任意收件箱写任意通知（recipient_id 可控、无限流、无 FE 调用方）—— 垃圾/钓鱼面。整条死链一并移除 |

## 7. 管理 `/admin`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/admin/user/:id/purge` | ⚙️ | `adminH.PurgeUser` | ✅ | 未执行（不可逆）；静态核对：单事务、清 RESTRICT FK、重算计数、撤销 session，admin-only |

## 8. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/chat/room` | 🔒 | `chatH.CreateRoom` | ✅ | admin-only（`HasRole("admin")`）；无 FE 调用方 |
| `POST /api/v1/chat/room/join` | 🔒 | `chatH.JoinRoom` | ✅ | OnConflict DoNothing 幂等 |
| `POST /api/v1/chat/room/private` | 🔒 | `chatH.StartPrivate` | ✅ | 自聊双重拦截；link 归一 `<low>-<high>`；唯一索引 + race fallback |
| `POST /api/v1/chat/room/:link/message` | 🔒 | `chatH.CreateMessage` | ✅ | 空内容+空文件拒；返回富化单条 |
| `POST /api/v1/chat/message/:id/reaction` | 🔒 | `chatH.ToggleReaction` | 🔧 | **IDOR**：原未校验房间成员，可对私聊房间他人消息加表情（实测复现）→ 加 `IsMember` 校验 |

## 9. 上传 `/upload`（组级 `auth`，D10：minio-go 预签名直传）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/upload/small/init` | 🔒 | `uploadH.InitSmall` | ✅ | 扩展名白名单 + 200MB + 配额预检；shape 对齐（实测）|
| `POST /api/v1/upload/small/complete` | 🔒 | `uploadH.CompleteSmall` | 🔧 | 幂等标记早于扣减→扣减瞬时失败后重试不扣配额。改：失败 defer 释放标记 |
| `POST /api/v1/upload/multipart/init` | 🔒 | `uploadH.InitMultipart` | 🔧 | `part_count` 未与 `file_size` 核对（放大）→ 强制 `== ceil(size/10MiB)`（实测）|
| `POST /api/v1/upload/multipart/complete` | 🔒 | `uploadH.CompleteMultipart` | 🔧 | 同 small/complete（共用 `verifyAndFinalize`）|
| `POST /api/v1/upload/multipart/abort` | 🔒 | `uploadH.AbortMultipart` | ✅ | 无配额；key 为不可猜随机串 |
| `POST /api/v1/upload/image-service` | 🔒 | `uploadH.UploadImageService` | ✅ | 校验实测；真实上传需上游 image_service（静态核对，UploadResult 对齐）|

## 10. 搜索

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `POST /api/v1/search` | 🌐 | `searchH.Search` | ✅ | 代理 Wiki Meilisearch；shape 对齐 |
