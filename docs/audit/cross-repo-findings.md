# moyu 跨仓审计发现核对

> 来源：`../kungal-docs/claude` 与 `../kungal-docs/gpt` 两份外部审计（对
> `kun-oauth-admin` / `kun-galgame-nuxt4` / **`kun-galgame-patch-next`(moyu，本仓)** 三仓的只读安全/正确性审计）。
>
> 本文**只摘录与本仓（moyu）有关的发现**，并逐条以**当前代码**核对真伪、给出当前
> `file:line` 证据与处理建议。核对方式：逐条读源码 + 对外部审计的“证伪项”反向确认。
>
> 与 [`docs/api/checks/`](../api/checks/README.md)（2026-05-29 本仓 API 字段对齐审计 + 修复）
> 的关系见 [§4](#四-与本仓-api-审计的关系)。核对日期：2026-05-30。
>
> 分类：**成立(OPEN)** 待处理 · **有意(BY-DESIGN)** 机制属实但为有意权衡 · **已证伪** 外部审计判为不成立、本文复核确认。

## 摘要

外部审计列入本仓（moyu）的发现共 **22 条**（HIGH 2 / MEDIUM 4 / LOW 16，含 cross 跨仓项的 moyu 部分）+ 证伪 5 条。本文逐条以当前代码复核结论：

| 结论 | 数量 | 说明 |
|---|---:|---|
| 成立(OPEN)，建议处理 | 19 | 其中 **HIGH 2**（F004、GPT-H01）/ MEDIUM 1（F025）/ LOW 16 |
| 有意(BY-DESIGN)/可接受 | 3 | F072、F024、F032（F024/F032 实为同一议题）|
| 已证伪（复核确认非缺陷）| 5 | F006、F026、F027、F028、F071（行删除正确；多级回复一旦上线有潜在计数漂移）|

> 合计 22 条 moyu 相关发现（19 OPEN + 3 BY-DESIGN）+ 5 条证伪。

## 修复进度（2026-05-30）

19 条 OPEN 中已修复 **17 条**（`go build`/`go test`/`go vet` 全绿；关键项已在重启后的本地栈实测）。
余 2 条 OPEN 为有意保持的 best-effort/广面项（见下）。

| 状态 | 发现 | 说明 |
|---|---|---|
| 🔧 已修 + 实测 | **F004** | `CreateMessage` 校验引用消息属于本房间 + `enrichMessages` 按房间过滤引用。实测：跨房间引用 → `400 无效的引用消息`（0 行写入），同房间引用 → 200 正常 |
| 🔧 已修 | **GPT-H01** | `pkg/imageclient` 改解 `{code,message,data}` 信封（成功取 `data`，错误取扁平 `{code,message}` + 整数码映射）；订正 `docs/image_service/03-api-design.md`。（image_service 当前未运行，按 image_service 自带测试确认的信封 + 代码逻辑核对；上传链路待上游起来后实测）|
| 🔧 已修 | **F025** | wiki 同步改**逐消息事务**：保留 exactly-once，但在途 OAuth HTTP 从单事务最多 1000 次降为每事务 1 次，消除连接钉死/整页毒化 |
| 🔧 已修 + 实测 | **F029** | follow/unfollow 的关系写入 + 计数更新合进单事务。实测：follow/unfollow 计数与关系表精确同步、无关系 unfollow 被拒（计数不变）、FK→`用户不存在` |
| 🔧 已修 + 实测 | **GPT-M03 / F069** | `/resource/:id`、`/patch/resource/:id/download` 加 `optionalAuth + RateLimit(60/min)`。实测 `/resource/:id`：60 通过后 429 |
| 🔧 已修 + 实测 | **GPT-M04** | 签到改 `WHERE daily_check_in=0` 原子 check-and-set。实测：首次 `{moemoepoint:4}`、立即再签 → `already checked in today` |
| 🔧 已修 | **F066** | `CreateUser` 用 `ON CONFLICT DO NOTHING` + 回查规范行（并发首登不再 500）|
| 🔧 已修 | **F068** | nil `target_user_id` 的可执行消息记 `slog.Warn` |
| 🔧 已修 | **F070** | 评论被赞接上 `CreateLikeCommentNotification`（去重）|
| 🔧 已修 | **F073** | room list 的 `LatestMessagePerRoom` 错误改为记日志 |
| 🔧 已修 | **F074** | 四个 profile 计数 helper 改为出错记日志（不再静默 0）|
| 🔧 已修 | **F075** | `GetOrphanPatches` 捕获 count 错误 → 失败返回 500 而非假 0 |
| 🔧 已修 | **F077** | refresh 永久拒集合加入 `10014/15005/15008`，不再只靠 HTTP 状态码 |
| 🔧 已修 | **F083** | `GetRandomPatch` 对 `ErrRecordNotFound` 返回 404 而非 500 |
| 🔧 已修 | **F085** | cron `WithLocation(Asia/Shanghai)` + 签到键日期对齐同时区 |
| 🔧 已修 | **GPT-L02** | `KUN_IMAGE_SERVICE_BASE_URL`/`KUN_IMAGE_CDN_BASE` 在 prod 模式 fail-fast；`.env.example` 补齐 `KUN_IMAGE_*` |
| 🔧 部分修 | **F034** | 三个 toggle 端点（评论/资源/收藏点赞）的 not-found 由 400 改 404（其 service 仅返回 not-found，安全）。其余 ~20 处错误映射的广面治理（typed error 化）留待后续——改动面大且可能影响前端依赖的业务文案 |
| ⏭️ 保留 | **F067** | 可逆奖励按 relation id 作幂等键：与全仓 best-effort 发奖设计一致，accept-and-document（同 §1 备注）|
| ⏭️ 保留 | **F072 · F024/F032** | 见 [§二 有意保持](#二有意by-design-可接受--机制属实但为有意权衡)：聊天全局审核删除、下游"仅验签"封禁滞后窗口 |

> 注：本仓 API 端点字段审计（`docs/api/checks/`）的修复在 2026-05-29 已完成并标注；本节是 2026-05-30 对上述跨仓审计 OPEN 项的修复。

> ⚠️ **两条 HIGH 建议优先修复**：`F004`（聊天引用预览泄露任意私聊消息内容+发送者名，IDOR）、`GPT-H01`（图床客户端解析错信封 → 上传成功却回空 hash/url，截图/编辑器配图静默坏图）。两者本仓今日 API 审计**未覆盖**。

---

## 一、成立（OPEN）— 建议处理

### 🔴 高危

#### F004 · 聊天 `reply_to_id` 未做房间归属校验 → 引用预览泄露任意私聊消息（IDOR）

- 位置（当前代码）：
  - `internal/chat/service/service.go` `CreateMessage`：解析了**发帖**房间成员，但 `ReplyToID: replyToID` 原样写入，**未校验被引用消息属于本房间**。
  - `internal/chat/handler/handler.go:124` `MessagesByIDs(replyIDs)` → `internal/chat/repository/repository.go:316` `GetMessagesByIDs`：`r.db.Where("id IN ?", ids)` —— **无 `chat_room_id` 过滤**。
  - `handler.go:133-141` 把 `markdown.MustRender(q.Content)` 与发送者名回填进 `quote_message` 返回给房间所有成员。
- 影响：任一登录用户在自己所在房间发消息，把 `reply_to_id` 指向**自己不在场的私聊房间**里的任意 `chat_message.id`，下次拉消息时引用预览即返回那条私聊消息的渲染正文 + 作者名。逐 id 枚举即可读取任意私聊内容。
- 修复：`CreateMessage` 内，若 `replyToID != nil`，`GetMessage(*replyToID)` 后校验 `target.ChatRoomID == room.ID`，否则拒绝/置空；并让引用构建走**已存在的**按房间作用域版本 `ListMessagesByIDsInRoom`（`repository.go:192-201`，`Where("chat_room_id = ? AND id IN ?")`）作为纵深防御。
- 备注：今日 API 审计修了 `ToggleReaction` 的同类 IDOR（已加 `IsMember`），但**未触及本引用路径**。修复模式现成（同文件已有作用域版本）。

#### GPT-H01 · 图床客户端解析旧响应信封 → 上传成功却返回空 hash/url（截图/配图静默坏图）

- 位置（当前代码）：
  - `pkg/imageclient/client.go:163-164` 成功体解码进**裸** `UploadResult`（`client.go:90-98`，无 `data` 包裹）：`var out UploadResult; json.Unmarshal(raw, &out)`。
  - **决定性证据**：真实 image_service（上游 `kun-oauth-admin` 的 `platform/image`）的**自带 HTTP 测试** `internal/platform/image/handler_http_test.go` 把上传响应解成信封 `envelope{Code int; Message; Data json.RawMessage}`，再从 `env.Data` 里取 `hash/url/variant_urls`（成功 handler `Upload` 末行 `response.Success(c, result)`）。即服务**实际返回 `{code,message,data:{hash,url,variant_urls}}`（带信封）**。moyu 这份解裸对象 → `Hash/URL/VariantURLs` 全留零值，`json.Unmarshal` 不报错 → 代理回 200 但 payload 全空。
  - 误导根源：本仓 `docs/image_service/03-api-design.md`（约 97-110 行）画的是**无信封**的成功体（且把错误体画成 `{error:{...}}`），与运行中的服务（`{code,message,data}`，错误也是 `{code,message}`）都不一致 —— 客户端的成功体**和**错误体解析双双照这份过期文档写错了。
- 影响：`POST /api/v1/upload/image-service`（截图编辑器/milkdown 配图，`apps/web` 的 `useGalgameEdit.ts` 读 `data.hash/url/variant_urls`）上传成功却拿到空 hash/url → 插入空图/坏图。**静默失败**。
- 修复：`client.go:163` 改解 `var env struct{ Data UploadResult \`json:"data"\` }` 取 `env.Data`（对齐权威 SDK）；并订正 `docs/image_service/03-api-design.md` 的信封。
- 备注：GPT 原报告还点了 `/img` URL helper（`MainURL`/`VariantURL`），经核对这些 helper 在上传路径**未被调用**（dead code），真正的 bug 是上面的**信封不匹配**。今日 API 审计把该端点标了 ✅ 但注明“上游未实测”——正是没实测才漏了它。

### 🟠 中危

#### F025 · wiki 同步在单个 DB 事务内串行调用 OAuth 发奖 HTTP（I/O-in-tx，最大批 1000）

- 位置：`internal/infrastructure/cron/wiki_sync.go:75`（`db.Transaction` 包住整批）→ `:142` `mp.Adjust(ctx, ...)`（同步 HTTP）→ `:151` `tx.Exec("UPDATE \"user\" SET moemoepoint ...")`，`wikiBatchLimit=1000`（`:38`）。直接违反 `pkg/moemoepoint/awarder.go:24` 的约定“award 必须在任何 DB 事务**之外**调用”。
- 影响：OAuth 慢/抖时，一次 cron tick 可让单个 Postgres 事务持开数十秒（受 `cron.go` 2min ctx 上限），串行 ≤1000 次 HTTP，钉住连接 + 持锁；一条坏消息回滚整页、每 tick 重复中毒。正确性（幂等）无损，是**可用性/运维**风险。
- 修复：事务内只做幂等 INSERT + 通知写入；发奖 + 缓存镜像移到 commit 之后用 `Awarder.Award`（已为 post-commit、按稳定键 `moyu:wiki_approved:<id>` replay-safe）；或大幅调小 `wikiBatchLimit`。

### 🟡 低危（成立，建议成批治理）

| ID | 端点/位置（当前代码）| 问题 | 建议修复 |
|---|---|---|---|
| **F029** | `internal/user/service/service.go` Follow/Unfollow + `repository.go:128-153` | 关系写入与 denormalized 计数更新**不在同一事务**（今日修了 rowsAffected 守卫，但仍是两段事务）→ 两写之间崩溃会永久 drift 计数 | 把 `CreateFollow/DeleteFollow` 与 `UpdateFollowCounts` 合进一个 `db.Transaction`（实测库当前无 drift）|
| **F034** | `internal/patch/handler/handler.go`（23 处 `ErrBadRequest(err.Error())`）| 多处把 service 的 not-found/无权限/DB 错统一拍成 HTTP 400 并回显原始 `err.Error()`（如 `ToggleCommentLike`→`comment not found` 应 404）| service 返回哨兵错误（ErrNotFound/ErrForbidden），handler `errors.As` 分类映射；`ErrBadRequest` 只留给 Bind/Validate |
| **GPT-M03** | `internal/app/router.go:312` `/resource/:id`（无限流）+ `internal/common/handler.go:438-461` | 主资源 `content/code/password` 公开下发、**无限流**，可逐 id 枚举绕过 `/link` 的 30/min | 给 `/resource/:id` 路由加同款 `RateLimit(...,30,time.Minute)`（payload 不变，FE 详情页仍需就地渲染下载，见 §4）|
| **GPT-M04** | `internal/user/service/service.go:254-273` + `repository.go:208-211` | 签到读 `daily_check_in` 后**无条件** `Update("daily_check_in",1)`（无 `WHERE ...=0`/rowsAffected）→ 并发两请求都成功、各回一个随机奖励数（萌萌点因稳定键不会双发）| repo 改 `Where("id=? AND daily_check_in=0").Update(...)` 返回 rowsAffected；service 把 0 视为“今日已签到” |
| **F066** | `internal/auth/service/service.go:145-160` + `auth/repository CreateUser` | `FindOrCreateUserByID` 读后插、无 `OnConflict` → 全新用户并发首登一个 200 一个 500 | `Clauses(clause.OnConflict{DoNothing:true}).Create` 后重查；重复键映射为重取而非 500 |
| **F067** | `internal/patch/service/service.go` like/favorite（`:609/621`、`:1051/1062`、`:1081/1090`）| 可逆奖励用 relation 自增 id 作幂等键，丢失的 unlike 会留下永久 +1（fire-and-forget，无重试）| 若要精确反转，过 durable outbox/重试，或两向都用 `(content,liker)` 稳定键带 on/off 后缀（当前为有意的 best-effort，可接受+记录）|
| **F068** | `internal/infrastructure/cron/wiki_sync.go:130-194`（nil 守卫 132/164/176/188）| `target_user_id==nil` 的可执行消息被静默消费（幂等标记已写、无效果、**无日志**；kungal 同处会 `slog.Warn`）| 跳过时补 `slog.Warn`，并考虑该情形不写幂等标记以便修正后重投 |
| **F069** | `internal/app/router.go:93` `PUT /patch/resource/:id/download` | 无 auth、无限流 → 下载计数可被任意刷（污染排行/排序）| 至少加 `RateLimit`（理想再加 optionalAuth/按 session 去重）|
| **F070** | `internal/patch/service/service.go:1180-1186` | `CreateLikeCommentNotification` 已实现但**全仓无调用方**（死代码）→ 评论被赞从不通知作者 | like 成功后 `go CreateLikeCommentNotification(...)`，或删除死代码 |
| **F073** | `internal/chat/handler/handler.go:194` | `lastMsgs, _ := h.svc.LatestMessagePerRoom(...)` 吞错 → DB 抖动时房间列表静默丢“最后一条预览” | 捕获并 `slog.Warn`（仍可返回列表）|
| **F074** | `internal/user/repository/repository.go:31-53` | 四个 profile 计数 helper 丢弃 `Count().Error` → DB 错时公开资料把计数显示为 0 | 返回 `(int64,error)` 并上抛/记录（与同文件下方 list helper 一致）|
| **F075** | `internal/admin/handler/handler.go:420` | `pending, badVndb, _ := CountOrphanPatches()` 丢错 → DB 抖动时孤儿补丁面板把 pending/bad_vndb 显示为 0 | 检查并 `ErrInternal`，或标注部分数据 |
| **F077** | `internal/middleware/auth.go:506-518` | refresh 永久拒判定只认 `{10002,10003,15003}` + HTTP 401/403；而 OAuth 文档里 `15005`(grant 未启用)/`15008`(client secret 错)/`10014`(封禁)可走 HTTP 200 信封 → 落到 transient 分支 → **每请求无限重试、永不清会话** | 把 `10014/15005/15008` 加入永久集合，别只靠 HTTP 状态码（文档明示“业务 401 走 HTTP 200”）|
| **F083** | `internal/patch/service/service.go:399-419`（`:414` 返回 `ErrRecordNotFound`）+ `handler.go:685-691`（`:688` `ErrInternal`）| `GetRandomPatch` 在 SFW 采样全 NSFW 时把 not-found 映射成 HTTP 500（罕见但会触发告警）| handler 里 `errors.Is(err, gorm.ErrRecordNotFound)` → 返回 404 或空 `{id:0}` |
| **F085** | `internal/infrastructure/cron/cron.go:26` `cron.New()`（无 `WithLocation`）+ `:29` `"0 0 * * *"` | 每日重置（`daily_check_in/image_count/upload_size`）按**进程本地时区**午夜；签到幂等键日期也用本地 `time.Now()` | `cron.New(cron.WithLocation(loc))` 显式 `Asia/Shanghai`，并对齐签到键日期 |
| **GPT-L02** | `pkg/config/config.go:133-134`（`KUN_IMAGE_SERVICE_BASE_URL`/`KUN_IMAGE_CDN_BASE` 默认 `127.0.0.1`）+ `.env.example` 缺 `KUN_IMAGE_*` | 生产漏配不 fail-fast，静默回落 localhost；`.env.example` 没有这些键，照抄即落默认 | 这两个 URL 在 prod 模式用 `mustGetEnv`/校验非 localhost；`.env.example` 补 `KUN_IMAGE_*`（凭据回落到项目 OAuth client 是**有意设计**，见 `app.go:142-148` 注释，保留）|

---

## 二、有意（BY-DESIGN）/ 可接受 —— 机制属实，但为有意权衡

- **F072 · 特权删除不做房间作用域**（`internal/chat/service/service.go:148-158` + `handler.go:425`）：admin/mod 可软删自己不在场私聊里的消息。机制属实，但方法注释明示“发送者或 admin/mod 可删”，软删留 `deleted_by_id` 审计、不泄露内容（生成墓碑）。属**有意的全局审核能力**，不必改（若要私聊豁免审核，可收紧为仅 admin）。
- **F024 / F032 · 下游封禁/降权滞后到 token 刷新**（`internal/middleware/auth.go:180`、soft-window 后台刷新 `:171-177`，包头注释 1-9）：角色读自缓存的 access_token JWT、不逐请求回查 OAuth；窗口 ≈ access TTL（且 soft-window 那一次请求会带旧角色放行）。这是**短时令牌下游“仅验签”**的标准设计且代码有注释说明；硬过期路径对永久拒判会删会话 fail-closed。属有意权衡（敏感操作若需即时回收权限，可缩短 TTL 或对高危路由回查）。

---

## 三、已证伪（复核确认非缺陷）

| ID | 外部审计原断言 | 本文复核结论 |
|---|---|---|
| **F006** | “moyu 完全没有 image client，也不 reference-ping → 头像/banner 会被 GC” | **证伪成立**：moyu 有 `pkg/imageclient`（`New/Upload/MainURL/VariantURL`，`app.go:149-155` 接线），故“没有 client”不实；moyu 不做 reference-ping 是**有意**（保活责任在上游 image_service/OAuth 的自有 cmd job）。注：保活无害性依赖上游 repo，本仓内只能确认 client 存在 + 故意不 ping。|
| **F026** | “签到幂等键取自 goroutine 里的 `time.Now()`” | **证伪成立**：键是稳定的 `moyu:checkin:<uid>:<date>`（`service.go:271-272`），`time.Now()` 只取日历日；once-per-day 由 DB flag 把关。（另见 GPT-M04：DB flag 的 check-and-set 非原子，是另一回事。）|
| **F027** | “ghost/null-galgame 的 approved 消息先标记已处理 → 永久丢 +3” | **证伪成立**：`wiki_sync.go:126-128` 对 `m.Galgame==nil` 直接 `return nil`，按文档 §7 是**约定的 no-op**（galgame 已被硬删，无补丁页可通知、无奖励对象）。|
| **F028** | “UpdateResource 在 key 未变时跳过 s3_key 前缀校验 → 旧/伪造 key 持久化” | **证伪成立**：`service.go:823-828` 仅在 `update.S3Key != existing.S3Key` 时校验前缀（即**未引入新 key**才跳过）；跨行篡改另有 owner 校验（`:794`）+ s3_key 唯一索引兜底。|
| **F071** | “删父评论不为孙级回复减 `comment_count`” | **行删除部分证伪成立**：`patch_comment.parent_id` FK `ON DELETE CASCADE` 递归删除子+孙行，无孤儿。**但存在潜在计数漂移**：`CountCommentAndReplies`（`repository.go:207-212`）只数 `id 或直接 parent_id`，CASCADE 却连孙级一起删 → 一旦**多级回复**可创建，`comment_count` 会按已计入的孙级数量上漂。当前前端无多级回复 composer，故 dormant。若上线多级回复，建议把计数改为 `WITH RECURSIVE`。（与今日 API 审计 `patch-write` 域记的“嵌套回复计数”遗留同源。）|

---

## 四、与本仓 API 审计的关系

[`docs/api/checks/`](../api/checks/README.md)（2026-05-29）是本仓**自审 159 个 API 端点的字段对齐 + 修复**；本文是核对**外部跨仓审计**对本仓的发现。两者部分重叠，已在上文逐条注明：

- **已被今日 API 审计修复、本轮无需再处理的同类项**：聊天 `ToggleReaction` IDOR（≠F004 的 reply 路径）、公开列表流批量泄露下载载荷（`/home`/`/resource`/recs/`/user/:id/resource` 已 `StripResourceSecrets`；但 **GPT-M03 的单资源 `/resource/:id` 主体仍按 FE 需要保留** → 仅缺限流，见 §1）、`/link` 限流改按 user 键、admin 删评论计数、评论创建 critical 修复等。
- **本文新增、今日 API 审计未覆盖的**：**F004**、**GPT-H01**（两条 HIGH）、F025、F029 的原子性、F034 的错误映射、GPT-M04 签到竞态、F066/F067/F068/F069/F070/F073/F074/F075/F077/F083/F085/GPT-L02。

## 五、建议处理顺序

1. **立即**：`F004`（私聊内容泄露 IDOR）、`GPT-H01`（图床上传静默坏图）—— 两条 HIGH，修法明确且小。
2. **其次**：`F025`（I/O-in-tx，OAuth 抖动下连接/锁风险）、`GPT-M03`/`F069`（给 `/resource/:id`、下载计数加限流）、`GPT-M04`/`F066`（check-and-set 原子化）。
3. **成批治理 LOW**：`.Error` 吞错簇（F073/F074/F075）、错误码/状态码规范（F034/F083/F077）、时区（F085）、配置 fail-fast（GPT-L02）、计数原子性（F029）、死代码/通知（F070）。
4. **记录备查**：F072、F024/F032（有意）、F071（多级回复上线前的潜在项）。

> 本文仅为核对与列举；除已在 `docs/api/checks/` 修复的重叠项外，上述 OPEN 项**尚未改动代码**。
