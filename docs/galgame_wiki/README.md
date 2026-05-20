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

> 🔴 **强制范围变更**：galgame 的编辑面（PR、修订历史、关系、分类轴增删改）**不再是 wiki-only**——kungal 与 moyu **各自必须完整实现一份**（后端代理 + 前端 UI，与 wiki 对齐）。权威清单见 [00-handbook §15](./00-handbook-for-downstream.md#15-kungal--moyu-必须各自完整实现的-galgame-编辑面强制全覆盖)。

> 📦 **2026-Q2 升级摘要（PR1–PR5，已上线）**：
> - **PR1**：`released` 字符串拆为 `release_date` (`YYYY-MM-DD` 或 `""`) + `release_date_tba` (bool)。两者均参与 revision/PR diff。详见 [01-galgame.md PUT 端点](./01-galgame.md#put-galgamegid)。
> - **PR2 / PR5**：新增 `galgame_cover` / `galgame_screenshot` 关联表（hash 化的 image_service 资源）；`banner_image_hash` **字段已退役**，banner 由 `covers[sort_order=0]` 唯一表达；响应里有派生只读字段 `effective_banner_hash`。详见 [03-relations.md 封面/截图段](./03-relations.md#封面--截图pr2-新增) 与 [00-handbook §15 PR5 BREAKING](./00-handbook-for-downstream.md#15-kungal--moyu-必须各自完整实现的-galgame-编辑面强制全覆盖)。
> - **PR4**：tag / official / engine / series 4 种 taxonomy 实体获得**完整版本历史 + 回滚**，端点形态与 galgame 修订完全对齐（`GET /tag/:id/revisions`, `POST /tag/:id/revert` 等共 12 个新端点）。详见 [04-taxonomy.md §修订与回滚](./04-taxonomy.md#修订与回滚-pr4-新增4-实体同款)。

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
