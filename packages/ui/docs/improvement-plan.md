# @kun/ui 改进方案（v0.0.1 → v0.1.0）

> 范围：`packages/ui/app/components/kun/` 下所有 ~40 个组件 + `composables/` + `utils/`
> 目标消费方：kungal、moyu、wiki (kun-oauth-admin/apps/wiki)、oauth (kun-oauth-admin/apps/web)
> 撰写时间：2026-05-20

---

## 摘要

总体定位（自定义语义色 + variant × color 矩阵，HeroUI 风味）是对的，但**跨组件一致性 ~70%**，存在以下结构性问题：

1. **Tailwind 动态类拼接**：Tab / Progress 两处用 `` `bg-${color}` `` 这种字面量拼装，JIT 在生产构建下大概率不出色，是 latent bug。
2. **variant × color 颜色表重复了 4 份**：Button / Badge / Info / Progress 各写一份，已经在偏移。
3. **颜色体系泄漏**：Input / Textarea / Select 等组件混用 `text-red-500` 这类 Tailwind 固有色，违反 CLAUDE.md 规约。
4. **`KunUIColor` 类型缺 `info`**：实际 `Info.vue` 已经支持 7 色，但中央类型只列 6 色，下游 `<KunButton color="info">` 报 TS。
5. **Modal `modelValue` 拼成 `modalValue`**：靠 `v-model:modal-value` 显式绑定能 work，但默认 `v-model` 不行，违反约定。
6. **`KunLucide*` 自定义 SVG 包装组件**：项目已经接入 `@nuxt/icon`，再单独包 6 个 lucide 图标组件纯属冗余。
7. **`defineModel` 未采用**：所有组件还在手写 `modelValue` + `update:modelValue`，Slider 是唯一例外。机械化重构能让每个组件少 ~5 行。

外加用户已点名的两个具体诉求：

- **Tab 整体重做**：variant 死板、无动画、字号能改但 padding 不变所以"太大"、动态类拼接是 latent bug。
- **缺一个跨项目可复用的 TagInput**：项目里至少 4 处临时实现（kungal `pr/Alias.vue`、kungal `topic/MetadataEditor.vue` 等），UX 各有打磨缺失。

文档剩余部分按这个顺序展开：

- §1 跨切面问题（7 条系统性 issue + 修复策略）
- §2 按组件清单（严重度表）
- §3 各组件具体发现（只列"需要改"）
- §4 KunTab 重设计（5 variant + ASCII mockup）
- §5 KunTagInput 终稿（已经过四问四答的最终 spec）
- §6 落地路线图（3 批次）

---

## §1 跨切面问题

### 1.1 ⚠️ Tailwind 动态类拼接（Tab、Progress）

**症状**：
```ts
// tab/Tab.vue:68,78,81,132
isSelected ? `bg-${props.color} text-white` : `hover:text-${props.color}`
isSelected && `after:bg-${props.color}`

// progress/Progress.vue:88
`bg-gradient-to-r from-${props.color}-400 to-${props.color}-600`
```

Tailwind JIT 编译期扫源码字面量，**不识别** `` `bg-${color}` `` 这种 runtime 拼接。当前能"看似工作"是偶然——`bg-primary` 等类被 Button.vue / Badge.vue 静态写过，于是被打包到 CSS 里。一旦某个 variant 只在 Tab 出现，prod 就完全不出色。

**修法**：替换为 `Record<KunUIColor, string>` 静态映射（Button.vue:90-147 已经是这种写法，照抄）。

### 1.2 variant × color 颜色表重复 4 份

`Button.vue:90-147` ＋ `Badge.vue:39-96` ＋ `Info.vue:45-109` ＋ `Card.vue:39-47` 都自己写 variant×color → class 映射，且 **已经在偏移**：

| 文件 | success solid | warning solid | danger solid |
|---|---|---|---|
| Button.vue | `bg-success-600 dark:bg-success-300` | `bg-warning` | `bg-danger` |
| Badge.vue | `bg-success-600 dark:bg-success-300` | `bg-warning dark:bg-warning-300` | `bg-danger-600 dark:bg-danger-300` |
| Info.vue  | `bg-success-600` | `bg-warning` | `bg-danger-600` |

**修法**：抽到 `app/components/kun/ui/variants.ts`：

```ts
export const kunVariantClasses = (
  variant: KunUIVariant,
  color: KunUIColor
): string => kunVariantTable[variant][color]
```

Button / Badge / Tab / Chip / Alert / Info / Progress 全部消费同一个。改一处就一致。

### 1.3 颜色体系泄漏（违反 CLAUDE.md）

CLAUDE.md：「**不使用 Tailwind 固有颜色（gray、indigo、blue、green、red 等）**」。但当前：

| 文件 | 位置 | 问题 |
|---|---|---|
| Input.vue | 110, 132–134, 163 | `text-red-500` / `border-red-300` / `focus:border-red-500` / `focus:ring-red-500` / `text-red-600` |
| Select.vue | 124 | `bg-white dark:bg-black`（应用 `bg-content1`） |
| Popover.vue | 291 | `bg-white shadow-lg dark:bg-black`（同上） |
| Tooltip.vue | 98, 111 | `bg-white dark:bg-black` ×2 |
| DatePicker.vue | 226 | `bg-white dark:bg-black` |
| Pagination.vue | 157, 169 | `border-default-300` / `bg-primary` 拼裸 Tailwind |
| Loading.vue | 62 | `color: var(--color-white)` 内联 CSS |

**修法**：
- `red-*` → `danger-*`
- `bg-white dark:bg-black` → `bg-content1`（弹层）或 `bg-background`（页面级）
- Pagination 改用 KunInput + KunButton

### 1.4 `KunUIColor` 类型缺 `info`

`ui/type.d.ts:10-16` 只声明 6 色，但 `Info.vue` 已经把 `info` 加进自己的 `KunInfoColor` 类型并铺了一整套 variant × color 表。下游写 `<KunButton color="info">` 是 TS 报错。

**修法**：把 `info` 加进中央 `KunUIColor`，`Info.vue` 的本地 `KunInfoColor` 类型可以删。需要同步加颜色到 `tailwindcss.css` 的 info 50–950 调色板（如果还没加的话——CLAUDE.md 写了 info 是青色但我没看到 `bg-info-100` 之类的 token 定义，需要核对）。

### 1.5 `KunUISize` 在 Input/Tab 没被复用

```ts
// Input.vue:14
size?: string             // ← 弱类型

// tab/type.d.ts:16
export type KunTabSize = 'sm' | 'md' | 'lg'  // ← 自定义又小一档

// 中央：
export type KunUISize = 'xs' | 'sm' | 'md' | 'lg' | 'xl'
```

**修法**：全部统一成中央 `KunUISize`。

### 1.6 Modal 拼写成 `modalValue`

```ts
// packages/ui/app/components/kun/Modal.vue:7,24
modalValue: boolean
'update:modalValue': [value: boolean]
```

实际所有调用方都写 `v-model:modal-value="x"`（Vue 支持自定义 v-model 参数），所以**没坏**，但：

- 默认 `v-model="x"` 不工作
- 不符合 Vue 官方推荐的 `modelValue` 约定
- 命名歧义（modal **state** vs modal-as-noun）

**修法**：改成 `modelValue`，全仓 `v-model:modal-value` → `v-model`，破坏性但一次到位（仓内一共 12 个调用点，可机械替换）。

### 1.7 KunLucide* 与 KunIcon 重复

```
icon/LucideAlertTriangle.vue
icon/LucideCheckCircle2.vue
icon/LucideInfo.vue
icon/LucideX.vue
icon/LucideXCircle.vue
icon/Markdown.vue
```

每个都是 8–12 行内联 SVG。项目已经接入 `@nuxt/icon` (`<KunIcon name="lucide:check" />`)。

**修法**：删掉这 6 个文件，`alert/MessageItem.vue` 里改用 `<KunIcon name="lucide:..." />`。

### 1.8 `defineModel` 未采用

仓内只有 `Slider.vue:8` 用了 `defineModel<number>({ required: true })`，其他全部手写 `modelValue` + `update:modelValue` + watch 同步。

**修法**：迁移到 `defineModel`。每个组件可省 ~5 行 + 自动支持 modifiers (`v-model.trim` 之类)。

### 1.9 其他小型一致性问题

| 问题 | 位置 |
|---|---|
| `darkBorder` 默认 `true`、命名反人类 | Input, Textarea, Select, DatePicker, Card |
| z-index 散落硬编码（z-10 / z-50 / z-1007 / z-1100 / z-2000 / z-[7777]） | Select, Modal, ContextMenu, Alert, MessageContainer |
| 滚动锁直接写 `body.style`，不 ref-count | Modal.vue:28-36 |
| 全局 `document` click 监听 + `useEventListener` 在 watch 内 | Popover.vue:209-247 |

---

## §2 按组件清单（严重度）

| 组件 | 严重度 | 说明 |
|---|---|---|
| **Tab** | 🔴 重做 | 动态类 + variant 死板 + 无动画 |
| **Modal** | 🟠 改 | `modalValue` 拼写 + 滚动锁不 ref-count |
| **Input** | 🟠 改 | 红色硬编码 + size 弱类型 + 缺 clearable |
| **Textarea** | 🟡 微调 | 缺 color prop + autoGrow 已有但 charCounter 不计字素 |
| **Select** | 🟠 改 | 无键盘导航 + 无 multi + 无 search + bg-white/black |
| **Popover** | 🟠 改 | document click 泄漏 + 双 role + 应优先用 Tooltip 共享 floating 引擎 |
| **Tooltip** | 🟡 微调 | bg-white/black + 缺 delay + 缺 Teleport |
| **Pagination** | 🟠 改 | 全局 ArrowLeft/Right 监听 + 用裸 input 不用 KunInput |
| **CheckBox** | 🟠 拆 | `type='single'` 实际是 radio 语义 — 拆出 `KunRadio` |
| **Slider** | 🟡 微调 | 缺键盘 + 缺 disabled + 缺 step ARIA + 默认值 17/77 奇怪 |
| **Rating** | 🟡 微调 | hover-scale 引起 layout shift + 默认 icon 是 lollipop（可保留作 Easter egg 但需 `icon` prop 暴露） |
| **Switch** | 🟡 微调 | 缺 color prop + 缺 size |
| **DatePicker** | 🟠 改 | **month 显示 0-indexed bug** + 无年视图 + 范围选择圆角断裂 |
| **Badge** | 🟠 改名 | 当前实现是 Chip 语义；新建一个真正的 Badge（dot/count） |
| **Button** | 🟢 OK | 仅小瑕疵：icon prop + slot icon 双 API 冗余 |
| **Card** | 🟠 改 | `isPressable` 默认导航到 `/` 是隐藏坑；href/click 两种模式应分离 |
| **Upload** | 🟠 改名 | 实际是 ImageUpload，重命名 `KunImageUpload`，再写一个通用 `KunFileUpload` |
| **Avatar** | 🟡 微调 | `floatingPosition` / `disableFloating` props 声明了但**模板里没用**（死 props） |
| **AvatarGroup** | 🟡 微调 | `visibleCount` 与 `total` 联动逻辑反直觉 |
| **Header** | 🟡 微调 | slot 命名 `endContent` vs `headerEndContent` 二选一即可 |
| **Brand** | 🔴 抽象 | **硬编码 `kungal.titleShort` + "论坛" badge**，不能跨项目用；改成 props |
| **Info (top-level)** | 🟢 OK | 唯一 7 色 variant 表完整的组件；保留 + 抽到 §1.2 共享后简化 |
| **Alert (Info loli)** | 🟡 重命名 | 与 `info/Info.vue` 文件同名，但语义完全不同；建议 `alert/Loli.vue` |
| **Alert (Alert.vue)** | 🟢 OK | confirm dialog，简洁 |
| **MessageItem** | 🟡 微调 | 用 KunLucide* 包装组件 → 改用 KunIcon |
| **MessageContainer** | 🟢 OK | 6 锚点位置 + TransitionGroup，可保留 |
| **ContextMenu** | 🟡 微调 | 缺键盘上下导航 + z-1100 硬编码 |
| **Lightbox** | 🟢 OK | 405 行但功能扎实（pinch / wheel / 双击 / swipe），仅缺 alt-text on chevron buttons |
| **Link** | 🔴 bug | `<NuxtLink :is="tag">` 中 `tag` 在 props 里没声明 |
| **Loading** | 🟡 微调 | 192×192 loading 图过大 |
| **Image** | 🟢 OK | NuxtImg 薄包装 |
| **Native** | 🟢 OK | 原生 img 薄包装 |
| **Copy** | 🟢 OK | KunButton 薄包装 |
| **Divider** | 🟡 微调 | `withLabel` prop 声明但不参与渲染（用的是 `$slots.default`），二选一 |
| **Progress** | 🟠 改 | `gradient` variant 用动态类（§1.1） |
| **Ripple** | 🟢 OK | — |
| **Icon (KunIcon)** | 🟢 OK | `@nuxt/icon` 薄包装 |
| **icon/LucideX 等 6 个** | 🔴 删 | §1.7 |
| **icon/Favicon** | 🟡 改名 | 是 kungal 眼镜 SVG，应叫 `KunIconGlasses` 或 `KunIconLogo` |
| **icon/Markdown** | 🟢 OK | 单一 SVG |
| **animation/FadeCard** | 🟢 OK | 14 行 |
| **animation/GlassShatter** | 🔴 删/恢复 | **整个 `<script setup>` 都被注释掉了，渲染 `<span/>`**；要么恢复 gsap 实现，要么删除 |
| **scroll/Shadow** | 🟢 OK | scroll-shadow 实现扎实 |
| **content/Content** | 🟢 OK | DOMPurify + spoiler 合理 |
| **content/Text** | 🟡 微调 | 注入 U+200B 零宽空格——会破坏 copy-paste 路径；文档要写明 |
| **user/User** | 🟢 OK | 已有 null-safe `user?.name` |
| **Null** | 🟠 改名 | 改成 `KunEmpty` 或 `KunEmptyState` |
| **Rating** 默认 lollipop icon | 🟡 | 加 `icon` prop，默认仍可保留 lollipop |

---

## §3 各组件具体发现（仅列需要改的）

### 3.1 Tab — 见 §4 单独章节

### 3.2 Modal
- `modalValue` → `modelValue`（§1.6）
- 滚动锁直接 `document.body.style.overflow = 'hidden'`，**多个 Modal 嵌套时**内层关闭就解锁外层应该锁定的滚动。改为 `useScrollLock` (vueuse) 或自己 ref-count
- 没有 focus trap：Tab 键能漂出 Modal
- `z-1007` 硬编码 → CSS var

### 3.3 Input
- `text-red-500` / `border-red-300` / `focus:border-red-500` / `focus:ring-red-500` / `text-red-600` → `danger`（§1.3）
- `size?: string` → `KunUISize`（§1.5）
- `autofocus` 非响应式（只在 onMounted 跑一次），如果业务上需要重新 autofocus 没法做
- 缺：`clearable` prop（很常见的需求，目前所有调用方都自己加 × 按钮）
- `aria-describedby` 没接到 helperText / error，屏幕阅读器读不到错误文案

### 3.4 Textarea
- 与 Input 同款颜色泄漏问题（line 131 `focus:ring-primary-500` 硬编码）
- `placeholder` 默认是大段日文台词，跨项目使用会很怪：应该改成 `''`，台词作为 kungal 业务层的 default
- `showCharCount` 用 `localValue.length` 计 UTF-16 code unit 数；输入 emoji 会数错（"🐱".length === 2）。要么文档说明，要么用 `Intl.Segmenter`
- 缺 `color` prop（焦点 ring 颜色无法配置）

### 3.5 Select
- 缺键盘上下/回车选中
- 缺搜索过滤（项目里 series/tag 选择都需要）
- 缺 multi 模式（tag_ids / official_ids 编辑场景需要）
- `bg-white dark:bg-black`（§1.3）
- dropdown 在 Modal 内会被压住（z-10 < Modal 的 z-1007），需要 Teleport
- `defineModel`（§1.8）

### 3.6 Popover
- `useEventListener(document, 'click', close)` 是全局监听，每个 Popover 实例都会响应整个文档的 click。应该收敛到 `containerRef`，或用 `onClickOutside`
- `useEventListener` 在 `watch` 回调内调用，每次 open 都注册新 listener，关闭后没销毁
- trigger 容器自带 `tabindex=0 role="button"`，包了一个用户的 button 就是 nested button + 双 role
- 与 Tooltip 应共享 floating 引擎（建议引入 `@floating-ui/vue`），不要双实现
- `bg-white shadow-lg dark:bg-black`（§1.3）

### 3.7 Tooltip
- `hidden sm:block` 默认在 mobile 完全不显示，应该改成 prop
- 缺 `delay-show` / `delay-hide`（hover 瞬间跳出体验差）
- 没有 Teleport，父级 `overflow: hidden` 会裁掉
- `bg-white dark:bg-black`（§1.3）

### 3.8 Pagination
- `onKeyStroke('ArrowLeft', ...)` 是**全局**键盘监听，用户在任意输入框按 ← → 都会翻页。应该 scope 到组件或加 `if (document.activeElement is in form)` 排除
- `watch(props.isLoading)` 完成后自动 `window.scrollTo(0)` —— 这种副作用应该是消费者的事，不该在 UI 组件里
- 翻页输入框用裸 `<input type="number">`，没用 KunInput
- 底部 `跳转` 按钮额外加了一层 `bg-primary text-white` 内联类，把 KunButton 的 variant 系统旁路了

### 3.9 CheckBox
- `type='single'` 时实际渲染圆形 + 单选语义，但 inner `<input type="checkbox">` 仍是 checkbox。这种"伪 radio"**没有**单选互斥效果（同名 group 不会自动取消其他）
- **应拆**：保留 `KunCheckBox`（多选），新增 `KunRadio` + `KunRadioGroup`（单选 + name binding）

### 3.10 Slider
- 没有键盘 ←/→ 调整
- 没有 `disabled` prop（拖拽事件无 guard）
- `aria-valuetext` 缺失（屏幕阅读器只读得到数字，不知含义）
- 默认 `min: 17, max: 77` 奇怪——是 kungal 内梗（17岁、77岁）但作为 UI 组件默认不合理。改成 `0..100`
- 不支持双 thumb 范围

### 3.11 Rating
- `hover:scale-110` 会让星星变大引起后续元素位移；改成 transform-origin 居中 + 用 inline-block + fixed 宽度
- 默认 icon 是 `lucide:lollipop`（棒棒糖）——很萌但作为 UI 组件默认偏怪。建议：
  - 默认 `lucide:star`
  - 加 `icon` prop，kungal 业务层显式传 `lollipop`
- 没有半星精度

### 3.12 Switch
- 整个组件硬编码 `bg-primary-500`，不支持 `color` prop
- 缺 size prop

### 3.13 DatePicker（**有 bug**）
- **line 256**：`{{ viewingDate.getMonth() }}` 显示 0-indexed 月份 —— 12 月显示 "11"，1 月显示 "0"。应该 `+ 1` 或用 `i18n.months[idx]`
- 无年视图：选 1970 年要点 50+ 次 ←
- `keydown.prevent.capture` 阻止 default —— 如果未来在 picker 内放搜索框（按年/月跳转），输入会被吞
- 范围选择起止天的圆角处理（line 309-310）会让首末日出现 `rounded-r-none` 但中间天是 `rounded-none`，端点视觉上断裂

### 3.14 Badge / Chip 命名
- 当前 `KunBadge` 实际是行业惯例里的 "Chip"（带 hover/× 的标签胶囊）
- 真正的 Badge 是右上角红点 / 数字（`<KunBadge dot>` / `<KunBadge count="9+">`），项目内没有
- **建议**：
  - 把 `Badge.vue` rename 为 `Chip.vue`，导出 `KunChip`
  - 新建 `Badge.vue`，导出真正的 Badge（dot / count overlay）

### 3.15 Button
- `icon` prop（开关）+ `<slot name="icon" />` + 内嵌 `<KunIcon v-if="loading" />` 三套 icon 入口让人选择困难
- 建议统一：只保留 `<slot name="leading" />` / `<slot name="trailing" />`，删掉 `icon` boolean + `iconPosition` prop
- 其他 OK

### 3.16 Card
- 默认 `href: '/'`：消费方写 `<KunCard isPressable>` 没传 href，点一下跳首页。**隐性坑**
- `isPressable` 同时承担"可点击"和"渲染为 NuxtLink"两件事——这两件应该分离：可点击 → button，可跳转 → link
- 建议拆 `clickable: boolean`（emit click）+ `href?: string`（is NuxtLink），互斥

### 3.17 Upload
- 实际是图片上传 + 裁剪 + resize → webp，**不是通用文件上传**。重命名 `KunImageUpload`
- 硬编码 `image/jpeg/png/webp` accept、`image/webp` 输出 quality 0.77。所有这些都应该是 props
- `size` prop 名字误导（看起来像 UI size 实际是裁剪目标尺寸）→ rename `targetSize` 或 `pixelSize`
- 缺多文件 / 拖拽列表 / 上传队列等通用功能；那是另一个 `KunFileUpload` 的活

### 3.18 Avatar
- `floatingPosition` / `disableFloating` 在 `props` 声明并 `withDefaults` 给了默认值，但**模板里没引用**。死 props，删
- `cursor-pointer` 总是开，即使 `isNavigation=false`。条件化
- `userAvatarSrc` 的 `.replace(/\.webp$/, '-100.webp')` 假定 CDN 文件名约定，kungal-only，不通用

### 3.19 Brand（**严重，跨项目阻塞**）
```vue
<template>
  <div ...>
    <KunImage src="/favicon.webp" :alt="kungal.titleShort" />
    <span class="text-xl">{{ kungal.name }}</span>
    <KunBadge size="md" color="primary">论坛</KunBadge>
  </div>
</template>
```
全部硬编码：站名、favicon 路径、"论坛" badge 文案。这是 kungal 业务组件，**不能放在 @kun/ui**。

**修法**：
- 抽 props：`<KunBrand :name :badge :icon-src :to />`
- 当前文件作为示例放进 `apps/web/components/`，从 @kun/ui 移走

### 3.20 Alert / Info 文件同名歧义
- `kun/Info.vue`（顶层）= 信息卡片（用 variant 矩阵渲染 icon + 标题 + 描述的横向 Banner）
- `kun/info/Info.vue` = 同上但更厚（实际两个文件功能重叠！需要 diff）
- `kun/alert/Info.vue` = loli 彩蛋 Toast

**修法**：
- 比较 `kun/Info.vue` vs `kun/info/Info.vue`，合并成一个
- `kun/alert/Info.vue` rename `kun/alert/Loli.vue`

### 3.21 Link
```vue
<NuxtLink :is="tag" ...>
```
`tag` 在 props 里没声明、没 default、没类型。要么补 prop，要么删 `:is="tag"`（既然走 NuxtLink 就不需要 `:is`）

### 3.22 Loading
- 192×192 的 kun.webp 当 loading 占位太大，应该 96 或更小
- `<KunImageNative>` 不会有 lazy/blur，loading 状态本身就是要立刻显示的，OK

### 3.23 Divider
- `withLabel` prop 声明但模板里逻辑用的是 `v-if="$slots.default"`。删 prop，统一用 slot

### 3.24 Progress
- `gradient` variant 用动态类拼装（§1.1）
- `circle` variant 的 `<circle :class="colorClasses[color]">` 用 className 给 SVG 上色没用，要用 `:stroke` 属性

### 3.25 content/Text
- 注入 `​` 零宽空格在 `_` / `/` 后让长 URL 可断行。但用户复制粘贴会带上零宽——比如复制路径粘到终端会失败
- 至少加注释和 docs；理想是 `breakable` prop 默认 false（业务层显式启用）

### 3.26 ContextMenu
- 缺键盘上下导航（应该 ↑↓ 选中、Enter 触发、Esc 关）
- z-1100 硬编码 → CSS var

### 3.27 Lightbox
- alt 缺失：左/右/下载/关闭按钮都没 aria-label（line 333-377）
- 其余 OK

### 3.28 animation/GlassShatter
- **整个 `<script setup>` 被多行注释包起来了**（line 1-188 全是 block comment），实际渲染 `<span />`。
- 要么恢复（需要 `gsap` 依赖；当前 `package.json` 没有），要么删除
- 看上去是早期实验

### 3.29 KunLucide* + 重复 SVG 包装（§1.7）

删除：`LucideAlertTriangle.vue` / `LucideCheckCircle2.vue` / `LucideInfo.vue` / `LucideX.vue` / `LucideXCircle.vue`

`alert/MessageItem.vue` 改用 `KunIcon name="lucide:..."`。

---

## §4 KunTab 重设计

### 4.1 问题回顾

1. 动态类 `bg-${color}` — JIT 失效（§1.1）
2. 只有 `solid`、`underlined` 2 个 variant
3. 没有滑动指示器动画
4. `size` 只改文字大小不改 padding，所以"sm 看起来还是太大"
5. `overflow-scroll` 总是开，短列表也出现潜在滚动
6. 没有键盘 ←/→ 在 tabs 间导航（只能 Enter/Space 触发当前焦点）
7. 用旧的 `modelValue` + `update:modelValue` 模式

### 4.2 新 API

```ts
type KunTabVariant =
  | 'underlined'   // 默认；底部一条会滑动的线
  | 'solid'        // 当前选中填色 chip
  | 'bordered'     // 整体外框 + 单 tab 选中描边
  | 'light'        // 选中态淡色背景（最轻量）
  | 'pills'        // 圆角胶囊（顶部 nav 常见）

type KunTabOrientation = 'horizontal' | 'vertical'
type KunTabSize = 'sm' | 'md' | 'lg'   // 真改 padding，不只改文字

interface KunTabProps {
  items: KunTabItem[]
  variant?: KunTabVariant       // 默认 'underlined'
  color?: KunUIColor            // 默认 'primary'
  size?: KunTabSize             // 默认 'md'
  orientation?: KunTabOrientation  // 默认 'horizontal'
  fullWidth?: boolean
  disabled?: boolean
  disableAnimation?: boolean
  scrollable?: boolean          // 默认 false；true 时溢出可滑动
  className?: string
}

const value = defineModel<string>({ required: true })
```

### 4.3 视觉 ASCII mockup

```
size=sm padding x=10 y=6,  size=md padding x=12 y=8,  size=lg padding x=16 y=10

variant='underlined' (默认):
┌──────┬──────┬──────┐
│ Tab1 │ Tab2*│ Tab3 │  ← selected 文字色 = primary
└──────┴══════┴──────┘  ← 底部一条 2px primary 线
                          滑动指示器随选中 tab 的 offsetLeft/offsetWidth transition

variant='solid':
┌──────┬─░░░░░┬──────┐
│ Tab1 │ Tab2 │ Tab3 │  ← Tab2 整体 bg-primary + text-white
└──────┴─░░░░░┴──────┘    其他 hover:text-primary

variant='bordered':
╔══════════════════════╗
║ Tab1 │┌Tab2*┐│ Tab3  ║  ← 外框包裹整体，选中 tab 单独描边
╚══════════════════════╝

variant='light':
                          
  Tab1   ░Tab2*░   Tab3   ← 选中态 bg-primary/20 圆角
                          

variant='pills':
                          
 ╭──╮  ╭══════╮  ╭──╮     ← 每个 tab 独立胶囊，选中填色
 │T1│  │ T2* │  │T3│      
 ╰──╯  ╰══════╯  ╰──╯     

orientation='vertical' (左侧 nav 常见):
┌──────┐
│ Tab1 │
├──────┤
│ Tab2*│←  指示器在右侧（horizontal 在底部）
├──────┤
│ Tab3 │
└──────┘
```

### 4.4 滑动指示器实现

```ts
const indicatorStyle = computed(() => {
  const el = tabRefs.value[currentIndex]
  if (!el) return { width: '0', transform: 'translateX(0)' }
  return {
    width: `${el.offsetWidth}px`,
    transform: `translateX(${el.offsetLeft}px)`,
    transition: disableAnimation ? 'none' : 'all .25s cubic-bezier(.4,0,.2,1)'
  }
})
```

只有 `underlined` / `solid` / `light` / `pills` 用绝对定位的指示器层。`bordered` 单 tab 描边自己 transition border 颜色即可。

### 4.5 键盘导航

```
horizontal: ← / →  循环；Home / End 跳首末
vertical:   ↑ / ↓  循环；Home / End 跳首末
任何方向:    Enter / Space 触发（如果 type=manual）；arrow 切换即触发（如果 type=auto，默认）
```

### 4.6 静态颜色映射（替代动态拼接）

```ts
const indicatorBg: Record<KunUIColor, string> = {
  default: 'bg-default',
  primary: 'bg-primary',
  secondary: 'bg-secondary',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-info',
}

const tabTextActive: Record<KunUIColor, string> = {
  default: 'text-default-700',
  primary: 'text-primary',
  /* … */
}
```

实际生产中复用 §1.2 抽出的 `kunVariantClasses()`。

---

## §5 KunTagInput 终稿 spec

> 这是上一轮四问四答后定下来的最终 spec，落地位置 `packages/ui/app/components/kun/tag-input/`。

### 5.1 文件结构

```
packages/ui/app/components/kun/tag-input/
├── TagInput.vue
└── type.d.ts
```

### 5.2 Props

```ts
const tags = defineModel<string[]>({ default: () => [] })

interface KunTagInputProps {
  label?: string
  placeholder?: string
  helperText?: string
  error?: string

  // 限制
  maxTags?: number              // 默认 Infinity
  maxTagLength?: number         // 默认 100
  minTagLength?: number         // 默认 1

  // 去重 / 规范化
  allowDuplicates?: boolean     // 默认 false（去重，case-insensitive）
  caseSensitive?: boolean       // 默认 false（dedupe 时 lowercase 比较）
  trim?: boolean                // 默认 true
  transform?: (raw: string) => string
  validate?: (tag: string, all: string[]) => true | string

  // 分隔符
  delimiters?: string[]         // 默认 ['Enter']（键盘提交键）
  splitChars?: (string | RegExp)[]  // 默认 ['\n', ',', '，', ';']
  splitOnPaste?: boolean        // 默认 true

  // 行为
  confirmOnBlur?: boolean       // 默认 true
  respectComposition?: boolean  // 默认 true（IME composition 中 Enter 不提交）

  // 样式（沿用 KunUI）
  color?: KunUIColor            // 默认 'primary'
  size?: KunUISize              // 默认 'md'
  variant?: 'bordered' | 'flat' // 默认 'bordered'
  disabled?: boolean
  readonly?: boolean
  showCounter?: boolean         // 默认 false；显示 {tags.length}/{maxTags}
  className?: string
}
```

### 5.3 Emits

```ts
defineEmits<{
  add: [tag: string]
  remove: [tag: string, index: number]
  invalid: [reason: 'duplicate' | 'too-long' | 'too-short' | 'max-reached' | 'custom', raw: string, detail?: string]
}>()
```

### 5.4 Slots

```ts
defineSlots<{
  tag(props: { tag: string; index: number; remove: () => void }): any
}>()
```

### 5.5 行为表

| 触发 | 行为 |
|---|---|
| `Enter`（非 IME composition） | commit 当前 input；按 `splitChars` 拆，逐个 add |
| `Enter`（IME composition 中） | 不提交（compositionend 后才允许） |
| `Backspace`（input 非空） | 普通退格 |
| `Backspace`（input 空，第一次） | `canDelete` 置 true，**不删**（防误删） |
| `Backspace`（input 空，第二次） | 删最后一个 tag |
| 任意字符输入 | `canDelete` 重置 false |
| `←` / `→`（input 空） | 焦点跳到前一个 / 后一个 chip，chip 上 Enter/Delete 删 |
| paste | 按 `splitChars` 拆，逐个 add（去重/校验照走） |
| blur（`confirmOnBlur`） | pending input 当作一次 commit |
| click 容器空白 | focus input |
| `disabled` | input + chip × 都禁用 + `aria-disabled` |
| `readonly` | 显示 chips 但无 × 也无 input |

### 5.6 a11y

- 容器 `role="group"` + `aria-label={label}`
- 每个 chip `role="listitem"`
- × 按钮 `aria-label="移除标签 {tag}"`
- input `aria-describedby` 接 helper/error
- 容器 `aria-invalid` 跟随 `error`

### 5.7 视觉

- 外框沿用 Input 同款：`border border-default-200 rounded-lg`
- focus-within：`ring-2 ring-{color}/40 border-{color}`（用 §1.2 静态映射表）
- chip 沿用 KunBadge → 重命名后的 KunChip，`size` 跟随容器 `size`
- 容器 `min-h` 跟随 `size`：sm=36、md=42、lg=48
- chips wrap 后容器自动长高
- 错误态：边框 `border-danger`，下方 `text-danger text-sm`
- counter（可选）：右下角 `text-default-400 text-xs`

### 5.8 测试覆盖（Vitest）

```
✓ 加 tag（Enter）
✓ 删 tag（× 按钮 / Backspace 两段式）
✓ IME composition 期 Enter 不提交
✓ 粘贴 4 种分隔符（\n / , / ， / ;）正确拆
✓ 去重 case-insensitive
✓ 去重 case-sensitive（caseSensitive=true）
✓ maxTags 达上限 → emit invalid + 拒绝
✓ maxTagLength 超限 → emit invalid + 拒绝
✓ validate 返回 string → emit invalid + 拒绝
✓ transform 规范化（如 toLowerCase）生效
✓ confirmOnBlur=true 时 blur commit pending input
✓ confirmOnBlur=false 时 blur 丢弃
✓ tag slot 自定义渲染替换默认 chip
✓ readonly 模式无 × 无 input
✓ disabled 模式键盘和点击都失效
```

---

## §6 落地路线图

### 批 1（修 latent bug + 颜色泄漏，不改 API；1-2 天）— ✅ 2026-05-20 落地

- [x] §1.1 Tab 动态类替换为静态映射表（实际改成完整重做，见批 2 §4）
- [x] §1.1 Progress.vue gradient variant 同款修复（改成静态 `gradientClasses` map）
- [x] §1.3 颜色泄漏：Input / Textarea / Select / Popover / Tooltip / DatePicker / Pagination 全部 red → danger，bg-white/black → bg-content1
- [x] §3.13 DatePicker `getMonth()` 0-indexed bug
- [x] §3.18 Avatar 死 props 行为修正（接口保留 backward-compat，模板里 ignore）
- [x] §3.21 Link `:is="tag"` 修复（删 `:is` 属性 + 从 KunLinkProps 删 `tag` 字段，NuxtLink 自动对外链 fall back 到 `<a>`）
- [x] §3.28 GlassShatter 删除（注释掉的代码无 gsap 依赖）

**风险**：低，全部是内部实现修正，调用方零感知。**实际验证**：`pnpm -F web exec nuxt build` + `pnpm -F wiki exec nuxt build` 全通过。

### 批 2（KunUI v0.1.0，API breaking；3-5 天）— ✅ 2026-05-20 落地

- [x] §1.2 抽 `kunVariantClasses` 共享映射（`packages/ui/app/components/kun/ui/variants.ts`）+ `kunBgClasses` / `kunTextClasses` / `kunBorderClasses` / `kunRingClasses` 细分静态 map
- [x] §1.4 `info` 加进中央 `KunUIColor`（tailwindcss.css 已有完整 50–950 + dark mode 调色板，无需补）
- [x] §1.5 size 类型统一 `KunUISize`（Input 已迁移；Tab 自己的 `KunTabSize` 保留为 3 档 sm/md/lg）
- [x] §1.6 Modal `modalValue` → `modelValue` + 仓内 20 个调用点一并改（apps/web 8 处 + apps/wiki 9 处 + packages/ui 内部 3 处）
- [x] §1.7 删 5 个 KunLucide*（AlertTriangle/CheckCircle2/Info/X/XCircle）+ MessageItem + alert/Loli 全部改用 `KunIcon name="lucide:..."`
- [x] §1.8 `defineModel` 迁移：Input / Textarea / Select / Switch / CheckBox / Rating / Tab / Modal（Slider 之前已迁）
- [x] **§4 KunTab 重做**：5 variant（underlined/solid/bordered/light/pills）+ 滑动指示器 + horizontal/vertical + 键盘 ←→ Home/End + 全静态颜色映射
- [x] **§5 KunTagInput 新建**：`tag-input/TagInput.vue` + `type.d.ts`，IME composition 屏蔽、4 种粘贴分隔符、双段式 Backspace、← chip 导航、`tag` slot、KunRing 焦点环
- [x] §3.14 Badge → Chip 重命名：`Chip.vue` + 8 处调用点改为 `KunChip`；新建真正的 `Badge.vue`（dot/count overlay）
- [x] §3.16 Card 拆 `clickable` vs `href`：移除 `isPressable` + `href: '/'` 默认，新增 `clickable: boolean` + `@click` emit
- [x] §3.19 Brand 抽 props：`name` / `iconSrc` / `badge` / `badgeColor` / `to` 全部可配置
- [x] §3.20 alert/Info → alert/Loli + 同时把 `alert/loli.ts` 改名 `loliAssets.ts` 避免 Nuxt 自动 import 命名冲突

**风险**：中高，10+ 处 API 变更，但 v0.0.1 → v0.1.0 是合理时机。**实际验证**：build 通过，无 type error 也无 component-name 冲突警告。

### kungal / moyu 同步检查清单

下游消费方拿到这份文档后需要在自己仓内做的事（按本批次的 breaking 列）：

| 改动 | grep / sed 关键词 | 替换为 |
|---|---|---|
| Modal | `v-model:modal-value` / `modal-value=` / `update:modal-value` | `v-model` / `model-value=` / `update:model-value` |
| Badge → Chip | `KunBadge` | `KunChip`（dot/count 用例改用新 `KunBadge`） |
| Card | `is-pressable` | `clickable` 或 `:href`（确认 intent 后选其一） |
| Brand | `<KunBrand />` 无参用法 | 显式传 `name` / `icon-src` / `badge` |
| KunLucide* | `<KunIconLucideX>` / `<KunIconLucideAlertTriangle>` 等 | `<KunIcon name="lucide:x">` / `<KunIcon name="lucide:alert-triangle">` |
| alert/Loli | `useKunLoliInfo` 调用 | 无需改（composable 名字不变；内部组件改名透明） |

Tab / TagInput 是新组件 + 新 API，下游按 §4 / §5 spec 实现即可，没有"旧的要替换"。

---

## §8 v0.1.1 复审后加固（2026-05-21）

上线前用 `pr-review-toolkit` 的 4 个 agent（code-reviewer / silent-failure-hunter / type-design-analyzer / comment-analyzer）对 v0.1.0 commit 跑了一轮深度复审。共发现 6 个 🔴 + 8 个 🟠，全部已修。

| # | Severity | 改动 |
|---|---|---|
| 1 | 🔴 | **Modal scroll-lock 单实例 bug**：`let scrollLockCount = 0` 写在 `<script setup>` 内是**每实例**而非模块级，注释里写的"shared across instances"是错的。嵌套 Modal 时内层关闭仍会解锁 body。修复：抽到 `app/composables/useBodyScrollLock.ts` 单例 composable，并加 HMR dispose 避免热重载残留正数 |
| 2 | 🔴 | **Avatar 对 null user 崩溃**：`KunAvatarProps.user: KunUser` 不可空，但 User.vue 注释撒谎说"Avatar 已 tolerate null"。修：type 改 `KunUser \| null \| undefined`，Avatar.vue 用 `user?.avatar` 收敛 + 空 user 时用 `getRandomSticker('')` fallback |
| 3 | 🔴 | **Select 类型有"幽灵 modelValue"**：`KunSelectProps.modelValue` 仍声明，但 Select.vue 用 `Omit` 屏蔽。修：从 type 直接删 `modelValue`，移除 Omit |
| 4 | 🔴 | **TagInput 长按 Backspace 删光所有标签**：第一次 keydown 设 `canDelete=true`，OS repeat 立刻第二次 keydown 命中删除。修：onKeydown 里加 `if (e.repeat) return` 短路 |
| 5 | 🔴 | **TagInput chipRefs 残留 stale DOM**：删 tag 后 chipRefs 数组不缩，残留 stale 节点。修：`removeAt` 里 `chipRefs.value.length = tags.value.length` 截短 |
| 6 | 🔴 | **Lucide icon 名 deprecated alias**：`lucide:check-circle-2` / `x-circle` / `alert-triangle` 都是 iconify alias，上游可能去掉。修：换 canonical `circle-check` / `circle-x` / `triangle-alert` |
| 7 | 🟠 | **Tab moveFocus 在 currentIndex=-1 时 ArrowLeft wrap 错**：`(-1 - 1 + n) % n = n - 2` 而非末尾。修：当 cur<0 直接 `enabled[delta > 0 ? 0 : enabled.length - 1]` |
| 8 | 🟠 | **Tab setTabRef 静默丢弃非 HTMLElement**：fragment ref 没人知道。修：dev 模式 `console.warn` |
| 9 | 🟠 | **TagInput splitChars 空字符串导致逐字符拆**：`['']` → `new RegExp('')` 匹配每个空隙。修：过滤空 delimiter |
| 10 | 🟠 | **TagInput splitChars regex 没 wrap 成 non-capturing**：带 capturing group 的 RegExp 会让 `String.split` 把 capture 也插进结果。修：`(?:${d.source})` 包装 |
| 11 | 🟠 | **TagInput chip × 加倍 Tab 停留点**：10 个 tag 要 20 个 Tab 停留。修：× 设 `tabindex="-1"`（键盘走 chip 的 Backspace/Delete，× 留给鼠标） |
| 12 | 🟠 | **Modal HMR scrollLockCount 残留**：dev 热重载后 count 可能滞留正数。修：composable 内 `import.meta.hot.dispose()` 重置 |
| 13 | 🟠 | **Pagination ← → 偷 focused KunTab/listbox 的键**：原 `isEditableTarget` 只查 INPUT/TEXTAREA/SELECT。修：加 ARIA role 黑名单（tab/option/menuitem/slider/spinbutton/combobox/tree/treeitem） |
| 14 | 🟠 | **Progress 圆环 variant `stroke="currentColor"` + `bg-*` class 颜色不出**：`bg-primary` 不设 `color`，圆环走 inherited foreground。修：加 `strokeColorClasses` 静态 map（text-* 系列），用它 |

**新增文件**
- `packages/ui/app/composables/useBodyScrollLock.ts` —— 跨组件共享的 body scroll-lock 单例 + HMR dispose

**清理（comment-analyzer 提议）**
- 删除 8 处迁移叙事注释（Chip / Badge / Card / Brand / Link / Avatar / Tab type / TagInput "legacy Alias.vue"），改成只描述当前行为
- 删除 Avatar.vue 两段 HTML 死代码注释
- 删除 Textarea.vue 两行 `// readonly ? ...` `// error ? ...` commented-out class
- 删除 Tab.vue / TagInput.vue 几条纯 section-label 注释（"// Keyboard navigation."、"// Backspace on empty input — two-stage delete." 这种重复代码本意的）

**验证**：`pnpm -F web exec nuxt build` + `pnpm -F wiki exec nuxt build` 全过，grep 残留扫描全清。

**未采纳的建议（agent 提了但保留判断不改）**
- type-design #18：CheckBox `defineModel({ default: false })` 改 `required: true` —— Checkbox 在表单里经常是单独 v-model 可省略的（"默认勾"或"默认不勾"场景多），保留 default。
- type-design #19：`KunTagInputVariant = Extract<KunUIVariant, 'bordered' | 'flat'>` —— 不值，独立 type 字面量更清晰。
- type-design #20：`Card.color = 'background'` 改 'surface' —— 跨调用点 breaking，价值低，留待 v0.2。
- silent-failure #15：TagInput `transform → ''` 静默吞 invalid —— 边缘场景，加 emit 又会让"用户主动 normalize 成空"也报错。文档明确即可。
- silent-failure #16：Progress unknown color dev warn —— TS `KunUIColor` 已经强约束，dev warn 反而干扰。

下游（kungal / moyu）拿到本节后，照 §7 同步清单 + v0.1.1 这 14 条按需对照。Modal 的 `useBodyScrollLock` 是 @kun/ui 的新 composable，下游不需要单独导入（消费 KunModal 就能享受到，自己写 Modal 的话才需要 import）。

### 批 3（功能增强 + 体验打磨；持续）

- [ ] §3.5 Select 加键盘导航 + search filter + multi 模式
- [ ] §3.9 拆 KunRadio + KunRadioGroup
- [ ] §3.10 Slider 加键盘 + disabled + 双 thumb 范围 + 改默认值
- [ ] §3.13 DatePicker 加年视图、范围圆角修复、Picker 内 keypress 不再 `prevent.capture`
- [ ] §3.17 Upload rename + 暴露 accept/output prop + 新建 KunFileUpload
- [ ] §3.6/§3.7 引入 `@floating-ui/vue`，Popover / Tooltip 共享定位引擎
- [ ] §3.8 Pagination 全局键盘事件改 scoped + 删 scrollTo 副作用
- [ ] §3.2 Modal focus trap + scroll lock ref-count
- [ ] §1.9 z-index 全部收编到 CSS var

**风险**：低，全部是增量。

---

## §7 Open questions（无）

上一轮已经回答的 4 个 TagInput 问题：

| 问题 | 决议 |
|---|---|
| IME composition 行为 | 屏蔽 IME 中 Enter |
| 自动补全弹层 | 不做进同一个组件，留 slot |
| 粘贴分隔符 | `\n` + `,` + `，` + `;` |
| 对外发版位置 | @kun/ui / `KunTagInput` |

Tab / 其他组件的具体决策点请在批 2 启动前再 confirm 一次（特别是 Modal 的 `modelValue` 重命名 + Badge → Chip 改名带来的 breaking）。

---

*文档维护：每次完成批次后更新对应章节的 checkbox。新增组件评审进 §2 表 + §3 新章节。*
