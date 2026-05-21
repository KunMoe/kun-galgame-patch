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

## §13 v0.4.2 — KunFileInput：补齐文件交互三层（2026-05-21）

moyu 反馈了 7 处仍在用 native `<input type="file">` 的场景（banner / 截图 / patch hash 校验 / 头像 cropper bridge）。原本提案 "加一个 KunFileInput 组件"，但若不与 v0.4.0 已铺好的 `useFilePicker` 协同就会造重复实现，所以本节按**分层架构**收口。

### 文件交互的三层 API（终态）

| 层 | 名字 | 用途 | 起源 |
|---|---|---|---|
| 高层 | `KunUpload` | 图片上传 + 裁剪 + 拖拽预览（重型） | v0.0.1 |
| **中层** | **`KunFileInput`** | **声明式按钮 + v-model 文件选择（轻量）** | **v0.4.2 ⭐** |
| 低层 | `useFilePicker` | 程序化选择，触发器自由 | v0.4.0 |

三者明确分工：

- **要做图片裁剪 / 拖放 / 预览** → KunUpload
- **要简单的"按钮 + 选文件 + v-model"** → KunFileInput
- **触发器是任意自定义元素 / 不需要 DOM 节点** → useFilePicker

KunFileInput 内部**直接复用 useFilePicker**，零重复实现 —— 这点很重要：未来扩展（mime 校验、size formatter、多文件 dedup 等）只改 composable 一处。

### 设计要点

#### 1. 双命名 v-model 区分单 / 多文件

```vue
<!-- 单文件：v-model = File | null -->
<KunFileInput v-model="bannerFile" accept="image/*" />

<!-- 多文件：v-model:files = File[]，加 `multiple` 启用 -->
<KunFileInput v-model:files="screenshots" multiple accept="image/*" />
```

为什么不用 union `File | File[] | null`：消费者每次都要 narrow，TS 表达力损失大。两个命名 model（Vue 3.4+ 原生支持）让单 / 多场景的类型各自干净，约定"用 `multiple` 时只用 `v-model:files`"即可。

#### 2. 默认 trigger 是 KunButton；slot 完全自定义

```vue
<!-- 80% 用例：默认按钮 -->
<KunFileInput
  v-model="bannerFile"
  accept="image/*"
  :max-size="10 * 1024 * 1024"
  trigger-text="选择 banner"
  trigger-icon="lucide:image-up"
  hint="JPEG / PNG / WebP，最大 10 MB"
/>

<!-- 20% 用例：卡片 / 头像 / 任意元素当触发器 -->
<KunFileInput v-model="patchFile" accept=".zip" v-slot="{ pick, fileName }">
  <KunCard clickable @click="pick">
    <Icon name="lucide:file-archive" />
    <p>{{ fileName ?? '点击选择 patch 包' }}</p>
  </KunCard>
</KunFileInput>
```

slot 暴露 `pick / fileName / disabled` 三个状态，让自定义触发器无需 ref 或 expose 就能完整控制。这是 Vue 的标准 "render-prop / scoped slot" 模式。

#### 3. `@change` 始终发数组

```ts
@change="(files: File[]) => { /* files[0] 单文件场景 / files.forEach 多文件 */ }"
```

单文件模式发 `[file]`、多文件发 `[f1, f2, ...]`。统一数组让消费者无需 `isArray` 分支，逻辑同构。

#### 4. 用户取消保留旧选择

`useFilePicker` 在取消时不清空 `files`，KunFileInput 的 watch 里 `if (next.length === 0) return` 保留 v-model 旧值。与 native `<input type="file">` 的取消语义对齐 —— 取消不丢之前的选择。

#### 5. 默认按钮 vs slot 触发器的视觉一致性

默认按钮直接长成 KunButton 的样子（继承 KunUI 全套 variant / color / size / rounded 系统）。slot 触发器完全由消费者决定。两条路径下，"已选文件名"行（`showFileName` prop 控制）始终在统一位置展示。

### 实施

#### 新增文件

```
packages/ui/app/components/kun/file-input/FileInput.vue   ← 新组件
packages/ui/app/components/kun/file-input/type.d.ts       ← 类型
```

#### 本仓采用情况

galgame/Create.vue 和 galgame/EditModal.vue 之前用 `useFilePicker` 写得有点啰嗦（手写 `pickedBannerFiles` watch + 状态同步），改用 KunFileInput 后**单文件场景减少 8 行代码**：

```vue
<!-- 之前：useFilePicker（v0.4.0） -->
<script setup>
const { pickFiles: pickBanner, files: pickedBannerFiles } = useFilePicker({
  accept: 'image/jpeg,image/png,image/webp',
  maxSize: 10 * 1024 * 1024,
  onError: (msg) => useKunMessage(msg, 'warn')
})
watch(pickedBannerFiles, ([f]) => {
  if (!f) return
  bannerFile.value = f
  bannerObjectUrl.value = URL.createObjectURL(f)
})
</script>
<template>
  <KunButton size="sm" variant="flat" @click="pickBanner">
    <Icon name="lucide:image-up" class="mr-1 size-4" />
    选择 banner
  </KunButton>
</template>

<!-- 之后：KunFileInput（v0.4.2） -->
<script setup>
watch(bannerFile, (f) => {
  bannerObjectUrl.value = f ? URL.createObjectURL(f) : ''
})
</script>
<template>
  <KunFileInput
    v-model="bannerFile"
    accept="image/jpeg,image/png,image/webp"
    :max-size="10 * 1024 * 1024"
    trigger-text="选择 banner"
    trigger-icon="lucide:image-up"
    trigger-size="sm"
    @error-pick="(msg) => useKunMessage(msg, 'warn')"
  />
</template>
```

### 兼容性

完全非破坏性。`useFilePicker` 接口不变，老代码继续工作；想要更简洁的写法时切到 KunFileInput。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 本仓 2 处 banner 上传跑通：选文件 → 预览 → 清空，三态切换正常

### 这一步封口了什么

KunUI 至此对"文件交互"主题给出完整三层方案，**没有任何 native `<input type="file">` 是合理的留存**（除非是 KunUI 内部实现细节，比如 KunUpload 自己的实现）。下游消费方仓库（kungal / moyu / wiki / oauth）的每一个 file input 都应该对应到三层之一。

---

## §14 v0.4.3 — `getRandomSticker` 双层修复：useState + 客户端 Map 缓存（2026-05-21）🔴

moyu 上报的运行时崩溃 bug，**严重度 critical**。任何下游 app 只要在 computed / watcher 重算路径上渲染 `<KunAvatar :user>` 或 `<KunNull description>` 就会撞上。

> **本节双层教训**：第一层是 moyu 抓到的原始 bug，第二层是**我修复时引入的新 bug**（SSR 模块级缓存跨请求泄漏）。两层都已经修，但保留中间过程作为反面教材 —— 改 SSR 代码时如果不同时想清楚 server / client / setup / reactive effect 四个执行上下文，很容易"修一个 bug 引一个新 bug"。

### 症状

页面正常加载 → 调用任何会刷新数据的 action（resource refresh / list reload / 任何 `refresh()` 调用）→ 整个页面崩，控制台 `Cannot read properties of null (reading '$nuxt')`。

### 第一层根因（moyu 诊断）

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

### 第二层根因（我修复时引入的新 bug）

我第一版修复用"**模块级 `Map<string, Ref<string>>` 缓存 + 纯 `ref()`**"，看似避开了 Nuxt context 依赖。但**这个 Map 在 SSR 进程里跨请求泄漏**，触发新一类水合不一致：

| | F5 #1 | F5 #2（同进程下一个请求） |
|---|---|---|
| Server module cache | 空 | **已有 F5 #1 留下的 ref** ← 泄漏点 |
| Server 是否调 `useState` | ✅ 调 → 选 URL X → 入 payload | ❌ cache hit 直接返，**`useState` 没调，payload 没 X** |
| Server 渲染 HTML | X | X（从 stale ref） |
| Client 水合：payload 里有 X？ | ✅ 有 → 渲 X | ❌ 没有 → `useState` 调 initFn → 选 Y → **渲 Y ✗** |

Server 进程是长驻的（Node 的 `process.on('request', ...)` 一直跑），`const cache = new Map()` 模块作用域变量在两次请求间继续活着。但 **`NUXT.payload` 是按请求重新构造的**。两个寿命不对齐时，module cache 把 `useState` 短路掉，server 渲了 X，client 不知道是 X，自己算了 Y → 水合 mismatch。**只在 F5 #2+ 出现**，#1 永远正常 —— 比第一层 bug 更隐蔽。

### 最终修复（双层都覆盖）

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

### 接受的退化（更精确的描述）

只剩**一种**残留：client reactive 路径上第一次见某个全新 id 时，`tryUseNuxtApp()` 返 null → 走 fallback `ref()` 路径 → 选的 URL 与"如果 server 看到这个 id 会选的 URL"可能不同。**但这个 id 根本没经过 SSR**（refresh 后才出现），所以没有"参照真值"可对比，不存在 hydration mismatch。

### 给 KunUI 立两条工程规则（在 v0.4.3 加上）

#### 规则 1：layer utils 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门

> 任何依赖 Nuxt context 的 composable（`useState` / `useFetch` / `useAsyncData` / `useRoute` / `useRuntimeConfig` 等）调用前，要主动审视"是否可能从 reactive effect 重入路径触发"。如果可能，**必须 `tryUseNuxtApp()` 守门 + 准备 plain Vue 原语 fallback**。

#### 规则 2：layer utils 里**永远不要加模块级 mutable 缓存而不区分 server / client**

> Nuxt SSR 进程长驻，模块作用域变量跨请求泄漏，会把 `useState` / `useFetch` 这种 per-request scoped 机制短路，导致**只在首次 F5 后才出现的水合 / payload 不一致 bug** —— 难诊断、产线很常见。
>
> 安全模式：
> ```ts
> const cache = import.meta.client ? new Map() : null
> ```
> 或者干脆挂到 `nuxtApp.payload._xxxCache` 之类的请求作用域里。
>
> **静态只读常量（如 `KEY_OWNING_ROLES = new Set([...])` 这种 lookup table）不在此约束内** —— 不写入就没有泄漏。

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

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- moyu 上报：`nuxt prepare` 无 dup warning、`tsc + vue-tsc` 通过、`vitest 12/12 spec 85/85 case` 通过 ✅
- 运行时验证：moyu 编辑提交 → 资源 refresh → 不再炸 ✅
- F5 多次刷新水合警告消失 ✅

### 给 kungal 的 patch notes

> ▎ **v0.4.3 patch — `getRandomSticker` 修复 reactive effect 崩溃 + SSR 水合不一致**
> ▎ 1) 避开 `useState` 在 microtask 重算里调用导致的 null context 崩溃；2) cache 只在 client 端启用，避免 SSR 模块级缓存跨请求泄漏破坏 payload 序列化。
> ▎ 下游消费方零迁移成本（API 不变）。如果你 fork 了 `getRandomSticker.ts`，请覆盖到 v0.4.3 版本。

---

## §15 v0.4.4 — `useKunMessage` 裸 `render()` 漏 appContext 才是一连串崩溃的真根因（2026-05-21）🔴🔴

moyu 在多轮调试后揪到的"**统一根因**"，**严重度 critical**，且**意外解释了 §14 之前那一连串看似不相关的 `$nuxt null` 崩溃报告**。这一节是 KunUI 自 v0.1.0 起最关键的一个 hotfix。

> **本节是一个完整侦探故事**：表象 A、B、C、D 看起来是四个独立的 Nuxt context bug，调试者（先是我，再是 moyu）顺着每个表象局部对症，结果只是把崩溃点推迟。最终发现**全是同一个 root cause** —— `useKunMessage` 第一次被调用时挂载 MessageContainer 的 `render(vNode, container)` 没带 appContext，**整个 Nuxt 实例从此被腐化**，后续任何 Nuxt composable 调用都可能拿到 null。

### 真正的 bug — 一行代码

`packages/ui/app/composables/useKunMessage.ts:42`：

```ts
// 之前（炸）
const vNode = h(MessageContainer)
render(vNode, containerRef)   // ❌ 裸 render，vNode 处在孤立 app context
```

Vue 3 的 `render(vNode, container)` API 创建的 vNode 默认**处在一个孤立的 app context**：没有 Nuxt 实例、没有 `@nuxt/icon` 插件、没有 pinia、什么都没有。

`<MessageContainer>` 被这样裸 render → 内部 `<KunAlertMessageItem>` → 渲染 `<KunIcon>`（`@nuxt/icon` 的包装）→ `<NuxtIcon>` 的 `setup()` 第一行 `useNuxtApp()` → `tryUseNuxtApp()` 返 null → **崩**。

Stack trace 一目了然：

```
at setup (index.js:32:21)       ← NuxtIcon 的 setup
at <NuxtIcon name="...">
at <KunIcon name="...">
at <KunAlertMessageItem>
at <KunAlertMessageContainer>   ← 被 bare render() 挂载的
```

### 一行修复

```ts
const vNode = h(MessageContainer)

const nuxtApp = tryUseNuxtApp()
if (nuxtApp?.vueApp) {
  vNode.appContext = nuxtApp.vueApp._context
}

render(vNode, containerRef)
```

`vNode.appContext = nuxtApp.vueApp._context` 是 **Vue 3 文档化的 "render 独立 vNode 但保留 app context" 模式**。这一行让 MessageContainer 子树里所有 Nuxt composable（NuxtIcon 的 `useNuxtApp`、`useKunMessageState` 等）都能拿到原 Nuxt 实例。

### 之前那一连串错误的统一解释

| 表象 | 我（和 moyu）以为的根因 | **真实根因** |
|---|---|---|
| A. `getRandomSticker $nuxt null` | computed 重算路径 useState 失败 | 大部分是真，但部分是 B 引起的腐化 |
| B. `kunFetch useRuntimeConfig $nuxt null` (在 `LinkDetailModal.vue::watch(open)` 里) | watch 调度从 microtask 跑，丢了 Nuxt context | **`useKunMessage` 之前已经把 Nuxt 实例搞坏了**，watch 拿到的本来就是腐化态 |
| C. 编辑 modal 关闭报 `$nuxt` | 关闭流程里某个 composable 调度问题 | **关闭时 `useKunMessage(10550, 'success')` 触发首次挂载 MessageContainer → NuxtIcon 炸** |
| D. 后续任意操作都开始报 `$nuxt` | 多个独立 bug | **C 把 Vue 内部状态弄坏后，后续所有 useNuxtApp 一片 null** |

A 确实是独立的 reactive recompute 问题（§14 修过），但 B、C、D **不是独立 bug**，是 useKunMessage 首次挂载失败的连锁反应。**之前所有 `runWithContext` / `nextTick` / refactor 的修复都在错误的地方修**，只是把崩溃点推迟，没解决真正的源头。

### 为什么"首次崩溃会腐化整个 Nuxt 实例"

Vue 3 `render(vnode, container)` 失败时，Vue 内部的渲染器状态可能进入半挂载态。如果失败的子树有 effect / setup 已经注册到全局 reactive system 但没成功绑到 component instance 上，**这些 dangling effects 会污染后续 `useNuxtApp()` 的查找路径**。具体机制要看 Vue 源码细节，但实测表现是：useKunMessage 首次失败 → 后续任意 setup 调用 `useNuxtApp()` 也开始返 null。

**这就是为什么我之前用 `runWithContext` 包 LinkDetailModal 的 watch 一度看似有效但又不稳定** —— `runWithContext` 把 Nuxt 实例显式注入到回调里，绕过了 `useNuxtApp()` 的查找，所以**在被腐化的实例还能用时**修复有效；一旦腐化更深，连显式注入的实例本身都不健康。

### 修复后的连锁效应

| 表象 | 修复 v0.4.4 后 |
|---|---|
| A. `getRandomSticker` | §14 的双层修复已经独立解决了 |
| B. `kunFetch` in watch | **自动消失** —— 真正的根因是 C 引起的腐化，C 修了 B 自然好 |
| C. 编辑 modal 关闭 `$nuxt` | **自动消失** —— MessageContainer 现在带着正确 appContext 挂载，NuxtIcon 拿得到 Nuxt 实例 |
| D. 后续任意操作崩溃 | **自动消失** —— 没有 C 的连锁腐化就不存在 |

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- moyu 上报：`nuxt prepare` / `tsc + vue-tsc` / `vitest 85/85` 全过 ✅
- 运行时验证：编辑提交触发 success message → 正常显示 + 后续操作不再炸 ✅

### 给 KunUI 立第三条工程规则（v0.4.4 起生效，与 §14 那两条合并成 KunUI 三铁律）

#### 规则 3：用 Vue 原生 `render(vnode, container)` 命令式挂载组件时，**必须**显式 graft Nuxt appContext

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

### KunUI v0.1.x → v0.4.x 三铁律总结

把 §14 + §15 的反思合并成 KunUI layer / composable / util 的三条强制规则：

| # | 规则 | 反面教材 |
|---|---|---|
| 1 | layer utils 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门 + plain Vue 原语 fallback | `getRandomSticker` v0.1.x（§14） |
| 2 | layer utils 里 module-level mutable cache 必须 `import.meta.client`-gated；读写都禁止跨请求泄漏 | `getRandomSticker` 我修第一版（§14） |
| 3 | 命令式 `render(vnode, container)` 必须 graft `nuxtApp.vueApp._context` 到 vNode.appContext | `useKunMessage` v0.1.x（§15） |

### 反思 — 调试 SSR / Nuxt context bug 的元教训

这一轮调试有三层教训叠加：

**层 1 — 局部对症 vs 根因**：我和 moyu 都犯过"看到 stack trace 直接修 stack trace 顶部"的错。`getRandomSticker` 是真 bug（§14），但 `kunFetch in watch` / `editModal close` / `subsequent useNuxtApp null` 这三个"看起来是 watch 调度 / async timing / cleanup 的问题" **统统是 useKunMessage 一次性挂载失败的连锁反应**。

**层 2 — Nuxt 实例的"腐化"是真存在的**：以前我以为 Nuxt context 失败只是"局部拿不到"，不会"污染整个 app"。这次实测发现 `render()` 失败会让后续 `useNuxtApp()` 全军覆没 —— 一旦某个表象出现，赶紧问自己"**这是不是某个早一点的 render / mount 失败的余波**？"

**层 3 — 多 bug 同时报告时，先找"最早触发的那个"**：调试时按时间顺序排查事件，不要按当前报错位置排查。如果 useKunMessage 在 t=0 失败，但你的注意力被 t=5 的 LinkDetailModal watch 报错吸走，就会一直跟着 t=5 的红鲱鱼跑。

### 给 kungal 的 patch notes

> ▎ **v0.4.4 patch — `useKunMessage` 修复整个 KunUI 调试链最深处的真根因** 🔴🔴
> ▎ 原因：`useKunMessage` 内部用 Vue 的 `render(vnode, container)` 裸挂载 MessageContainer，没设 `vNode.appContext`，导致 MessageContainer 子树里任何调用 Nuxt
> ▎ composable 的组件（首当其冲是 `<NuxtIcon>`）都崩，**且首次崩溃会腐化整个 Nuxt 实例**，后续看似不相关的 `useState` / `useFetch` / `useNuxtApp` 一片 null。
> ▎
> ▎ 修复（packages/ui/app/composables/useKunMessage.ts）：
> ▎ ```ts
> ▎ const vNode = h(MessageContainer)
> ▎ const nuxtApp = tryUseNuxtApp()
> ▎ if (nuxtApp?.vueApp) vNode.appContext = nuxtApp.vueApp._context
> ▎ render(vNode, containerRef)
> ▎ ```
> ▎
> ▎ 下游消费方零迁移成本（API 不变）。**强烈建议立即覆盖 v0.4.4 版本**。如果你之前为绕这个 bug 加过 `nuxtApp.runWithContext(() => useKunMessage(...))` 或类似 wrap，**可以删掉了**（修复后不再需要），但留着也无害。

---

## §16 v0.4.5 — z-index 设计系统：浮层不再被 Modal 压住（2026-05-21）

**症状**：KunModal 里打开 KunSelect / KunPopover / KunDatePicker / KunTooltip，下拉 / 浮层**视觉上沉到 Modal 下面**，看不见。

### 根因 — 散乱 magic number z-index 缺乏统一秩序

v0.1.x 起 z-index 一直是各组件自己拍脑袋写的硬编码值，没有统一秩序。审计后实际是：

| 组件 | 旧 z-index | 是哪种浮层 |
|---|---|---|
| Popover / Select dropdown / DatePicker dropdown / Tooltip | **z-50** | 全局浮层（Teleport to body） |
| Modal | z-1007 | 全局阻塞容器 |
| ContextMenu | z-[1100] | 右键菜单 |
| Alert（确认对话框） | z-2000 | 阻塞 modal |
| Loli toast | z-2000 | 通知 |
| MessageContainer | z-[7777] | toast 容器 |

Modal 在 z-1007，四个 popover 类组件在 z-50。**Teleport 让 popover 离开 Modal 的 DOM 子树**，但**没改变 z-index 排序** —— body 下两个并列子树 z-50 vs z-1007，Modal 视觉上完全压住 popover。这是 v0.1.x 以来的隐藏 bug。

### 修复 — 仿 rounded 系统的 token 化

`packages/ui/app/styles/tailwindcss.css` 的 `@theme` 块加 5 个 token：

```css
@theme {
  /* ... 既有 color / radius tokens ... */
  --z-kun-sticky: 30;
  --z-kun-modal: 1000;
  --z-kun-popover: 1500;
  --z-kun-alert: 2000;
  --z-kun-message: 9000;
}
```

Tailwind v4 自动生成对应 utility：`z-kun-sticky` / `z-kun-modal` / `z-kun-popover` / `z-kun-alert` / `z-kun-message`。

**核心层级规则**：

> 一个浮层永远在"触发它的层"之上。

```
sticky (30) < modal (1000) < popover (1500) < alert (2000) < message (9000)
```

- popover 高于 modal → 在 Modal 内打开 Select 时下拉在 Modal **之上** ✓
- alert 高于 popover → KunAlert 确认对话框打开时永远盖住普通 popover ✓
- message 最高 → toast 总能被看见，即便在 alert / modal 之上 ✓

### 9 个组件全部迁移

| 组件 | 旧 | 新 | 备注 |
|---|---|---|---|
| `Modal.vue` | `z-1007` | `z-kun-modal` | |
| `Popover.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `select/Select.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `date-picker/Picker.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `tooltip/Tooltip.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `context-menu/ContextMenu.vue` | `z-[1100]` | `z-kun-popover` | 语义对齐 |
| `alert/Alert.vue` | `z-2000` | `z-kun-alert` | |
| `alert/Loli.vue` | `z-2000` | `z-kun-message` | toast 性质，应在 message 层 |
| `alert/MessageContainer.vue` | `z-[7777]` | `z-kun-message` | |

### 故意不动的 z-index

这些是**局部 stacking context 内的相对层级**，不是全局浮层 —— 保持原样：

| 组件 | z-index | 说明 |
|---|---|---|
| `Badge.vue` | `z-10` | 在 avatar 内的角标 |
| `Tab.vue` | `z-10` | tab 内 indicator 之上 |
| `scroll/Shadow.vue` | `z-10` | 容器内滚动阴影 |
| `content/Content.vue` | `z-1` / `z-5` | markdown 内部排序 |
| `Loading.vue` | `z-50` | 卡片内 overlay |
| `lightbox/Lightbox.vue` 内按钮 | `z-50` | lightbox 内导航按钮 |

判定标准：**只要不是 `fixed` / `absolute` 跨出父容器、不参与全局浮层栈的，相对 z-index 是合理的 —— 它们生活在各自父级的 stacking context 里，不会与 Modal / Popover 在全局栈竞争**。

### 消费侧覆盖

下游 app 如果需要与第三方 widget z-index 对接（比如某个 vendor 库的 modal 写死 z-9999），可以在自己的 `:root` 里覆盖：

```css
:root {
  --z-kun-modal: 9998;     /* 让 KunModal 比那个 vendor modal 低 1 */
  --z-kun-popover: 10500;  /* KunSelect 仍然高于 vendor modal */
}
```

整套 token 化的本质是**让全局浮层栈成为可配置的 design system，而不是 9 个文件里 9 个 magic number**。

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 视觉手测：Modal 里打开 Select / Popover / DatePicker / Tooltip，浮层全部**正确出现在 Modal 之上**
- 全仓 grep `z-50\|z-1007\|z-2000\|z-\[7777\]\|z-\[1100\]` 在 9 个 global float 组件里 0 命中

### 反思 —— §1.9 的 TODO 终于还了

v0.1.0 §1.9 "其他小型一致性问题"里早就列过这条：

> z-index 散落硬编码（z-10 / z-50 / z-1007 / z-1100 / z-2000 / z-[7777]） — Select, Modal, ContextMenu, Alert, MessageContainer

当时标"低优"留到后面。这次因为用户报告"Select 在 Modal 内被压住"才被强制推上日程 —— 一个早就识别但被低估优先级的 latent bug。**视觉层级 bug 在没人实际把 Select 放进 Modal 之前是看不见的，但放进去那天就是 critical**。

教训：design system 类的一致性 issue 即使当前没有可见报错，也应该作为"在产线之前先做完"的功课。比"等到用户报 bug 才动"省事。

---

## §17 v0.4.6 — KunImage 5 个 prop 透传 + KunUI layer 注册 `none` provider（2026-05-21）

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

---

## §18 v0.4.4 / v0.4.5 反思修正 — `runWithContext` 没那么万能，外加"孤儿 store"陷阱（2026-05-21）

moyu 在他们 fork 调试一个"点删除按钮无反应"的 bug 时，发现自己之前为了绕 §14 / §15 那一批 `$nuxt null` 问题、在好几条 click 路径上加的 `nuxtApp.runWithContext(...)` 包装**根本没起作用** —— 真根因是另一个完全独立的"老 store 没迁，promise 永不 resolve"的孤儿 bug。这次复盘让我们也得修正之前在 §14 / §15 给出的工程规则，免得规则被泛化成"任何 await 都包 runWithContext"。

> 本节是**文档修正 + 反思**，不是新代码改动。kun-oauth-admin 本仓没有同款孤儿 store 残留（grep 验证过），但教训对所有 fork 都适用。

### 修正 1 — `runWithContext` 真正必须的两个场景

之前 §14 / §15 末尾给出的"layer util 调 Nuxt composable 必须 `tryUseNuxtApp` 守门"规则**仍然正确**，但常被误读为"凡是 await / 跨 tick 都该 `runWithContext`"。准确的边界是：

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

### 修正 2 — `tryUseNuxtApp()` 的查找路径解释为什么 (3-6) 不需要

`tryUseNuxtApp()` 内部查找顺序大致是：

```
1. nuxtAppCtx.tryUse()                            ← Nuxt 自维护的 AsyncLocalStorage
2. getCurrentInstance().appContext.app.$nuxt      ← Vue 当前实例的 app context
```

Vue 3 的事件处理器 / lifecycle hook **执行期都有 `getCurrentInstance()` 不为 null** —— 路径 2 直接命中，根本走不到路径 1 需要 `runWithContext` 强制注入的情况。

### 修正 3 — "孤儿 store"陷阱（新加入排查清单）

下游 app 从老 alert / dialog / message store 迁到 KunUI 的 `useKunAlert` / `useKunMessage` 时，如果**只迁了 UI 组件**（把老的 `<MyAlert>` 删了换 `<KunAlert>`），但**老 store 的 `alert(...)` 方法还在被旧调用点使用**，会触发"孤儿 store"bug：

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

**症状特征**：
- 点击按钮后**完全静默** —— 没报错、没 network 请求、没 console output
- 跟 `$nuxt null` 不同，**完全无报错**（promise 死锁不是 error）
- DevTools Performance 看到点击事件触发但没后续

**修复模板** —— 桥接老 store：

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

### 修正后的 $nuxt-null / 按钮静默故障 排查清单

按这个顺序，省时间：

| 步 | 检查 | 命中怎么修 |
|---|---|---|
| 1 | 点击**完全静默**？(没报错没请求) | 怀疑**孤儿 store**，grep 老 alert / message store 是否还有人调 |
| 2 | 报 `$nuxt null` 且**屏幕崩**？ | 怀疑 §15 的 render() 裸 mount 漏 graft appContext |
| 3 | 报 `$nuxt null` 在 watch / watchEffect 里？ | 用 `nuxtApp.runWithContext(() => …)` 包回调体 |
| 4 | 报 `$nuxt null` 在 @click handler 里？ | **99% 不需要 runWithContext**，先找别的原因（更可能是 §1 或 §2） |

### KunUI 工程规则更新（v0.4.4 三铁律 → v0.4.5 + 本节后变成五条）

| # | 规则 | 来源 |
|---|---|---|
| 1 | layer util 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门 + plain Vue 原语 fallback | §14 |
| 2 | layer util module-level mutable cache 必须 `import.meta.client ? new Map() : null` | §14 |
| 3 | 命令式 `render(vnode, container)` 必须 graft `nuxtApp.vueApp._context` | §15 |
| 4 | Teleport-to-body 的 fixed/absolute 浮层必须用 `z-kun-*` token | §16 |
| 5 | 下游迁 KunUI 状态 composable（`useKunAlert` 等）时，老 store 接口要**桥接 delegate**而不是删，避免孤儿 hang | §18（本节） |

同时**修正**之前规则 1 / 3 隐含的"runWithContext 是万能护身符"印象 —— 它只在规则 1 / 3 列出的两个场景必须用，撒在 @click handler 上是过度防御（不致命但污染代码）。

### 反思 — 这次"过度防御"被纠错的元教训

moyu 之前一连串撞 $nuxt null 时，我（们）总结的工程规则**没错**，但**容易被泛化滥用**。"任何依赖 Nuxt context 的代码都该用 runWithContext 守门" 听起来合理，实际只在两个特定场景必须，剩下场景撒了：

- ✗ 不会修 bug（路径 2 已经 work，不缺路径 1）
- ✗ 增加心智负担（每个 await 都问"要不要包"）
- ✗ 让真正必须用的场景反而**藏在噪声里**

这次 moyu 找到孤儿 store 才意识到：**之前那些 runWithContext 包装其实并没有解决任何 bug，只是恰好与 $nuxt-null 报告同时出现，被我误归因**。这是个经典的"相关 ≠ 因果"工程教训：观察到"加了 X 之后 bug A 消失了"不代表 X 修了 A，可能只是 A 本来就和 X 无关、由别的修复（v0.4.4 真根因 useKunMessage appContext graft）解决了。

下次给规则归因前，**严格分离"哪些 commit 改了哪些行为"**，不要把一连串改动打包归因到某个最显眼的那一条。

---

## §19 v0.4.7 — z-index 升档 + Select dropdown 选项溢出修复（2026-05-21）

moyu 又报了两个 UI bug：

1. **主页 hover topbar 元素 → tooltip / popover 出现在其他元素下方**
2. **KunSelect 弹出的选项溢出容器**

第 1 个是上一轮 v0.4.5 没考虑到 legacy app 的 sticky header 用 `z-9999` 这种 nuclear z-index，1500 还是不够竞争。第 2 个是 flex + truncate 一个经典坑，CSS 老问题但容易漏。

### 问题 1 — z-token 升档

#### 复盘 v0.4.5 的判断失误

v0.4.5 §16 把 popover token 设成 1500，假设是"app 侧 sticky header 顶多用 z-50 / z-100"。实际产线中：

- 老项目里的 navbar / topbar 不少用 `z-9999` 或 `z-[9999]`（来自不知道哪个旧 boilerplate）
- 第三方组件库（部分 admin template）默认 header 用 `z-1000` 或更高
- 我们 v0.4.5 的 popover @ 1500 在这些场景**仍然被压**

#### 修复 —— 整体抬到 9000-9999 区间

`packages/ui/app/styles/tailwindcss.css` 的 token 值升档：

```diff
- --z-kun-sticky: 30;
- --z-kun-modal: 1000;
- --z-kun-popover: 1500;
- --z-kun-alert: 2000;
- --z-kun-message: 9000;

+ --z-kun-sticky: 30;
+ --z-kun-modal: 9000;     /* was 1000 */
+ --z-kun-popover: 9300;   /* was 1500 */
+ --z-kun-alert: 9700;     /* was 2000 */
+ --z-kun-message: 9999;   /* was 9000 */
```

相对层级**保持不变**（modal < popover < alert < message），整体抬到 9000+ 区间，确保 KunUI 浮层在 99% 实际场景下能赢过 app 侧的 sticky / fixed 容器。

#### 为什么不直接所有都 z-99999

更高的值看着安全，但有真实成本：

- 跟 vendor 库的 toast / modal 协作变难（很多 vendor 写死 z-9999，你 z-99999 会盖住他们的）
- 调试时数值太大不直观
- 浏览器对 z-index 整数大小没有上限，但视觉上"9999 与 99999"用户无法区分

9000-9999 是"足够大到赢过 99% legacy 代码，但不至于把 vendor 库也碾过去"的平衡点。

#### 这个改动的语义边界

> **本节抬升 z-index 不是"用魔法数字硬刚"，是承认 KunUI 浮层在视觉栈中天然属于"最高优先级 UI"。**

弹出层之所以叫弹出层，就是因为**用户主动触发**它们时，期望它们盖住一切。这种语义在 v0.4.5 我用 1000-2000 区间表达得不够强，本轮纠正。

### 问题 2 — Select dropdown 选项溢出

#### 症状

`<KunSelect>` 的选项里如果有比 trigger button 宽的 label（典型的：长 URL / 长中文标题 / 长 SHA hash），下拉里这些 label **横向溢出 dropdown 边界**，可能盖住相邻 UI，也可能产生横向滚动条。

#### 根因 — flex + truncate 的经典坑

Select.vue 的 option `<li>`：

```vue
<!-- 之前（溢出） -->
<li class="... flex items-center justify-between ...">
  <span class="block truncate">{{ option.label }}</span>
  <KunIcon ... class="ml-2 shrink-0" />
</li>
```

`<span>` 是 `<li>` 的 flex item。**CSS 规定 flex item 的 `min-width` 默认为 `auto`，等价于 `min-content`**。对一段不可断行的长文本（特别是 CJK 之外的长 URL / hash），min-content 就是整段文本宽度 —— 等于 span 不会缩。

`truncate` shorthand 包含 `overflow:hidden; text-overflow:ellipsis; white-space:nowrap`。但 truncate 的前提是**容器宽度小于内容宽度时才裁切**。如果容器（span）自身因为 `min-width: auto` 被撑到内容宽度，**truncate 永远不触发**，文本完整渲染并撑爆父级。

#### 修复 — `min-w-0 flex-1` 标准修法

```vue
<!-- 之后 -->
<li class="... flex items-center justify-between ...">
  <span class="block min-w-0 flex-1 truncate">{{ option.label }}</span>
  <KunIcon ... class="ml-2 shrink-0" />
</li>
```

- `min-w-0`：把 span 的 min-width 从 `min-content` 改成 0，**允许 flex 真的把它压缩到任意小**
- `flex-1`：告诉 flex 布局 span 占用所有剩余空间（基础 width:0, grow:1, shrink:1）
- `truncate`：此时容器宽度可能小于内容宽度，**ellipsis 真正生效**

trigger button 的 selected label 也有同款问题（长 selectedLabel 会撑爆 button 自身），一并修了。

#### 同时收紧 `<ul>` overflow

```diff
- class="scrollbar-hide overflow-auto rounded-md text-sm focus:outline-none"
+ class="scrollbar-hide overflow-x-hidden overflow-y-auto rounded-md text-sm focus:outline-none"
```

`overflow-auto` 是双轴 = 横向也可以滚动 = 如果有什么东西 truncate 没拦住，会出现横向滚动条。改成 `overflow-y-auto overflow-x-hidden` 保证**纵向可滚（多选项时）、横向永远裁切**。这是浮层下拉的标准 hardening。

### 这次"flex + truncate 不生效"的元教训

`truncate` 在非 flex 上下文（普通 block）工作得很自然，导致大家形成"truncate 是一个原子级 utility，写了就能用"的错觉。但 **flex 上下文里 `min-width: auto` 默认值是 flex item 的隐藏地雷**：

- truncate 不生效
- `width: X` 在 flex item 上也不生效（被 min-content 顶起来）
- `flex-shrink: 1` 写了也不收缩
- 直到加 `min-w-0` 才解锁

这是 CSS 历史上一个有争议的默认（CSS WG 在 2017 年讨论过改默认但因为兼容性放弃）。**对所有"在 flex 容器里要做 truncate / 自适应宽度"的 span / div**，先加 `min-w-0` 是安全网。

### 全仓审计 —— 还有哪里可能踩同款坑

```bash
# 这种组合是潜在 bug 点
grep -rn "flex.*truncate\|truncate.*flex" packages/ui/app/components --include="*.vue"
```

跑了一遍。结果只有 Select.vue 这一处有真实问题（其他出现 flex + truncate 组合的，要么 truncate 在正确层级，要么宽度由父级显式约束）。这次修就够了。

下次新增组件如果用到 truncate，**默认加 `min-w-0`**，纳入 KunUI lint 检查清单。

### KunUI 工程规则更新（第 6 条）

| # | 规则 |
|---|---|
| 1-5 | 略（见 §14 / §15 / §16 / §18） |
| 6 | **flex 容器里的 truncate 必须搭配 `min-w-0`**，否则不生效。视为"truncate 在 flex 里"的标配。 |

### 验证

- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
- 视觉手测：
  - 长 label option → 显示 ellipsis，不再溢出 dropdown
  - tooltip / popover 在 hover topbar / sticky header 时，正确显示在最上层
  - Modal 内打开 Select 仍然正常（popover > modal 关系保持）

---

## §20 v0.4.8 — z-token 终于真的生效（Tailwind v4 不自动从 `--z-*` 生成 utility）🔴🔴

### 诚实开头 —— 之前三轮"修复"都没生效

v0.4.5 / v0.4.6 / v0.4.7 三个版本里我陆续：

- 加 `--z-kun-*` token 到 `@theme` 块
- 把 9 个组件的硬编码 `z-50` / `z-1007` / `z-[7777]` 改成 `z-kun-popover` / `z-kun-modal` / `z-kun-message`
- 调整 token 数值（1000-9000 → 9000-9999）
- 写了三段 handoff 给 moyu / kungal 说"这次浮层 z-index 终于正确了"

**结果**：moyu 报告"popover 看着 OK，但 tooltip / message 还在其他元素下方，Select 选项还溢出"。我以为是 token 数值不够大。

直到这次我 grep 编译产物：

```bash
grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css
# (空 —— 一个匹配都没有)

grep -ho '\.rounded-kun-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css
# .rounded-kun-md{border-radius:var(--radius-kun-md)}
# .rounded-kun-lg{border-radius:var(--radius-kun-lg)}
# ... 5 个全在
```

**`rounded-kun-*` utility 正常生成；`z-kun-*` utility 一个都没生成。** 三轮所谓的 z-index 修复，**实际上从未被应用**。组件里写的 `class="z-kun-popover"` 等同于 `class="some-undefined-class"` —— 浏览器忽略，元素拿默认 `z-auto`。

### 根因 — Tailwind v4 不是所有 `@theme` 变量都自动生成 utility

Tailwind v4 的 `@theme` 自动生成 utility 类**只覆盖特定命名空间**：

| 自动生成 utility 的 `@theme` 前缀 | 例 |
|---|---|
| `--color-*` | `bg-*` / `text-*` / `border-*` |
| `--radius-*` | `rounded-*` ✅ 我们的 radius token 走这条命中 |
| `--spacing-*` | `p-*` / `m-*` / `gap-*` / `w-*` |
| `--font-*` | `font-*` |
| `--text-*` | `text-*` (size) |
| `--shadow-*` | `shadow-*` |
| `--animate-*` | `animate-*` |
| `--ease-*` / `--blur-*` / `--breakpoint-*` 等 | 各自对应 utility |

**不在自动生成的命名空间**：
- `--z-*`（无 utility 自动生成）
- `--cursor-*`
- `--list-*`
- 等等

我以为"radius 走通了，z 同理"，没核实。这是把"看起来对"等同于"实际对"的经典工程失误。

### 真正的修复 — Tailwind v4 的 `@utility` directive 显式声明

`packages/ui/app/styles/tailwindcss.css` 加：

```css
@utility z-kun-sticky {
  z-index: var(--z-kun-sticky);
}
@utility z-kun-modal {
  z-index: var(--z-kun-modal);
}
@utility z-kun-popover {
  z-index: var(--z-kun-popover);
}
@utility z-kun-alert {
  z-index: var(--z-kun-alert);
}
@utility z-kun-message {
  z-index: var(--z-kun-message);
}
```

这是 Tailwind v4 的 `@utility` directive —— 显式注册 utility 类。`var(--z-kun-*)` 让 `@theme` 里的 token 值参与，外层覆盖（`:root { --z-kun-modal: ... }`）依然生效。

`@theme` 里的 `--z-kun-*` 变量声明**保留**（用来定义默认值 + 让消费方覆盖），`@utility` 块**新增**（让 utility 类真正生成）。两者一起才能正常工作。

### 验证 — 这次真的看了输出

```bash
$ grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' apps/web/.output/public/_nuxt/*.css | sort -u
.z-kun-alert{z-index:var(--z-kun-alert)}
.z-kun-message{z-index:var(--z-kun-message)}
.z-kun-modal{z-index:var(--z-kun-modal)}
.z-kun-popover{z-index:var(--z-kun-popover)}
```

`z-kun-sticky` 没出现是因为本仓没组件用它（Tailwind v4 tree-shake 未引用的 utility），这是预期行为。其他四个**首次真实出现在编译产物里**。

### 为什么 v0.4.5/6/7 之间看起来"popover 在上方了"

完全是 DOM order 偶然性 ——

- Popover 被 Teleport 到 body 时挂载在 body 末尾，**渲染顺序晚于大部分页面元素**
- 在所有元素 z-index 都是 `auto` 的栈中，**后挂载的元素覆盖先挂载的**（CSS 默认行为）
- 所以 popover 看着像"在上方"，其实是 stacking-context 内的 painting order，根本没走 z-index

但其他组件挂载时机不同：

- `<KunMessage>` 第一次调用时挂载 MessageContainer（中等时机）
- `<KunTooltip>` 在 hover 触发时挂载，但有 `delayShow` 100ms → 挂载更晚
- Sticky topbar 是页面初始 render 一部分（早挂载），但 `position: sticky` + 浏览器对 sticky 在某些场景给一个隐式 stacking context

所以会出现"popover 看着 OK / tooltip / message 看着不 OK" 这种**完全无规律**的表现。本质是**所有这些浮层的 z-index 都没生效**，谁先谁后纯靠运气。

Select 的"溢出"看似是布局问题（v0.4.7 加 `min-w-0 flex-1` 处理过），但 moyu 那边复测仍报问题 —— 很可能是**Select dropdown 被 z-index 更高的元素遮住部分**，视觉上像"选项被切了 / 溢出到旁边"。z-index 修好后这个症状大概率自动消失。

### 教训 — 这次最该立成铁律的事

**修改 design token / Tailwind config / CSS 系统级的东西之后，必须 grep 编译产物确认 utility 真的生成了。** 不要止于"源码里写得对"。

具体操作：

```bash
# 任何 @theme / @utility 改动之后
pnpm -F your-app exec nuxt build
grep -ho '\.your-new-utility-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css | sort -u
# 期望：能看到你声明的所有 utility，每个都 deref 到对应 CSS 变量
```

如果上面 grep 空 → 你的 utility 没被 Tailwind 生成 → class 名是个空字符串 → 视觉上什么都没发生。

> **加进 KunUI 三铁律 → 现在变成四铁律 + 一个流程规则**：
>
> | # | 规则 | 来源 |
> |---|---|---|
> | 1-6 | 略 | §14 / §15 / §16 / §18 / §19 |
> | **流程** | **修改 Tailwind theme / utility / 任何系统级 CSS 后，grep 编译产物验证 utility 真实生成** | **§20（本节）** |

这条流程规则比之前那 6 条普通规则更基础 —— 它防的是**整套修复机制被静默架空**。比某一条具体 bug 更危险，因为发现晚（要等用户报告）+ 误导（前面三轮的 "fix" doc 看起来都对）。

### 反思 — 我为什么犯这个错

- **类比泛化**：`rounded-kun-md` 工作，我以为 `z-kun-popover` 同理。没意识到 Tailwind v4 的 `@theme` 自动 utility 是**精挑的白名单**，不是"`--xxx-*` 都行"
- **没看产出**：三轮修复都只确认"build 通过 + 视觉上某个场景 OK"，没拿编译产物 grep 验证
- **错误归因**：moyu 报 "tooltip 还在下方" 时，我以为是 token 数值不够，又升一档（v0.4.7）—— 还是没生效，但因为 popover 在 DOM 末尾**碰巧好**，我以为 fix 部分生效了

这次纠错是 moyu 的复测把误归因暴露出来。**不复测 = 工程债。**

### 给 kungal / moyu 的 v0.4.8 patch notes

> ▎ **v0.4.8 patch — z-index utility 终于真的生效** 🔴🔴
> ▎ v0.4.5/6/7 三轮加的 `--z-kun-*` token 实际上从未生成 `.z-kun-*` utility（Tailwind v4 不自动从 `--z-*` 生成 utility，只覆盖 `--color/--radius/--spacing/--font` 等特定命名空间）。tailwindcss.css 加 5 个 `@utility` 显式声明后修复。**所有下游必须重新同步 tailwindcss.css 并 rebuild**，否则所有"修过"的 z-index 浮层（Modal / Popover / Select / DatePicker / Tooltip / ContextMenu / Alert / Loli / MessageContainer）实际行为还是默认 `z-auto`。

---

## §21 v0.4.9 — KunSelect dropdown 真·溢出修复（height 维度的 flex+min-* 同款 bug）

moyu 在 v0.4.8 之后报"Select 还是溢出"。我先以为是同款 z-index 问题没生效。**第一轮调查南辕北辙** —— 跑去 moyu.moe 生产部署，发现是老 Next.js + HeroUI（用户最后澄清生产部署是旧项目，moyu 测试服在本地 `http://127.0.0.1:6969`）。这次教训是 [[feedback-debug-falsify-user-assumption]] 的反向应用 —— 我没先确认报告的运行环境。

切到 moyu 本地 dev server (`127.0.0.1:6969/galgame`) 用 Playwright 复现 + DOM probe，才看到真 bug：

### 真根因 — `size()` middleware 的 maxHeight 写到 outer div，但 outer div `overflow: visible`

probe 结果：

| 元素 | 关键属性 |
|---|---|
| outer floating div | `style="max-height: 240px"`、`overflow: visible`、height 实际 **240px** |
| inner `<ul>` | `overflow-y-auto`、scrollHeight 360px、`clientHeight = scrollHeight` → **不溢出自己**，**scroll 不触发** |

`size()` middleware 的 `apply` 把 `maxHeight: 240` 写在外层 div 上，但**外层默认 `overflow: visible`**。UL 按自然 content height (360px) 渲染，溢出外层的 240px 边界**直接画在外层之外**。UL 本身没溢出（height: auto = content），所以 `overflow-y-auto` 的 scroll 永远不触发。

视觉表现：dropdown 看起来"撑爆"了它的 240px 限制，多出的 120px **叠加在下层卡片上**（被用户感知为 "Select 溢出容器"）。

### 修复 — 让 maxHeight 真正约束 UL

```diff
-<div ... :class="cn('bg-content1 ... z-kun-popover border p-1 shadow-lg', roundedClass)">
+<div ... :class="cn('bg-content1 ... z-kun-popover flex flex-col overflow-hidden border p-1 shadow-lg', roundedClass)">
-  <ul class="scrollbar-hide overflow-x-hidden overflow-y-auto rounded-md ...">
+  <ul class="scrollbar-hide min-h-0 flex-1 overflow-x-hidden overflow-y-auto rounded-md ...">
```

两个加的 class 各自负责一件事：

| 加的 class | 作用 |
|---|---|
| outer `flex flex-col` | 让 UL 在 flex column 上下文里，可以受 outer 高度约束 |
| outer `overflow-hidden` | 兜底，防止任何子节点溢出 outer 边界后视觉可见 |
| ul `flex-1` | UL fill outer 的可用高度（受 outer maxHeight 约束） |
| **ul `min-h-0`** | **解锁 flex item 的 `min-height: auto` 默认，让 UL 可以被压到 content 高度之下** |

`min-h-0` 是关键 —— 跟 v0.4.7 给 truncate span 加的 `min-w-0` **完全同款 bug，只是换到 height 维度**：

> **flex item 的 `min-width: auto` / `min-height: auto` 默认值 = `min-content`**，意味着 flex item 无法被父级的尺寸约束压缩到内容自然尺寸之下。要让父级 max-* 约束真正传递到 flex item，必须 `min-w-0` / `min-h-0` 显式解锁。

### 验证（Playwright + DOM probe）

修复前：

```
outer:   height: 240px  overflow: visible
ul:      height: 360px  scrollHeight: 360  clientHeight: 360  (no scroll triggered)
visual:  options 1-9 painted, options 7-10 overlap onto card grid below
```

修复后：

```
outer:   height: 240px  overflow: hidden
ul:      height: 230.4px (= outer 240 - p-1*2)  scrollHeight: 360  clientHeight: 230
scroll:  works, scrollTop: 0 → 100 → 最后一项 "其它" 可滚动到可见
visual:  dropdown cleanly capped at 240px; cards below fully visible
```

### v0.4.7 → v0.4.9 修法相互呼应

| 版本 | 维度 | flex 容器 | 被约束元素 | 解锁 prop |
|---|---|---|---|---|
| v0.4.7 | **width** | `<li class="flex">` | `<span class="truncate">` (label) | `min-w-0` |
| v0.4.9 | **height** | outer floating div `<div class="flex flex-col">` | `<ul>` (option list) | `min-h-0` |

两次修复机制完全对称。**KunUI 工程规则 6 应该扩展**：

| # | 旧 (v0.4.7) | 修正 (v0.4.9) |
|---|---|---|
| 6 | flex 容器里的 truncate 必须搭配 `min-w-0` | flex 容器里**任何要受父级尺寸约束的子元素**都要搭配对应的 `min-w-0` / `min-h-0`。truncate / overflow-auto / max-height 等约束都隐含这个前提 |

### 反思 — 这次第一轮跑偏的元教训

用户报 "Select 还溢出" → 我直接以为是 v0.4.8 z-index 漏的某个边角 → 跑去**生产部署**测 → 发现是 HeroUI → 写了一大段"用户看的不是 KunSelect"的报告。**全是错的**。

用户提醒"测试项目跑在本地 127.0.0.1:6969"后，我立刻测对了环境 + 用 DOM probe 抓到真 bug。从误诊 → 真修，时间差 5 分钟。

教训：**用户报告任何视觉 bug，第一件事是确认我看的环境跟用户看的环境一致**：

| 用户说 | 我必须先确认 |
|---|---|
| "本地" | 哪个端口？是 dev server 还是 build 产物？ |
| "moyu / kungal 上有 bug" | 部署版还是本地 dev？同一 commit 吗？ |
| "看 /xxx 页" | 拉一下页面 DOM 看是不是 KunUI 渲染的（react-aria 标记？data-slot？） |
| "modal / popover" | DOM 里搜 `KunModal` / `kun-` id prefix 确认是 KunUI |

5 行 Playwright `evaluate` 就能定性，比凭直觉硬猜快得多。这条加入 [[feedback-debug-falsify-user-assumption]]。

### 验证 (本仓 build + moyu HMR)

- `pnpm -F web exec nuxt build` ✅
- 同步 `Select.vue` 到 moyu 仓 + HMR 自动刷新 → Playwright DOM probe + 截图 "FIXED ✓"
- scroll 行为：`scrollTop: 0 → 100`，最后一项 "其它" 滚动到可见 ✅
- 卡片网格不再被 dropdown 选项 visually overlap ✅

### 给 moyu 的同步指令

```bash
KUN_OAUTH=/home/kun/Desktop/code/website/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue \
   /home/kun/Desktop/code/website/kun-galgame-patch-next/packages/ui/app/components/kun/select/Select.vue
```

（我刚才已经帮你 cp 了一次到 moyu 本地仓做验证，所以你那边的 dev server 现在应该已经热重载到正确状态。Git 也会显示这次 sync 是 modified —— 别 revert）

---

*文档维护：每次完成批次后更新对应章节的 checkbox。新增组件评审进 §2 表 + §3 新章节。*
