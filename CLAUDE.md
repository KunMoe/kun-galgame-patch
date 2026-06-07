# kun-galgame-patch（moyu）— AI 代理项目指南

galgame **补丁 / 资源站**。`apps/api` = Go Fiber v3 + GORM + Postgres，`apps/web` = Nuxt 4。
本仓是 **kun-galgame-infra（OAuth / 身份 / 契约中枢）的下游**之一（另一个是 kun-galgame-forum / kungal）。

## 跨服务契约（不可违反 — 由 kun-galgame-infra 拥有）

权威契约文档以**只读镜像**同步在 `docs/{oauth,image_service,galgame_wiki}/`（文件头有 GENERATED banner）。
**改契约请去 infra 源头改，别动这里的副本**；副本由 kungal-docs 的 `pnpm docs:sync` 重新生成。核心不变量：

- **身份（C1/C2）**：`user.id` 在本库、OAuth、另一下游中是**同一个整数**——永不重新编号用户；本地表用 `*_user_id` 对齐 OAuth `users.id`。OAuth 拥有身份并签发 JWT，本服务只**验签、不签发任何 token**（见 `internal/middleware/auth.go`）。
- **用户资料（C6）**：**不持久化到本地 user 表、不当真源**（短 TTL 内存缓存可以——`pkg/userclient` 已内置约 10min TTL）；按 id 列表走 `GET /users/batch`（OAuth Client Basic Auth，≤100 个 id，**不返回** email / moemoepoint / created_at）。@提及补全用 `GET /users/search`（**勿缓存**）；当前用户用 `/oauth/userinfo`。OAuth 不发 SDK，自己实现薄客户端。
- **萌萌点 moemoepoint（C3）**：每用户单一余额，**单源在 OAuth**；本地若有该列只是缓存视图。发放/扣除走 s2s API，幂等键 = `<app>:<event>:<ref>`（如 `moyu:wiki_approved:1207`）。下游可用 reason：`content_approved` / `content_removed` / `daily_checkin` / `liked`；**OAuth 保留、s2s 禁用**：`admin_grant` / `admin_deduct` / `migration` / `register_gift`。s2s 端点**已实现**（`POST/GET /users/:id/moemoepoint`，`Adjust` 幂等；见 infra `internal/platform/auth/handler/moemoepoint_handler.go` 与 `cmd/oauth/main.go`）。⚠️ 源文档 `06-moemoepoint.md` 仍误标「待实现」，**以代码为准**（待在 infra 修正）。
- **图片（C4）**：内容寻址图床在 OAuth，**moyu 不自建 S3**，头像 / 图片均用 OAuth 图床。URL = `{base}/{aa}/{bb}/{hash}[_variant].webp`（两级十六进制分片）；传递 `*_image_hash` 字段，用 image client 解析。
- **Wiki 消息（C5）**：galgame-wiki 拥有 `galgame_message`；消费 `GET /galgame/messages/mine`（通知中心）与 `/galgame/messages/feed`（cron）。无 target 的消息只进 admin 队列。

完整细节见 `docs/oauth/`、`docs/image_service/`、`docs/galgame_wiki/`。

## 本仓要点

- **迁移后极简鉴权**：无本地登录 / 2FA，**不签发任何 token**（身份完全归 OAuth，本服务只验签）。
- `docs/image_service/03-api-design.md` 是**本仓自有变体**（记录「运行实现用扁平信封 `{code,message}`、非设计稿的 `{error}` 嵌套」这一实现修正 + moyu `pkg/imageclient` 踩坑）。它**不是镜像**，`docs:sync` 已配置跳过它，**勿删勿覆盖**；其余 `docs/{oauth,image_service,galgame_wiki}/` 文件都是 infra 镜像，要改去 infra 改。
