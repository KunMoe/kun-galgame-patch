# Lessons — 工程教训沉淀

这些都是从真实 bug / 修复 / 复盘中总结出来的**跨版本规则**，比某个具体 bug 修复更值钱 —— 它们防止下次踩同款坑。

## 文件清单

| 文件 | 来源版本 | 核心结论 |
|---|---|---|
| [nuxt-render-appcontext.md](./nuxt-render-appcontext.md) | v0.4.4 | 命令式 `render(vnode, container)` 必须 graft `nuxtApp.vueApp._context`，否则首次崩溃会**腐化整个 Nuxt 实例** |
| [nuxt-ssr-layer-utils.md](./nuxt-ssr-layer-utils.md) | v0.4.3 | layer util 4-axis audit：setup / reactive-effect-reentry / cross-request / hydration 四个轴都过一遍才算修完 |
| [runWithContext-precise-scope.md](./runWithContext-precise-scope.md) | v0.4.4 + §18 反思 | runWithContext 只在 watch/watchEffect 回调 + 裸 render() vNode 子树**必须用**；其他场景是过度防御 |
| [orphan-store-migration.md](./orphan-store-migration.md) | §17 迁移陷阱 | 下游迁 KunUI 状态 composable 时，老 store 接口要桥接 delegate 而不是删，否则 promise 永不 resolve |
| [flex-min-zero-defaults.md](./flex-min-zero-defaults.md) | v0.4.7 + v0.4.9 | flex 子元素的 `min-width/min-height: auto`（= min-content）默认值是隐藏地雷；truncate / overflow-auto / max-height 全部需要 `min-w-0` / `min-h-0` 解锁 |
| [verify-build-artifact.md](./verify-build-artifact.md) | v0.4.8 | 改 Tailwind theme / @utility 后必须 grep 编译产物确认 utility 真实生成。"源码看着对" + "build 通过" 都不够 |
| [falsify-user-assumption.md](./falsify-user-assumption.md) | v0.4.6 (`sharp` 假设) | 用户报性能 / 崩溃 bug 时，他们的症状是真的但假设通常是错的。先用 build artifact / DevTools 证伪假设再修真根因 |

## 元教训总览

| # | 教训 | 防止的失误 |
|---|---|---|
| 1 | 验证 build 产物 | 改了源码以为对，实际整套 utility 没生成 |
| 2 | 证伪用户假设 | 顺着错假设修，永远修不到根因 |
| 3 | 多 bug 同时报告时**按时间排序**而非 stack trace 当前位置 | 误以为是 N 个独立 bug，其实是 1 个 root + N-1 个 cascade |
| 4 | 修复 SSR 代码时四轴审计 | 修一个 bug 引一个 SSR 跨请求泄漏新 bug |
| 5 | 工程规则要**精确边界** | "撒着用更安全"的护身符（runWithContext）实际反而污染代码 |
| 6 | 修复机制的"看着对"不等于"真生效" | 三轮 z-index "修复" 其实 utility 一直没生成 |
