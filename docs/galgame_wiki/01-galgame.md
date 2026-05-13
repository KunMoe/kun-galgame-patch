> [📖 文档索引](./README.md) · 下一节：[02 — 版本历史 + PR](./02-revisions-and-prs.md)

## Galgame 核心 CRUD

### GET /galgame

列表（分页 + 搜索 + 排序）。

**查询参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| page | int | 否 | 1 | 页码 |
| limit | int | 否 | 24 | 每页数量 (1-50) |
| sort_field | string | 否 | created | 排序字段: `created`, `updated`, `view`, `resource_update_time` |
| sort_order | string | 否 | desc | 排序方向: `asc`, `desc` |
| search | string | 否 | | 搜索关键词（匹配四语言名称） |

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "items": [
      {
        "id": 1,
        "vndb_id": "v12345",
        "name_en_us": "Title",
        "name_ja_jp": "タイトル",
        "name_zh_cn": "标题",
        "name_zh_tw": "標題",
        "banner": "https://...",
        "banner_image_hash": "abcd1234...ef",
        "content_limit": "sfw",
        "view": 100,
        "created": "2026-01-01T00:00:00Z",
        "tag": [...],
        "official": [...]
      }
    ],
    "total": 42
  }
}
```

---

### GET /galgame/batch

批量获取 galgame 轻量信息（跨服务展示用，不加载关联数据）。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ids | int[] | 是 | galgame ID 数组，最多 100 个 |

**请求示例**：`GET /galgame/batch?ids=1,2,3`

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": [
    {
      "id": 1,
      "vndb_id": "v12345",
      "name_en_us": "Title",
      "name_ja_jp": "タイトル",
      "name_zh_cn": "标题",
      "name_zh_tw": "標題",
      "banner": "https://image.kungal.com/...",
      "banner_image_hash": "abcd...ef",
      "content_limit": "sfw",
      "status": 0,
      "user_id": 1,
      "resource_update_time": "2026-01-01T00:00:00Z",
      "original_language": "ja-jp",
      "age_limit": "all"
    }
  ]
}
```

| 字段 | 备注 |
|---|---|
| `status` | `0` 已发布；`3` / `4` 仅当请求带 Bearer 且调用者是该条 galgame 的 submitter 时才会返回（pending / declined 草稿）。匿名 / 非 owner 调用看不到非 0 条目 |
| `banner_image_hash` | image_service 哈希；前端 resolveBannerUrl 优先用此字段拼新 CDN URL，缺失时 fallback 到 `banner` 老 URL |

不存在或对调用者不可见的 ID 会被过滤，不会报错。返回数组长度可能小于请求的 ID 数量。

**带 Bearer 的语义**：与 OAuth 终端用户 access_token 一起调用，wiki 解 JWT 得 `uid`，返回结果中额外包含调用者的 status=3/4 条目（参见 [07-submission.md GET /galgame/batch 增量行为](./07-submission.md#get-galgamebatch-增量行为)）。无 Bearer 时只返 status=0。

---

### GET /galgame/user/:uid/stats

获取用户的 Galgame 统计数据。

**路径参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| uid | int | 用户 ID |

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "galgame_created": 10,
    "galgame_created_today": 1,
    "galgame_contributed": 15,
    "revision_count": 42,
    "pr_submitted": 5,
    "pr_merged": 3,
    "pr_declined": 1,
    "pr_pending": 1
  }
}
```

| 字段 | 说明 |
|------|------|
| galgame_created | 用户创建的 galgame 总数（不含被封禁的） |
| galgame_created_today | 用户今日创建的 galgame 数量 |
| galgame_contributed | 用户参与贡献的 galgame 数量（含创建和编辑） |
| revision_count | 用户产生的版本记录总数 |
| pr_submitted | 用户提交的 PR 总数 |
| pr_merged | 已合并的 PR 数量 |
| pr_declined | 已拒绝的 PR 数量 |
| pr_pending | 待处理的 PR 数量 |

用户不存在时返回全零数据，不报错。

---

### GET /galgame/check

检查 VNDB ID 是否已存在 + 返回对应整数 `galgame_id`。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| vndb_id | string | 是 | VNDB ID (如 `v12345`) |

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "exists": true,
    "galgame_id": 1
  }
}
```

#### Recipe — 按 VNDB ID 取完整 galgame 信息

本服务**不提供**直接 `GET /galgame/by-vndb/:vndb_id` 端点（设计上 galgame 主键是整数 `id`）。要按 VNDB ID 取完整信息，标准做法是两步：

```bash
# Step 1: 拿到整数 galgame_id
GET /galgame/check?vndb_id=v17
# → { "exists": true, "galgame_id": 8329 }

# Step 2: 取完整信息（含 alias / tag / official / series 等 relations）
GET /galgame/8329
# → { "galgame": { ... 全字段 + relations ... }, "users": { ... } }
```

伪代码：

```go
// Go
exists, gid := api.CheckVNDB(ctx, "v17")
if !exists { return nil }
galgame := api.GetGalgame(ctx, gid)
```

```ts
// TypeScript
const r1 = await api.get<{exists: boolean; galgame_id: number}>('/galgame/check?vndb_id=v17')
if (!r1.data.exists) return null
const r2 = await api.get<{galgame: Galgame}>(`/galgame/${r1.data.galgame_id}`)
return r2.data.galgame
```

> 单次请求的"按 vndb_id 搜索"也可以用 `GET /galgame/search?q=v17&limit=1`（vndb_id 在 Meilisearch searchableAttributes 里且禁用 typo），但**这是 search 语义**——依赖 Meilisearch 在线 + 索引同步状态，不如 `/check` 端点强一致。常规跨服务集成走上面的 2 步 recipe。

---

### GET /galgame/:gid

获取详情（含全部关联数据 + 用户信息）。

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "galgame": {
      "id": 1,
      "vndb_id": "v12345",
      "name_en_us": "...",
      "name_ja_jp": "...",
      "name_zh_cn": "...",
      "name_zh_tw": "...",
      "banner": "...",
      "intro_en_us": "...",
      "intro_ja_jp": "...",
      "intro_zh_cn": "...",
      "intro_zh_tw": "...",
      "content_limit": "sfw",
      "original_language": "ja-jp",
      "age_limit": "r18",
      "view": 100,
      "user_id": 1,
      "series_id": null,
      "alias": [{"id": 1, "name": "别名"}],
      "tag": [{"galgame_id": 1, "tag_id": 2, "spoiler_level": 0, "tag": {"id": 2, "name": "RPG"}}],
      "official": [{"galgame_id": 1, "official_id": 3, "official": {"id": 3, "name": "开发商"}}],
      "engine": [...],
      "link": [{"id": 1, "name": "VNDB", "link": "https://vndb.org/v12345"}],
      "contributor": [{"id": 1, "user_id": 1}],
      "created": "...",
      "updated": "..."
    },
    "users": {
      "1": {"id": 1, "name": "KUN", "avatar": "https://..."}
    }
  }
}
```

`users` 是一个 user_id → 用户信息的 map，包含 galgame 创建者和所有贡献者的信息。

---

### POST /galgame

创建 Galgame。**需要认证 + admin/moderator 角色**。

> ⚠️ **此端点已锁到 admin/moderator**。它是 admin 直接发布的旁路：创建 `status=0` 条目，跳过审核队列。
>
> **普通用户必须走 [POST /galgame/submit](./07-submission.md#post-galgamesubmit)**——创建 `status=3` 待审稿，无需 VNDB ID，进入审核队列。
>
> 非 admin/moderator 调此端点返回 403。kungal/moyu 的发布 UI 应仅暴露 `/submit`。

创建时自动：创建 revision 1、添加创建者为贡献者、添加 VNDB 链接。

**支持两种 Content-Type**：
- `application/json` — 不上传 banner 文件时使用（请求体见下）
- `multipart/form-data` — 创建时直接带 banner 文件，详见
  [Banner 上传](#banner-上传通过-create--update--pr-端点的-multipart-模式)

**请求体**（JSON 模式）：

```json
{
  "vndb_id": "v12345",
  "name_en_us": "Title",
  "name_ja_jp": "タイトル",
  "name_zh_cn": "标题",
  "name_zh_tw": "標題",
  "banner": "https://...",
  "banner_image_hash": "abcd1234...ef",
  "intro_en_us": "...",
  "intro_ja_jp": "...",
  "intro_zh_cn": "...",
  "intro_zh_tw": "...",
  "content_limit": "sfw",
  "original_language": "ja-jp",
  "age_limit": "r18",
  "series_id": null,
  "aliases": "别名1,别名2",
  "tag_ids": [1, 2, 3],
  "official_ids": [1],
  "engine_ids": [1]
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| vndb_id | 是 | 格式 `v\d+`，必须唯一 |
| banner | 否 | 老的 URL 字符串字段；image_service 接入前的旧路径，迁移期保留作 fallback |
| banner_image_hash | 否 | image_service 内容哈希；通常通过 multipart 模式由后端自动写入，也可由调用方手动指定 |
| aliases | 否 | 逗号分隔的别名字符串 |
| tag_ids | 否 | 标签 ID 数组 |
| official_ids | 否 | 开发商 ID 数组 |
| engine_ids | 否 | 引擎 ID 数组 |
| content_limit | 否 | `sfw` (默认) 或 `nsfw` |
| age_limit | 否 | `r18` (默认) 或 `all` |

> **banner 字段优先级**：前端读取时优先 `banner_image_hash`（拼 image_service URL），缺失时回退 `banner` 老 URL。两个字段都可写，`banner_image_hash` 推荐用于新上传。

---

### PUT /galgame/:gid

更新 Galgame。**需要认证**。仅创建者或 admin 可操作。

每次更新自动创建新 revision。**所有字段（含 `banner_image_hash`）的变化都会进入 revision 快照与 PR diff**。

**支持两种 Content-Type**：
- `application/json` — 不修改 banner 或修改时只改 hash 字段
- `multipart/form-data` — 同时上传新 banner 文件，详见
  [Banner 上传](#banner-上传通过-create--update--pr-端点的-multipart-模式)

**请求体**（JSON 模式，所有字段可选）：

```json
{
  "name_zh_cn": "新标题",
  "banner_image_hash": "abcd1234...ef",
  "intro_zh_cn": "新简介",
  "is_minor": false
}
```

`is_minor` 为 `true` 时标记为小修改，在版本历史中可被过滤。

---

### Banner 上传：通过 Create / Update / PR 端点的 multipart 模式

**没有独立的"上传 banner"端点**。banner 文件作为可选 `file` 表单字段一并随
`POST /galgame`、`PUT /galgame/:gid`、`POST /galgame/:gid/prs` 的 multipart 请求提交，
后端会先把文件转给 image_service 拿到 hash，再把 hash 当作 `banner_image_hash` 字段，
跟其他字段一起进入同一次 revision / PR diff。

> 设计动机：图片上传与 article 编辑在业务上是同一次动作，应当原子。
> 不再有"上传成功但忘了点保存留下 orphan 文件"的情况——文件在浏览器内存里
> 暂存，没点保存就丢弃，从源头避免 orphan。

**两种 Content-Type 等价**，前端按需选用：

#### A. application/json — 不上传文件时使用（与以前完全相同）

```http
PUT /api/v1/galgame/:gid
Content-Type: application/json

{ ... fields including optional banner_image_hash ... }
```

#### B. multipart/form-data — 需要上传 banner 文件时使用

```http
PUT /api/v1/galgame/:gid
Content-Type: multipart/form-data; boundary=...

--boundary
Content-Disposition: form-data; name="data"

{"name_zh_cn": "新标题", ...other fields}
--boundary
Content-Disposition: form-data; name="file"; filename="banner.png"
Content-Type: image/png

<binary>
--boundary--
```

| 字段 | 必填 | 说明 |
|------|------|------|
| data | 是 | JSON 字符串，等同于 JSON 模式下的 body |
| file | 否 | 图片文件（image/jpeg / png / webp）；上传后后端把 hash 设为 `banner_image_hash` |

**错误码**（multipart 模式下额外可能出现的）：透传 image_service 的状态码与
错误码（如 `80008` 配额超限、`80015` 上传暂未开放、`60002` 审核拒绝），调用方
按需展示给用户。

**该 multipart 模式同样适用于：**
- `POST /galgame`（创建时直接带 banner 文件，避免"先创建再编辑改 banner"两步）
- `POST /galgame/:gid/prs`（PR 提案里直接附 banner 文件，reviewer 看 diff 时能看到新图缩略图）

---


---

下一节：[02 — 版本历史 + PR](./02-revisions-and-prs.md)
