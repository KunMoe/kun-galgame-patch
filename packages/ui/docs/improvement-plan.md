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

---

## §9 v0.2.0 浮层引擎统一 + a11y 工具链（2026-05-21）

新增 4 个 npm 依赖，把项目里"手写浮层定位"的 4 处全部收口到 `@floating-ui/vue`，并加 focus-trap + a11y lint + typecheck 三件套。

### 新依赖

| 包 | 装在哪 | 用途 |
|---|---|---|
| `@floating-ui/vue` | `@kun/ui` runtime | Popover / Tooltip / Select / DatePicker 的浮层定位 |
| `@vueuse/integrations` + `focus-trap` | `@kun/ui` runtime | Modal / Lightbox 的键盘焦点陷阱 |
| `eslint-plugin-vuejs-accessibility` | root devDep + apps/web + apps/wiki eslint 配置 | 静态 a11y 检查（防止下次又出"按钮嵌套按钮"这种） |
| `vue-tsc` | root devDep | 真正的 .vue + .ts 类型检查（之前 typecheck 只跑 nuxt prepare） |

### 浮层定位（4 个组件）

**Popover.vue**：手写 220+ 行的 `computePosition` / `candidatesFor` / `coordsFor` / `ResizeObserver` → 用 `useFloating()` 一行，加 `offset(8)` + `flip()` + `shift({padding:8})` middleware。
- 旧实现的 `useEventListener` 嵌在 `watch` 里，每次开都注册新 listener 不销毁 —— **泄漏 bug 自动消除**
- 旧实现监听整个 `document` click —— 改用 `onClickOutside` 收敛到 trigger + popover 两个元素
- 加 `Teleport to="body"` —— 父级 `overflow: hidden` 不会再裁掉浮层
- `defineExpose` 暴露 `open() / close() / toggle()` 给父组件命令式控制

**Tooltip.vue**：从 119 行精简 + **功能增强**
- 加 `delayShow` / `delayHide` props（默认 100ms / 0ms）—— 防止鼠标快速划过闪烁
- 加 `arrow()` middleware + `arrowStyles` computed —— 真正的箭头跟随飞行边
- 加 `Teleport` —— 同上原因
- `hideOnMobile` prop 可关 —— 之前 `hidden sm:block` 写死

**Select.vue**：浮层用 `size()` middleware：
- 自动让下拉宽度 = 触发按钮宽度（旧版用 `w-full` 但不准）
- 自动 `maxHeight = Math.min(240, availableHeight - 8)` —— 视口空间不足时自动缩小，列表内部滚动
- Modal 内的 Select 不再被 Modal 的 z-1007 压住（Teleport + `z-50`）

**DatePicker.vue**：去掉手写的 `showAbove` 逻辑（用 vueuse `useElementBounding` + 自己算可用空间），改 `useFloating` 自动 flip。删除 scoped style 里的 `fadeUp`/`fadeDown` 类（不再被引用）。

### Focus trap

**Modal.vue** 加 `useFocusTrap` from `@vueuse/integrations`：
- Tab / Shift+Tab 不会再漂出 Modal
- 关闭时自动 restore focus 到打开前的元素
- `escapeDeactivates: false` —— Modal 自己处理 Escape，不让 trap 抢
- `allowOutsideClick: true` —— 允许背景点击关闭（Modal 的 `isDismissable` 仍然生效）
- 嵌套 Modal：内层 activate 时外层自动 deactivate；内层 close 后焦点回外层（trap 库本身的栈语义）

Lightbox 通过 KunModal 自动继承 trap，无需单独改。

### ESLint a11y

`apps/web/eslint.config.mjs` + `apps/wiki/eslint.config.mjs` 都接入 `eslint-plugin-vuejs-accessibility` 的 `flat/recommended` config。

少数规则降级到 warn 或关闭：
- `no-autofocus`: off（Input.vue 有 autofocus prop 是合理设计）
- `click-events-have-key-events` / `no-static-element-interactions`: warn（不少地方 KunCard 等已经通过 button/link 渲染，但 lint 静态看不出）

### vue-tsc typecheck

加 `typecheck` script：
- root `package.json`: `"typecheck": "pnpm -F \"./apps/**\" --parallel run typecheck"`
- `apps/web/package.json` + `apps/wiki/package.json`: `"typecheck": "nuxt typecheck"`

Nuxt 自带的 `nuxt typecheck` 命令在装了 `vue-tsc` 后能实际跑类型检查，覆盖 .vue 模板里的 prop 类型 / defineModel 推断错误。建议 CI 加 `pnpm typecheck` 阻塞合并。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 浮层手测：Popover / Tooltip / Select / DatePicker 在普通页面 + Modal 内部都能正确 flip / shift / size，不被父级 overflow 裁切
- focus trap 手测：Modal 内 Tab 转一圈仍在 Modal 内；关闭后焦点回到打开前的按钮

### v0.2.1 — 下游报回的 4 个 bug（2026-05-21）

来自 moyu 接入侧的 4 条具体 bug，全部已验证为真并修复。修复方法**与报告者建议不完全一致**，下面把每条的实际修法和被否定的"看起来对但其实有坑"的方案都标出来。

#### Bug 1 🔴 — `packages/ui/app/components/kun/Null.vue:2` import 路径错

```ts
// 之前（错）
import { getRandomSticker } from '../utils/getRandomSticker'
//                                ^^ 解析到 components/utils/，不存在
// 之后（对）
import { getRandomSticker } from '../../utils/getRandomSticker'
```

**为什么 nuxt build 没爆**：`getRandomSticker` 被 Nuxt 自动 import `utils/*` 全局兜底了 —— 那行显式 import 形同冗余，但 vue-tsc 严格模式仍报 TS2307。装了 v0.2.0 的 vue-tsc + CI 接 `pnpm typecheck` 后这类问题不会再静默漏过。

#### Bug 2 🔴 — Button.vue colorVariants 缺 `info` 色

**报告者建议**：在 Button.vue 的 7 个 variant inner record 各自追加一行 `info: 'bg-info ...'`。

**实际修法**：**删除** Button.vue 的整个本地 colorVariants 表（80 行），改 `import { kunVariantClasses } from '../ui/variants'`，由 `colorClasses` computed 直接调用。

> ⚠️ **反例标注（重要，未来 8 色时不要重蹈覆辙）**：
> 报告者建议的"每个 variant 追加 `info: '...'`"虽然能解决 TS 报错，但**会把 v0.1.0 §1.2 刚消灭的"4 份重复 variant×color 表"问题原样重新引入** —— Button / Badge / Info / Card 之前各有一份本地表，已经在 v0.1.0 统一到 `ui/variants.ts` 单一来源。Button.vue 在我们 v0.1.0 收口时**漏改了**（其他 3 个组件都已切换共享表），加 `info` 时本地表就报错。
>
> 正确解药是把这个漏网之鱼也收口到 `kunVariantClasses`。这样将来加任何新色（hypothetical `tertiary` / 第 8 色）只需要改 `ui/variants.ts` + `ui/type.d.ts` 两个文件，**永远不会出现"某个组件忘了同步"的可能**。
>
> 如果以后又有人发"组件 X 不支持新色 Y"的 bug 报告，**默认的反应应该是"组件 X 是不是该 import kunVariantClasses 了"，而不是"在 X 里手动加一份新色定义"**。

#### Bug 3 🟡 — KunSelect defineModel 推断 `T | undefined`

Vue 3.5 `defineModel<T>()` 不带 `required: true` 也不带 `default` → 推断 `Ref<T | undefined>`，emit 回调签名带 `undefined`，下游写 `(v: string \| number) => ...` 的回调 TS 报错。

**修法**：`defineModel<string | number>({ required: true })`，与原 KunSelectProps.modelValue 是 required 的语义一致。

#### Bug 4 🟡 — `KunUser.id` 改名 `uid`

详见下一节"v0.2.1 → v0.2.2 反向修正" —— 这一改是错的，方向反了，已回滚。

---

### v0.2.1 → v0.2.2 反向修正：KunUser `uid` 回滚回 `id`

v0.2.1 的 Bug #4 修复方向搞反了。原 bug 报告说"全栈语义里用户主键一律是 uid（auth uid）"，我据此把 `KunUser.id` 改成 `KunUser.uid`。但这建议本身是错的：

- **DB 列名是 `id`**（Prisma user.id）
- Go DTO JSON tag 是 `id`（apps/api/.../oauth_dto.go 注释明确把 `id` 列为"下游 kungal/moyu/wiki 共用的 FK 不变量"）
- nitro-server response 类型也用 `id`
- `uid` 只出现在 JWT claim 和 URL 路由参数 —— **auth/transport 层的本地标签**

正确架构：DB 字段名往上传播（id → DTO id → 前端类型 id → KunUser id），auth/transport 用 `uid` 只是本地命名习惯，不污染数据层。

**v0.2.2 操作**（已落地）：
- `packages/ui/shared/user.d.ts`：`uid: number` 回滚为 `id: number` + 注释解释 DB-truth chain
- `Avatar.vue::handleClickAvatar`：`user.uid` 回 `user.id`（URL 路由参数仍叫 `uid`，但实际值是 `user.id` 整数）
- 本仓 apps/web + apps/wiki 8 处 `{ uid: ..., name, avatar }` 回 `{ id: ..., name, avatar }`

**下游（kungal/moyu）的行动项**（如果你们已经按 v0.2.1 改成 uid 了）：

```bash
# 1. 改 nitro response 类型 / composable 返回类型：uid → id
# 2. 改 KunAvatar / KunUser 调用点：{ uid: u.id, ... } → 直接 :user="u"（如果 u 已经是 KunUser 形状）
# 3. 内部用户对象的 `uid: number` 字段全部 rename 为 `id: number`
```

如果你们之前的 user 对象一直就是 `uid` 字段名（不是因为 v0.2.1 才改的），那本身就是和 DB/DTO 错位的旧设计；建议借此次同步统一回 `id`。

### 给下游的同步说明

`kungal-moyu-handoff.md` 的 §1.1 文件复制清单要新加：
```
packages/ui/app/components/kun/Popover.vue              ← 全文重写
packages/ui/app/components/kun/tooltip/Tooltip.vue      ← 全文重写
packages/ui/app/components/kun/select/Select.vue        ← 全文重写
packages/ui/app/components/kun/date-picker/Picker.vue   ← 主要改 useFloating + Teleport
packages/ui/app/components/kun/Modal.vue                ← 加 useFocusTrap
```

下游 `package.json` 需补：
```bash
pnpm add @floating-ui/vue @vueuse/integrations focus-trap
pnpm add -D vue-tsc eslint-plugin-vuejs-accessibility
```

**Tooltip 新 API**：`delayShow` / `delayHide` / `hideOnMobile` 三个新 prop，旧调用不传也能用（默认 100/0/true，比之前体验更好）。
**Popover 新 API**：`defineExpose` 出 `open/close/toggle` 三个方法，可选用。
**Select 视觉变化**：下拉宽度严格匹配触发器，超出视口时自动 maxHeight + 内部滚动。
**DatePicker 视觉变化**：不再用 `showAbove` 字面跳转，改 flip middleware 平滑。

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

## §10 v0.3.1 — KunTab solid/light indicator 错位修复（2026-05-21）

**症状**：solid / light variant 的 active 高亮条相对 button **向上偏移 4px**（视觉上看着像"上沿浮在 tab 上面"）。pills / bordered / underlined 不受影响。

**根因**：`packages/ui/app/components/kun/tab/Tab.vue::updateIndicator`

solid 容器 class 含 `p-1`（padding: 4px），第一个 tab button 的 `offsetTop = 4`。但 indicator class 是 `absolute top-0 left-0`，inline style 只有 `translateX(offsetLeft)`，**没有 translateY**。结果：

- indicator 实际位置：y=0（`top-0`）+ 0（无 translateY）= 0
- button 实际位置：y=4（被 padding 推下去）
- → indicator 比 button 高 4px

underlined 不爆是因为它 pin 到 `bottom-0`，与 padding 无关。

**修复**：把 `updateIndicator` 拆成两个分支：

```ts
// underlined: pin 到边，单轴 translate 即可
if (isLine) {
  indicatorStyle.value = isVertical.value
    ? { transform: `translateY(${el.offsetTop}px)`, height: `${el.offsetHeight}px`, width: '2px' }
    : { transform: `translateX(${el.offsetLeft}px)`, width: `${el.offsetWidth}px`, height: '2px' }
}
// solid / light: 双轴 translate，补偿父容器 padding
else {
  indicatorStyle.value = {
    transform: `translate(${el.offsetLeft}px, ${el.offsetTop}px)`,
    width: `${el.offsetWidth}px`,
    height: `${el.offsetHeight}px`,
  }
}
```

**这个 bug 在 v0.1.1 的 silent-failure-hunter 复审时被扫过**（"Tab.vue indicator math"），但 agent 当时关注的是"reflow 时同 offsetLeft 不重触发 transition"这种时序问题，**漏看了"父容器 padding → child offsetTop ≠ 0 → 单轴 translate 错位"的几何维度**。两种角度都是真实 silent failure，下次跑 silent-failure review 时，对 transform-based 定位的代码要主动让 agent 模拟父容器 padding / border / margin 三种几何场景。

**验证**：`pnpm -F web exec nuxt build` ✅，`pnpm -F wiki exec nuxt build` ✅。视觉手测 solid + light 都正确居中。

---

## §11 v0.4.0 — Primitives + Ergonomics（2026-05-21）

非破坏性增量，全部纯加 API。下游零修改即可继续工作；想用新功能时按需采纳。主题：**"补齐 form primitive 缺口 + 提升消费者人机工程学"**。

### §11.1 KunSelect — generic + readonly options

**问题**：`KunSelectProps.options: KunSelectOption[]`（可写数组），导致 `as const` 字面量传不进去（TS 报 readonly array 不能赋值给 mutable array）；同时 `value: string | number` 也擦除了字面量信息。

**修复**：把 Select 改成泛型组件，options 接受 readonly + value 类型可被 `as const` 推断为 union。

```ts
// type.d.ts
export type KunSelectValue = string | number

export interface KunSelectOption<T extends KunSelectValue = KunSelectValue> {
  value: T
  label: string
}

export interface KunSelectProps<T extends KunSelectValue = KunSelectValue> {
  options: readonly KunSelectOption<T>[]
  // ... 其余字段不变
}
```

```vue
<!-- Select.vue -->
<script
  setup
  lang="ts"
  generic="T extends KunSelectValue = KunSelectValue"
>
const props = defineProps<KunSelectProps<T>>()
const modelValue = defineModel<T>({ required: true })
const emit = defineEmits<{ set: [value: T, index: number] }>()
</script>
```

**调用方收益**：

```ts
const ROLES = [
  { label: '管理员', value: 'admin' },
  { label: '用户', value: 'user' }
] as const
const role = ref<'admin' | 'user'>('user')
// KunSelect 自动推断 T = 'admin' | 'user'，v-model 类型对齐
```

**兼容性**：默认泛型参数 = `string | number`，旧调用零修改。

### §11.2 KunTextarea / KunInput — expose 方法

**问题**：消费者无法在 chat 类场景做"在光标位置插入表情"这类操作 —— Textarea 完全没 `defineExpose`，Input 只暴露了 `focus / blur / select`。

**修复**：两边都补一套**分层暴露的方法 + 逃生通道 ref**。

```ts
// Textarea.vue / Input.vue 共用模式
const insertAtCaret = (text: string) => {
  const el = textareaRef.value  // 或 input.value
  if (!el) return
  const start = el.selectionStart ?? el.value.length
  const end = el.selectionEnd ?? el.value.length
  const next = el.value.slice(0, start) + text + el.value.slice(end)
  modelValue.value = next  // 走 v-model 而不是 .value =，触发 reactivity
  nextTick(() => {
    if (!el) return
    const pos = start + text.length
    el.setSelectionRange(pos, pos)
    el.focus()
  })
}

defineExpose({
  focus, blur, select,
  insertAtCaret,
  // 高阶场景的逃生通道
  textareaRef  // 或 inputRef
})
```

**为什么不直接 `defineExpose({ textareaRef })`**：裸 ref 把内部实现固化成 public API。以后想把 `<textarea>` 换成 contenteditable / 加 wrapper 都会破坏调用方。Vue 推荐 expose **方法**（签名稳定），ref 作为兜底。

**消费者示例**：

```vue
<KunTextarea ref="ta" v-model="msg" />
<KunButton @click="taRef.insertAtCaret('🐱')">插入猫</KunButton>
```

### §11.3 `useFilePicker` composable（不是组件！）

**问题**：KunUpload 是重型图片裁剪器（含 cropper / blob / 拖放），但"选一个 ZIP 上传"这种纯文件场景被迫绕一圈用 `<input type="file" hidden>` + 自己写按钮触发，没有统一封装。

**为什么做 composable 而不是组件**：
- 触发器应当完全自由 —— 任意 `KunButton` / `KunCard` / `<a>` 都能驱动
- 不产生持久 DOM 节点（用完即丢的 transient input）
- accept / multiple / size 校验统一收口在 composable 里
- 跟 KunUpload 视觉职责零重叠，不会让消费者陷入"这俩到底用哪个"的纠结

```ts
// packages/ui/app/composables/useFilePicker.ts
export const useFilePicker = (options?: {
  accept?: string
  multiple?: boolean
  maxSize?: number  // bytes，超过即拒收整次选择
  onError?: (msg: string, file: File) => void
}): {
  files: Ref<File[]>
  pickFiles: () => void
  clear: () => void
}
```

**用法**：

```vue
<script setup>
const { pickFiles, files } = useFilePicker({
  accept: '.zip',
  maxSize: 100 * 1024 * 1024,
  onError: (msg) => useKunMessage(msg, 'error')
})
watch(files, ([f]) => f && uploadZip(f))
</script>
<template>
  <KunButton @click="pickFiles">选择 ZIP</KunButton>
  <span v-if="files[0]">{{ files[0].name }}</span>
</template>
```

`maxSize` 拒收策略选了**整次中断**（任意一个超限就放弃所有），与 native form `<input>` 行为对齐 —— 而不是静默丢弃部分文件。

### §11.4 KunRadioGroup（新组件，classic + card 两种 variant）

**问题**：KunUI 完全缺 radio primitive。消费者 fallback 到 KunSelect（多一次点击 + 隐藏 affordance + 移动端体验差）或自己 `<input type="radio">`（破坏统一视觉）。

**决策**：

| 候选方案 | 取舍 |
|---|---|
| 加 KunRadioGroup（独立组件） | ✅ 做 |
| 给 KunTab 加 radio variant | ❌ 拒绝 |

为什么不复用 Tab：ARIA 角色不同（`tablist/tab` vs `radiogroup/radio`），键盘交互模型也不同（Tab 的 ←→ 切换 view，Radio 的 ←→ 移焦并激活）。强行复用让 Tab 变成"既切 view 又传 form value"的混合体，违反单一职责。**真正想要"胶囊状互斥选择器"视觉的场景，pills variant 已经能用 v-model 当 form value 用**，本来就没缺口。

**两种 variant**：

| variant | 长相 | 适用 |
|---|---|---|
| `classic` | 圆形单选按钮 + label（+ 可选 description） | 标准 form 场景 |
| `card` | 矩形卡片（带 border + 选中淡色 tint） | 大目标点击、选项含较多信息 |

**关键设计**：

1. **泛型 `T extends string \| number`**，与 Select 同款 readonly options：
   ```ts
   export interface KunRadioOption<T extends KunRadioValue = KunRadioValue> {
     value: T; label: string
     description?: string  // 可选副文本
     disabled?: boolean
   }
   ```

2. **完整 ARIA**：`role="radiogroup"` + `aria-labelledby` / `aria-label` + 每项 `role="radio" aria-checked aria-disabled`

3. **Roving tabindex**：整组只有一个 `tabindex=0`（优先 selected，否则第一个非禁用项），其余 `tabindex=-1` —— 符合 WAI-ARIA radio pattern

4. **键盘**：↑↓←→ 移焦并**立即激活**（per ARIA spec，与 listbox/menu 的"仅移焦"不同）+ Space/Enter 激活；自动跳过 disabled 项

5. **方向**：`orientation: 'vertical' | 'horizontal'`

6. **颜色 / 尺寸**：复用 KunUI 标准 `color` × `size` 矩阵；圆角 prop 仅 card variant 生效（classic 的 indicator 永远是圆）

**用法**：

```vue
<KunRadioGroup
  v-model="role"
  :options="[
    { value: 'admin', label: '管理员', description: '完整权限' },
    { value: 'user', label: '普通用户' },
    { value: 'guest', label: '访客', disabled: true }
  ]"
  variant="card"
  color="primary"
  label="选择角色"
/>
```

### §11.5 新增静态颜色映射 `kunSoftBgClasses`

为 RadioGroup card variant 的"选中淡色背景"加了 `bg-{color}/5` 静态表（同 JIT-safety 规则：keys 必须字面量）：

```ts
// ui/variants.ts
export const kunSoftBgClasses: Record<KunUIColor, string> = {
  default: 'bg-default/5',
  primary: 'bg-primary/5',
  // ...
}
```

任何后续组件需要"barely-there 着色背景"都可以直接复用。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 所有改动**零破坏性**：Select 默认泛型参数 = `string | number`、Textarea/Input 只是新增 expose、useFilePicker 与 KunRadioGroup 都是新文件

### 落地清单（给 kungal / moyu 同步）

| 文件 | 操作 |
|---|---|
| `packages/ui/app/components/kun/select/Select.vue` | 改：generic `<script setup>` |
| `packages/ui/app/components/kun/select/type.d.ts` | 改：generic interface + readonly |
| `packages/ui/app/components/kun/Textarea.vue` | 改：补 defineExpose |
| `packages/ui/app/components/kun/Input.vue` | 改：补 insertAtCaret + inputRef |
| `packages/ui/app/components/kun/ui/variants.ts` | 改：新增 `kunSoftBgClasses` |
| `packages/ui/app/composables/useFilePicker.ts` | **新增** |
| `packages/ui/app/components/kun/radio-group/RadioGroup.vue` | **新增** |
| `packages/ui/app/components/kun/radio-group/type.d.ts` | **新增** |

详细 sed / cp 同步命令见 `packages/ui/docs/kungal-moyu-handoff.md`。

---

## §12 v0.4.1 — 浮层"从角落飞来"动画错位修复（2026-05-21）

**症状**：`KunSelect` / `KunPopover` / `KunDatePicker` 打开时，弹出层不是在原地淡入，而是**从页面某个角落（通常 body 左上角）滑动 + 缩放到目标位置**。视觉上像"飞过来"，非常 jank。`KunTooltip` / `KunContextMenu` 无此问题。

**根因 —— 同一 DOM 上 transform 双写竞态**：

`@floating-ui/vue` 的 `useFloating()` 默认通过 inline `transform: translate3d(X, Y, 0)` 给浮层定位。同一个 `<div>` 又被 Vue `<Transition>` 加了 transform 类（如 `-translate-y-1` / `scale-95`）。理论上 inline style 永远赢 class，但实际触发挂载竞态：

| 时刻 | 状态 |
|---|---|
| t0 | `v-if=true`，div 挂载，floating-ui 还没异步算完位置 |
| t0 | 此时 `floatingStyles` = `{ position: absolute, top: 0, left: 0 }`（无 transform），div 渲染在 body 左上角 |
| t0 | Vue Transition 加 enter-from-class，`transform: translateY(-4px)` 生效，div 仍在左上角 |
| t1 | floating-ui 写 inline `transform: translate3d(X, Y, 0)` |
| t1 → 完成 | 浏览器看到 `transform` 属性在变，按 `transition` 时间曲线插值 → **从左上角飞到目标位置** |

Tooltip 不爆是因为它的 transition class 只动 opacity；ContextMenu 不爆是因为它**根本不用 floating-ui**，自己写 `top/left`。

**修复 —— 官方推荐 `transform: false` 选项**：

查 [floating-ui.com/docs/useFloating#transform](https://floating-ui.com/docs/useFloating) 找到：

> "CSS transforms are more performant, but can cause conflicts with transform animations."

`useFloating()` 接受 `transform: false`，让定位走 `top` / `left` 而不是 `transform`。这样 `transform` 属性彻底空出来给 Vue Transition 用：

```ts
const { floatingStyles } = useFloating(triggerRef, popoverRef, {
  placement: 'bottom-start',
  open: isOpen,
  whileElementsMounted: autoUpdate,
  transform: false,   // ← 关键一行
  middleware: [...]
})
```

之后注入的 inline style：

```css
/* 之前 */ position: absolute; top: 0; left: 0; transform: translate3d(384px, 120px, 0);
/* 之后 */ position: absolute; top: 120px; left: 384px;
```

零结构变更，下滑 / 缩放动画完整保留。

**与官方另一方案对比**：

官方文档同时提到"wrapper 模式"——外层 div 承载 floating-ui transform，内层 div 承载动画 transform。这是双层结构 + leave 动画需要 `@after-leave` 协调，~25 行/组件。比较：

| 方案 | 代码量 | 性能 | 是否保留动画 |
|---|---|---|---|
| `transform: false` | +1 行/组件 | top/left 触发 layout（开启瞬间一次） | ✓ |
| 双层 wrapper | +25 行/组件 | composite-only（GPU） | ✓ |
| 砍 transform 改 opacity | +0 行 | 无影响 | ✗ 失去下滑感 |

对"按钮触发的下拉，开了就静止"这种交互，layout 触发频率 = 打开次数（每秒最多几次），性能差距可忽略。**`transform: false` 是最优解**。

**影响范围**（受影响 3 个，已修）：

| 组件 | floating-ui transform | 动画 transform | 修复 |
|---|---|---|---|
| `Select.vue` | ✓ | `-translate-y-1` | + `transform: false` |
| `Popover.vue` | ✓ | `scale-95` | + `transform: false` |
| `date-picker/Picker.vue` | ✓ | `-translate-y-1` | + `transform: false` |

**关于 width**：`transform: false` 时浮层最好有显式 width 或 `width: max-content`（否则可能因为 absolute 默认行为塌缩）。Select 用了 `size()` middleware 显式设宽 ✓；Popover/DatePicker 默认 `width: auto` 跟内容走，没问题。

**验证**：

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 视觉手测：3 个组件打开 / 关闭都不再"飞" —— 直接在 trigger 旁淡入淡出 + 下滑

**反思**：这种 bug 应当被 silent-failure-hunter 在 v0.2.0 浮层引擎统一时抓到 —— 但 agent 当时验证的角度是"floating-ui 单独工作"和"Vue Transition 单独工作"，没模拟"两者叠加在同一元素上"的复合场景。下次 review 涉及 Teleport + Transition + 第三方定位库 的组合时，**主动让 agent 检查同一 DOM 节点是否被两个独立系统写同一 CSS 属性（transform / opacity / filter）**。这类 race 在静态 lint / unit test / build 都无法发现，只有视觉测试或专门的"双写检测"能抓。

---

*文档维护：每次完成批次后更新对应章节的 checkbox。新增组件评审进 §2 表 + §3 新章节。*
