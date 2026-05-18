> [📖 文档索引](./README.md) · 上一节：[03 — 链接 / 别名 / 贡献者](./03-relations.md) · 下一节：[05 — 搜索](./05-search.md)

> 🔴 **下游强制范围**：本节 tag / official / engine / series 的增删改查（含本次新增的 `POST /tag` `POST /official` `POST /engine` 与各 `DELETE`），**kungal 与 moyu 各自必须完整实现一份**（后端透传 Bearer 代理 + 前端「选已有/没有就新建」UI，功能与 wiki 对齐，不得只做子集）。详见 [00-handbook §15](./00-handbook-for-downstream.md#15-kungal--moyu-必须各自完整实现的-galgame-编辑面强制全覆盖)。

## 标签 (Tag)

### GET /tag

标签列表（分页，按关联 galgame 数量排序）。

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| page | int | 1 | 页码 |
| limit | int | 50 | 每页数量（max 100） |

### GET /tag/search

搜索标签。**由 Meilisearch 驱动**，替代原 DB LIKE 实现。详见 [搜索 (Search)](#搜索-search) 章节。

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| q | string | `""` | 搜索词；空时按 `galgame_count` 倒序返回热门 tag |
| category | string | — | `content` / `sexual` / `technical` |
| limit | int | 50 | max 100 |

**响应**：
```json
{
  "items": [
    { "id": 45, "name": "校园", "aliases": ["学园"], "category": "content", "galgame_count": 850 }
  ],
  "total": 1,
  "processing_time_ms": 4
}
```

### GET /tag/multi?tag_ids=1,2,3

多标签筛选，返回同时拥有所有指定标签的 galgame。

**查询参数**：`page`, `limit`, `tag_ids`（数组）

### GET /tag/:name

标签详情 + 关联的 galgame 列表。

> ⚠️ **`:name` 路径段仅用于 URL 美观 / 分享**（如 `/tag/校园?tag_id=42`），实际的查询条件是 `tag_id` query 参数。后端不读 `:name`，传任意字符串都会按 `tag_id` 查找。这与 Wikipedia 的 `/wiki/Article_Name?oldid=N` 设计一致。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| tag_id | int | 是 | tag 主键，**实际查询字段** |
| page | int | 否 | 页码 |
| limit | int | 否 | 每页数量 |
| sort_field | string | 否 | `created` / `resource_update_time` / `view` |
| sort_order | string | 否 | `asc` / `desc` |
| content_limit | string | 否 | `sfw` / `nsfw` —— 仅返回对应分级 galgame，`total` 同步反映过滤后数量 |

### PUT /tag

更新标签。**需要认证（admin/moderator）**。

```json
{
  "tag_id": 1,
  "name": "新名称",
  "category": "content",
  "description": "描述",
  "alias": ["别名1", "别名2"]
}
```

事务内替换全部别名。

### POST /tag

新建标签。**需要认证（任意登录用户）** —— 这是为了让 kungal/moyu 用户给「VNDB 没有的原创 / 同人作品」补一个 wiki 里尚不存在的 tag（与 `POST /series` 同权限模型）。

```json
{
  "name": "标签名",
  "category": "content",
  "description": "可选描述",
  "alias": ["别名1", "别名2"]
}
```

- `category` 必填，取值 `content` / `sexual` / `technical`。
- 同名已存在 → `400`，`message` 提示「已存在同名 tag」（name 全局唯一）。前端遇此应改用 `GET /tag/search` 选用既有 tag。
- 成功返回新建的 tag 实体（含 `id`），随后写入 Meilisearch。

### DELETE /tag/:id

删除标签。**需要 role > 1（admin/moderator；普通用户 role=1 一律 403）**。

安全默认 + 强制清除两段式（同 `DELETE /admin/image/:hash?force=true` 约定）：

- **默认（不带 `?force=true`）**：若该 tag 仍被任意 galgame 引用 → **拒绝**，返回
  `code:7`，message 含被引用的 galgame 数。避免一删就静默把它从 N 个作品上摘掉。
- **`?force=true`**：一键「清除全部引用 → 硬删」。事务内级联删除该 tag 的别名
  （`galgame_tag_alias`）与全部 `galgame_tag_relation` 关联，再硬删 tag 行，并从
  Meilisearch 移除，无悬挂引用。
- 引用数为 0 时，带不带 `force` 都直接删（无引用可清）。

成功响应：

```json
{ "deleted": true, "forced": true, "purged_relations": 2, "purged_aliases": 0 }
```

`forced` 表示本次是「带 force 且确实清理了引用」。`purged_relations` / `purged_aliases`
为本次清掉的行数（审计用）。

---

## 开发商 (Official)

### GET /official

开发商列表。**查询参数**：`page`, `limit`

### GET /official/search

搜索会社。**由 Meilisearch 驱动**，详见 [搜索 (Search)](#搜索-search)。

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| q | string | `""` | 搜索词；空时按 `galgame_count` 倒序 |
| category | string | — | `company` / `individual` / `amateur` |
| lang | string | — | 按主语言过滤（`ja`, `en`, `zh-Hans` 等） |
| limit | int | 50 | max 100 |

### GET /official/:name

详情 + 关联 galgame。

> ⚠️ **`:name` 路径段仅用于 URL 美观**，实际查询字段是 `official_id` query 参数（同 [GET /tag/:name](#get-tagname) 的设计）。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| official_id | int | 是 | official 主键，**实际查询字段** |
| page | int | 否 | 页码 |
| limit | int | 否 | 每页数量 |
| sort_field | string | 否 | `created` / `resource_update_time` / `view` |
| sort_order | string | 否 | `asc` / `desc` |
| content_limit | string | 否 | `sfw` / `nsfw`，只返回对应分级 galgame，`total` 同步反映过滤后数量 |

### PUT /official

更新。**需要认证（admin/moderator）**。

```json
{
  "official_id": 1,
  "name": "新名称",
  "link": "https://...",
  "category": "company",
  "lang": "ja",
  "description": "描述",
  "alias": ["别名1"]
}
```

### POST /official

新建开发商 / 会社。**需要认证（任意登录用户）**，同 `POST /tag` 的用途与权限模型。

```json
{
  "name": "会社名",
  "category": "company",
  "original": "原文名（可选，日文等）",
  "link": "https://...（可选）",
  "lang": "ja（可选）",
  "description": "可选描述",
  "alias": ["别名1"]
}
```

- `category` 必填，取值 `company` / `individual` / `amateur`。
- 同名已存在 → `400`「已存在同名 official」（name 全局唯一）。
- 成功返回新建实体，写入 Meilisearch。

### DELETE /official/:id

删除开发商。**需要 role > 1（admin/moderator）**。与 [`DELETE /tag/:id`](#delete-tagid) 完全同款两段式：默认被引用则**拒绝**（返回引用数），`?force=true` 才一键清除别名（`galgame_official_alias`）+ `galgame_official_relation` 后硬删并移出 Meilisearch；成功返回 `{deleted,forced,purged_relations,purged_aliases}`。

---

## 引擎 (Engine)

### GET /engine

全量列表（数据量小，不分页）。

### GET /engine/:name

详情 + 关联 galgame。

> ⚠️ **`:name` 路径段仅用于 URL 美观**，实际查询字段是 `engine_id` query 参数（同 [GET /tag/:name](#get-tagname) 的设计）。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| engine_id | int | 是 | engine 主键，**实际查询字段** |
| page | int | 否 | 页码 |
| limit | int | 否 | 每页数量 |
| content_limit | string | 否 | `sfw` / `nsfw`，只返回对应分级 galgame，`total` 同步反映过滤后数量 |

### PUT /engine

更新。**需要认证（admin/moderator）**。

```json
{
  "engine_id": 1,
  "name": "新名称",
  "description": "描述",
  "alias": ["别名1"]
}
```

引擎的 `alias` 以 JSONB 数组存储（与 tag/official 的关联表不同）。

### POST /engine

新建引擎。**需要认证（任意登录用户）**，同 `POST /tag` 的用途与权限模型。

```json
{
  "name": "引擎名",
  "description": "可选描述",
  "alias": ["别名1"]
}
```

- 同名已存在 → `400`「已存在同名 engine」（name 全局唯一）。
- 引擎**不进 Meilisearch**（无 `/engine/search`），故无搜索写回。
- 成功返回新建实体。

### DELETE /engine/:id

删除引擎。**需要 role > 1（admin/moderator）**。同款两段式：默认被引用则**拒绝**（返回引用数），`?force=true` 才一键清除 `galgame_engine_relation` 后硬删（引擎别名为行内 JSONB，随行删除；引擎不进 Meili，无索引移除）；成功返回 `{deleted,forced,purged_relations}`（无 `purged_aliases`，引擎无别名表）。

---

## 系列 (Series)

### GET /series

系列列表（含前 5 个 galgame 预览）。**查询参数**：`page`, `limit`

### GET /series/search?keywords=xxx

搜索 galgame（按名称、VNDB ID、标签、别名），用于系列分配。

返回最多 20 条。

### GET /series/:id

系列详情 + 全部 galgame。

### POST /series

创建系列。**需要认证**。

```json
{
  "name": "系列名",
  "description": "描述",
  "galgame_ids": [1, 2, 3]
}
```

### POST /series/modal

按 ID 批量获取 galgame（模态框用）。**需要认证**。

```json
{
  "ids": [1, 2, 3]
}
```

返回结果按输入 ID 顺序排列。

### PUT /series/:id

更新系列。**需要认证**。

```json
{
  "name": "新名称",
  "galgame_ids": [1, 2, 4]
}
```

`galgame_ids` 会**替换**系列中的所有 galgame。

### DELETE /series/:id

删除系列。**需要认证（admin/moderator）**。关联的 galgame 的 `series_id` 会被置为 `null`。

---


---

下一节：[05 — 搜索](./05-search.md)
