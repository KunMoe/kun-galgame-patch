# Handoff — 给下游 fork 的同步指令

KunUI 在 `kun-galgame-infra/packages/ui` 是 canonical 源。下游（kungal / moyu / ...）通过同步文件 + 验证 build 来跟随 upstream。

## 文件结构

| 文件 | 内容 | 适用场景 |
|---|---|---|
| [initial-v0.1.1.md](./initial-v0.1.1.md) | 首次大迁移（KunUI v0.0.1 → v0.1.1）的完整 checklist：8 个新文件、6 个改名、4 处 sed、5 类视觉验证 | 第一次接入 KunUI 的新 fork |
| [per-version/](./per-version/) | 每个版本的增量同步指令（cp 命令 + 验证 + 已知坑） | 已经接入的 fork 升级到下一版 |

## 通用同步流程

```bash
# 1. 拿到 KunUI 仓
KUN_OAUTH=/path/to/kun-galgame-infra

# 2. 看目标版本的 handoff/per-version/v0.X.X.md，照里面 cp 命令同步文件

# 3. 验证
cd /path/to/your-app
pnpm prepare              # 不报 dup warning
pnpm typecheck            # 通过
pnpm -F your-app exec nuxt build   # 通过

# 4. 视觉手测（每版 handoff 末尾有 checklist）
```

## 各版本同步页索引

| 版本 | 性质 | 同步页 |
|---|---|---|
| v0.6.1 | 新增功能 + drift 收口 | [per-version/v0.6.1.md](./per-version/v0.6.1.md)（含删除本地 useKunLightbox 副本 + 手搓 prose → KunContent 迁移）|
| v0.6.0 | 重大重构 + 2 个新组件 | 同步指令直接看 [changelog/v0.6.0.md §7](../changelog/v0.6.0.md#7-下游-handoff-简表) |
| v0.5.2 | 新增 prop | [per-version/v0.5.2.md](./per-version/v0.5.2.md) |
| v0.5.1 | 新增组件 KunDrawer | [per-version/v0.5.1.md](./per-version/v0.5.1.md) |
| v0.5.0 | 新增功能 KunImage skeleton | [per-version/v0.5.0.md](./per-version/v0.5.0.md) |
| v0.4.9 | bug fix Select height overflow | [per-version/v0.4.9.md](./per-version/v0.4.9.md) |
| v0.4.8 🔴🔴 | z-utility 真生效 | [per-version/v0.4.8.md](./per-version/v0.4.8.md) |
| v0.4.7 | z-bump + Select width overflow | [per-version/v0.4.7.md](./per-version/v0.4.7.md) |
| v0.4.6 | KunImage 5 prop + none provider | [per-version/v0.4.6.md](./per-version/v0.4.6.md) |
| v0.4.5 | z-index token v1 | [per-version/v0.4.5.md](./per-version/v0.4.5.md) |
| v0.4.4 🔴🔴 | useKunMessage appContext | [per-version/v0.4.4.md](./per-version/v0.4.4.md) |
| v0.4.3 🔴 | getRandomSticker | [per-version/v0.4.3.md](./per-version/v0.4.3.md) |
| v0.4.2 | KunFileInput | [per-version/v0.4.2.md](./per-version/v0.4.2.md) |
| v0.4.1 | floating animation | [per-version/v0.4.1.md](./per-version/v0.4.1.md) |
| v0.4.0 | Primitives + Ergonomics | [per-version/v0.4.0.md](./per-version/v0.4.0.md) |
| v0.3.1 | KunTab indicator fix | [per-version/v0.3.1.md](./per-version/v0.3.1.md) |
| v0.3.0 | rounded system | （在 initial-v0.1.1.md 末尾或 v0.3.0 的 changelog） |
| v0.1.1 / v0.1.0 | 首次大迁移 | [initial-v0.1.1.md](./initial-v0.1.1.md) |

## KunUI 同步通用约定（已成规则）

| 规则 | 来源 |
|---|---|
| 升级时**必须 grep build 产物**验证新加的 utility class 真实生成 | v0.4.8 教训 |
| 老的 alert / message store 在 KunUI 接管 UI 后**保留接口、内部 delegate** 而非删除，避免 promise 永不 resolve | v0.4.4 / §17 教训 |
| 升级 KunUI 后如果出现 `$nuxt null` 系列错误，**按时间最早出现的那个排查**，而不是按当前 stack trace | v0.4.4 教训 |
| 跨 fork 的 image / oauth / api endpoint 用 `https://oauth.kungal.com` 等 canonical URL，不要硬编码 IP | 见 [`../architecture/design-principles.md`](../architecture/design-principles.md) |
