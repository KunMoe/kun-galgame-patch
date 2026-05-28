# improvement-plan.md（已拆分）

> **此文件 2026-05-22 起不再维护**。原内容（v0.0.1 → v0.5.2 全部跨切面设计、组件 spec、版本 changelog、工程教训）已经全部拆分到结构化目录。

## 新位置

| 旧内容 | 新位置 |
|---|---|
| §1 跨切面问题（7 + 2 = 9 条原则） | [`architecture/design-principles.md`](./architecture/design-principles.md) |
| §2 组件清单严重度 | [`architecture/design-principles.md`](./architecture/design-principles.md)（合并） |
| §3 各组件具体发现 | 拆到 [`components/<name>.md`](./components/) 各组件文件 |
| §4 KunTab 重设计 | [`components/kun-tab.md`](./components/kun-tab.md) |
| §5 KunTagInput 终稿 spec | [`components/kun-tag-input.md`](./components/kun-tag-input.md) |
| §6 落地路线图 + §7 Open Q | [`changelog/v0.1.0.md`](./changelog/v0.1.0.md) |
| §8 v0.1.1 复审加固 | [`changelog/v0.1.1.md`](./changelog/v0.1.1.md) |
| §9 v0.2.0 浮层引擎统一 | [`changelog/v0.2.0.md`](./changelog/v0.2.0.md) + [`changelog/v0.2.1.md`](./changelog/v0.2.1.md) + [`changelog/v0.2.2.md`](./changelog/v0.2.2.md) |
| §10-§23（v0.3.1 → v0.5.2 每版） | [`changelog/v0.3.1.md`](./changelog/v0.3.1.md) ... [`changelog/v0.5.2.md`](./changelog/v0.5.2.md) |
| §18 反思修正（runWithContext + 孤儿 store） | [`lessons/runWithContext-precise-scope.md`](./lessons/runWithContext-precise-scope.md) + [`lessons/orphan-store-migration.md`](./lessons/orphan-store-migration.md) |
| 跨版本反思（render appContext / 4-axis audit / flex+min-* / grep build artifact / 证伪假设） | [`lessons/`](./lessons/) 各文件 |
| rounded 系统设计 | [`architecture/rounded-system.md`](./architecture/rounded-system.md) |
| z-index 系统设计 | [`architecture/z-index-system.md`](./architecture/z-index-system.md) |

完整导航见 [README.md](./README.md)。

## 为什么拆

- 单文件 2800+ 行难以维护、检索
- 设计原则、changelog、工程教训混在一起，新人不知道从哪看
- 加新版本必须 append 到巨型文件，git diff 显示一堆"巨石移动"

拆分后每个文件专注一个主题（一个版本 / 一个组件 / 一个原则 / 一个教训），新加内容只需创建一个新文件而不是 append。
