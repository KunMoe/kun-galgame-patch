# KunLightbox

Lightbox 是 KunUI 的图片查看器组件家族，由 3 个组件组成。v0.6.0 重写为以原生 `<dialog>` 为底座、view-transitions API 加持的现代实现。

| 组件 | 角色 | 适用场景 |
|------|------|----------|
| `KunLightbox` | 底层 primitive | 已知 images 数组 + 程序化 open 控制 |
| `KunLightboxGallery` | 声明式包装 | 90% 画廊场景（cover 集合、截图集合、贴图列表） |
| `KunLightboxGalleryItem` | Gallery 子项 | 缩略图触发器 |

## 选哪个？

| 场景 | 用 | 理由 |
|---|---|---|
| 用户点 patch 列表里某张图，弹出查看 | `<Gallery>` + `<Item v-for>` | 缩略图和大图列表同 v-for，0 样板 |
| MDX 文章内每张 `<img>` 想点开 | `<Gallery>` 包整个文章 + 渲染器把 `<img>` 包成 `<Item>` | 同上 |
| 头像点开看大图（程序化触发） | `<KunLightbox v-model:is-open="x">` 直接控制 | 没有"列表"概念 |
| 一次性显示某张图（如错误页 hero） | 用 `<KunImage>` 即可，不需要 Lightbox | 没有缩放需求 |

## API — `<KunLightbox>`（底层）

### Props

| prop | 类型 | 默认 | 说明 |
|------|------|------|------|
| `images` | `ImageItem[]` | required | `{ src: string, alt?: string }[]` |
| `isOpen` | `boolean` | required | v-model:isOpen 双向绑定 |
| `initialIndex` | `number` | `0` | 打开时定位的图片索引 |

### Emits

| event | payload | 说明 |
|-------|---------|------|
| `update:isOpen` | `boolean` | `v-model:is-open` 反向 |

### 用法

```vue
<script setup>
const isOpen = ref(false)
const idx = ref(0)
const images = [
  { src: '/big1.jpg', alt: 'cover' },
  { src: '/big2.jpg', alt: 'screenshot' }
]
</script>

<template>
  <button @click="isOpen = true; idx = 0">查看图集</button>
  <KunLightbox
    v-model:is-open="isOpen"
    :images="images"
    :initial-index="idx"
  />
</template>
```

## API — `<KunLightboxGallery>` + `<KunLightboxGalleryItem>`（声明式）

### Gallery — 无 prop

包装一个区域，里面的 Item 自动注册到 Gallery 的内部 lightbox。

### Item Props

| prop | 类型 | 默认 | 说明 |
|------|------|------|------|
| `src` | `string` | required | 大图 URL（lightbox 缩放需要全分辨率）|
| `alt` | `string` | `''` | 大图 alt 文本 |
| `as` | `string` | `'span'` | 触发器包装元素（inline 用 span，block 用 div） |
| `wrap` | `boolean` | `true` | false 时不渲染 wrapper，靠 scoped-slot `{ open }` 手动触发 |

### Scoped Slot 绑定

`v-slot="{ open }"` — 拿到 open 函数，可在 slot 内任意元素调用。

### 用法（90% 场景）

```vue
<KunLightboxGallery>
  <KunLightboxGalleryItem
    v-for="cover in covers"
    :key="cover.id"
    :src="imageHashUrl(cover.image_hash, { cdnBase })"
    :alt="cover.caption"
  >
    <img :src="imageHashUrl(cover.image_hash, { cdnBase, variant: 'mini' })" />
  </KunLightboxGalleryItem>
</KunLightboxGallery>
```

### 用法（render-prop 进阶）

card 里既要显示缩略图又要其他按钮，不想 wrapper 抢点击：

```vue
<KunLightboxGalleryItem :src="big" :wrap="false" v-slot="{ open }">
  <div class="card">
    <img :src="thumb" @click="open" class="cursor-zoom-in" />
    <button @click="like">点赞</button>  <!-- 不会触发 lightbox -->
  </div>
</KunLightboxGalleryItem>
```

## 交互手势全集

| 触发器 | 行为 |
|--------|------|
| 鼠标滚轮 | 锚定光标位置缩放（0.2 步进） |
| 鼠标双击 | 1x ↔ 2x toggle，锚定双击位置 |
| 鼠标拖拽（zoom > 1）| 平移图片，越界自动 constrain |
| 鼠标拖拽（zoom = 1）| 水平拖动 + 释放速度阈值 → swipe 切图 |
| 手指单触 + 拖（zoom > 1）| 平移 |
| 手指单触 + 拖（zoom = 1）+ 释放 | swipe 切图（速度 + 距离阈值）|
| 手指双指捏合 | 锚定中点缩放，比例敏感（distRatio 算法）|
| 手指双指捏合 + 平移 | 缩放 + 平移同时进行 |
| 手指双击 | 同鼠标双击（手动 300ms 检测，不依赖 @dblclick）|
| 键盘 ← / → | prev / next 切图 |
| 键盘 ESC | 关闭（原生 dialog 行为）|
| 键盘 Enter / Space | （Item 上）触发 open |

### 手势状态机要点

- `isDragging` + `isPinching` 两个 ref 联动 `transformStyle.transition`：手势中 `none`（帧对帧跟手），离散操作 `0.3s ease-out`（平滑过渡）
- 双指→单指退化时重置 drag baseline 到剩余那根手指（避免瞬移）
- container 有 `touch-action: none`，浏览器原生 pinch-zoom 不抢手势

## 工具条布局

底部居中 floating，3 区段分隔：

```
┌─────────────────────────────────────────┐
│ [−] [100%] [+] │ [↺] [↻] │ [⟲] [⬇] │
└─────────────────────────────────────────┘
   缩放（含实时百分比）│ 旋转 │ 重置/下载
```

PC 在 toolbar 上方还有缩略图条（max-w-[80vw] 横滚），mobile 则是 pagination dots。两者互斥（CSS `md:hidden` / `hidden md:flex`）。

边角 chrome：
- top-left: counter `1 / N`（>1 时显示）
- top-right: close `×`
- PC 左/右边缘中部：大箭头（mobile 隐藏，靠 swipe）

所有 chrome 都用 `bg-black/70 border-white/10 text-white backdrop-blur-md` —— **永远深色玻璃**（业界图片查看器惯例：与主题无关，对比内容最强）。

## 缩放 / 旋转 / 位置的实现细节

### 缩放（wheel + double-click + pinch）

锚点缩放公式，三处共用：
```ts
newPos.x = anchor.x - (anchor.x - oldPos.x) * scaleChange
```

`anchor` 表达方式：
- wheel：`event.client* - container.rect.left/top`（光标位置）
- double-click：同上
- pinch：双指中点 - container 中心（注意原点不同，因为 transform-origin 是图片中心）

`scale` 范围 `[1, 5]`。constrainPosition 限制 panning 不超出图片可见区域。

### 旋转

`rotation` 存储 unbounded（±∞）。每次 ±90 后 CSS transition 始终插值用户点击方向的最短路径（90°）。如果取模到 `[0, 360)`，在 wrap 边界（270 → 0）会插值 -270° 反方向。

重置时 snap：`rotation = Math.round(rotation / 360) * 360`。视觉等同 0°，但 CSS transition 路径 ≤180°（避免 0.3s unwind N 圈眩晕）。

### 位置 + transition

`transformStyle.transition` 三态：
- `isDragging || isPinching` → `'none'`（手势中零延迟）
- 否则 → `'transform 0.3s ease-out'`（离散操作平滑）

## CORS 兼容的下载

```ts
try {
  const response = await fetch(url, { mode: 'cors' })
  // ... fetch → blob → <a download> ...
} catch (error) {
  // CDN 不带 CORS 头时降级
  window.open(url, '_blank', 'noopener,noreferrer')
}
```

happy path 同源 / CORS 友好：直接存盘。
fallback：新窗打开，用户右键保存。

长期方案：CDN 配 `Access-Control-Allow-Origin`，happy path 自动生效。

## 内部架构

```
KunLightboxGallery
 ├─ provide(KunLightboxGalleryKey, { register, open })
 │   ├─ items[] ref<RegisteredItem>
 │   ├─ isOpen ref
 │   └─ initialIndex ref
 └─ <KunLightbox v-model:is-open="isOpen" :images="items" :initial-index="initialIndex" />
       │
       ▼
KunLightboxGalleryItem (× N)
 ├─ inject(KunLightboxGalleryKey)
 ├─ Symbol id (per instance)
 ├─ register on mount, unregister on unmount
 └─ <span @click="ctx.open(this.item)"><slot /></span>
```

`KunLightboxGalleryKey` 是 InjectionKey<GalleryContext>，**导出在 `Gallery.vue` 的非-setup `<script>` 块**（非独立 `.ts` 文件）。理由：独立 `.ts` 会被 Nuxt 当成组件扫描注册（即使无 default export），污染 `components.d.ts`。

## 弹层底座 — 原生 `<dialog>`

v0.6.0 起从 `KunModal` 切到原生 `<dialog>`：

| 能力 | 原生 dialog 免费 | 仍手动 |
|------|------------------|--------|
| Focus trap | ✓ `showModal()` | — |
| ESC 关闭 | ✓ | — |
| Background `inert` | ✓ | — |
| `::backdrop` 伪元素 | ✓ | — |
| Body scroll lock | ✗ | 用 `useBodyScrollLock` |
| Backdrop 点击关闭 | ✗ | `@click` + `target === dialog` 判定 |
| 视图过渡 | ✗ | `document.startViewTransition` 兼容包装 |

## 切图动画

Vue `<Transition :name="slideDir">` 包装 image slide div（绝对定位重叠）：

```css
.slide-next-enter-from { transform: translateX(100%); opacity: 0; }
.slide-next-leave-to { transform: translateX(-100%); opacity: 0; }
/* slide-prev 镜像 */
.slide-*-enter-active, .slide-*-leave-active {
  transition: transform 0.35s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.35s ...;
}
```

`slideDir` 在 next() / prev() / goToIndex(i) 调用时设置。`goToIndex` 比较新旧 index 决定方向（点击右侧缩略图 → next 方向）。

## View Transitions API（opt-in 缩略图 morph）

打开 / 关闭 / 翻页都包了 `withViewTransition(() => mutate())`：

```ts
const withViewTransition = (mutate: () => void) => {
  const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
  if (typeof doc.startViewTransition === 'function') doc.startViewTransition(mutate)
  else mutate()
}
```

支持浏览器（Chrome/Edge 111+、Safari 18+）自动 cross-fade。

**进阶**：缩略图 morph 到全屏图。consumer 给缩略图加：
```vue
<img class="thumbnail" style="view-transition-name: kun-lightbox-image" />
```
名字与 Lightbox 内 image 的 `view-transition-name: kun-lightbox-image`（写死在 scoped CSS）一致，浏览器自动算 morph 路径。

⚠️ 当前实现 view-transition-name 是单一固定值（`kun-lightbox-image`）。如果一页同时有多张缩略图都标这个名字，会冲突。需要 per-instance 名字时，未来可改为动态 `view-transition-name: kun-lightbox-image-{id}` + 让 Item 自动给缩略图加同 id。

## a11y 完备性

- `<dialog aria-label="图片查看器">`
- counter `aria-live="polite"`（切图时屏读自动播报新位置）
- zoom 百分比 `aria-live="polite"`
- 所有按钮带 `aria-label`
- 缩略图 / dots 带 `aria-current` + `aria-label="跳转到第 N 张"`
- 键盘焦点 trap 在 dialog 内（原生）
- ESC 关闭（原生）
- ←/→ 翻页

## 已知限制

1. **旋转 90°/270° 时宽图溢出 container**：CSS transform 不改 layout box。宽图旋转 90° 后可视超出，用户需平移或点重置。
2. **CDN CORS** 跨域时下载走 fallback（新窗），不直接存盘。
3. **View transitions 单一名字**：同页多张标 `kun-lightbox-image` 会冲突。
4. **`KunLightbox` primitive 调用方负责处理 thumbnails 显示**：底层组件不渲染缩略图，只在 Gallery wrapper 里有。

## 演化历史

| 版本 | 变化 |
|------|------|
| v0.0.x | 起点，包 KunModal，写死的工具条按钮散在四角 |
| v0.6.0 | 重写为 `<dialog>` + view-transitions + 滑动切图 + 旋转左右键 + 工具条重排（参考用户提供截图）+ dark mode 修复 + 下载 CORS fallback + 手势 bug 全清 + 新增 Gallery/Item 子组件 |
