# Domain: message

> 审计于 run1（schema 强制输出），此处为整理后的最终结论。

## Summary
5 路由（组级 `auth`，D9 无 WS 无关）全部 handler→service→repository→model + FE 消费方 + DB 约束端到端核对。发现 3 个问题（2 已修复，1 随端点删除而 moot）。

## Endpoints

### GET /api/v1/message/ — MessageHandler.GetMessages
- verdict: ok
- tested: live `?type=mention&page=1&limit=50` → `{items:[],total:0}`；分页 `{items,total}`、UserMessage json tags 与 message.d.ts 逐字段对齐；sender_id NULL → 省略 `sender`（omitempty）→ Card.vue '系统' fallback。

### GET /api/v1/message/all — MessageHandler.GetAllMessages
- verdict: ok
- tested: live；system 消息 `sender_id:null` 正确省略 `sender`；shape 与 notice.vue 一致。

### GET /api/v1/message/unread — MessageHandler.GetUnreadTypes
- verdict: ok
- tested: 裸 `string[]`（非分页）；`WHERE recipient_id=? AND status=0 DISTINCT type`；User.vue + messageStore 消费。

### ~~POST /api/v1/message/~~ — MessageHandler.CreateMessage  → **已删除**
- verdict: fix（已删除整条链）
- issue [high][security]：组级 `auth` 之外无限流/角色门；`recipient_id`/`type`/`content`/`link` 全客户端可控（FK 仅要求目标是真实本地用户 = 任何人都可被投递）；`type` 无 enum。Card.vue 将 `msg.link` 作为卡片 href → 垃圾/钓鱼面。**无任何 FE 调用方**（合法通知由 patch `createDedupMessage` 内部产生）。`sender` 强制为调用者（唯一缓解）。
- fix：删除路由 + handler/service/repo/dto 整条死链（router.go 留注释说明）。
- issue [low][bug]（随删除 moot）：`recipient_id` FK 违反时 handler 把所有 service 错误拍成 `50000 ErrInternal("")`，无法区分客户端错与服务端错。

### PUT /api/v1/message/read — MessageHandler.MarkAsRead
- verdict: ok
- tested: `{type}`（或 all），强制 recipient=本人；`WHERE recipient_id=? AND status=0 [AND type=?]` → status=1；notice.vue `put('/message/read',{type:'all'})` 对齐。

## Cross-cutting
- [high][fe-be-mismatch]（**已前端修复**）：`app/pages/message/mention.vue` 把 GET /message 响应当裸数组 `Message[]` 并 `v-for data` / `data?.length`，而 BE 返回 `{items,total}` → @消息页永远空。其余 4 个消息页都用 `{items,total}`。修复：mention.vue 改用 `{items,total}` + `data?.items`。BE 不动（分页 envelope 是全站约定）。
