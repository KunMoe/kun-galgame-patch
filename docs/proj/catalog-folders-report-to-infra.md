# 给 infra 的报告 · catalog 收藏夹（user folder）

**来源**：moyu 用户 121089 报障 —— 「补丁站显示有 10 个收藏，但点击"收藏"后是空的」。
**日期**：2026-09-09。**取数**：生产 `kun_catalog` / `kungalgame_patch`，只读。
**moyu 侧的三个缺陷已在本仓修复**（commit `45ca7647`，见 §5），本报告只列**必须由 infra 决定或实现的四件事**。

> **2026-09-09 当日结案**：四件全部有了答复（infra PR #179，spec 2.22.0）。§6 记录了落地结果，以及 §4 在拿到新事实后的结论变化——那条删除面 moyu 决定**不接**。

一句话版本：

1. **数据 + 脚本**：09-07 那次把 moyu 收藏搬进 catalog 的回填，建出来的 3633 个默认收藏夹**全是 private，且没有写 import 台账**。其中 3632 个人名下没有任何公开收藏夹，整份收藏在所有公开面上不可见。请裁定是否改回 public，并给回填脚本补上 visibility 与台账。
2. **缺面**：`contains_work_id` 只接一个 work，没有批量成员判定。
3. **缺面**：没有「谁持有这个 work」的反查，moyu 的「你收藏的游戏发布了新补丁」通知因此停在 09-06 的名单上。
4. **问题**：`purgeUserFolders` 要 moderator **用户令牌**，下游的账号清除流程该不该走这条路？

---

## §1 09-07 回填把默认收藏夹建成了 private，且没有台账

### 事实

`catalog_user_folder.visibility` 的列默认值是 `0` = private（`model.FolderVisibilityPrivate`）。按天看 `is_default` 的可见性分布：

```
     d      | is_default | visibility | count
 2026-09-08 | t          |          0 |     5
 2026-09-08 | t          |          1 |     7
 2026-09-07 | t          |          0 |  3634   ← 异常
 2026-09-07 | t          |          1 |    32
 2026-09-06 | t          |          0 |     2
 2026-09-06 | t          |          1 |    36
 2026-09-05 | t          |          0 |     2
 2026-09-05 | t          |          1 |    32
```

其它每一天默认收藏夹都是压倒性 public——那是应用自己建的，moyu 的 `ensureDefaultFolder` 显式传 `visibility: "public"`。09-07 反过来。

用 `catalog_user_folder_import` 可以把这批精确圈出来：

```
-- 09-07 当天建的默认收藏夹，按「有没有 import 台账」拆开
 visibility | has_import | count
          0 | f          |  3633   ← moyu 回填
          0 | t          |     1   ← forum
          1 | t          |    32   ← forum
```

`catalog_user_folder_import` 里 `site` 只有一个值 `forum`（8283 行）。**moyu 那次回填一行台账都没写**，所以这批既没有 provenance 也没有幂等键；反过来说，「is_default 且 visibility=0 且无 import 行且 created_at=2026-09-07」正好等于这批，3633 个，无歧义。

规模：

```
folders | items | empty | 名下没有任何公开收藏夹的人
   3633 | 44194 |     0 |                      3632
```

### 影响

`/v2/folders?owner_uid=` 只答 public。任何拿不到本人 token 的读者——包括下游把「看别人的收藏」和「看自己的收藏」写成同一条路由的时候——都读到空。moyu 这一侧还叠了自己的路由缺陷（§5），两个一起才让本人也看不见自己的收藏；但即使 moyu 修好，**这 3633 个人的收藏在任何第三方视角下依然全是不可见的**。

把私有折叠到人头上看（含 forum 用户）：**3953 个账号名下的收藏全部位于私有收藏夹，合计 68782 条**；另有 107 个账号只有一部分可见。

### 要裁定的

**这批该不该改成 public？** moyu 的立场是「应该」，理由是它**恢复而不是放宽**原有语义：

- 切换之前，moyu 的收藏本来就是公开的——旧的 `GET /api/v1/user/:id/favorite` 是一条无鉴权路由，任何人都能看任何人的收藏列表。
- 用户没有做过「设为私密」这个动作。回填只是漏传了一个字段，吃了列默认值。
- 同一批人在 forum 侧的收藏夹是 public（forum 的导入器传了）。同一个人的同一份收藏，因为从哪个站搬过来的而可见性不同，这本身就是个 bug。

但这是 3633 个账号的隐私面批量写，**决定权在 infra**，moyu 不会去动 `kun_catalog`。如果裁定改，建议：

```sql
UPDATE catalog_user_folder f SET visibility = 1, updated_at = now()
WHERE f.is_default AND f.visibility = 0
  AND f.created_at::date = DATE '2026-09-07'
  AND NOT EXISTS (SELECT 1 FROM catalog_user_folder_import i WHERE i.folder_id = f.id);
-- 期望 3633 行
```

如果裁定不改（尊重现状），也请告诉我们，moyu 会在收藏夹页面上明确提示「你的默认收藏夹是私密的，只有你自己能看见」，并给一个改公开的入口——现在两边都没有，用户只会以为收藏丢了。

### 要修的脚本

无论怎么裁定，回填器本身有两处：

1. **没传 `visibility`**。建议 `FolderCreate` 在导入路径上把 visibility 变成必填（或者至少让导入器显式传），而不是让列默认值来决定一次批量迁移的隐私姿态。
2. **没写 `catalog_user_folder_import`**。forum 的导入器写了，moyu 的没写。台账缺失意味着这次迁移不可审计、不可重跑、不可回滚——上面那条 SQL 能精确圈中这批，靠的是「无台账」这个副作用，而不是设计。建议补写 `site='moyu'` 的行（`source_id` 用 moyu 的 `user_patch_favorite_relation.id` 或 user_id），至少让它可追溯。

---

## §2 缺面：批量成员判定

`GET /v2/me/folders?contains_work_id=<id>` 只接一个 work。

moyu 的发售日历一屏是一个月的游戏，要回答的是「这一屏里哪些是我收藏过的」。今天只能二选一：

- 一行一个请求 → 一页 30~60 个请求；
- 或者把这个人的整份收藏读回来做集合 → moyu 选了这条。

代价（生产分布，10669 个有收藏的账号）：

```
avg 23 | p50 2 | p90 39 | p99 374 | max 4028   条/人
```

`limit=100` 分页，所以 p50 一次请求、p99 四次、最大的那个人 41 次——**为了在日历上点亮几个心**。

**请求**：`/v2/me/folders` 的 `contains_work_id` 支持多值（逗号分隔，≤100，和 `ids=` 一致），或者更直接一点，开一个

```
GET /v2/me/folders/holdings?work_ids=285,898,1551
→ {"object":"list","items":[{"work_id":"285","folder_ids":["11","12"]}, ...]}
```

后者更好用：调用方问的是「这些里哪些在我的收藏里」，不是「哪些收藏夹装了它」。

---

## §3 缺面：按作品反查持有者（通知扇出）

moyu 有一条通知：**某个游戏发布了新补丁资源时，通知收藏过这个游戏的人**。

它需要的是「谁的收藏夹里有这个 work」。catalog 只答「**我的**哪些收藏夹里有这个 work」，没有反向面。所以这条通知今天仍然读 moyu 那张已经冻结的 `user_patch_favorite_relation`——**名单停在 2026-09-06**：09-06 之后收藏的人收不到，09-06 之后取消收藏的人还在被打扰。这是 moyu 唯一一处没修掉的地方，因为没有东西可读。

**请求（二选一，倾向 A）**：

**A. s2s 点查**

```
GET /v2/folders/holders?work_id=<id>&limit=&cursor=
→ {"object":"list","items":[{"owner_uid":"121089"}], "next_cursor":null}
```

- **必须包含私有收藏夹的持有者**：一个人私下收藏了游戏，他照样想要这条通知；这个面的用途是扇出，不是展示。
- 正因如此它**只能给应用 key**，用户令牌一律 403，并且**只回 uid，不回收藏夹 id、名称或可见性**——否则它就成了「某某收藏了什么」的探针。
- 建议挂一个运营授予、不可自助申请的 scope，和 `claim_events:read` 同一档。
- 成本很低：`idx_catalog_user_folder_item_work_id` 已经在表上了。

**B. 变更流**

```
GET /v2/catalog/folder-events?cursor=
```

和 `claim-events` 同形，下游自己维护订阅表。更符合 catalog 现有的设计（`idx_catalog_user_folder_item_sync` 就是 `(owner_uid, updated_at)`），但下游要多一张表和一个 cron，而且 moyu 刚刚才把本地副本删掉。

如果两个都不做，也请明说——那 moyu 会把这条通知改成只覆盖「在 moyu 点的心」，也就是重新建一张本地订阅表，语义上从「你收藏的游戏」退成「你在本站收藏的游戏」。

---

## §4 账号清除没有触达收藏夹

`DELETE /v2/moderation/users/{uid}/folders` 已经有了，文档也写得很清楚：「账号删除路径必须够到正典存储，否则人走了收藏还留在一个他再也打不开的面上」。同意。

问题是它要 **moderator 的用户令牌**。moyu 的用户清除是后台管理动作，手上确实有管理员的 access token，理论上打得通。但：

1. 下游的**自动化**账号清除走「借某个管理员的用户令牌」这条路合适吗？还是应该有一条应用 key 的 s2s 等价面？
2. 没有对应的**读**面。清除前 moyu 要给管理员看一份「将删除什么」的预览，收藏那一栏现在只能空着——`purgeUserFolders` 的回执是删完才给的。有没有可能给一个 `GET /v2/moderation/users/{uid}/folders`（只回计数）？

在这两点定下来之前，moyu **没有**接这个调用（不想把一个未经验证的跨服务破坏性删除塞进已有的破坏性流程里），并且已经把后台文案改成明说「收藏夹归 catalog，本操作不涉及」。

---

## §5 moyu 侧已经做完的（供对照，不需要 infra 做什么）

commit `45ca7647`，三个缺陷：

1. **档案页的收藏计数来自已冻结的表**。`user_patch_favorite_relation` 在切换后无人写入（`ToggleFavoriteInCatalog` 只写 catalog），计数从部署那天起就不动了。121089 的 10 行全是切换前的，所以数字看着还对；121959 有 1417 条收藏夹条目、0 行旧表，读出来是 0。现在计数和列表走同一条管线、同一个读者、同一个 NSFW 门。
2. **`/user/:id/favorite` 路由没挂任何鉴权中间件**（隔壁 `/user/:id/folder` 有 `optionalAuth`）。切换前这个 handler 只读本地表，不需要身份；切换后它要拿读者的 token 问 catalog，但路由注册没跟着改，于是每个读者都是陌生人，只有 public 收藏夹会回答。和 §1 叠加，本人也看不见自己的收藏。
3. **收藏夹详情面没有富化**，直接吐裸的 `model.Patch`。前端把它当 `GalgameCard` 用，于是没有 name、没有封面、`count.resource` 不存在 → 每张卡都是「暂无补丁」的空框，但 `id` 在，所以点了能跳对页面——就是用户描述的那个现象。

另外顺手修掉的：账号清除会用**冻结的旧表**重算 `patch.favorite_count`，把切换后的每一个心都抹回 09-06 的值。那个计数器现在由 `settleFavoriteSideEffects` 增量维护，本地无法重算，已从重算 SQL 里删掉。

---

## 附：复现

```
$ curl -s 'https://www.moyu.moe/api/v1/user/121089/favorite?page=1&limit=24'
{"code":0,"message":"OK","data":{"items":[],"total":0}}

$ curl -s 'https://www.moyu.moe/api/v1/user/121089'
{... "favorite_count":10 ...}
```

```sql
-- kun_catalog：这个人唯一的收藏夹，10 项，private，09-07 建
SELECT id, owner_uid, name, visibility, is_default, item_count, created_at
FROM catalog_user_folder WHERE owner_uid = 121089;
--  11819 | 121089 |  | 0 | t | 10 | 2026-09-07 12:13:09.974358+00
```

---

## §6 结果（2026-09-09 当日）

infra PR #179 合入 `6442f577`，spec 2.21.0 → 2.22.0，零迁移，当天 16:21 部署。

| 本报告 | infra 的答复 | moyu 侧 |
|---|---|---|
| §1 3633 个 private 默认夹 | 已翻 public。09-07 那批 `is_default` 现为 **3665 public / 1 private**，121089 的 11819 号夹在内 | 报障闭合 |
| §2 批量成员判定 | `GET /v2/me/folders/holdings?work_ids=`（≤100，用户令牌） | 日历已接 |
| §3 反查持有者 | `GET /v2/folders/holders?work_id=`（只回 uid，含私密夹，应用 key + `folder_holders:read`） | 通知扇出已接 |
| §4 清除预览 | `GET /v2/moderation/users/{uid}/folders` | 预览已接，**删除面不接**，见下 |

`folder_holders:read` 已授予 moyu 的生产 key（`developer_api_keys` id 54
`moyu-patch-internal-s2s-v2`）。

### §4 的结论变了：moyu 不调用 `purgeUserFolders`

原来的理由是「未经验证的跨服务破坏性删除」。查证之后有一条更硬的：

```
kun_catalog=# \d catalog_user_folder
 id | owner_uid | name | description | visibility | is_default | item_count | created_at | updated_at
```

**没有 site 列。** 一个人一份收藏夹，moyu 和 kungal 看的是同一份；生产 11995 个夹里
**8283 个是论坛的导入器建的**，7333 个人名下至少有一个来自论坛。而 moyu 的「用户清除」
删的是**本站账号**，页面上写明「kungal 不受影响」。从这个入口调 `purgeUserFolders`，
会把一个论坛用户的整份收藏一起删掉。

所以：**读接、删不接**。预览里现在会显示「该账号有 N 个收藏夹、共 M 个游戏 · 本操作不删除」，
读不到就显示原因（管理员在 catalog 没有审核权时是 403，如实写出来而不是显示 0）。

删除中央账号的收藏夹属于**删除中央账号**的那条路径——OAuth 侧的账号注销——不属于任何一个
下游站点的本地清除。如果 infra 认为下游也该有这个能力，那需要的是一条按 site 限定的面，
而不是现在这条按 owner_uid 全删的面。

### 一条附带发现

`/v2/folders/holders` 回的是**中央登录 uid**，不是任何一个站点的用户 id——文档里写了，
但值得在这里再说一次，因为代价是静默的：moyu 的 `user_message.recipient_id` 有指向本地
`user` 表的外键，而 **11005 个持有收藏夹的账号里有 5308 个在 moyu 没有行**。不做求交就直接
扇出，那 5308 个人的插入全部违反外键，而 `createDedupMessage` 丢掉了 `Create` 的错误——
一次静默的半数丢失。moyu 已在扇出前和本地 `user` 表求交。

---

## §7 2026-09-12 追加：同一个收藏夹，两个站点渲染出不同的游戏

**来源**：用户报障 ——「补丁站的收藏夹还是没图，而且同样是默认收藏夹，补丁站的默认收藏夹
好歹还是存的我以前收藏的游戏，主站收藏夹里的游戏大部分我完全没收藏过，除了数量完全不像
相同的收藏夹。」

两件事，各归各家。

### 补丁站「没图」：moyu 自己的缺陷，已修，待部署

`GET /api/v1/folder/:id` 把 `model.Patch` 原样发了出去。生产实测：

```
$ curl https://www.moyu.moe/api/v1/folder/11819
{"patches":[{"id":5355,"vndb_id":"v21429","status":0,"download":1013, ...}]}
```

没有 `name`，没有 `galgame`，因而没有 `effective_portrait_hash`——卡片的封面取自那个
字段，取不到就画 `lucide:image-off` 的占位框，`count.resource` 也不存在，于是每张卡都是
一个空框加「暂无补丁」。修复是 commit `45ca7647`（`EnrichPatchCards`），**在 PR #36 里，
尚未合并**，所以线上仍是坏的。这条与 catalog 无关，不需要 infra 做任何事。

### 主站「大部分没收藏过」：收藏夹项的 id 空间被读错了

收藏夹里存的是什么，先落实：`catalog_user_folder_item.work_id` 上没有外键，但生产
**252,544 条项 100% 能解析成 `catalog_work.id`**（只有 250,720 条能解析成 gid），所以它
是 **catalog work id**，不是任何下游站点的 id。moyu 按这个读（`patch.catalog_work_id`，
按 vndb 锚点回填），读出来的就是用户当年收藏的那些游戏——和报障一致。

kungal 的合集详情页把同一批 `work_id` 当成**自己的 gid** 喂给了按 gid 解析的批量面
（`CollectionService.GetDetail` → `HydrateCardsByIDs` → `GetBatchPublic` →
`CatalogRowsByGIDs`），封面预览那条路径同样（`resolvePreviewCovers` 把 `it.WorkID`
直接塞进 `allGids`）。两个 id 空间是错位的，于是解析得到的是**另一个游戏**。

以报障用户的默认收藏夹 11819（10 项）为例，左边是收藏夹里真正的作品，右边是按 gid 解析
后会渲染出来的作品：

```
work 5259 ビッチ学園が清純なはずがないっ！！？  ->  work 5165 ようこそ種付け村へ…
work 4417 創刻のアテリアル                      ->  work 4367 その花びらにくちづけを…
work 3788 珊海王の円環                          ->  work 3768 状況開始っ!
work 3168 忍堕とし                              ->  work 3159 雪花 -きら-
work 2927 大番長 -Big Bang Age-                 ->  work 2920 幻創のイデア
work 1551 姫と穢欲のサクリファイス              ->  work 1547 隷嬢管理棟
work  898 デモニオン                            ->  work  898 デモニオン        （偶然对上）
work  285 戦巫〈センナギ〉                       ->  work  285 戦巫〈センナギ〉  （偶然对上）
work 207230 屋根裏の眠り姫                      ->  （无 kungal 认领，整条消失）
work 210962 ぐるぐる痴漢電車                    ->  （同上）
```

八对二，数量却都是 8——这正是「除了数量完全不像相同的收藏夹」。全量同样：

```sql
SELECT count(*) AS items,
       count(*) FILTER (WHERE g.id IS NULL)              AS 渲染不出来,
       count(*) FILTER (WHERE g.id = i.work_id)          AS 碰巧对,
       count(*) FILTER (WHERE g.id IS NOT NULL AND g.id <> i.work_id) AS 画成了别的游戏
  FROM catalog_user_folder_item i
  LEFT JOIN catalog_work g ON g.site = 'kungal' AND g.product_work_id = i.work_id;

 252544 | 1686 | 58680 | 192178
```

**76% 的收藏夹项在主站上被画成了另一个游戏**，23% 碰巧对上（两个 id 空间都是稠密的小
整数，撞上是常态），0.7% 干脆消失。这解释了为什么它「看起来像个别人的收藏夹」而不是
「看起来像坏了」——后者会被立刻发现，前者不会。

修复在 kungal 仓，不在这里：喂进 `HydrateCardsByIDs` / `GetBatchPublic` 之前，
`work_id` 要先经过一次 work → gid 的映射，和 moyu 的 `PatchIDsByWorkIDs` 同一层意思。

**给 infra 的那一条**：这是同一份数据被两个下游按两种 id 空间读的第一个实例，而且失败是
静默的——没有 404，没有报错，只是换了个游戏。如果 catalog 愿意在 `/v2/folders/{id}/items`
的响应里除 `work_id` 外再带上调用方 site 的 `product_work_id`（或者干脆让 `include=`
支持把 work 一起展开），这一类错误就没有发生的余地了。要不要做由 infra 定，moyu 这边不阻塞。
