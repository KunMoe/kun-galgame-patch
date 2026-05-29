# moyu 后端 — PATCH API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.get.md](./moyu.get.md) · [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [README](./README.md)
>
> 状态：全部 ⏳ 待审计（inventory）。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 PATCH 端点：**2**
  - 认证 1 · Galgame 代理 1

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PATCH /api/v1/auth/me` | 🔒 | `authH.UpdateMe` | ⏳ | 代理 OAuth 改**展示层**（仅 name / bio）；身份层（改密码/邮箱/2FA/注销）不代理，跳转 oauth.kungal.com/profile |

## 2. Galgame 投稿代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `PATCH /api/v1/galgame/:gid` | 🔒 | `patchH.PatchGalgameDraft` | ⏳ | 改草稿投稿（代理 Wiki；草稿态走 PATCH，已发布走 PUT）|
