# Architecture

跨组件设计原则、token 系统、design system 一致性规则。

## 文件

| 文件 | 内容 |
|---|---|
| [design-principles.md](./design-principles.md) | 7 条跨切面原则（variant×color 矩阵、颜色系统、defineModel、命名规范等）+ 组件清单严重度表 |
| [rounded-system.md](./rounded-system.md) | 5-bucket 半径 token 体系：CSS @theme variable + Vue useKunUIConfig provider + 组件 rounded prop 三层覆盖 |
| [z-index-system.md](./z-index-system.md) | 5-tier 浮层 z-index token：sticky / modal / popover / alert / message |

## 长期约束（KunUI 工程规则）

1. **layer util 调 Nuxt composable** 必须 `tryUseNuxtApp()` 守门 + plain Vue 原语 fallback（适用范围：watch/watchEffect、render() 裸 mount；事件 handler 不需要）
2. **layer util module-level mutable cache** 必须 `import.meta.client ? new Map() : null`，避免 SSR 进程跨请求泄漏
3. **命令式 `render(vnode, container)`** 必须 graft `nuxtApp.vueApp._context` 到 vNode.appContext
4. **Teleport-to-body 浮层** 必须用 `z-kun-*` token，禁止 magic number
5. **下游迁 KunUI 状态 composable**（`useKunAlert` 等）时，老 store 接口要桥接 delegate 而不是删
6. **flex 容器里要受父级尺寸约束的子元素** 必须搭配 `min-w-0` / `min-h-0`（truncate / overflow-auto / max-height 都隐含这个前提）
7. **Tailwind theme / utility 改动后** 必须 grep 编译产物验证 utility 真实生成（v0.4.5/6/7 三轮 z-token 没生效就是因为没做这一步）
