# moyu 后端 — GET API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：**审计完成（2026-05-29）**。详细逐端点报告见 [`_audit/`](./_audit/)。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 GET 端点：**80**
  - 认证 1 · 补丁 8 · Galgame 代理 11 · 分类代理（基础 12 + 修订 8）20 · 用户 11 · 消息 3 · 管理 11 · 公共 8 · 聊天 3 · 外部 2 · 关于 2
- 本轮：🔧 修复 8 · ⏭️ 代理透传 28 · ✅ 其余对齐无误

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/auth/me` | 🔒 | `authH.Me` | ✅ | MeResponse 与 userStore.user / settings 逐字段对齐（实测）；`moemoepoint` 取本地读缓存 |

## 2. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/patch/duplicate` | 🔒 | `patchH.CheckDuplicate` | ✅ | 返回 `{exists:bool}`，与 VNDBInput.vue 对齐 |
| `GET /api/v1/patch/:id` | 🔐 | `patchH.GetPatch` | ✅ | `PatchHeader = GalgameCard + is_favorite`，NSFW gate 实测 |
| `GET /api/v1/patch/:id/detail` | 🔐 | `patchH.GetPatchDetail` | 🔧 | 空 `tags/officials/wiki_engine_ids` 原序列化为 `null`（FE 非空数组）→ enricher 初始化为 `[]` |
| `GET /api/v1/patch/:id/comment` | 🔐 | `patchH.GetComments` | ✅ | 分页 `{items,total}`，仅 `parent_id IS NULL AND status=0` 计数，reply 预载 status=0 |
| `GET /api/v1/patch/:id/resource` | 🔐 | `patchH.GetResources` | ✅ | 裸数组；**有意**保留 content/code/password（前端补丁页就地揭示 + 编辑回填）；s3 直链读时materialize |
| `GET /api/v1/patch/:id/contributor` | 🌐 | `patchH.GetContributors` | ✅ | NSFW gate 读 query content_limit（无 auth 中间件也有效）|
| `GET /api/v1/patch/comment/:commentId/markdown` | 🌐 | `patchH.GetCommentMarkdown` | ✅ | gate 查所属 patch 的 content_limit，防匿名按 id 拉 NSFW 评论 |
| `GET /api/v1/patch/resource/:resourceId/link` | 🌐 ⏱️ | `patchH.GetResourceDownloadInfo` | 🔧 | 限流键原恒落 IP（前置无 auth）→ 路由加 `optionalAuth`，登录用户按 `user:<id>` 30/min |

## 3. Galgame 投稿 / 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/galgame/mine` | 🔒 | `patchH.ListMyGalgames` | ✅ | 我的投稿（分页 `{items,total}`）|
| `GET /api/v1/galgame/search/publish` | 🔒 | `patchH.SearchGalgameForPublish` | ✅ | 发布流程内搜索 |
| `GET /api/v1/galgame/messages/mine` | 🔒 | `patchH.GetMyWikiMessages` | ✅ | 暂无 FE 调用方（dead-but-correct）|
| `GET /api/v1/galgame/messages/read-state` | 🔒 | `patchH.GetWikiMessagesReadState` | ✅ | 同上 |
| `GET /api/v1/galgame/:gid/revisions` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理透传；NSFW gate 对 `:gid` fail-closed（实测）|
| `GET /api/v1/galgame/:gid/revisions/:rev` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/galgame/:gid/revisions/:rev/diff` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/galgame/:gid/prs` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/galgame/:gid/prs/:prid` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/galgame/:gid/links` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理（响应含未声明 `user_id`，无害）|
| `GET /api/v1/galgame/:gid/aliases` | 🔐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

## 4. 分类代理 `/tag /official /engine /series`（→ Wiki）

### 4.1 基础读（12）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/tag/search` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理（literal 先于 `:name`）|
| `GET /api/v1/tag/multi` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/tag/:name` | 🌐 | `patchH.WikiTaxonomyDetailProxy` | 🔧 | 重写 `galgame→galgames`(GalgameCard)；降级卡 `type/language/platform` 原为 `null` → 初始化 `[]`（`created` 零值时间见 README 遗留）|
| `GET /api/v1/official` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/official/search` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/official/:name` | 🌐 | `patchH.WikiTaxonomyDetailProxy` | 🔧 | 同 `/tag/:name` 降级卡修复 |
| `GET /api/v1/engine` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/engine/:name` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 通用透传（无 GalgameCard 重写）|
| `GET /api/v1/series` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/series/search` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/series/:id` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

### 4.2 修订历史读（8 = 4 实体 × 2）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/tag/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/official/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/official/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/engine/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/engine/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/series/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |
| `GET /api/v1/series/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏭️ | 代理 |

## 5. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/user/search` | 🔒 | `userH.SearchUsers` | ✅ | query 参数为 `query`（非 keyword）；暂无 FE 调用方 |
| `GET /api/v1/user/moemoepoint/log` | 🔒 | `userH.GetMoemoepointLog` | 🆕✅ | 自助流水；id 取 session 非路径参（无 IDOR）；`{items,has_more}` 对齐 |
| `GET /api/v1/user/:id` | 🔐 | `userH.GetUserInfo` | ✅ | 14 字段对齐 UserInfo；OAuth 失败优雅降级 |
| `GET /api/v1/user/:id/floating` | 🌐 | `userH.GetUserFloating` | ✅ | 悬浮卡；暂无 FE 调用方 |
| `GET /api/v1/user/:id/patch` | 🌐 | `userH.GetUserPatches` | ✅ | 低优先级遗留：`min=1` 使缺省补值成死代码（见 README）|
| `GET /api/v1/user/:id/resource` | 🌐 | `userH.GetUserResources` | 🔧 | 个人页资源卡不读秘密字段 → `StripResourceSecrets` 清 content/code/password/s3_key |
| `GET /api/v1/user/:id/favorite` | 🌐 | `userH.GetUserFavorites` | ✅ | EnrichPatches；NSFW 过滤 |
| `GET /api/v1/user/:id/comment` | 🌐 | `userH.GetUserComments` | ✅ | total 为未过滤计数（filter-after-paginate，跨域一致）|
| `GET /api/v1/user/:id/contribute` | 🌐 | `userH.GetUserContributions` | ✅ | 子查询 contribute_relation 正确 |
| `GET /api/v1/user/:id/follower` | 🔐 | `userH.GetFollowers` | ✅ | 每行 is_followed（单查询）|
| `GET /api/v1/user/:id/following` | 🔐 | `userH.GetFollowing` | ✅ | 方向正确（follower_id=:id）|

## 6. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/message/` | 🔒 | `messageH.GetMessages` | ✅ | 分页 `{items,total}`；@消息页前端 shape 不符已**前端修复**（见 README #7）|
| `GET /api/v1/message/all` | 🔒 | `messageH.GetAllMessages` | ✅ | system 消息 `sender_id:null` 正确省略 `sender` |
| `GET /api/v1/message/unread` | 🔒 | `messageH.GetUnreadTypes` | ✅ | 裸 `string[]`（铃铛红点）|

## 7. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/admin/comment` | 🛡️ | `adminH.GetComments` | ✅ | （低）`status<>0` vs FE `===1`，仅 0/1 取值无差异 |
| `GET /api/v1/admin/resource` | 🛡️ | `adminH.GetResources` | ✅ | FE `r.user?.name` 安全访问 |
| `GET /api/v1/admin/resource/:id/history` | 🛡️ | `adminH.GetResourceFileHistory` | ✅ | FileHistoryItem 字段逐一对齐 |
| `GET /api/v1/admin/user/:id/purge-preview` | ⚙️ | `adminH.GetUserPurgePreview` | ✅ | admin-only 确认；dry-run |
| `GET /api/v1/admin/setting/comment-verify` | 🛡️ | `adminH.GetCommentVerify` | ✅ | `{enabled}` |
| `GET /api/v1/admin/setting/creator-only` | 🛡️ | `adminH.GetCreatorOnly` | ✅ | `{enabled}` |
| `GET /api/v1/admin/stats` | 🛡️ | `adminH.GetStats` | 🔧 | `new_patch_resource`→`new_resource`（前端"新发布补丁"卡原恒 0）|
| `GET /api/v1/admin/stats/sum` | 🛡️ | `adminH.GetStatsSum` | 🔧 | `patch_resource_count`/`patch_comment_count`→`resource_count`/`comment_count`（两卡原恒 0）|
| `GET /api/v1/admin/log` | 🛡️ | `adminH.GetLogs` | ✅ | `l.user?.name ?? '系统'` 安全 |
| `GET /api/v1/admin/galgame` | 🛡️ | `adminH.GetGalgame` | ✅ | 富化 GalgameCard（content_limit=all）|
| `GET /api/v1/admin/patch/orphans` | 🛡️ | `adminH.GetOrphanPatches` | ✅ | 手搓 map（带 pending/bad_vndb count）；（低）共享 FE 类型陈旧，页面自带正确局部类型 |

## 8. 公共（无前缀组）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/home` | 🌐 | `commonH.GetHome` | 🔧 | resources 切片清秘密字段（卡片不读，防批量收割）|
| `GET /api/v1/home/random` | 🌐 | `patchH.GetRandomPatch` | ✅ | `{id}`；NSFW 采样过滤 |
| `GET /api/v1/galgame` | 🌐 | `commonH.GetGalgameList` | ✅ | `{galgames,total}`；必填 SelectedType/SortField/SortOrder |
| `GET /api/v1/comment` | 🌐 | `commonH.GetGlobalComments` | ✅ | `{items,total}`；status=0 过滤 |
| `GET /api/v1/resource` | 🌐 | `commonH.GetGlobalResources` | 🔧 | 全站资源流（全表分页）清秘密字段（最大批量收割面）|
| `GET /api/v1/resource/:id` | 🔐 | `commonH.GetResourceDetail` | 🔧 | **主体保留** content/code/password（揭示面）；**recommendations 清秘密字段** |
| `GET /api/v1/ranking/user` | 🌐 | `commonH.GetUserRanking` | ✅ | RankingUser 对齐；banned-skip 正确 |
| `GET /api/v1/ranking/patch` | 🌐 | `commonH.GetPatchRanking` | ✅ | 裸 `GalgameCard[]`；sort 别名兼容 |

## 9. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/chat/room` | 🔒 | `chatH.ListRooms` | ✅ | 空房过滤；PRIVATE peer 覆写 |
| `GET /api/v1/chat/room/:link` | 🔒 | `chatH.GetRoomDetail` | ✅ | 成员鉴权（非成员 → 房间不存在）|
| `GET /api/v1/chat/room/:link/message` | 🔒 | `chatH.ListMessages` | 🔧 | 4 种取数模式实测正确；`ids` 模式缺 `limit` 原 422 → `limit` 改 `omitempty`（reaction `null` vs `[]` 见 README 遗留）|

## 10. 外部 API

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/hikari` | 🌐 | `commonH.GetHikari` | ⏭️ | 外部接口；仅 s3 清 content，netdisk content/code/password 仍下发（疑有意共享，见 README 遗留）|
| `GET /api/v1/moyu/patch/has-patch` | 🌐 | `commonH.GetMoyuHasPatch` | ✅ | 裸 vndb_id 数组（有补丁资源的）；无敏感字段 |

## 11. 关于 / 文档（静态 .mdx）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/about/posts` | 🌐 | `aboutH.ListPosts` | ✅ | `{items,tree}` |
| `GET /api/v1/about/post` | 🌐 | `aboutH.GetPost` | 🔧 | `..` 路径穿越原返回 50000 → 改 `os.ErrNotExist` → 404 |
