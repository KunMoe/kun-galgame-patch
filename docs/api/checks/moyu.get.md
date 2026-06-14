# moyu 后端 — GET API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：**审计完成（2026-05-29）**。详细逐端点报告见 [`_audit/`](./_audit/)。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：对齐 = 无问题 · 已修 = 已修复 · 保持 = 有意保持 · 待审计 · 新增
鉴权：公开 · 可选登录 = OptionalAuth · 登录 · 管理 = admin/mod · 仅admin · 限流

## 统计

- 本服务 GET 端点：**80**
  - 认证 1 · 补丁 8 · Galgame 代理 11 · 分类代理（基础 12 + 修订 8）20 · 用户 11 · 消息 3 · 管理 11 · 公共 8 · 聊天 3 · 外部 2 · 关于 2
- 本轮：已修复 8 · 代理透传 28 · 其余对齐无误

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/auth/me` | 登录 | `authH.Me` | 对齐 | MeResponse 与 userStore.user / settings 逐字段对齐（实测）；`moemoepoint` 取本地读缓存 |

## 2. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/patch/duplicate` | 登录 | `patchH.CheckDuplicate` | 对齐 | 返回 `{exists:bool}`，与 VNDBInput.vue 对齐 |
| `GET /api/v1/patch/:id` | 可选登录 | `patchH.GetPatch` | 对齐 | `PatchHeader = GalgameCard + is_favorite`，NSFW gate 实测 |
| `GET /api/v1/patch/:id/detail` | 可选登录 | `patchH.GetPatchDetail` | 已修 | 空 `tags/officials/wiki_engine_ids` 原序列化为 `null`（FE 非空数组）→ enricher 初始化为 `[]` |
| `GET /api/v1/patch/:id/comment` | 可选登录 | `patchH.GetComments` | 对齐 | 分页 `{items,total}`，仅 `parent_id IS NULL AND status=0` 计数，reply 预载 status=0 |
| `GET /api/v1/patch/:id/resource` | 可选登录 | `patchH.GetResources` | 对齐 | 裸数组；**有意**保留 content/code/password（前端补丁页就地揭示 + 编辑回填）；s3 直链读时materialize |
| `GET /api/v1/patch/:id/contributor` | 公开 | `patchH.GetContributors` | 对齐 | NSFW gate 读 query content_limit（无 auth 中间件也有效）|
| `GET /api/v1/patch/comment/:commentId/markdown` | 公开 | `patchH.GetCommentMarkdown` | 对齐 | gate 查所属 patch 的 content_limit，防匿名按 id 拉 NSFW 评论 |
| `GET /api/v1/patch/resource/:resourceId/link` | 公开 限流 | `patchH.GetResourceDownloadInfo` | 已修 | 限流键原恒落 IP（前置无 auth）→ 路由加 `optionalAuth`，登录用户按 `user:<id>` 30/min |

## 3. Galgame 投稿 / 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/galgame/mine` | 登录 | `patchH.ListMyGalgames` | 对齐 | 我的投稿（分页 `{items,total}`）|
| `GET /api/v1/galgame/search/publish` | 登录 | `patchH.SearchGalgameForPublish` | 对齐 | 发布流程内搜索 |
| `GET /api/v1/galgame/messages/mine` | 登录 | `patchH.GetMyWikiMessages` | 对齐 | 暂无 FE 调用方（dead-but-correct）|
| `GET /api/v1/galgame/messages/read-state` | 登录 | `patchH.GetWikiMessagesReadState` | 对齐 | 同上 |
| `GET /api/v1/galgame/:gid/revisions` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理透传；NSFW gate 对 `:gid` fail-closed（实测）|
| `GET /api/v1/galgame/:gid/revisions/:rev` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/galgame/:gid/revisions/:rev/diff` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/galgame/:gid/prs` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/galgame/:gid/prs/:prid` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/galgame/:gid/links` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理（响应含未声明 `user_id`，无害）|
| `GET /api/v1/galgame/:gid/aliases` | 可选登录 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 4. 分类代理 `/tag /official /engine /series`（→ Wiki）

### 4.1 基础读（12）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/tag/search` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理（literal 先于 `:name`）|
| `GET /api/v1/tag/multi` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/tag/:name` | 公开 | `patchH.WikiTaxonomyDetailProxy` | 已修 | 重写 `galgame→galgames`(GalgameCard)；降级卡 `type/language/platform` 原为 `null` → 初始化 `[]`（`created` 零值时间见 README 遗留）|
| `GET /api/v1/official` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/official/search` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/official/:name` | 公开 | `patchH.WikiTaxonomyDetailProxy` | 已修 | 同 `/tag/:name` 降级卡修复 |
| `GET /api/v1/engine` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/engine/:name` | 公开 | `patchH.WikiEditProxy` | 保持 | 通用透传（无 GalgameCard 重写）|
| `GET /api/v1/series` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/series/search` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/series/:id` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |

### 4.2 修订历史读（8 = 4 实体 × 2）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag/:id/revisions` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/tag/:id/revisions/:rev` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/official/:id/revisions` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/official/:id/revisions/:rev` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/engine/:id/revisions` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/engine/:id/revisions/:rev` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/series/:id/revisions` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |
| `GET /api/v1/series/:id/revisions/:rev` | 公开 | `patchH.WikiEditProxy` | 保持 | 代理 |

## 5. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/user/search` | 登录 | `userH.SearchUsers` | 对齐 | query 参数为 `query`（非 keyword）；暂无 FE 调用方 |
| `GET /api/v1/user/moemoepoint/log` | 登录 | `userH.GetMoemoepointLog` | 新增·对齐 | 自助流水；id 取 session 非路径参（无 IDOR）；`{items,has_more}` 对齐 |
| `GET /api/v1/user/:id` | 可选登录 | `userH.GetUserInfo` | 对齐 | 14 字段对齐 UserInfo；OAuth 失败优雅降级 |
| `GET /api/v1/user/:id/floating` | 公开 | `userH.GetUserFloating` | 对齐 | 悬浮卡；暂无 FE 调用方 |
| `GET /api/v1/user/:id/patch` | 公开 | `userH.GetUserPatches` | 对齐 | 低优先级遗留：`min=1` 使缺省补值成死代码（见 README）|
| `GET /api/v1/user/:id/resource` | 公开 | `userH.GetUserResources` | 已修 | 个人页资源卡不读秘密字段 → `StripResourceSecrets` 清 content/code/password/s3_key |
| `GET /api/v1/user/:id/favorite` | 公开 | `userH.GetUserFavorites` | 对齐 | EnrichPatches；NSFW 过滤 |
| `GET /api/v1/user/:id/comment` | 公开 | `userH.GetUserComments` | 对齐 | total 为未过滤计数（filter-after-paginate，跨域一致）|
| `GET /api/v1/user/:id/contribute` | 公开 | `userH.GetUserContributions` | 对齐 | 子查询 contribute_relation 正确 |
| `GET /api/v1/user/:id/follower` | 可选登录 | `userH.GetFollowers` | 对齐 | 每行 is_followed（单查询）|
| `GET /api/v1/user/:id/following` | 可选登录 | `userH.GetFollowing` | 对齐 | 方向正确（follower_id=:id）|

## 6. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/message/` | 登录 | `messageH.GetMessages` | 对齐 | 分页 `{items,total}`；@消息页前端 shape 不符已**前端修复**（见 README #7）|
| `GET /api/v1/message/all` | 登录 | `messageH.GetAllMessages` | 对齐 | system 消息 `sender_id:null` 正确省略 `sender` |
| `GET /api/v1/message/unread` | 登录 | `messageH.GetUnreadTypes` | 对齐 | 裸 `string[]`（铃铛红点）|

## 7. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/admin/comment` | 管理 | `adminH.GetComments` | 对齐 | （低）`status<>0` vs FE `===1`，仅 0/1 取值无差异 |
| `GET /api/v1/admin/resource` | 管理 | `adminH.GetResources` | 对齐 | FE `r.user?.name` 安全访问 |
| `GET /api/v1/admin/resource/:id/history` | 管理 | `adminH.GetResourceFileHistory` | 对齐 | FileHistoryItem 字段逐一对齐 |
| `GET /api/v1/admin/user/:id/purge-preview` | 仅admin | `adminH.GetUserPurgePreview` | 对齐 | admin-only 确认；dry-run |
| `GET /api/v1/admin/setting/comment-verify` | 管理 | `adminH.GetCommentVerify` | 对齐 | `{enabled}` |
| `GET /api/v1/admin/setting/creator-only` | 管理 | `adminH.GetCreatorOnly` | 对齐 | `{enabled}` |
| `GET /api/v1/admin/stats` | 管理 | `adminH.GetStats` | 已修 | `new_patch_resource`→`new_resource`（前端"新发布补丁"卡原恒 0）|
| `GET /api/v1/admin/stats/sum` | 管理 | `adminH.GetStatsSum` | 已修 | `patch_resource_count`/`patch_comment_count`→`resource_count`/`comment_count`（两卡原恒 0）|
| `GET /api/v1/admin/log` | 管理 | `adminH.GetLogs` | 对齐 | `l.user?.name ?? '系统'` 安全 |
| `GET /api/v1/admin/galgame` | 管理 | `adminH.GetGalgame` | 对齐 | 富化 GalgameCard（content_limit=all）|
| `GET /api/v1/admin/patch/orphans` | 管理 | `adminH.GetOrphanPatches` | 对齐 | 手搓 map（带 pending/bad_vndb count）；（低）共享 FE 类型陈旧，页面自带正确局部类型 |

## 8. 公共（无前缀组）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/home` | 公开 | `commonH.GetHome` | 已修 | resources 切片清秘密字段（卡片不读，防批量收割）|
| `GET /api/v1/home/random` | 公开 | `patchH.GetRandomPatch` | 对齐 | `{id}`；NSFW 采样过滤 |
| `GET /api/v1/galgame` | 公开 | `commonH.GetGalgameList` | 对齐 | `{galgames,total}`；必填 SelectedType/SortField/SortOrder |
| `GET /api/v1/comment` | 公开 | `commonH.GetGlobalComments` | 对齐 | `{items,total}`；status=0 过滤 |
| `GET /api/v1/resource` | 公开 | `commonH.GetGlobalResources` | 已修 | 全站资源流（全表分页）清秘密字段（最大批量收割面）|
| `GET /api/v1/resource/:id` | 可选登录 | `commonH.GetResourceDetail` | 已修 | **主体保留** content/code/password（揭示面）；**recommendations 清秘密字段** |
| `GET /api/v1/ranking/user` | 公开 | `commonH.GetUserRanking` | 对齐 | RankingUser 对齐；banned-skip 正确 |
| `GET /api/v1/ranking/patch` | 公开 | `commonH.GetPatchRanking` | 对齐 | 裸 `GalgameCard[]`；sort 别名兼容 |

## 9. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/chat/room` | 登录 | `chatH.ListRooms` | 对齐 | 空房过滤；PRIVATE peer 覆写 |
| `GET /api/v1/chat/room/:link` | 登录 | `chatH.GetRoomDetail` | 对齐 | 成员鉴权（非成员 → 房间不存在）|
| `GET /api/v1/chat/room/:link/message` | 登录 | `chatH.ListMessages` | 已修 | 4 种取数模式实测正确；`ids` 模式缺 `limit` 原 422 → `limit` 改 `omitempty`（reaction `null` vs `[]` 见 README 遗留）|

## 10. 外部 API

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/hikari` | 公开 | `commonH.GetHikari` | 保持 | 外部接口；仅 s3 清 content，netdisk content/code/password 仍下发（疑有意共享，见 README 遗留）|
| `GET /api/v1/moyu/patch/has-patch` | 公开 | `commonH.GetMoyuHasPatch` | 对齐 | 裸 vndb_id 数组（有补丁资源的）；无敏感字段 |

## 11. 关于 / 文档（静态 .mdx）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/about/posts` | 公开 | `aboutH.ListPosts` | 对齐 | `{items,tree}` |
| `GET /api/v1/about/post` | 公开 | `aboutH.GetPost` | 已修 | `..` 路径穿越原返回 50000 → 改 `os.ErrNotExist` → 404 |
