> [📖 文档索引](./README.md) · 上一节：[01 — Galgame 核心 CRUD](./01-galgame.md) · 下一节：[03 — 链接 / 别名 / 贡献者](./03-relations.md)

## 版本历史 (Wiki)

每次编辑（创建、更新、PR 合并、回滚）都会创建一个 revision，存储 galgame 的完整状态快照。

### GET /galgame/:gid/revisions

版本列表。

**查询参数**：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| limit | int | 20 | 每页数量 |
| include_minor | bool | false | 是否包含小修改 |

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 3,
        "galgame_id": 1,
        "revision": 3,
        "user_id": 2,
        "action": "merged",
        "note": "更新简介",
        "is_minor": false,
        "reverted_to": null,
        "created": "2026-01-03T00:00:00Z"
      },
      {
        "id": 2,
        "revision": 2,
        "action": "updated",
        "note": "",
        "is_minor": true
      },
      {
        "id": 1,
        "revision": 1,
        "action": "created",
        "note": ""
      }
    ],
    "total": 3
  }
}
```

`action` 取值：`created`, `updated`, `merged`, `reverted`, `declined`

---

### GET /galgame/:gid/revisions/:rev

查看特定版本的完整快照。

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "id": 1,
    "galgame_id": 1,
    "revision": 1,
    "user_id": 1,
    "action": "created",
    "snapshot": {
      "vndb_id": "v12345",
      "name_zh_cn": "标题",
      "aliases": ["别名1"],
      "tag_ids": [1, 2],
      "official_ids": [1],
      "engine_ids": [],
      "links": [{"name": "VNDB", "link": "https://vndb.org/v12345"}]
    },
    "created": "..."
  }
}
```

---

### GET /galgame/:gid/revisions/:rev/diff

计算该版本与前一版本的差异（实时计算，不存储）。

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "changed_keys": {
      "name_zh_cn": true,
      "tag_ids": true
    },
    "old": {
      "name_zh_cn": "旧标题",
      "tag_ids": [1, 2]
    },
    "new": {
      "name_zh_cn": "新标题",
      "tag_ids": [1, 2, 3]
    }
  }
}
```

`old` 和 `new` 是完整的 snapshot 对象，前端可以只展示 `changed_keys` 中标记的字段。对于大文本字段（intro_*），前端可以用 diff 库展示行级差异。

---

### POST /galgame/:gid/revert

回滚到指定版本。**需要认证**。仅创建者或 admin 可操作。

回滚会创建一个新 revision（action=reverted），不会删除历史记录。

**请求体**：

```json
{
  "revision": 1
}
```

---

## PR (编辑请求)

非创建者/非 admin 通过 PR 提交编辑。PR 支持字段级自动 rebase。

### GET /galgame/:gid/prs

PR 列表。

**查询参数**：

| 参数 | 类型 | 默认值 |
|------|------|--------|
| page | int | 1 |
| limit | int | 20 |

---

### GET /galgame/:gid/prs/:id

PR 详情，包含与 base revision 的差异。

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "pr": {
      "id": 1,
      "galgame_id": 1,
      "user_id": 2,
      "status": 0,
      "note": "修改标题",
      "base_revision": 1,
      "snapshot": { ... },
      "completed_by": null,
      "revision_id": null,
      "created": "..."
    },
    "changed_keys": {
      "name_zh_cn": true
    }
  }
}
```

`status`：`0` = pending, `1` = merged, `2` = declined

---

### POST /galgame/:gid/prs

提交 PR。**需要认证**。

提交时只需提供要修改的字段，未提供的字段保持当前值。

**支持两种 Content-Type**：
- `application/json` — 普通 PR
- `multipart/form-data` — PR 提案里直接附 banner 文件，reviewer 看 diff 时可直接看到新图缩略图。详见
  [Banner 上传](#banner-上传通过-create--update--pr-端点的-multipart-模式)

**请求体**（JSON 模式）：

```json
{
  "name_zh_cn": "新标题",
  "tag_ids": [1, 2, 3],
  "note": "修改标题和标签"
}
```

支持的字段与创建/更新 galgame 相同，另外支持：

| 字段 | 类型 | 说明 |
|------|------|------|
| aliases | string[] | 别名数组（替换全部） |
| tag_ids | int[] | 标签 ID 数组（替换全部） |
| official_ids | int[] | 开发商 ID 数组（替换全部） |
| engine_ids | int[] | 引擎 ID 数组（替换全部） |
| links | object[] | 链接数组 `[{name, link}]`（替换全部） |
| note | string | PR 说明 |

---

### PUT /galgame/:gid/prs/:id/merge

合并 PR。**需要认证**。仅 galgame 创建者或 admin 可操作。

合并时如果 PR 的 base_revision 落后于最新版本，系统会自动检查字段冲突：
- **无冲突**：自动 rebase（PR 的改动应用到最新版本上）
- **有冲突**：返回错误，列出冲突字段名

**冲突响应示例**：

```json
{
  "code": 10,
  "message": "字段冲突: name_zh_cn 已被其他编辑修改，请基于最新版本重新提交"
}
```

---

### PUT /galgame/:gid/prs/:id/decline

拒绝 PR。**需要认证**。仅 galgame 创建者或 admin 可操作。

---


---

下一节：[03 — 链接 / 别名 / 贡献者](./03-relations.md)
