# moyu 后端 — GET API 清单

> 服务: **moyu**（`apps/api/cmd/server`） · Base URL: `/api/v1`
>
> 路由源: `apps/api/internal/app/router.go`
>
> 配套: [moyu.post.md](./moyu.post.md) · [moyu.put.md](./moyu.put.md) · [moyu.delete.md](./moyu.delete.md) · [moyu.patch.md](./moyu.patch.md) · [README](./README.md)
>
> 状态：全部 ⏳ 待审计（inventory）。图例见 [README](./README.md#图例--审计状态)。

## 图例（简）

审计：✅ 无问题 · 🔧 已修 · ⏭️ 有意保持 · ⏳ 待审计 · 🆕 新增
鉴权：🌐 公开 · 🔐 OptionalAuth · 🔒 登录 · 🛡️ admin/mod · ⚙️ 仅 admin · ⏱️ 限流

## 统计

- 本服务 GET 端点：**80**
  - 认证 1 · 补丁 8 · Galgame 代理 11 · 分类代理（基础 12 + 修订 8）20 · 用户 11 · 消息 3 · 管理 11 · 公共 8 · 聊天 3 · 外部 2 · 关于 2

---

## 1. 认证 / 身份

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/auth/me` | 🔒 | `authH.Me` | ⏳ | 当前登录用户 profile（含 moemoepoint 余额）|

## 2. 补丁 / 评论 / 资源 `/patch`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/patch/duplicate` | 🔒 | `patchH.CheckDuplicate` | ⏳ | 查重：vndb_id 是否已存在 |
| `GET /api/v1/patch/:id` | 🔐 | `patchH.GetPatch` | ⏳ | 补丁/galgame 概要；viewer 的 like/收藏态 |
| `GET /api/v1/patch/:id/detail` | 🔐 | `patchH.GetPatchDetail` | ⏳ | 详情 |
| `GET /api/v1/patch/:id/comment` | 🔐 | `patchH.GetComments` | ⏳ | 评论树 |
| `GET /api/v1/patch/:id/resource` | 🔐 | `patchH.GetResources` | ⏳ | 资源列表 |
| `GET /api/v1/patch/:id/contributor` | 🌐 | `patchH.GetContributors` | ⏳ | moyu 侧贡献者（上传资源者，非 Wiki 贡献者）|
| `GET /api/v1/patch/comment/:commentId/markdown` | 🌐 | `patchH.GetCommentMarkdown` | ⏳ | 评论 markdown 源（编辑回填用）|
| `GET /api/v1/patch/resource/:resourceId/link` | 🌐 ⏱️ | `patchH.GetResourceDownloadInfo` | ⏳ | 下载直链；`RateLimit 30/min`（按 userID/IP）防批量爬 |

## 3. Galgame 投稿 / 编辑代理 `/galgame`（→ Wiki）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/galgame/mine` | 🔒 | `patchH.ListMyGalgames` | ⏳ | 我的投稿 |
| `GET /api/v1/galgame/search/publish` | 🔒 | `patchH.SearchGalgameForPublish` | ⏳ | 发布流程内搜索 |
| `GET /api/v1/galgame/messages/mine` | 🔒 | `patchH.GetMyWikiMessages` | ⏳ | 我的 Wiki 审核消息 |
| `GET /api/v1/galgame/messages/read-state` | 🔒 | `patchH.GetWikiMessagesReadState` | ⏳ | 消息已读态 |
| `GET /api/v1/galgame/:gid/revisions` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：修订列表 |
| `GET /api/v1/galgame/:gid/revisions/:rev` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：单条修订 |
| `GET /api/v1/galgame/:gid/revisions/:rev/diff` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：修订 diff |
| `GET /api/v1/galgame/:gid/prs` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：PR 列表 |
| `GET /api/v1/galgame/:gid/prs/:prid` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：单个 PR |
| `GET /api/v1/galgame/:gid/links` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：关联链接 |
| `GET /api/v1/galgame/:gid/aliases` | 🔐 | `patchH.WikiEditProxy` | ⏳ | 代理：别名 |

## 4. 分类代理 `/tag /official /engine /series`（→ Wiki）

### 4.1 基础读（12）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag` | 🌐 | `patchH.WikiEditProxy` | ⏳ | 标签列表 |
| `GET /api/v1/tag/search` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/tag/multi` | 🌐 | `patchH.WikiEditProxy` | ⏳ | 批量 |
| `GET /api/v1/tag/:name` | 🌐 | `patchH.WikiTaxonomyDetailProxy` | ⏳ | 详情；重写 galgame 列表为 moyu 的 GalgameCard 形态 |
| `GET /api/v1/official` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/official/search` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/official/:name` | 🌐 | `patchH.WikiTaxonomyDetailProxy` | ⏳ | 详情；同 `/tag/:name` 重写 |
| `GET /api/v1/engine` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/engine/:name` | 🌐 | `patchH.WikiEditProxy` | ⏳ | 通用透传（无 GalgameCard 重写）|
| `GET /api/v1/series` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/series/search` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/series/:id` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |

### 4.2 修订历史读（8 = 4 实体 × 2）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/tag/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/tag/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/official/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/official/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/engine/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/engine/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/series/:id/revisions` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |
| `GET /api/v1/series/:id/revisions/:rev` | 🌐 | `patchH.WikiEditProxy` | ⏳ | |

## 5. 用户 `/user`

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/user/search` | 🔒 | `userH.SearchUsers` | ⏳ | 用户名搜索 |
| `GET /api/v1/user/moemoepoint/log` | 🔒 | `userH.GetMoemoepointLog` | 🆕 | 自助：查自己萌萌点流水（代理 OAuth 精简视图，id 取 session 非路径参）|
| `GET /api/v1/user/:id` | 🔐 | `userH.GetUserInfo` | ⏳ | 用户主页资料 |
| `GET /api/v1/user/:id/floating` | 🌐 | `userH.GetUserFloating` | ⏳ | 悬浮卡 |
| `GET /api/v1/user/:id/patch` | 🌐 | `userH.GetUserPatches` | ⏳ | |
| `GET /api/v1/user/:id/resource` | 🌐 | `userH.GetUserResources` | ⏳ | |
| `GET /api/v1/user/:id/favorite` | 🌐 | `userH.GetUserFavorites` | ⏳ | |
| `GET /api/v1/user/:id/comment` | 🌐 | `userH.GetUserComments` | ⏳ | |
| `GET /api/v1/user/:id/contribute` | 🌐 | `userH.GetUserContributions` | ⏳ | |
| `GET /api/v1/user/:id/follower` | 🔐 | `userH.GetFollowers` | ⏳ | viewer 的 is_followed 态 |
| `GET /api/v1/user/:id/following` | 🔐 | `userH.GetFollowing` | ⏳ | 同上 |

## 6. 消息 `/message`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/message/` | 🔒 | `messageH.GetMessages` | ⏳ | 按类型分页 |
| `GET /api/v1/message/all` | 🔒 | `messageH.GetAllMessages` | ⏳ | |
| `GET /api/v1/message/unread` | 🔒 | `messageH.GetUnreadTypes` | ⏳ | 未读类型（铃铛红点）|

## 7. 管理 `/admin`（组级 `auth` + `RequireRole("admin","moderator")`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/admin/comment` | 🛡️ | `adminH.GetComments` | ⏳ | |
| `GET /api/v1/admin/resource` | 🛡️ | `adminH.GetResources` | ⏳ | |
| `GET /api/v1/admin/resource/:id/history` | 🛡️ | `adminH.GetResourceFileHistory` | ⏳ | 文件替换审计轨迹 |
| `GET /api/v1/admin/user/:id/purge-preview` | ⚙️ | `adminH.GetUserPurgePreview` | ⏳ | 清除用户预览（dry-run）；账户级 → 仅 admin |
| `GET /api/v1/admin/setting/comment-verify` | 🛡️ | `adminH.GetCommentVerify` | ⏳ | 读“评论需审核”开关 |
| `GET /api/v1/admin/setting/creator-only` | 🛡️ | `adminH.GetCreatorOnly` | ⏳ | 读“仅创作者可发布”开关 |
| `GET /api/v1/admin/stats` | 🛡️ | `adminH.GetStats` | ⏳ | 时序统计 |
| `GET /api/v1/admin/stats/sum` | 🛡️ | `adminH.GetStatsSum` | ⏳ | 汇总 |
| `GET /api/v1/admin/log` | 🛡️ | `adminH.GetLogs` | ⏳ | 操作日志 |
| `GET /api/v1/admin/galgame` | 🛡️ | `adminH.GetGalgame` | ⏳ | 全部补丁浏览（分页 + vndb_id 搜索）|
| `GET /api/v1/admin/patch/orphans` | 🛡️ | `adminH.GetOrphanPatches` | ⏳ | 孤儿补丁（vndb_id 非 `^v\d+`）|

## 8. 公共（无前缀组）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/home` | 🌐 | `commonH.GetHome` | ⏳ | 首页聚合 |
| `GET /api/v1/home/random` | 🌐 | `patchH.GetRandomPatch` | ⏳ | 随机补丁 |
| `GET /api/v1/galgame` | 🌐 | `commonH.GetGalgameList` | ⏳ | galgame 列表（筛选 / 排序）|
| `GET /api/v1/comment` | 🌐 | `commonH.GetGlobalComments` | ⏳ | 全站评论流 |
| `GET /api/v1/resource` | 🌐 | `commonH.GetGlobalResources` | ⏳ | 全站资源流 |
| `GET /api/v1/resource/:id` | 🔐 | `commonH.GetResourceDetail` | ⏳ | 资源详情；viewer like 态 |
| `GET /api/v1/ranking/user` | 🌐 | `commonH.GetUserRanking` | ⏳ | 用户榜（萌萌点 / 补丁数 / 评论数）|
| `GET /api/v1/ranking/patch` | 🌐 | `commonH.GetPatchRanking` | ⏳ | 补丁榜 |

## 9. 聊天 `/chat`（组级 `auth`）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/chat/room` | 🔒 | `chatH.ListRooms` | ⏳ | 房间列表 |
| `GET /api/v1/chat/room/:link` | 🔒 | `chatH.GetRoomDetail` | ⏳ | 房间详情 |
| `GET /api/v1/chat/room/:link/message` | 🔒 | `chatH.ListMessages` | ⏳ | 消息分页 |

## 10. 外部 API

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/hikari` | 🌐 | `commonH.GetHikari` | ⏳ | Hikari 外部接口 |
| `GET /api/v1/moyu/patch/has-patch` | 🌐 | `commonH.GetMoyuHasPatch` | ⏳ | 老 moyu 兼容查询（某作是否有补丁）|

## 11. 关于 / 文档（静态 .mdx）

| 路径 | 鉴权 | Handler | 状态 | 备注 |
|---|---|---|---|---|
| `GET /api/v1/about/posts` | 🌐 | `aboutH.ListPosts` | ⏳ | .mdx 列表 |
| `GET /api/v1/about/post` | 🌐 | `aboutH.GetPost` | ⏳ | 单篇 |
