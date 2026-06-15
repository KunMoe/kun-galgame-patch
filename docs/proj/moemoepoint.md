# 萌萌点（moemoepoint）—— moyu 侧用法全览

> 本文档汇总 **moyu 这边**萌萌点的两件事:**①对用户有哪些限制 ②所有增减来源**(金额、reason、ref、幂等键、触发点、是否可反转)。
>
> 跨服务契约(余额单源、reason 枚举、s2s 端点、负余额、铸币白名单等)由 **kun-galgame-infra 拥有**,权威文档是只读镜像 `docs/oauth/06-moemoepoint.md`(**契约要改去 infra 改**)。本文只讲 moyu 如何**消费**这套契约——改任何发放逻辑前请同时读那份契约。
>
> 核心不变量(C3):**余额单源在 OAuth,全生态(kungal / moyu / 未来站点)共享一个余额**;moyu 本地 `user.moemoepoint` 只是**读缓存**,每次发放后用 OAuth 返回的权威余额回写;moyu **从不本地 `+=`**(否则会和统一余额双重计数)。

## TL;DR

- **对用户的限制:目前为零。** 萌萌点在 moyu **不门控任何用户操作**(下载、评论、发布、认领、签到都不看余额)。它纯粹是展示性的 karma 分。**余额允许为负**。详见 §1。
- **moyu 能发放的 reason 只有 4 个**:`content_approved` / `content_removed` / `daily_checkin` / `liked`。`admin_grant` / `admin_deduct` / `migration` / `register_gift` 是 **OAuth 保留**,moyu(s2s)用了会被拒(16003)。
- **增减来源共 13 处**(12 个内联 `Awarder.Award` + 1 个 cron `Adjust`),全部列在 §2 的总表。
- **发放是尽力而为**:内联发放都是 `go ...Award(...)`(异步、不阻塞主流程、失败仅记日志→偶尔可能丢一分);**只有 wiki-cron 那条是事务性**(失败回滚 + 重试,永不丢)。
- **反转不对称**(§3):删资源会回收 −3,但**删词条 / 删评论 / wiki 封禁不回收**。

---

## 1. 对用户的限制(restrictions)

### 1.1 萌萌点不限制任何用户操作

对全代码库做过读取扫描:**没有任何一处读取 `moemoepoint` 来门控用户行为**。具体确认:

- **不存在**"余额不足 X 不能下载 / 评论 / 发布 / 认领 / 签到"之类的门槛。
- 资源下载只有**频率限制**(`resource-detail` / `resource-link` 60/min,见 `internal/app/router.go`),与萌萌点无关。
- "仅创作者可发布 Galgame"开关是**角色门控**(`IsCreatorOnlyEnabled` + `admin`/`moderator`,`internal/patch/handler/handler.go:82`),**不是**萌萌点门控。
- 契约 `06-moemoepoint.md §9` 明确把"单次/单日上限、可兑换、per-app 玩法"列为**故意没做**——所以这不是 moyu 漏实现,是整体设计就把萌萌点定位成软 karma。

> 结论:萌萌点目前**只增减、只展示,不约束**。如果将来要加门槛(如"发资源需 ≥N 分"),是新功能,不是 bug。

### 1.2 余额可以为负

OAuth 侧**不做非负约束**(契约 §3.1:"保证回收/反转永不被挡")。所以一个余额只有 1 分的用户,其资源被删(−3)后余额会变成 **−2**;moyu 不 clamp、不拦截。展示层照实显示负数。

### 1.3 系统级约束(约束的是"发放动作",不是用户)

这些来自契约,moyu 的薄客户端(`pkg/moemoepoint`)遵守:

| 约束 | 说明 | 错误码 |
|---|---|---|
| reason 白名单 | moyu s2s 只能用 4 个下游 reason(见 TL;DR) | 16003 |
| `\|delta\| ≤ 1,000,000` 且 ≠ 0 | 防呆上限;`delta==0` 被 Awarder 直接跳过(签到抽到 0 分即此情况) | 16002 |
| 幂等键必填且全局唯一 | 同键重放=空操作(`applied:false`);同键不同体=报错 | 16004 |
| 铸币白名单 | 只有 `www.moyu.moe` / `www.kungal.com` 的 client 能 POST 发放;moyu **在**白名单内 | 403/16005 |

---

## 2. 所有增减来源(13 处)

发放统一经 `Awarder.Award(ctx, userID, delta, reason, ref, idemKey)`(`pkg/moemoepoint/awarder.go`),它调 OAuth s2s 后把权威余额回写本地缓存。**唯一例外**是 wiki-cron 直接调 `client.Adjust`(因为它要在同一事务里回写缓存 + 失败回滚)。

### 2.1 总表

| # | 触发场景 | delta | reason | 受益/受损方 | ref | 幂等键 | 代码位置 | 可反转? |
|---|---|---|---|---|---|---|---|---|
| 1 | 创建 galgame 词条 | **+3** | content_approved | 创建者 | `galgame:<id>` | `moyu:patch_create:<id>` | `patch/service/service.go:151` | ✗ 删词条不回收 |
| 2 | 认领 galgame | **+3** | content_approved | 认领者 | `galgame:<id>` | `moyu:claim:<id>` | `patch/service/service.go:316` | ✗ |
| 3 | 发布补丁资源 | **+3** | content_approved | 发布者 | `resource:<id>` | `moyu:resource_publish:<id>` | `patch/service/service.go:868` | ✓ 见 #5 |
| 4 | Wiki 投稿通过审核(cron) | **+3** | content_approved | 投稿者 | `galgame:<id>` | `moyu:wiki_approved:<msgId>` | `cron/wiki_sync.go:184` | ✗ 封禁不回收 |
| 5 | 删除补丁资源 | **−3** | content_removed | 资源**所有者** | `resource:<id>`(同 #3) | `moyu:resource_delete:<id>` | `patch/service/service.go:1139` | —(本身就是反转) |
| 6 | 每日签到 | **+0~7**(随机) | daily_checkin | 签到者 | (空) | `moyu:checkin:<uid>:<日期>` | `user/service/service.go:277` | —(单向) |
| 7 | 词条收到评论 | **+1** | liked | 词条所有者 | `comment:<id>` | `moyu:comment:<id>` | `patch/service/service.go:590` | ✗ 删评论不回收 |
| 8 | 评论被点赞 | **+1** | liked | 评论所有者 | `comment:<id>` | `moyu:comment_like:<relId>` | `patch/service/service.go:713` | ✓ #9 |
| 9 | 评论取消点赞 | **−1** | liked | 评论所有者 | `comment:<id>` | `moyu:comment_unlike:<relId>` | `patch/service/service.go:701` | —(反转 #8) |
| 10 | 资源被点赞 | **+1** | liked | 资源所有者 | `resource:<id>` | `moyu:resource_like:<relId>` | `patch/service/service.go:1272` | ✓ #11 |
| 11 | 资源取消点赞 | **−1** | liked | 资源所有者 | `resource:<id>` | `moyu:resource_unlike:<relId>` | `patch/service/service.go:1261` | —(反转 #10) |
| 12 | 游戏被收藏 | **+1** | liked | 词条所有者 | `galgame:<id>` | `moyu:favorite:<relId>` | `patch/service/service.go:1344` | ✓ #13 |
| 13 | 游戏取消收藏 | **−1** | liked | 词条所有者 | `galgame:<id>` | `moyu:unfavorite:<relId>` | `patch/service/service.go:1328` | —(反转 #12) |

### 2.2 三类来源说明

**A. 产出被采纳 `content_approved` (+3)** —— #1~#4。奖励"贡献了内容"的人:建/认领词条、发资源、wiki 投稿过审各 +3。#1~#3 在各自 service 的事务**提交后**异步发放;#4 在 cron 事务**内**发放(可回滚重试)。

> 注意 #1(本地建词条)与 #4(wiki 投稿过审)是**两条不同链路**、幂等键不同,各自独立计一笔。是否会对同一次贡献双发,取决于 wiki 侧是否对 moyu 直建的词条也产生一条 `approved` 消息——这归 wiki 契约管,moyu 这边两条键互不去重。

**B. 互动 `liked` (±1)** —— #7~#13。**永远奖励"内容所有者",从不奖励动作发起人**,且**自己操作自己的内容不发**(`owner != actor` 才发)。点赞/收藏 +1,取消 −1,对称回收。注意 #7"收到评论"也用 `liked` reason(reason 枚举很粗,`liked` ≈ "你的内容获得了正向互动",不止字面"点赞")。幂等键用**关系行 id**(`relId`):取消后再点赞会插入新关系行→新 id→新键,所以 赞→取消→再赞 = +1,−1,+1 净 +1,正确。

> **谁是"内容所有者"**:资源/评论级(#8~#11)是 `resource.user_id` / `comment.user_id`(资源发布者 / 评论作者)。**词条/游戏级(#7 收到评论、#12/#13 收藏)是 `patch.user_id`** —— 即"**补丁发布者 / 在 moyu 建这条 patch 记录的人**",**不一定等于 wiki 的 galgame 创建者**(`galgame.user_id`)。这俩在 moyu 是两个概念(见 memory `galgame-creator-vs-patch-publisher`:`patch.user_id` 是 owner-gating 键,展示用的"创建者"才取 wiki 的 `galgame.user_id`)。萌萌点发放一律走 `patch.user_id`。同理 #1/#2 的 +3 发给"建/认领这条 patch 的人"(成为 `patch.user_id`)。

**C. 每日签到 `daily_checkin` (+0~7)** —— #6。`rand.Intn(8)` 抽 0~7 分,抽到 0 分 Awarder 跳过(满足 OAuth"非 0"要求)。幂等键含 **Asia/Shanghai 日期**(与每日重置 cron 的"天"边界一致)→ 同日重复签到即便绕过 `daily_check_in` 标志也不会重复发。原子 check-and-set 防并发双发。

---

## 3. 反转 / 对账与已知不对称

可回收的产出约定用**相同 `ref`** 让 OAuth 对账(`content_removed` 抵 `content_approved`)。moyu 目前的实现**只对资源做了完整反转**,其余三处缺口:

| 发放 | 是否回收 | 说明 |
|---|---|---|
| 发布资源 +3(#3) | ✓ 删资源 −3(#5,同 ref) | 完整。即便版主/管理删的是**别人**的资源,扣的也是**资源所有者**的分(`DeleteResource` 用 `resource.UserID`,非操作者) |
| 点赞/收藏 +1(#8/#10/#12) | ✓ 取消 −1(#9/#11/#13) | 对称 |
| **建/认领词条 +3(#1/#2)** | ✗ | `DeletePatch` **不发放任何反转**——删词条不回收创建奖励 |
| **Wiki 投稿过审 +3(#4)** | ✗ | cron 的 `declined`/`banned`/`unbanned` 消息**只发通知,不扣分**。`declined`=从未过审(无 +3 可扣,正常);`banned`=曾过审拿过 +3 但不回收(**潜在缺口**) |
| **词条收到评论 +1(#7)** | ✗ | `DeleteComment` 不回收词条所有者的 +1 |

> 这些不对称是**当前实现现状**,不一定是 bug——软 karma "偶尔多一分"无伤大雅(契约 §0)。但如果要严格对账,#1/#2/#4/#7 的回收是明确的待补项。补的话:`DeletePatch` 对 `patch.UserID` 发 `content_removed -3`(ref 同 `galgame:<id>`);`DeleteComment` 对词条所有者发 `liked -1`;wiki `banned` 分支补一笔 `content_removed`。

---

## 4. moyu 之外、但影响同一余额的来源

余额是**全生态共享**的,所以下面这些虽不由 moyu 触发,也会改变用户在 moyu 看到的余额:

- **注册欢迎礼 `register_gift` +7** —— OAuth 注册成功时一次性发(`note=「鲲给予你的第一份礼物」`)。OAuth 内部,s2s 不可用。**这是新用户的初始余额来源**。
- **迁移 `migration`** —— ID 统一时把各站本地值求和作为统一起始余额,一次性回填一笔。
- **管理员 `admin_grant` / `admin_deduct`** —— **只能在 OAuth 管理台**操作。moyu 管理面板**无权**直接增减萌萌点(用了保留 reason 会被拒)。
- **kungal(论坛)及未来站点** —— 通过各自 s2s 往**同一钱包**发放。

---

## 5. 余额怎么读 / 展示在哪

- 权威值在 OAuth;moyu 本地 `user.moemoepoint` 是缓存,**每次发放后**由 Awarder(或 cron)用 OAuth 返回的 `balance` 回写。
- **流水(萌萌点记录)**:`UserService.GetMoemoepointLog` → `Awarder.Log` → OAuth `/users/:id/moemoepoint/log`(精简视图,无 `note`/`actor_user_id`)。前端入口:顶栏头像菜单的「萌萌点」行 → `MoemoepointLog.vue` 弹窗。
- **展示点**:顶栏下拉(`UserDropdown.vue`)、用户排行榜(`ranking/user.vue`)、个人主页 / `/auth/me`。这些读的是本地缓存列(排行榜需要可排序),所以缓存回写若失败会短暂与权威值不一致(下次发放或回读时自愈)。

---

## 6. 可用性 / 注意事项

- **不要让发放阻塞主流程**:内联发放都用 `go ...Award(...)`,且在触发动作**事务提交之后**调用。OAuth 抖动最坏是丢一分(幂等键保证之后重试安全),绝不卡住用户的发资源/点赞/签到。
- **cron 那条是例外且必须事务性**:wiki 消息→+3 由会重放的 cron 触发,所以走"发放成功才推进游标 + 同事务回写缓存,失败整批回滚下轮重试",保证不丢不重(契约 §4 点名的典型场景)。
- **新增一种发放来源** = 选一个合适的现有 reason(多半是 `content_approved`/`liked`)+ 设计一个稳定幂等键 `moyu:<event>:<唯一id>`,调 `Awarder.Award`。不要新造 reason(reason 枚举归 OAuth)。
- **schema 备注**:本功能不涉及 moyu 本地表结构变更(`user.moemoepoint` 列早已存在,只读缓存)。如未来要补本地对账,才需迁移。
