# 开放 API · `moyu` 面

moyu 在 NextMoe 开发者平台上的公开只读面。**代码、契约、网关标签都已在本仓**；线上还没生效——compose-only 改动不触发 Dokploy 重部署，要点一次 Deploy，然后冒烟（§4）。

| 文件 | 内容 |
|---|---|
| [moyu-openapi.yaml](./moyu-openapi.yaml) | 面的 OpenAPI 3.1 契约（4 op，`redocly lint` 0 warning） |

## 1. 三十秒版本

- 这个面回答一个问题：**这个游戏，moyu 上有哪些补丁资源**。入口是调用方手里已有的锚——VNDB 号或 catalog work id。
- **不带游戏名、封面、标签、角色、Staff**。那些是 catalog 的，moyu 一份都不存；同一把 key 同时能读 `/v2/catalog/works`，每行都带 `catalog_work_id` 就是为了那一跳。
- **不带下载链接、提取码、密码**。站内取链是另一条按资源限流的请求，存在的理由就是链接不能被批量抓走；闸门放行任意有效 key，这里给链接等于把那条限流端点作废。每行给 `web_url`。
- 鉴权本服务一行没写。B 档 ForwardAuth 在网关上做完了。

## 2. 面

```
GET /v2/moyu/patches                     ?limit&cursor&include_total&include&ids&refs&nsfw&sort&type&language&platform&has_resources
GET /v2/moyu/patches/{id}                ?include
GET /v2/moyu/patches/{id}/resources      ?limit&cursor&include_total&include
GET /v2/moyu/resources/{id}              ?include
```

`ids` / `refs` 是**批量查询**：一次最多 100 个锚，没命中的原样回在 `missing` 里。

```
GET /v2/moyu/patches?refs=vndb:v65869,catalog:61311,vndb:v-nope
→ {"object":"list","items":[…],"missing":["vndb:v-nope"],"next_cursor":null}
```

这条替代两个旧接口：`/api/v1/hikari?vndb_id=` 的单条查询，和 `/api/v1/moyu/patch/has-patch`——后者今天是把**全表的 vndb_id 一次吐出来**，调用方拿它做本地存在性判断。

## 3. 与 catalog `/v2` 一致的地方

一把 key 横跨两个面，解码器只写一遍：

- id 全是**字符串**；`patch.id`、`vndb_id`、`catalog_work_id` 是三个 id 空间，互不相等
- 游标分页（`cursor` / `next_cursor`，不透明 `cur_…`），`total` 要花一次 count 所以 `include_total=true` 才给
- 关联走 `include=`，缺省什么都不带
- 时间是 RFC 3339 UTC，日期是 `YYYY-MM-DD`
- 错误是 RFC 9457 `application/problem+json`，`code` 取自平台封闭注册表
- 缓存头逐字沿用 catalog `/v2` 公开档：`public, max-age=300, s-maxage=1800, stale-while-revalidate=3600` + ETag/304
- `limit > 100` 报 `LIMIT_TOO_LARGE` 而**不静默截断**——被截断的页会让调用方以为自己读完了

站内 `/api/v1` 不受影响，仍是 `{code,message,data}` 信封 + cookie 会话；`globalErrorHandler` 按路径分流，两套错误语言互不干扰。

## 4. 还差的接线

infra 侧全部就位（2026-09-08 实测）：

- 面注册表里 `"moyu": "/v2/moyu/*"` 已在（`devapi/forwardauth.go`）
- 免 scope 已生效：拿只有 `catalog:read` 的生产 key 打 `forward-auth?face=moyu` 回 **204**
- 未注册的 face 回 500，可作对照

本仓侧：

- [x] `docker-compose.prod.yml` 的 `moyu-api` 加 Traefik 标签（`moyu-face-pub` / `moyu-face-forwardauth`，端口 `5214`，`face=moyu`，`PathPrefix(/v2/moyu)`），并显式写 `priority: '100'`
  - infra 的兜底 router `infra-v2-pub`（`Host(api.nextmoe.dev) && PathPrefix(/v2)`）自己没写 priority，Traefik 按 rule 长度算给它 44；我们的 rule 是 49，不写这一行就只靠 5 个字符赢，infra 给 `/v2` 的 rule 加一个条件就会静默吃掉这个面。
  - Dokploy 面板上原有的 `www.moyu.moe /api/v1` 两条 router 是 Dokploy 自己生成的，标签里不重复，也不受影响。
- [x] `KUN_SITE_BASE_URL: https://www.moyu.moe` 写进 compose 的 env 锚（`moyu-api`/`migrate`/`tools` 共用）。值和代码缺省一样，写出来是为了让它跟域名走而不是跟版本走。

剩下的（都不在本仓）：

- [ ] **去 Dokploy 点 Deploy**。compose-only 改动不触发重部署，标签不会自己生效。
- [ ] 冒烟。**"router 没挂"在这里不表现为裸 Traefik 404**，而是 infra 的 problem 文档，看 `type` 三选一：
  - `problems/platform/not-found` + `request_id` → 标签没生效，还在走兜底
  - ForwardAuth 的 401 → 标签生效了，key 的问题
  - 正常 JSON（`{"object":"list",…}`）→ 通了
- [ ] spec 进门户 docs-model + oasdiff 破坏门 + operation-count 守卫（4 op）+ kungal-docs 登记

## 5. hikari 已退役（2026-09-19）

`/api/v1/hikari` 与 `/api/hikari`（Nitro 兼容代理）现在对任何请求都回 **410**，body 仍是旧信封 `{success:false, message, data:null}`，`message` 指向 developer.nextmoe.dev 申请 key、改用 `/v2/moyu/patches?refs=vndb:<id>&nsfw=true&include=resources`。旧 hikari 不看 NSFW，所以等价查询要带 `nsfw=true`。

退役而不是修，是因为它已经在给错链接：合作方用 `data.resource[].patch_id` 拼 `https://www.moyu.moe/patch/<patch_id>/resource`，而 037 之后 `patch_id` 是 catalog work id、`/patch/<n>` 仍解析改号前的旧页号（铁律 3）。2026-09-19 实测它能回答的 11,096 个页面里 8,486 个链到别的游戏、1,590 个 404，只有 1,020 个正确。`/v2/moyu` 每行给 `web_url`，不再让调用方拼地址。

CORS 白名单（18 个合作方域名）和 Nitro 代理都保留：合作方页面是在浏览器里 fetch 的，没有 ACAO 它们只会看到 CORS 错误，读不到这条 `message`。已知的两个前端调用方（touchgal、kungal）都按 `success` 分支，拿到 410 会把 moyu 补丁区渲染成空。

平台面**浏览器直连用不了**——ForwardAuth 对所有方法生效，`OPTIONS` 预检不带 key 必被 401，这是有意的（`nmk_` key 不该进浏览器）。迁移的合作方必须改成服务端调用。

`/api/v1/moyu/patch/has-patch` 没有一起退役。

## 6. 一条给 infra 的观察

闸门自己的 401 / 429 走的是平台的 `{code, message}` 信封（实测 `{"code":10001,"message":"未授权，请先登录"}`，中文），而同一条 `/v2` 路径上其它所有回答都是 RFC 9457 problem+json。第三方在同一个前缀下仍然要解两种错误格式，而这正是下游面统一用 problem+json 想避免的事。契约里已如实写明，但值得 infra 考虑让 ForwardAuth 的拒绝也回 problem 文档。

**已采纳**（infra `fc0d7c29`，2026-09-18）：闸门改回同一份注册表的 problem 文档——401 `MISSING_CREDENTIAL` / `INVALID_CREDENTIAL`，429 `RATE_LIMITED` / `QUOTA_EXCEEDED`，状态码不变。契约的顶部说明、`RateLimited` 响应与 `Problem.code` 枚举已同步改写。
