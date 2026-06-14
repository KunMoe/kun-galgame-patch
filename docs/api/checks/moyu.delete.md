# moyu 后端 — DELETE API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：**审计完成（2026-05-29）**。详细逐端点报告见 [`_audit/`](./_audit/)。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：对齐 = 无问题 · 已修 = 已修复 · 保持 = 有意保持 · 待审计 · 新增
鉴权：公开 · 可选登录 = OptionalAuth · 登录 · 管理 = admin/mod · 仅admin · 限流

## 统计

- 本服务 DELETE 端点：**14**
  - 补丁/评论/资源 3 · Galgame 代理 3 · 分类代理 4 · 用户 1 · 管理 2 · 聊天 1
- 本轮：已修复 2（`/user/:id/follow` 计数损坏、`/admin/comment/:id` 计数漂移）· 代理 6 · 其余

---

## 1. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/patch/:id` | 登录 | `patchH.DeletePatch` | 对齐 | owner/admin；删前先排空 S3（resource + file-history old key），CASCADE 子表（未执行，静态核对）|
| `DELETE /api/v1/patch/comment/:commentId` | 登录 | `patchH.DeleteComment` | 对齐 | owner/privileged；按 status=0 计数递减；CASCADE 回复（实测回滚验证）|
| `DELETE /api/v1/patch/resource/:resourceId` | 登录 | `patchH.DeleteResource` | 对齐 | owner/privileged；-3 萌萌点给资源作者(`content_removed`)，同 ref 供 OAuth 对账 |

## 2. Galgame 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/galgame/:gid` | 登录 | `patchH.DeleteGalgameDraft` | 对齐 | 删草稿投稿（代理 Wiki）|
| `DELETE /api/v1/galgame/:gid/links` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理（Wiki 强制 owner/admin）|
| `DELETE /api/v1/galgame/:gid/aliases` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 3. 分类代理 `/tag /official /engine /series`（→ Wiki，Wiki 强制 admin/mod）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/tag/:id` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `DELETE /api/v1/official/:id` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `DELETE /api/v1/engine/:id` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `DELETE /api/v1/series/:id` | 登录 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 4. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/user/:id/follow` | 登录 | `userH.Unfollow` | 已修 | **无关注关系也照样 follower_count -1**（DeleteFollow 忽略 RowsAffected）→ 任意人可刷低他人粉丝数（实测复现 11→10）。改：返回 rowsAffected，仅确有删除才扣计数 |

## 5. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/admin/comment/:id` | 管理 | `adminH.DeleteComment` | 已修 | 原不递减 `patch.comment_count` → 计数只增不减漂移。改：事务内补齐递减（镜像用户侧）|
| `DELETE /api/v1/admin/resource/:id` | 管理 | `adminH.DeleteResource` | 对齐 | S3 best-effort 清理（snapshot key→删→WARN）；（低）同 comment 类计数未减，但资源列表通常重取，影响小 |

## 6. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `DELETE /api/v1/chat/message/:id` | 登录 | `chatH.DeleteMessage` | 对齐 | 发送者 OR admin/mod 软删；status=DELETED + deleted_at/by；FE 渲染墓碑 |
