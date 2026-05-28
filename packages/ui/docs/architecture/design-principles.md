# KunUI 设计原则

> 提炼自 [v0.1.0 改进方案](../changelog/v0.1.0.md) 的 §1 跨切面问题 + §2 组件清单。这些是跨组件、长期生效的设计约束 —— 比单个组件的 bug 修复更值钱。
>
> 范围：`packages/ui/app/components/kun/` 下所有 ~40 个组件 + `composables/` + `utils/`。
> 目标消费方：kungal、moyu、wiki (kun-oauth-admin/apps/wiki)、oauth (kun-oauth-admin/apps/web)。

## 摘要

总体定位（自定义语义色 + variant × color 矩阵，HeroUI 风味）是对的，但**跨组件一致性 ~70%**，存在以下结构性问题：

1. **Tailwind 动态类拼接**：Tab / Progress 两处用 `` `bg-${color}` `` 这种字面量拼装，JIT 在生产构建下大概率不出色，是 latent bug。
2. **variant × color 颜色表重复了 4 份**：Button / Badge / Info / Progress 各写一份，已经在偏移。
3. **颜色体系泄漏**：Input / Textarea / Select 等组件混用 `text-red-500` 这类 Tailwind 固有色，违反 CLAUDE.md 规约。
4. **`KunUIColor` 类型缺 `info`**：实际 `Info.vue` 已经支持 7 色，但中央类型只列 6 色，下游 `<KunButton color="info">` 报 TS。
5. **Modal `modelValue` 拼成 `modalValue`**：靠 `v-model:modal-value` 显式绑定能 work，但默认 `v-model` 不行，违反约定。
6. **`KunLucide*` 自定义 SVG 包装组件**：项目已经接入 `@nuxt/icon`，再单独包 6 个 lucide 图标组件纯属冗余。
7. **`defineModel` 未采用**：所有组件还在手写 `modelValue` + `update:modelValue`，Slider 是唯一例外。机械化重构能让每个组件少 ~5 行。

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

## §2 组件清单 / 严重度索引

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
