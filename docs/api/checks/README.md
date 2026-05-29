# API 字段对齐审计

> 目的：逐方法记录 moyu 后端**全部 API 端点** + FE↔BE 字段对齐审计状态。仿照
> [`kun-oauth-admin/docs/api/checks/`](../../../../kun-oauth-admin/docs/api/checks/README.md)。
>
> 当前进度：**审计完成（2026-05-29）** —— 全部 **159 端点**
> （GET 80 / POST 37 / PUT 26 / DELETE 14 / PATCH 2）已逐条审计：静态读码
> （handler → service → repository → model/dto ↔ 前端 `.d.ts` + 实际调用方）
> + 对**运行中的整套本地栈**做动态验证（GET 全部实测；变更类用一次性测试数据
> 跑通并回滚）。
>
> 路由唯一来源：`apps/api/internal/app/router.go`（`RegisterRoutes`）。该文件之外**无**任何
> 路由注册（仅 `app.Use` 挂中间件），无 `/health`、无静态文件服务。
>
> 逐域详细报告见 [`_audit/`](./_audit/)（每个功能域一份 Markdown，含每个端点的
> 实测结果 / 证据 file:line / 建议）。

## 审计结论（2026-05-29）

共发现并**修复 17 处问题**（其中 1 critical / 6 high / 4 medium / 6 low），
其余端点对齐无误或为有意的代理透传。所有修复均已 `go build` / `go test` /
前端 `typecheck` 通过，并在运行栈上实测确认（air 热重载）。

### 🔴 已修复 —— 阻断级 / 高危

| # | 端点 | 问题 | 修复 |
|---|---|---|---|
| 1 | `POST /patch/:id/comment` | **评论功能完全不可用**：DTO `galgame_id` 标了 `required`，校验在 handler 从路径注入之前运行，前端只发 `{content}` → 恒返回 `40000 "GalgameID is required"` | BE：`galgame_id` 改 `omitempty`（路径参数才是权威来源）|
| 2 | `POST /message/` | 任意登录用户可向**任意用户收件箱**写任意通知（recipient_id/type/content/link 全可控、无限流、无 FE 调用方）—— 垃圾/钓鱼面 | BE：**删除该路由 + 整条死链**（handler/service/repo/dto）。合法通知由 patch 服务 `createDedupMessage` 内部产生 |
| 3 | `DELETE /user/:id/follow` | 即使没有关注关系也会把对方 `follower_count -1`（`DeleteFollow` 忽略 RowsAffected）→ 任何人可刷低/骚扰他人粉丝数（实测复现）| BE：`DeleteFollow` 返回 rowsAffected，仅在确有删除时才扣计数 |
| 4 | `GET /home` `GET /resource` `GET /resource/:id`(recs) `GET /user/:id/resource` | 公开列表流逐行下发完整下载载荷（`content` 直链 + `code` + `password` + `s3_key`），前端卡片根本不读 → 爬虫翻页即可批量收割，彻底架空限流的 `/patch/resource/:id/link` | BE：新增 `patchModel.StripResourceSecrets`，在这些 feed 上清空四个秘密字段；保留单资源详情主体与 `/patch/:id/resource`（前端就地渲染的揭示面）|
| 5 | `POST /chat/message/:id/reaction` | **IDOR**：未校验房间成员，任何登录用户可对**私聊房间**里不属于自己的消息加/去表情（实测复现）| BE：`ToggleReaction` 增加 `IsMember` 校验 |
| 6 | `GET /admin/stats` & `GET /admin/stats/sum` | json key 与前端不符：`new_patch_resource`/`patch_resource_count`/`patch_comment_count` ↔ 前端读 `new_resource`/`resource_count`/`comment_count` → 三块统计卡恒显 0 | BE：改 3 个 json tag 对齐前端 |
| 7 | `GET /message`（@消息页）| `mention.vue` 把响应当裸数组 `Message[]`，但 BE 返回分页 `{items,total}` → @消息页永远空 | FE：`mention.vue` 改用 `{items,total}`（对齐其余 4 个消息页）|

### 🟠 已修复 —— 中危

| # | 端点 | 问题 | 修复 |
|---|---|---|---|
| 8 | `PUT /user/:id/follow` | 被关注者无本地 user 行时把原始 Postgres FK 报错串（SQLSTATE 23503）回给前端 | BE：识别 FK 冲突 → 返回 `用户不存在` |
| 9 | `DELETE /admin/comment/:id` | 管理员删评论不递减 `patch.comment_count` → 计数只增不减漂移 | BE：事务内补齐 `comment_count` 递减（镜像用户侧逻辑）|
| 10 | `GET /patch/resource/:resourceId/link` | 限流键始终落 IP（路由前无 auth/optionalAuth，`GetUserID` 恒 0），与"按 userID 30/min"的注释不符 → 同 NAT 后登录用户被合并限流 | BE：路由加 `optionalAuth`（在限流中间件之前）|
| 11 | `/admin/comment`（comment.vue）| `c.user.name` 无防护，而 BE `user` 为 `omitempty`（OAuth brief 取不到时为空）→ 空指针崩页 | FE：`c.user?.name` + `v-if="c.user"` |

### 🟡 已修复 —— 低危

| # | 端点 | 问题 | 修复 |
|---|---|---|---|
| 12 | `GET /about/post` | `..` 路径穿越探测返回 50000（应 4xx）| BE：改返回 `os.ErrNotExist` → 404 |
| 13 | `POST /upload/{small,multipart}/complete` | SETNX 幂等标记在配额扣减**之前**置位；扣减若瞬时失败，重试走 `!first` 直接成功而**不扣配额** → `daily_upload_size` 少计 | BE：失败路径释放标记（defer），仅成功才保留 |
| 14 | `POST /upload/multipart/init` | `part_count` 不与 `file_size` 核对，可请求上万个分片 URL（放大）| BE：`part_count` 必须等于 `ceil(file_size / 10MiB)` |
| 15 | `GET /patch/:id/detail` | 空 `tags`/`officials`/`wiki_engine_ids` 序列化为 JSON `null`（FE 类型为非空数组，`.map/.length` 会炸）| BE：enricher 初始化为 `[]` |
| 16 | `GET /tag/:name` `GET /official/:name`（降级卡）| `CardFromBrief` 的 `type/language/platform` 为 `null`（FE 类型 `string[]`）| BE：初始化为 `[]` |
| 17 | `GET /chat/room/:link/message`（`ids` 模式）| 不带 `limit` 时被 `min=1` 提前拒（handler 默认值成死代码）| BE：`limit` 改 `omitempty,min=1,max=100` |

### 已知低优先级遗留（不影响使用，已记录未改）

- `GET /user/:id/{patch,resource,favorite,comment,contribute}` 共享 DTO 的 `min=1`
  使 handler 的"缺省补 1"成死代码：省略 `page/limit` 会 422。前端始终传参，故无实际影响。
- 嵌套"回复的回复"删除时 `comment_count` 可能少减（前端无多级回复 UI，潜在）。
- `PUT /admin/comment/:id` `PUT /admin/resource/:id`：DB 错误映射成 400、改不存在 id 静默成功（可选硬化）。
- `GET /tag/:name` 降级卡 `created` 为零值时间（前端应做哨兵判断）。
- `GET /hikari`（外部 API）：仅对 `s3` 存储清 `content`，netdisk 的 `content/code/password` 仍下发（疑为对老 moyu 的有意共享面）。
- 未匹配路由（如已删的 `POST /message`）经全局 error handler 统一返回 500（非 404）——
  这是 `globalErrorHandler` 对所有未注册路由的既有行为，无前端调用方受影响。

### 无前端调用方（dead-but-correct，供产品确认）

`GET /user/search`、`GET /user/:id/floating`、`POST /user/image`（被 `/upload/image-service` 取代）、
`GET /galgame/messages/*`、`PUT /chat/room/:link/seen`（已读回执未接）、`POST /chat/room`、
`PUT/DELETE /patch/comment/*`（无评论编辑/删除 UI）。逻辑均正确，仅暂无界面入口。

### 无法动态测试（已静态核对）

需真实 OAuth 授权码：`POST /auth/oauth/callback` 成功路径、`POST /auth/logout`（会销毁审计会话）。
需真实文件上传：`POST /auth/me/avatar`、`POST /user/image`、`POST /upload/image-service`、
`/upload/*/complete`。不可逆销毁类（仅给出 ready curl，未执行）：`POST /admin/user/:id/purge`、
`DELETE /patch/:id`、各 Wiki 写代理（merge/decline PR、revert、删 tag/official 等）。

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
> 结论：代理层全部正确（NSFW gate 对 `:gid` 子资源 fail-closed、路由顺序 literal 先于 `:param`、
> Wiki 错误码 flatten 到 HTTP 400 但保留 code+message、query/path/body 字段名透传无误）。

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
| 消息 `/message` | 3 | 1→0 | 1 | — | — | 5→4 |
| 管理 `/admin` | 11 | 1 | 5 | 2 | — | 19 |
| 公共 `/home /galgame /comment /resource /ranking` | 8 | — | — | — | — | 8 |
| 聊天 `/chat` | 3 | 5 | 2 | 1 | — | 11 |
| 上传 `/upload` | — | 6 | — | — | — | 6 |
| 搜索/外部/关于 | 4 | 1 | — | — | — | 5 |
| **合计（审计时）** | **80** | **37** | **26** | **14** | **2** | **159** |

> 注：`POST /message` 因安全问题被删除，现存 POST 为 **36**、总计 **158**。

## 图例 — 审计状态

- ✅ 已审计，对齐无问题
- 🔧 已审计，发现问题并修复（本轮）
- ⏭️ 已审计，有意保持当前行为（多为 Wiki/OAuth 代理透传）
- ⏳ 待审计
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

> moyu 的 `Auth`/`OptionalAuth` 基于 `moyu_session`（Redis 会话，httpOnly cookie），
> 校验会话并按需后台刷新上游 OAuth token；非纯 JWT 验签。详见
> `internal/middleware/auth.go`。角色映射（`docs/user-migration/02-data-mapping.md §7`）：
> legacy role 4 → `admin`，role 3 → `moderator`。

## 配套清单

- [moyu.get.md](./moyu.get.md) — GET（80）
- [moyu.post.md](./moyu.post.md) — POST（37 → 36）
- [moyu.put.md](./moyu.put.md) — PUT（26）
- [moyu.delete.md](./moyu.delete.md) — DELETE（14）
- [moyu.patch.md](./moyu.patch.md) — PATCH（2）
- [_audit/](./_audit/) — 逐域详细审计报告（12 份）
