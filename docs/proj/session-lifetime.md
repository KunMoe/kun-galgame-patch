# 会话寿命：滑动会话（2026-06-12）

> 本仓自有工程笔记（**非** infra 镜像）。记录 moyu 本地会话的生命周期模型与
> 2026-06 的「每周掉线」修复。kungal（kun-galgame-forum）有一份对称实现，见其
> `docs/proj/session-lifetime.md`。

## 背景：moyu 的会话是 BFF 不透明会话

moyu「不签发任何 token、只验签」指的是它不自建登录；但它确实维护**自己的
Backend-for-Frontend（BFF）会话**：

- 登录走完 OAuth code 交换后，后端在 **Redis** 里存 `SessionData`（OAuth
  access/refresh token + `id`/`sub`），key 为 `moyu:session:<id>`；浏览器只拿到一个
  **httpOnly 不透明 cookie** `moyu_session`。OAuth 的 access/refresh token **永不落到
  浏览器**；角色从会话里 access token 的 JWT `roles` claim 解出（`decodeJWTRoles`）。
- 每个请求,`Auth` 中间件取 Redis 会话，按**两档**刷新 access token（见
  `refreshOAuthToken`）：
  - **硬过期**（`now >= ExpiresAt`）：**同步**刷新，失败且会话确已死则清 cookie 退出。
  - **软窗口**（`T-5min .. T`）：**后台 goroutine** 刷新，放行本次请求。

> cookie 名 / 前缀刻意与 kungal 不同：本地 dev 两站同在 127.0.0.1、共享一个 Redis，
> cookie 按域不按端口隔离，前缀撞了会互相读写/删对方会话。务必保持站点唯一。

## 修复的 Bug：固定 7 天 cookie ⇒ 每周必掉线

旧实现里 `SessionTTL = 7 天`，`CreateSession` 把 cookie `MaxAge` 和 Redis TTL 都设成
7 天，**登录后再不重发 cookie**。于是无论多活跃，**登录满 7 天浏览器丢 cookie →
掉线**。上游 OAuth 本支持 90 天滑动 refresh token（infra
`oauth_clients.refresh_token_ttl_seconds` 默认 7776000），是本地这层砍到了一周。

## 现在：90 天滑动窗口 + 上游 refresh token 兜底

会话改成**滑动**（参考 OWASP 会话管理 / ASP.NET Core `SlidingExpiration`）：

1. **窗口 = 90 天**，对齐上游 refresh token。`SessionTTL = 90 * 24 * time.Hour`，
   `CreateSession` 的 cookie `MaxAge` 与 Redis TTL、`refreshOAuthToken` 的回写都引用它。
2. **活跃即续期**：`renewSlidingSession` 在 `Auth` / `OptionalAuth` 校验通过后调用，
   把 cookie 和 Redis TTL 一起向前滑动。
3. **「过半才续签」节流**：marker key `moyu:session-renew:<id>`（TTL = 半窗口）做节流，
   `SetNX` 只在距上次续期 > `SessionTTL/2` 时成功，避免每个请求都 `Set-Cookie`。
4. **续期只 `EXPIRE`、不重写会话内容** ——**这一点对 moyu 尤其关键**：moyu 的软窗口
   刷新跑在**后台 goroutine** 里，会和续期并发。若续期也重写会话 blob，就可能用旧
   token 覆盖 goroutine 刚轮换出的新 refresh token → 下次刷新被永久拒 → 掉线。续期
   只对 Redis key 做 `EXPIRE`（只动 TTL、不动值），与 goroutine 刷新**零竞态**。
5. **绝对上限 = 上游 refresh token，fail-closed**：本地不设硬上限。活跃用户两边都滑动
   → 实际不掉线；闲置约 90 天后，本地 Redis 过期与上游 refresh token 失效**同时发生**，
   `refreshOAuthToken` 删会话、用户重新登录。

净效果：**活跃用户不再每周掉线**；闲置约 90 天后自然过期（与上游一致）。

## 平滑迁移

`renewSlidingSession` 不依赖任何新会话字段，`SessionData` 结构未变。线上既有的 7 天
cookie 会话，下次请求时被续成 90 天滑动窗口，**无需迁移脚本**。

marker key 用独立前缀 `moyu:session-renew:`，**不**匹配 `RevokeUserSessions` 的
`moyu:session:*` 扫描，因此管理员清号的会话吊销逻辑不受影响。

## 安全权衡

被盗 cookie 有效期从 7 天变为「最长 90 天滑动」，但只是对齐上游 OAuth 本就允许的窗口，
未放大授权；cookie 仍 httpOnly + Secure（prod）+ SameSite=Lax，封禁 / refresh token
失效在下次刷新即时 fail-closed。后续可选硬化：定期轮换 session id（OWASP renewal
timeout）——本次未做。

## 改了哪些文件

| 文件 | 改动 |
|---|---|
| `internal/middleware/auth.go` | `SessionTTL` 7d→90d；新增 `sessionRenewPrefix`、`renewSlidingSession`；`Auth`/`OptionalAuth` 调用续期。`CreateSession`/`refreshOAuthToken` 本就引用 `SessionTTL`，自动跟随 |

`SecureCookies` 本仓已有（`internal/app/app.go` 按 `Server.Mode` 设置），续期 cookie 直接复用。
