# moyu 私信迁移到 NextMoe chat

> 状态：**A 已完成（开关关着上线）**，B（切换）待发。
> 合同是 infra 的 `docs/chat/01-service-and-contract.md` 与 `docs/chat/openapi.yaml`
> （线上 `GET https://api.nextmoe.dev/v2/chat/openapi.json`）。本文只记 moyu 这一侧
> 的决定和上线顺序，不复述合同。参照实现是 letmoe（`kun-letmoe-community` #10 #11）。

## 0. 是什么

会话属于人、不属于站：同一对用户在 moyu、kungal、letmoe 看到的是同一个会话。
moyu **不存任何私信数据**，没有表、没有缓存、没有迁移。旧的 `chat_*` 表只剩
GROUP 房间在用（`link=kun` 这类，群组阶段 W6 再迁），PRIVATE 部分在 B 之后冻结，
以后统一退役。

## 1. 数据流

```
浏览器 ──/api/v1/im/*──▶ moyu-api ──Bearer 会话令牌──▶ http://chat:9285/v2/chat/*
浏览器 ◀──── wss://api.nextmoe.dev/connection/websocket（Centrifugo，令牌经 BFF 取）
```

- 转发层 `internal/im`：路径、查询串、请求体、`Content-Type`（传图是 multipart）、
  `Idempotency-Key` 原样转发；去掉 `useApi` 自动附加的 `content_limit` 和
  `include_empty`。路径反转义后含 `..` `?` `#` `\` 一律 404、不打上游：chat 进程上
  还挂着 `/trust/callback` 这类内部路由。
- 成功：上游 JSON 原样放进信封 `data`（snake_case、十进制字符串 id，KunUI 的
  `KunChat*` 组件直接吃）。**上游 204 改成 200**：204 不带 body，页面会读到
  `undefined`，letmoe 的「接受」按钮就是这样崩的。
- 失败：状态码保留，`data` 是上游 problem，`message` 是中文。

| 上游 | moyu code | 前端 |
|---|---|---|
| 403 `SCOPE_REQUIRED` | 40313 | `/messages` 显示「重新登录」，不重试 |
| 401 | 40101 | `useApi` 统一登出 |
| 5xx / 连不上 / 未配置 | 50322 | 「私信服务暂不可用」 |
| 其他 4xx | 状态码 ×100 | 按 `CHAT_*` 等码给中文 |

40313 不复用 40399：chat 客户端每个页面都会启动，40399 的长提示会在老会话的每一
页弹一次。

## 2. 前端

- `stores/chatStore.ts` + `shared/utils/chatModel.ts`：本地模型按更新流推进
  （合同 §7）。推送只作提示；`update_seq` 跳号时等 0.5s 再 `GET /updates?after=`，
  `too_long` 整体重载。未读一律从 `GET /state` 取，不在本地累加。
- `plugins/chat.client.ts`：`onNuxtReady` 之后才启动（先于水合会让两端状态不一
  致）；Centrifugo `connected` 和页面回到前台时各补一次更新流。
- `/messages/[[id]]`：`definePageMeta({ key: 'messages' })`，切会话不重挂整页；
  依赖 chat 状态的部分都在 `<ClientOnly>` 里。`?peer=<uid>` 打开与该用户的会话，
  老会话重新登录后经 `returnTo` 回到这里接着开。
- 入口：顶栏信封（`sm` 以上，未读 = `unread_conversation_count > 0 || request_count > 0`），
  手机上在汉堡菜单里并在汉堡按钮上打点；用户主页「发消息」= `PUT /im/direct/{uid}`；
  消息中心「私信」标签。
- 多图发送同一个 `media_group_id`，对方按相册显示。

## 3. 开关与上线顺序

登录时**无条件**请求 `chat:read chat:write`（infra 已给 moyu 的 client 授权，
生产与本地都核过 `allowed_scopes`）：A 上线后新登录的会话就带上了，B 时需要重登的
人更少。授权端点对 client 未获准的 scope 整请求拒绝，所以这一步必须晚于 infra 授权。

| 变量 | 容器 | 作用 |
|---|---|---|
| `KUN_CHAT_API_BASE` | moyu-api | `http://chat:9285`；空 = `/im/*` 全部 50322 |
| `KUN_CHAT_ENABLED` | moyu-api | B：旧 PRIVATE 房间的写接口回 41000「私信已迁移」 |
| `NUXT_PUBLIC_CHAT_ENABLED` | web | B：私信入口、`/messages`、旧 PRIVATE 页跳新页 |

两个开关写在 `docker-compose.prod.yml` 里，**同一个提交一起翻**。

- **A**：开关关着上线。线上行为与之前一致（`/messages` 404、无入口、无 `/im/*`
  请求、旧私聊照常），只多了登录时的 chat scope。
- **B**：把两个开关改成 `"true"` 部署。旧的 `POST /chat/room/private`，以及 PRIVATE
  房间上的发消息、编辑、删除、回应回 410；读取照常。GROUP 房间不受影响。
  `/message/chat/<小uid>-<大uid>` 服务端 302 到 `/messages?peer=<对方>`。
- **C**：B 上线后立刻通知 infra 跑一次 `cmd/import-chat` 补齐。导入是 2026-09-27
  22:34 CST 做的，这之后到切换之间在旧站发的私信要靠这次补齐搬过去；B 之后旧站
  不再产生 PRIVATE 消息，所以只需补这一次。

回滚：两个开关改回 `"false"`。旧表没动，PRIVATE 房间恢复可写。但回滚期间在旧站
发的私信不一定能再搬回 chat：已经在 chat 里原生聊过的一对，导入会跳过
（`ErrNativeHistory`，旧消息插不到已有消息之前）。所以回滚只适合 B 刚上线就出问题、
还没人在新私信里聊过的时候。

## 4. 验收（2026-09-27，本地 infra chat + Centrifugo，user 100 / 101 / 200）

请求（带链接被拒并提示）→ 接受 → 实时新消息约 100ms、打字中、已读双勾 → 回复与引用
→ 表情回应 → 置顶与置顶服务消息 → 编辑 → 断网期间对方发送与为双方删除、恢复后补齐 →
丢一帧推送、525ms 按缺口补齐 → 删除消息请求 → 举报（`origin_site=moyu`，快照 2 条）
→ 会话置顶、静音、归档、标为未读 → 单图与相册 → 老会话 40313 只调一次并提示重登、
重登后回到原处 → 手机单栏与输入框不被遮挡 → B 的旧接口拦截与旧页跳转 → 开关关闭时
与线上一致。
