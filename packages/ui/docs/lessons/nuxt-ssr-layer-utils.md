# Nuxt SSR layer util 的 4 轴审计

> 首次识别于 [v0.4.3](../changelog/v0.4.3.md) — `getRandomSticker` 双层修复：useState + 客户端 Map 缓存。本文是该版本反思的方法论沉淀。
>
> **本节双层教训**：第一层是 moyu 抓到的原始 bug，第二层是**我修复时引入的新 bug**（SSR 模块级缓存跨请求泄漏）。两层都已经修，但保留中间过程作为反面教材 —— 改 SSR 代码时如果不同时想清楚 server / client / setup / reactive effect 四个执行上下文，很容易"修一个 bug 引一个新 bug"。

## 第一层根因（moyu 诊断）

`packages/ui/app/utils/getRandomSticker.ts` 用了 **Nuxt 的 `useState`**：

```ts
// 最初版（炸）
export const getRandomSticker = (id: string) => {
  const stickerUrl = useState<string>(`random-sticker-${id}`, () => { /* pick random */ })
  return stickerUrl
}
```

调用栈：

```
tryUseNuxtApp → useNuxtApp → useState → getRandomSticker (line 6:22)
  ← Avatar.vue::userAvatarSrc (computed)
    ← <KunAvatar :user> 在父组件 refresh 后的 microtask 重算
```

Vue 的 reactivity scheduler 在 microtask 里跑 computed 重算时，**`tryUseNuxtApp()` 拿不到 Nuxt 实例**（实例只在 setup / lifecycle hook 同步路径上可访问），返回 `null` → 取 `null.$nuxt` 崩。

这是 **Nuxt 3 的已知陷阱**：`useState` 必须在 setup / lifecycle hook 同步路径调用，不能从重入式 reactive effect 里调。同族表现还有 `LinkDetailModal.vue::watch(open)` 里 `useRuntimeConfig` / `kunFetch` 必须 `nuxtApp.runWithContext(...)` 包一下 —— 都是 Nuxt context 在 microtask 边界不自动激活造成的。

## 第二层根因（我修复时引入的新 bug）

我第一版修复用"**模块级 `Map<string, Ref<string>>` 缓存 + 纯 `ref()`**"，看似避开了 Nuxt context 依赖。但**这个 Map 在 SSR 进程里跨请求泄漏**，触发新一类水合不一致：

| | F5 #1 | F5 #2（同进程下一个请求） |
|---|---|---|
| Server module cache | 空 | **已有 F5 #1 留下的 ref** ← 泄漏点 |
| Server 是否调 `useState` | ✅ 调 → 选 URL X → 入 payload | ❌ cache hit 直接返，**`useState` 没调，payload 没 X** |
| Server 渲染 HTML | X | X（从 stale ref） |
| Client 水合：payload 里有 X？ | ✅ 有 → 渲 X | ❌ 没有 → `useState` 调 initFn → 选 Y → **渲 Y ✗** |

Server 进程是长驻的（Node 的 `process.on('request', ...)` 一直跑），`const cache = new Map()` 模块作用域变量在两次请求间继续活着。但 **`NUXT.payload` 是按请求重新构造的**。两个寿命不对齐时，module cache 把 `useState` 短路掉，server 渲了 X，client 不知道是 X，自己算了 Y → 水合 mismatch。**只在 F5 #2+ 出现**，#1 永远正常 —— 比第一层 bug 更隐蔽。

## 最终修复（双层都覆盖）

**关键洞察**：cache **只在 client 端**存在；server 端永远走 `useState` 的 per-request 路径。

```ts
import { ref, type Ref } from 'vue'
import { randomNum } from '../../shared/utils/random'

const KUN_STICKER_DOMAIN = 'https://sticker.kungal.com'

const makeUrl = (): string => {
  const randomPackIndex = randomNum(1, 5)
  const randomStickerIndex = randomNum(1, 80)
  return `${KUN_STICKER_DOMAIN}/stickers/KUNgal${randomPackIndex}/${randomStickerIndex}.webp`
}

// Client-only cache. Server-side stays null → every request goes
// through useState's per-request payload path → hydration stays
// consistent across F5s.
const clientCache = import.meta.client
  ? new Map<string, Ref<string>>()
  : null

export const getRandomSticker = (id: string): Ref<string> => {
  const key = `random-sticker-${id}`

  if (clientCache) {
    const existing = clientCache.get(key)
    if (existing) return existing
  }

  const nuxtApp = tryUseNuxtApp()
  let stickerUrl: Ref<string>
  if (nuxtApp) {
    // setup / lifecycle / SSR path — useState wires the URL into
    // NUXT.payload so server-picked value is what client hydrates with.
    stickerUrl = useState<string>(key, makeUrl)
  } else {
    // Reactive recompute path on client — tryUseNuxtApp returns null
    // here. The id is by definition post-hydration (SSR never saw it),
    // so no hydration parity exists to preserve; plain ref is safe.
    stickerUrl = ref(makeUrl())
  }

  if (clientCache) clientCache.set(key, stickerUrl)
  return stickerUrl
}
```

四条路径覆盖矩阵：

| 路径 | 行为 | 为什么对 |
|---|---|---|
| SSR pass | `clientCache=null` → 跳过 → 走 `useState` → 入 payload | per-request 序列化正确，无跨请求泄漏 |
| Client 首次挂载（cache miss + 有 nuxtApp） | 走 `useState` → 读 NUXT.payload | 拿到 server 选的 URL，水合一致 |
| Client reactive 重算（cache hit） | 直接返 cached ref | 不调任何 Nuxt composable，不崩 |
| Client 首见 refresh 后才出现的新 id（cache miss + 无 nuxtApp） | fallback `ref(makeUrl())` + 入 cache | 这个 id 从未参与 SSR，没水合配对要保护，任意 URL 都可 |

## 接受的退化（更精确的描述）

只剩**一种**残留：client reactive 路径上第一次见某个全新 id 时，`tryUseNuxtApp()` 返 null → 走 fallback `ref()` 路径 → 选的 URL 与"如果 server 看到这个 id 会选的 URL"可能不同。**但这个 id 根本没经过 SSR**（refresh 后才出现），所以没有"参照真值"可对比，不存在 hydration mismatch。

## 工程规则

### 规则 1：layer utils 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门

> 任何依赖 Nuxt context 的 composable（`useState` / `useFetch` / `useAsyncData` / `useRoute` / `useRuntimeConfig` 等）调用前，要主动审视"是否可能从 reactive effect 重入路径触发"。如果可能，**必须 `tryUseNuxtApp()` 守门 + 准备 plain Vue 原语 fallback**。

### 规则 2：layer utils 里**永远不要加模块级 mutable 缓存而不区分 server / client**

> Nuxt SSR 进程长驻，模块作用域变量跨请求泄漏，会把 `useState` / `useFetch` 这种 per-request scoped 机制短路，导致**只在首次 F5 后才出现的水合 / payload 不一致 bug** —— 难诊断、产线很常见。
>
> 安全模式：
> ```ts
> const cache = import.meta.client ? new Map() : null
> ```
> 或者干脆挂到 `nuxtApp.payload._xxxCache` 之类的请求作用域里。
>
> **静态只读常量（如 `KEY_OWNING_ROLES = new Set([...])` 这种 lookup table）不在此约束内** —— 不写入就没有泄漏。

## 反思 —— 4 轴审计方法论

### 为什么 v0.1.x silent-failure-hunter 没抓到第一层

v0.1.1 复审时 agent 跑过 `getRandomSticker.ts`，但当时关注的是"`useState` 是否被正确序列化到 payload"，**没有模拟"从重入式 reactive effect 调用"的场景**。Nuxt context 依赖的运行时检测在 SSR pass / CSR 第一次挂载时都能拿到实例 —— bug 只在数据 refresh 触发 computed 重算时出现，这个场景不在静态 review 的常规检查面里。

### 为什么第二层 bug 我自己也没第一时间看出

修复第一层时，我满足于"避开 `useState` → 用 `ref()`" 的局部对症，**没把"模块级 Map 在 Node SSR 长驻进程里的寿命"作为独立维度审视**。这种"修一个 bug 引一个新 bug"在 SSR 代码里很常见：每次改 layer util 都该同时拿这四个轴过一遍：

```
1. 在 setup 顶层调用    → OK?
2. 在 reactive effect 重入路径调用 → OK?
3. 在 SSR 进程的 N 个请求间共享状态 → OK?
4. 在 CSR hydration 时 server / client 状态对得上 → OK?
```

只通过 1+2 是不够的。这次的反思让 KunUI 的 silent-failure-hunter agent 下次跑 review 时也应该把 (3) (4) 显式加入检查清单。
