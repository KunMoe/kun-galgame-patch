# KunImage

KunImage 是对 NuxtImg 的轻量封装。v0.4.6 补齐了 5 个 prop 透传（让下游可以做 LCP / no-IPX / retina densities 等优化），v0.5.0 加了加载骨架屏（skeleton）。

## v0.4.6 — KunImage 5 个 prop 透传 + KunUI layer 注册 `none` provider（§17）

moyu 报告 `/about` 页卡顿，调查过程值得整体记录 —— **一开始的假设错了**，但调查方法对，且终态修复发现的根因揭示了 KunImage 一个真实 API 缺口。

### moyu 调查的精彩之处

**错误假设**：用户报告"sharp 在前端运行所以页面卡"。

**moyu 反驳**：

> Sharp 是 Node.js 原生绑定（libvips C 库），物理上不可能在浏览器里运行。浏览器没有 N-API 也没有 native binding 加载能力。我也跑了 build 验证：
>
> ```
> .output/server/node_modules/sharp      ← server bundle 里有
> .output/public/_nuxt/*.js              ← client bundle 里 0 sharp
> ```

直接证伪用户假设，没顺着错的方向修。这是 senior 调试的标志。

**真正根因**：

| 症状 | 成因 |
|---|---|
| 首次加载慢 | 4 张 card banner 并行触发 IPX 冷启动（每张几百 ms server-side sharp transcode） |
| 滚动卡顿 | 没 width/height → 每张图加载时 layout shift → 浏览器反复 reflow |
| 流量浪费 | banner 已经是 author 时压好的 AVIF (30-100KB)，IPX 再过一遍 sharp 0 收益 |
| 缓存抖动 | IPX 默认 FS 缓存 5 分钟过期，冷启动反复 |

**moyu 在他们仓的修复**：

```vue
<NuxtImg
  :src="banner"
  provider="none"           ← 不走 IPX
  loading="lazy"
  :width="512" :height="288" ← 预留空间，0 layout shift
  fetchpriority="high"       ← LCP 元素加 hint
/>
```

但**这里他们卡住了** —— KunImage 没有 `provider` / `fetchpriority` 这些 prop，他们被迫放弃 `<KunImage>` 改用裸 `<NuxtImg>`。**这是 KunImage 一个真实的 API 缺口**。

### 修复 1 — KunImage 加 5 个 prop 透传

`packages/ui/app/components/kun/image/Image.vue` 加：

| prop | 类型 | 用途 |
|---|---|---|
| `provider` | `string` | 切换图片 provider（`"none"` 跳过 IPX） |
| `densities` | `string` | retina srcset 提示，如 `"1x 2x"` |
| `sizes` | `string` | 响应式 sizes，如 `"sm:100vw md:50vw"` |
| `fetchpriority` | `'high' \| 'low' \| 'auto'` | HTML fetchpriority 属性，LCP 元素用 `"high"` |
| `decoding` | `'sync' \| 'async' \| 'auto'` | HTML decoding 属性，非 LCP 用 `"async"` |

所有 5 个都是可选透传，不影响现有调用。

### 修复 2 — KunUI layer 注册 `none` provider

`packages/ui/nuxt.config.ts` 加：

```ts
image: {
  providers: {
    none: { name: 'none', provider: '@nuxt/image/runtime/providers/none' }
  }
}
```

**为什么放在 layer 而不是让每个 app 自己加**：moyu 之前自己 fork 加这一条，三个下游（kungal / moyu / wiki / oauth）各加一次重复 = code drift。**放进 KunUI layer 一次定义，所有 downstream 免费获得** —— 这是 layer 系统的本意。

### moyu 的 `/about` 优化报告作为最佳实践 cheat sheet

下游用 `<KunImage>` 时按以下规则选 prop 组合，照抄即可：

#### 静态预优化图（已 AVIF / 已 WebP，author 时压过的）

```vue
<KunImage
  :src="post.banner"
  provider="none"            ← 跳过 IPX
  loading="lazy"             ← 视口外不下载
  :width="512" :height="288" ← 强制留空间，0 layout shift
  class-name="..."
/>
```

#### LCP 元素（首屏最大图片，比如博文详情页 banner）

```vue
<KunImage
  :src="banner"
  provider="none"
  loading="eager"            ← 立即加载
  fetchpriority="high"       ← 提示浏览器优先抓
  :width="1200" :height="400"
  class-name="..."
/>
```

#### 用户头像 / galgame banner（需要 runtime resize）

```vue
<KunImage
  :src="user.avatar"
  loading="lazy"
  :width="64" :height="64"
  densities="1x 2x"          ← retina 屏自动选 2x
/>
```

不传 `provider` → 默认走 IPX，符合 runtime resize 需求。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- moyu 那边的 vue-tsc + build 都过 ✅
- moyu 那边 `/about` 卡顿消失 ✅

### 反思 — sharp 假设给我们的元教训

用户报告"sharp 在前端运行"是错的，但他们能感知到"卡"是真的。**这种"症状真，假设错"的报告是最常见也最难处理的**：

- 顺着错假设修 → 修不到根因，用户继续抱怨
- 直接反驳 → 用户觉得你在踢皮球
- moyu 的做法：**用证据反驳假设 + 重新定义问题 + 找真根因** —— 这才是建设性回应

对应 KunUI 维护策略：用户报告卡 / 崩 / 闪烁时，**先问"我看到的物理证据是什么"**（build artifact 检查、network panel、devtools performance），再问"那真正的瓶颈是什么"，最后才修。不要被"用户给出的假设"牵着走。

### 给 KunUI 的可选未来 prop

moyu 这次只要 5 个，但 NuxtImg 还有这些常用 prop 未透传，下次有人提就再加：

| 未来 prop | 用途 |
|---|---|
| `fit` | cover / contain / fill / inside / outside |
| `modifiers` | 额外 IPX modifier 字典 |
| `preset` | nuxt.config.image.presets 里定义的命名预设 |
| `background` | 透明图片 fallback 背景色 |

不在 v0.4.6 范围内，等有真实需求时再补。

## v0.5.0 — KunImage 加载骨架屏（skeleton）（§22）

### 设计取舍 — 为什么不用 wrapper div

最直白的实现是给 KunImage 包一层 `<div>` 当 wrapper，wrapper 显示 skeleton，img 显示在 wrapper 内。但这是**破坏性改动** —— 任何调用方如果在 layout 里假设 KunImage 渲染出 `<img>`（比如 `flex` 子项、`object-fit` 样式、`<a><KunImage></a>` 嵌套等）会因为元素类型变化出现微妙 bug。

更安全的做法：**直接在同一个 `<img>` 上加 `bg-default-200 animate-pulse` 背景类**，加载完成后移除。

| 维度 | wrapper div 方案 | bg-class 方案（采纳）|
|---|---|---|
| 渲染元素类型 | 从 `<img>` 变成 `<div><img></div>` | 仍是单个 `<img>` |
| 现有调用方兼容性 | ❌ 破坏 | ✅ 零破坏 |
| skeleton 显示位置 | wrapper 上 | img 自身 background |
| skeleton 形状 | 由 wrapper 决定 | 由 img 的 width/height 决定 |
| 复杂度 | 多一层 DOM 节点 + class 分流 | 单元素 + 一个 reactive class |

### 实现

```ts
// Image.vue
const props = withDefaults(defineProps<KunImageProps>(), {
  // ... existing defaults
  skeleton: true
})

const isLoaded = ref(false)
const onLoad = () => { isLoaded.value = true }
// 错误也算 "loaded" —— 不让破图永远 pulse
const onError = () => { isLoaded.value = true }

const skeletonClass = computed(() =>
  props.skeleton && !isLoaded.value ? 'bg-default-200 animate-pulse' : ''
)
```

```vue
<NuxtImg
  :class="cn(skeletonClass, className)"
  ...
  @load="onLoad"
  @error="onError"
/>
```

### 关键细节

#### 1. SSR pass 时 `isLoaded` 是 false → HTML 里 skeleton class 已经写入

```html
<!-- 服务器直出 -->
<img class="bg-default-200 animate-pulse size-full object-cover ..." src="..." />
```

客户端 hydration 接管后图片开始加载，`@load` 触发 → Vue reactive 更新 → skeleton class 自动移除。**SSR-CSR 状态无缝衔接**，首帧不需要 client JS 就有 skeleton。

#### 2. 错误也清 skeleton（不让破图永远 pulse）

破图的 native 行为（broken image icon）比"持续 pulse" 更准确传达 "加载失败" 信号。`@error` 处理器和 `@load` 一样 set `isLoaded = true`。

#### 3. 必须依赖 `width` + `height` 来 reserve 空间

`<img>` 没 explicit 尺寸时 = `0×0` = skeleton 不可见。这跟 v0.4.6 文档已经在说的"防 layout shift 必传 width/height"是同一回事，本节复用这个前提，不额外强制。

#### 4. 可选关闭

```vue
<KunImage :src="..." :skeleton="false" />
```

适合小尺寸 icon / 装饰图（pulse 一闪反而 distracting），或者父级已经管 skeleton 的场景。默认 `true`，因为绝大多数图片场景受益。

#### 5. 与 NuxtImg 的 `placeholder` prop 共存

NuxtImg 自己的 `placeholder` 生成低质量模糊预览。三段加载体验：

```
[skeleton pulse]  →  [模糊 placeholder]  →  [清晰图]
```

`@load` 在最终图 ready 时触发，所以 skeleton 一直 cover 到模糊预览出现（视觉无缝过渡）。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 同步到 moyu 仓 + HMR 自动刷新
- curl SSR HTML 拿到 `class="bg-default-200 animate-pulse ..."` ✅
- 浏览器看 /galgame 页面 26 张图，首帧 skeleton 可见，加载完成逐张消失 ✅

### v0.5.0 标签

这是 KunUI 第一个**单独加新功能（不是 bug fix）**的版本，所以从 v0.4.x → v0.5.0。语义版本号上的 minor bump 表示加 API 不破坏旧 API。

`KunImageProps` 增加可选 `skeleton?: boolean`，默认 `true`。下游零修改即可享受 skeleton；想关掉就 `:skeleton="false"`。

### 给 kungal / moyu 同步指令

```bash
KUN_OAUTH=/path/to/kun-galgame-infra
cp $KUN_OAUTH/packages/ui/app/components/kun/image/Image.vue \
   ./components/kun/image/Image.vue
```

只 1 个文件，无配置 / 类型 / 依赖联动。
