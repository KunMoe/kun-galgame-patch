# moyu 评论区迁移到 NextMoe 社区原语

> 状态：**moyu 侧已完成**。上线被 infra 侧的导入器挡着（已写好、端到端测过、等发话跑）。
> infra 侧的契约、导入器规格和上线顺序在 `nextmoe-infra` 的
> `docs/community/{01-service-and-contract,02-moyu-cutover}.md`。本文只记 moyu 这
> 一侧的决定和缺口，不复述那两份。

## 0. 为什么

moyu 是 NextMoe 的直接下游，评论却一直留在本库的 `patch_comment` 里 —— kungal
论坛（2026-07）、letmoe、表情包三家早就迁到了社区原语（`cmd/community`，库
`kun_community`），只有这里没吃自己的狗粮。本波把两面评论墙整体迁过去。

## 1. 租户与锚点

社区原语的租户**不在线上传**，由调用方 OAuth client 的绑定推导。2026-09-16 之前
它读的是 `oauth_clients.catalog_site`，而 moyu 和论坛都绑 `kungal`（moyu 的这个
绑定是 catalog 认领链路要用的，不能改），于是两家共用一个租户，只能靠锚点前缀
（`moyu:` / `moyu-resource:`）隔开 —— 实测 moyu 有评论的 2,040 个 `galgame_id`
里 1,992 个已经是 kungal 的 `anchor_kind=1` 锚点、490 个已经带帖。

infra 把租户拆到了 `oauth_clients.community_site`（空则回落 `catalog_site`），
moyu 自成一个租户 `moyu`。**前缀因此撤掉，锚点就是本站的裸 id**：

| 墙 | anchor_kind | anchor_id | 页面 |
|---|---|---|---|
| 游戏评论区 | 1 (site_game) | `<patch.id>` | `/galgame/<id>?tab=comment` |
| 资源评论区 | 2 (site_resource) | `<resource.id>` | `/resource/<rid>` |

游戏墙的锚点是 patch id，也就是 catalog work id（铁律 3）—— infra 的
`cmd/retire-merged-comments` 正是靠这一点把 moyu 列进了「site_game 锚就是
catalog id」的站点表，所以合并遗留的墙它扫得动。

`internal/community/anchor` 仍是**唯一**决定一条评论是不是 moyu 的地方，认不出的
锚点一律丢弃而不是硬链：`GET /posts` 和 `GET /search/posts` 的租户闸是「本站
**加上**所有 catalog 锚的串」，catalog 锚按设计是全网共用的一场对话。

## 2. moyu 侧改了什么

- `pkg/communityclient`：S2S 客户端（Basic auth，房子信封）。
- `internal/community/anchor`：锚点的铸造与反解。
- `internal/community/engagement` + handler：已读回执 / 订阅等级 / 未读列表 / 红点。
- `internal/comment/{model,repository,service,handler}`：两面墙的读写、站内流、
  搜索、维护。本地只留 `patch_comment_community_map`（旧深链），**没有点赞镜像**：
  读面直接带 `reaction_count` / `viewer_reacted`。
- 迁移 `040_community_comments`（只建那张映射表）。
- 前端：`useCommentList` 改成游标 + 客户端组树；新增 `useCommentFeed`、
  `CommentSubscribe`、`CommentFlagModal`、`SearchComments`、`/message/comment`。

**新增的路由**

| 路由 | 用途 |
|---|---|
| `POST /patch/comment/:postId/flag` | 举报（走原语的加权举报 + 审核队列） |
| `GET /patch/comment/locate?legacy_id=` | 旧 id → post id（走映射表） |
| `GET /search/comment` | 站内评论搜索 |
| `GET /admin/comment?q=` / `GET /admin/comment/recent` | 管理队列 |
| `GET /community/unread` `/unread/count` | 未读评论区 + 红点 |
| `POST /community/thread/:id/read` `/notification` | 已读回执 / 订阅 |

**退役的路由**：`PUT /admin/comment/:id/approve`、
`GET|PUT /admin/setting/comment-verify`。

## 3. 明确的取舍与缺口

- **预审（comment-verify）退役**。原语没有 pre-moderation 开关。生产
  `site_setting` 是空表，这个开关从没打开过，7,060 条评论 status 全是 0 —— 实际
  没有损失。新人的前两帖改由原语的 TL0 沙箱持留，在它自己的审核队列里放行。
- **举报改走原语**。`enforce.Registry` 里的 `patch_comment` subject 已删。切换前
  的历史举报仍带这个 kind，所以前端标签保留为「补丁评论（已迁出）」。
- **`patch.comment_count` 只能由写路径维护**。SQL 再也算不出来了，`merge.Recount`
  和 admin 的重算 SQL 都把这一列摘了。上游审核队列 reject 一条评论会让它偏，和
  收藏数已经接受的偏差同一类。
- **墙上的计数包含已删除的帖子**。墙的 `total` 是上游线程的 `posts_count`，删帖只
  留占位、不减计数，占位行也照样显示，所以前端删帖后不再自减 —— 否则删完显示 N-1、
  刷新又变回 N。游戏页简介里的评论数是 `patch.comment_count`，统计游戏墙**加**该游戏
  所有资源墙，和游戏墙的 `total` 本来就不相等。
- **版主可以编辑他人的评论**（切换前只有作者能编辑）。原语带 `as_moderator` 记下
  这次编辑，帖子标「已编辑（管理）」，moyu 写一条 `updateComment` 管理日志；作者
  会收到一条链到该帖的系统通知；版主可选填原因，写进通知和管理日志，和版主删帖一致。
- **合并会遗留一面墙**。`merge.Fold` 不再搬评论，只记日志；infra 的
  `cmd/retire-merged-comments` 扫它（会把串**退役**，不是搬到幸存页）。
- **清除用户会先清上游**。`PurgeUser` 先调 `POST /authors/{id}/purge`，失败就整体
  中止；之后按锚点把 `patch.comment_count` 减回去。
- **管理面板的评论总数 / 新增评论数下线**，清除预览里的「评论点赞数」也下线 ——
  原语没有站点级总数面，也没有「某人点过多少赞」的面（清除后的回执里有数）。
- **评论里的图片靠新的扫描保活**。ref-ping 原来扫 `patch_comment.content`，现在
  正文在上游，改成翻 `GET /posts` 全量收集 `/image/<hash>`，只取本站锚点。
  **这一步失败必须让整次 ref-ping 失败** —— 扫一半 = 另一半图片不再被引用。
- **编辑面依赖 infra cce5b4a8**。`PATCH /posts/{id}` 原来不 hydrate 反应、恒答
  `reaction_count` 0，moyu 曾用编辑前 resolve 的计数回填；infra #215 修好后绕行已
  删，所以 community 必须先部署到这个提交，否则编辑完赞数显示 0（刷新即恢复）。
- **红点可能比列表多**。未读的 total 和列表查同一批行，而列表会丢掉别站的
  catalog 锚串，红点照算。生产目前 0 条 catalog 锚串（2026-09-16），暂不可达；
  真出现时要么 infra 给未读面加锚点过滤，要么 moyu 能把这类串链到别处。
