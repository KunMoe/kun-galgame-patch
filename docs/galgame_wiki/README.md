# Galgame Wiki API 参考

基础路径：`/api`

| 环境 | Base URL |
|------|----------|
| 开发 | `http://127.0.0.1:9280/api` |
| 生产 | `https://galgame.kungal.com/api` |

## 文档索引

| # | 文件 | 内容 |
|---|------|------|
| 00 | [handbook-for-downstream.md](./00-handbook-for-downstream.md) | **kungal / moyu 接入手册** — 端到端流程 + 决策回顾 + 工作量估算 + checklist |
| 01 | [galgame.md](./01-galgame.md) | Galgame 核心 CRUD（含 banner 文件上传） |
| 02 | [revisions-and-prs.md](./02-revisions-and-prs.md) | 版本历史 + PR 编辑请求流程 |
| 03 | [relations.md](./03-relations.md) | 链接 / 别名 / 贡献者子资源 |
| 04 | [taxonomy.md](./04-taxonomy.md) | Tag / Official / Engine / Series 分类轴 CRUD |
| 05 | [search.md](./05-search.md) | Meilisearch 驱动的搜索接口 |
| 06 | [admin.md](./06-admin.md) | 管理统计与状态变更 |
| 07 | [submission.md](./07-submission.md) | 用户投稿与审核流程（submit / claim / patch-draft） |
| 08 | [messages.md](./08-messages.md) | 消息系统（投稿事件流，wiki 单一来源） |
| 99 | [appendix.md](./99-appendix.md) | 错误码、端点总览、Meilisearch 运维 |

---

## 响应格式

```json
{
  "code": 0,
  "message": "成功",
  "data": { ... }
}
```

分页响应的 `data` 结构：

```json
{
  "items": [...],
  "total": 42
}
```

## 认证

- **读操作（GET）**：无需认证
- **写操作（POST/PUT/DELETE）**：需要 OAuth Bearer Token

```
Authorization: Bearer <access_token>
```

access_token 由 KUN OAuth 系统签发，JWT claims 中包含 `uid`（integer user ID）和 `roles`。

---

下一节：[01 — Galgame 核心 CRUD](./01-galgame.md)
