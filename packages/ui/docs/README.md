# KunUI 文档

KunUI 是 `packages/ui` 下的 Nuxt 4 组件库 layer，被 `apps/web`（鲲 Galgame OAuth）、`apps/wiki`（鲲 Galgame Wiki）以及下游 fork（kungal / moyu / ...）共同消费。

## 文档分四类

| 目录 | 内容 | 受众 |
|---|---|---|
| [`architecture/`](./architecture/) | 跨组件设计原则、token 系统、design system 一致性 | 维护者 / 想理解整体设计的下游 |
| [`components/`](./components/) | 单组件的 spec / API / 设计取舍演变 | 消费组件的应用层开发者 |
| [`changelog/`](./changelog/) | 每个版本的完整变更记录（v0.1.0 → v0.6.0） | 升级 / 排查回归 |
| [`lessons/`](./lessons/) | 工程教训（Nuxt SSR / Tailwind v4 / flex 默认值 / 调试方法论） | 长期维护规则 |
| [`handoff/`](./handoff/) | 下游 fork（kungal / moyu / ...）的同步指令、cp 命令、视觉验证 checklist | downstream 维护者 |

## 当前版本

**v0.6.0**（2026-05-27）—— 图片查看器（KunLightbox + 新增 Gallery/Item 子组件）重写 + KunTooltip 精简 + `--kun-background-blur` glass-blur token 修复

详细看 [`changelog/README.md`](./changelog/README.md) 的版本时间表。

## 快速导航

- 第一次接触 KunUI？看 [`architecture/design-principles.md`](./architecture/design-principles.md)
- 想了解某个组件？看 [`components/README.md`](./components/README.md)
- 升级到 vX.X.X？看 [`changelog/`](./changelog/) 对应版本
- 下游同步 KunUI 改动？看 [`handoff/README.md`](./handoff/README.md)
- 想避免踩前人的坑？看 [`lessons/README.md`](./lessons/README.md)

## 文档拆分历史

2026-05-22 之前 KunUI 文档分两个大文件：

- `improvement-plan.md`（2800+ 行）—— 设计 + 各版本 changelog + 反思 混在一起
- `kungal-moyu-handoff.md`（1700+ 行）—— 多版本下游同步指令累积

2026-05-22 拆分成现在的四类目录。所有内容**完整迁移、无遗漏**：

| 旧位置 | 新位置 |
|---|---|
| `improvement-plan.md` §1 跨切面问题 | `architecture/design-principles.md` |
| `improvement-plan.md` §2 组件清单严重度 | `architecture/design-principles.md`（合并） |
| `improvement-plan.md` §3 各组件具体发现 | 分散到 `components/<name>.md` |
| `improvement-plan.md` §4 KunTab 重设计 | `components/kun-tab.md` |
| `improvement-plan.md` §5 KunTagInput spec | `components/kun-tag-input.md` |
| `improvement-plan.md` §6 落地路线图 + §7 Open Q | `changelog/v0.1.0.md` |
| `improvement-plan.md` §8 v0.1.1 复审 | `changelog/v0.1.1.md` |
| `improvement-plan.md` §9 v0.2.0 浮层引擎 | `changelog/v0.2.0.md` + §9 拆里面 v0.2.1/2.2 → `changelog/v0.2.1.md` `v0.2.2.md` |
| `improvement-plan.md` §10-§23（每个版本） | `changelog/v0.3.1.md` ... `v0.5.2.md` |
| `improvement-plan.md` §18 反思 / 各 lessons | `lessons/` 各文件 |
| `kungal-moyu-handoff.md` §0-§9 初始迁移 | `handoff/initial-v0.1.1.md` |
| `kungal-moyu-handoff.md` §10+ 每版同步 | `handoff/per-version/v0.X.X.md` |
| `kungal-moyu-handoff.md` §17 孤儿 store 陷阱 | `lessons/orphan-store-migration.md` |
| `rounded-system.md`（独立文件） | `architecture/rounded-system.md` |
