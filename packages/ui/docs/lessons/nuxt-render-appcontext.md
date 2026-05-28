# 命令式 render() 必须 graft Nuxt appContext

> 首次识别于 [v0.4.4](../changelog/v0.4.4.md) — `useKunMessage` 裸 `render()` 漏 appContext 才是一连串崩溃的真根因。本文是该版本反思的三层教训沉淀。

## 真正的 bug — 一行代码

`packages/ui/app/composables/useKunMessage.ts:42`：

```ts
// 之前（炸）
const vNode = h(MessageContainer)
render(vNode, containerRef)   // ❌ 裸 render，vNode 处在孤立 app context
```

Vue 3 的 `render(vnode, container)` API 创建的 vNode 默认**处在一个孤立的 app context**：没有 Nuxt 实例、没有 `@nuxt/icon` 插件、没有 pinia、什么都没有。

`<MessageContainer>` 被这样裸 render → 内部 `<KunAlertMessageItem>` → 渲染 `<KunIcon>`（`@nuxt/icon` 的包装）→ `<NuxtIcon>` 的 `setup()` 第一行 `useNuxtApp()` → `tryUseNuxtApp()` 返 null → **崩**。

Stack trace 一目了然：

```
at setup (index.js:32:21)       ← NuxtIcon 的 setup
at <NuxtIcon name="...">
at <KunIcon name="...">
at <KunAlertMessageItem>
at <KunAlertMessageContainer>   ← 被 bare render() 挂载的
```

## 一行修复

```ts
const vNode = h(MessageContainer)

const nuxtApp = tryUseNuxtApp()
if (nuxtApp?.vueApp) {
  vNode.appContext = nuxtApp.vueApp._context
}

render(vNode, containerRef)
```

`vNode.appContext = nuxtApp.vueApp._context` 是 **Vue 3 文档化的 "render 独立 vNode 但保留 app context" 模式**。这一行让 MessageContainer 子树里所有 Nuxt composable（NuxtIcon 的 `useNuxtApp`、`useKunMessageState` 等）都能拿到原 Nuxt 实例。

## 之前那一连串错误的统一解释

| 表象 | 我（和 moyu）以为的根因 | **真实根因** |
|---|---|---|
| A. `getRandomSticker $nuxt null` | computed 重算路径 useState 失败 | 大部分是真，但部分是 B 引起的腐化 |
| B. `kunFetch useRuntimeConfig $nuxt null` (在 `LinkDetailModal.vue::watch(open)` 里) | watch 调度从 microtask 跑，丢了 Nuxt context | **`useKunMessage` 之前已经把 Nuxt 实例搞坏了**，watch 拿到的本来就是腐化态 |
| C. 编辑 modal 关闭报 `$nuxt` | 关闭流程里某个 composable 调度问题 | **关闭时 `useKunMessage(10550, 'success')` 触发首次挂载 MessageContainer → NuxtIcon 炸** |
| D. 后续任意操作都开始报 `$nuxt` | 多个独立 bug | **C 把 Vue 内部状态弄坏后，后续所有 useNuxtApp 一片 null** |

A 确实是独立的 reactive recompute 问题（[v0.4.3](../changelog/v0.4.3.md) 修过），但 B、C、D **不是独立 bug**，是 useKunMessage 首次挂载失败的连锁反应。**之前所有 `runWithContext` / `nextTick` / refactor 的修复都在错误的地方修**，只是把崩溃点推迟，没解决真正的源头。

## 为什么"首次崩溃会腐化整个 Nuxt 实例"

Vue 3 `render(vnode, container)` 失败时，Vue 内部的渲染器状态可能进入半挂载态。如果失败的子树有 effect / setup 已经注册到全局 reactive system 但没成功绑到 component instance 上，**这些 dangling effects 会污染后续 `useNuxtApp()` 的查找路径**。具体机制要看 Vue 源码细节，但实测表现是：useKunMessage 首次失败 → 后续任意 setup 调用 `useNuxtApp()` 也开始返 null。

**这就是为什么我之前用 `runWithContext` 包 LinkDetailModal 的 watch 一度看似有效但又不稳定** —— `runWithContext` 把 Nuxt 实例显式注入到回调里，绕过了 `useNuxtApp()` 的查找，所以**在被腐化的实例还能用时**修复有效；一旦腐化更深，连显式注入的实例本身都不健康。

## 工程规则

> 用 Vue 原生 `render(vnode, container)` 命令式挂载组件时，**必须**显式 graft Nuxt appContext。
>
> ```ts
> const vNode = h(SomeComponent)
> const nuxtApp = tryUseNuxtApp()
> if (nuxtApp?.vueApp) {
>   vNode.appContext = nuxtApp.vueApp._context
> }
> render(vNode, container)
> ```
>
> **不这样做的后果**：被 render 的子树处于孤立 app context，任何调用 Nuxt composable 的子组件（`<NuxtIcon>` / `<NuxtLink>` / `<NuxtImg>` 之类）都会在 setup 期崩，而且**首次崩溃可能腐化整个 Vue / Nuxt 实例**，后续 `useNuxtApp()` 一片 null。
>
> 涉及面：任何用 `render()` / `createVNode()` / `createApp().mount()` 命令式挂载 UI 的 KunUI 代码（弹窗、toast、command palette、context menu）。审查时主动 grep `\brender\(` 看是否都带了 appContext graft。
>
> **Vue 文档参考**：https://vuejs.org/api/render-function.html#h —— "createVNode() also accepts a third argument context to specify which app context to use"。

## 反思 — 调试 SSR / Nuxt context bug 的元教训

这一轮调试有三层教训叠加：

**层 1 — 局部对症 vs 根因**：我和 moyu 都犯过"看到 stack trace 直接修 stack trace 顶部"的错。`getRandomSticker` 是真 bug（[v0.4.3](../changelog/v0.4.3.md)），但 `kunFetch in watch` / `editModal close` / `subsequent useNuxtApp null` 这三个"看起来是 watch 调度 / async timing / cleanup 的问题" **统统是 useKunMessage 一次性挂载失败的连锁反应**。

**层 2 — Nuxt 实例的"腐化"是真存在的**：以前我以为 Nuxt context 失败只是"局部拿不到"，不会"污染整个 app"。这次实测发现 `render()` 失败会让后续 `useNuxtApp()` 全军覆没 —— 一旦某个表象出现，赶紧问自己"**这是不是某个早一点的 render / mount 失败的余波**？"

**层 3 — 多 bug 同时报告时，先找"最早触发的那个"**：调试时按时间顺序排查事件，不要按当前报错位置排查。如果 useKunMessage 在 t=0 失败，但你的注意力被 t=5 的 LinkDetailModal watch 报错吸走，就会一直跟着 t=5 的红鲱鱼跑。

## 修复后的连锁效应

| 表象 | 修复 v0.4.4 后 |
|---|---|
| A. `getRandomSticker` | [v0.4.3](../changelog/v0.4.3.md) 的双层修复已经独立解决了 |
| B. `kunFetch` in watch | **自动消失** —— 真正的根因是 C 引起的腐化，C 修了 B 自然好 |
| C. 编辑 modal 关闭 `$nuxt` | **自动消失** —— MessageContainer 现在带着正确 appContext 挂载，NuxtIcon 拿得到 Nuxt 实例 |
| D. 后续任意操作崩溃 | **自动消失** —— 没有 C 的连锁腐化就不存在 |
