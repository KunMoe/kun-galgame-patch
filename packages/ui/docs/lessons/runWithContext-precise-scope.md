# runWithContext 的精确适用边界

> 首次识别于 [v0.4.4 / v0.4.5 反思修正 §18](../changelog/v0.4.6.md) — moyu 在 fork 调试一个"点删除按钮无反应"的 bug 时，发现自己之前为了绕 `$nuxt null` 问题、在好几条 click 路径上加的 `nuxtApp.runWithContext(...)` 包装**根本没起作用**。这次复盘修正了之前 [v0.4.4](../changelog/v0.4.4.md) / [v0.4.5](../changelog/v0.4.5.md) 隐含的"runWithContext 是万能护身符"印象。

## 修正 1 — `runWithContext` 真正必须的两个场景

之前 [v0.4.3](../changelog/v0.4.3.md) / [v0.4.4](../changelog/v0.4.4.md) 末尾给出的"layer util 调 Nuxt composable 必须 `tryUseNuxtApp` 守门"规则**仍然正确**，但常被误读为"凡是 await / 跨 tick 都该 `runWithContext`"。准确的边界是：

```
✓ 必须用 runWithContext:
  1. Vue watch / watchEffect 回调内调 Nuxt composable
     （脱离当前组件 instance 的微任务）
  2. render(vNode, container) 裸 mount 的 vNode 子树
     （没有任何 instance binding —— v0.4.4 §15 已修过）

✗ 不需要用 runWithContext:
  3. @click / @input / @submit 等 Vue 事件处理器
     （Vue 3 的 withCtx 包装让 getCurrentInstance() 不为 null）
  4. setup() 同步路径
  5. onMounted / onUnmounted / 其他生命周期 hook
     （Vue 在 hook 内已保住 instance）
  6. 大多数 await kunFetch(...) 之后的代码
     （Nuxt 3 对常见 await 路径有内部 context patch）
```

## 修正 2 — `tryUseNuxtApp()` 的查找路径解释为什么 (3-6) 不需要

`tryUseNuxtApp()` 内部查找顺序大致是：

```
1. nuxtAppCtx.tryUse()                            ← Nuxt 自维护的 AsyncLocalStorage
2. getCurrentInstance().appContext.app.$nuxt      ← Vue 当前实例的 app context
```

Vue 3 的事件处理器 / lifecycle hook **执行期都有 `getCurrentInstance()` 不为 null** —— 路径 2 直接命中，根本走不到路径 1 需要 `runWithContext` 强制注入的情况。

## 修正后的 $nuxt-null / 按钮静默故障 排查清单

按这个顺序，省时间：

| 步 | 检查 | 命中怎么修 |
|---|---|---|
| 1 | 点击**完全静默**？(没报错没请求) | 怀疑**孤儿 store**，grep 老 alert / message store 是否还有人调（见 [orphan-store-migration.md](./orphan-store-migration.md)） |
| 2 | 报 `$nuxt null` 且**屏幕崩**？ | 怀疑 [nuxt-render-appcontext.md](./nuxt-render-appcontext.md) 的 render() 裸 mount 漏 graft appContext |
| 3 | 报 `$nuxt null` 在 watch / watchEffect 里？ | 用 `nuxtApp.runWithContext(() => …)` 包回调体 |
| 4 | 报 `$nuxt null` 在 @click handler 里？ | **99% 不需要 runWithContext**，先找别的原因 |

## 反思 — 这次"过度防御"被纠错的元教训

moyu 之前一连串撞 $nuxt null 时，我（们）总结的工程规则**没错**，但**容易被泛化滥用**。"任何依赖 Nuxt context 的代码都该用 runWithContext 守门" 听起来合理，实际只在两个特定场景必须，剩下场景撒了：

- ✗ 不会修 bug（路径 2 已经 work，不缺路径 1）
- ✗ 增加心智负担（每个 await 都问"要不要包"）
- ✗ 让真正必须用的场景反而**藏在噪声里**

这次 moyu 找到孤儿 store 才意识到：**之前那些 runWithContext 包装其实并没有解决任何 bug，只是恰好与 $nuxt-null 报告同时出现，被我误归因**。这是个经典的"相关 ≠ 因果"工程教训：观察到"加了 X 之后 bug A 消失了"不代表 X 修了 A，可能只是 A 本来就和 X 无关、由别的修复（v0.4.4 真根因 useKunMessage appContext graft）解决了。

下次给规则归因前，**严格分离"哪些 commit 改了哪些行为"**，不要把一连串改动打包归因到某个最显眼的那一条。
