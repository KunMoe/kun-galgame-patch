# 下游迁 KunUI 时的孤儿 store 陷阱

> 首次识别于 [v0.4.4 / v0.4.5 反思修正 §18](../changelog/v0.4.6.md) — moyu 在 fork 调试一个"点删除按钮无反应"的 bug 时挖出来的陷阱。本节内容也在 [handoff §17](../handoff/README.md) 出现，这里作为通用工程教训沉淀。

## 症状特征

下游 app 从老 alert / dialog / message store 迁到 KunUI 的 `useKunAlert` / `useKunMessage` 时，如果**只迁了 UI 组件**（把老的 `<MyAlert>` 删了换 `<KunAlert>`），但**老 store 的 `alert(...)` 方法还在被旧调用点使用**，会触发"孤儿 store"bug：

- 点击按钮后**完全静默** —— 没报错、没 network 请求、没 console output
- 跟 `$nuxt null` 不同，**完全无报错**（promise 死锁不是 error）
- DevTools Performance 看到点击事件触发但没后续

## 复现代码

```ts
// 老 store（迁移后变孤儿）
const useOldAlertStore = defineStore('alert', () => {
  const showAlert = ref(false)   // ← 没人 watch 这个 ref 了
  const alert = (...) => {
    showAlert.value = true       // ← 改 ref，但没有组件渲染弹窗
    return new Promise(...)      // ← 没人调 resolve → 永远 pending
  }
  return { alert, /* ... */ }
})

// 残留调用点（看起来在工作）
const store = useOldAlertStore()
const ok = await store.alert(...)   // ← 死锁在这一行
if (ok) doDelete()                  // ← 永远不到这里
```

UI 已经被 KunAlert 替换，但老 store 的 `showAlert.value = true` 没有任何组件在 watch，所以没有 UI 弹出，自然也没有"用户点确认"来 resolve 那个 promise。`await` 永远 pending，后续代码永远不执行 —— 用户感知就是"按钮点了没反应"。

## 修复模板 —— 桥接老 store

不要直接删老 store（会破坏所有残留调用点的 TS 编译），而是把它的 `alert` 方法**桥接到 KunUI 的 composable**：

```ts
const useOldAlertStore = defineStore('alert', () => {
  // 其他字段保留不动（不破坏依赖它们的其他流程）

  // alert 改成 useKunAlert 的薄包装
  const alert = (
    title?: string,
    message?: string,
    showCancel?: boolean
  ): Promise<boolean> =>
    useKunAlert({
      title,
      message,
      showCancel: showCancel ?? true
    })

  return { alert, /* ...其他字段... */ }
})
```

零修改所有调用点，UI 由 KunAlert 渲染，promise 由 useKunAlertState 正常 resolve。

## 工程规则

> **下游迁 KunUI 状态 composable（`useKunAlert` 等）时，老 store 接口要桥接 delegate 而不是删**，避免孤儿 hang。
>
> 排查 "点击完全静默 / 无报错" 类故障时，**第一件事**是 grep 老 alert / message / dialog store 是否还有调用方 —— 比怀疑 Nuxt context 类问题更快定位。

## 为什么这个陷阱比 `$nuxt null` 更难调

| 维度 | `$nuxt null` 崩溃 | 孤儿 store 死锁 |
|---|---|---|
| 是否报错 | ✅ 红字 stack trace | ❌ 完全静默 |
| DevTools 表现 | console 一目了然 | 只看到 click 事件触发，无后续 |
| 怀疑方向 | "Nuxt context 问题" | 容易往"事件未触发 / 网络 fail" 走 |
| 修复时间 | 看 stack trace → 几分钟 | 没线索可循 → 容易卡几小时 |

所以这条 lesson 优先级高 —— 知道陷阱存在，下次至少 5 分钟内能怀疑到对的方向。
