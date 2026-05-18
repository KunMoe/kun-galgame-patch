> [📖 文档索引](./README.md) · 上一节：[02 — 版本历史 + PR](./02-revisions-and-prs.md) · 下一节：[04 — 分类轴 (Tag/Official/Engine/Series)](./04-taxonomy.md)

> 🔴 **下游强制范围**：本节的链接 / 别名 / 贡献者增删操作，**kungal 与 moyu 各自必须完整实现一份**（后端透传 Bearer 代理 + 前端 UI，功能与 wiki 对齐）。详见 [00-handbook §15](./00-handbook-for-downstream.md#15-kungal--moyu-必须各自完整实现的-galgame-编辑面强制全覆盖)。

## 链接

### GET /galgame/:gid/links

链接列表。返回**纯数组**（不分页，非 `{items, total}` 形态）：

```json
{
  "code": 0,
  "data": [
    { "id": 1, "galgame_id": 1, "name": "官网", "link": "https://...", "created": "...", "updated": "..." }
  ]
}
```

### POST /galgame/:gid/links

添加链接。**需要认证**。自动创建 revision。

```json
{
  "name": "官网",
  "link": "https://example.com"
}
```

### DELETE /galgame/:gid/links

删除链接。**需要认证**。自动创建 revision。

```json
{
  "id": 1
}
```

---

## 别名

### GET /galgame/:gid/aliases

别名列表。返回**纯数组**（不分页）：

```json
{
  "code": 0,
  "data": [
    { "id": 1, "galgame_id": 1, "name": "Fate/EXTRA", "created": "...", "updated": "..." }
  ]
}
```

### POST /galgame/:gid/aliases

添加别名。**需要认证**。自动创建 revision。

```json
{
  "name": "新别名"
}
```

### DELETE /galgame/:gid/aliases

删除别名。**需要认证**。自动创建 revision。

```json
{
  "id": 1
}
```

---

## 贡献者

### GET /galgame/:gid/contributors

贡献者列表（含用户信息）。

**成功响应**：

```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "galgame_id": 1,
      "user_id": 1,
      "created": "...",
      "user": {
        "id": 1,
        "name": "KUN",
        "avatar": "https://..."
      }
    }
  ]
}
```

### DELETE /galgame/:gid/contributors/:uid

删除贡献者。**需要认证**。仅 galgame 创建者或 admin 可操作。

---


---

下一节：[04 — 分类轴](./04-taxonomy.md)
