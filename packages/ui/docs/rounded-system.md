# KunUI 统一 rounded 系统（v0.3.0+）

> 给所有 stylistic 组件加一个统一的圆角调整入口，三层（CSS token / Vue provider / 组件 prop）按优先级覆盖。形状本质的组件（Avatar / Chip / dot Badge / Switch thumb / Slider thumb）不参与统一 —— 它们的圆角是语义。

## 三层架构

```
Layer 1 (最底层): CSS 变量
  packages/ui/app/styles/tailwindcss.css 的 @theme:
    --radius-kun-none / --radius-kun-sm / --radius-kun-md
    --radius-kun-lg   / --radius-kun-full

  Tailwind v4 自动生成 utility 类:
    rounded-kun-none / rounded-kun-sm / rounded-kun-md
    rounded-kun-lg   / rounded-kun-full

  全局微调:
    /* 在消费 app 自己的 css 里 */
    :root { --radius-kun-md: 0.375rem; }  /* 改小 md 这一档 */

Layer 2 (Vue 注入): useKunUIConfig provider
  import { provideKunUIConfig } from '@kun/ui/composables/useKunUIConfig'

  // 在 app.vue / 顶层 layout
  provideKunUIConfig({ rounded: 'lg' })
  // → 所有未单独传 rounded 的组件默认使用 lg

  // 局部 subtree
  <KunUIScope :rounded="'sm'">
    <ProfileForm />  <!-- 这个子树内组件默认 sm -->
  </KunUIScope>

Layer 3 (组件实例 prop): 最细粒度
  <KunCard rounded="full">
  <KunInput rounded="none">
  <!-- 覆盖 provider + theme -->
```

**优先级**：组件 prop > useKunUIConfig 提供值 > 组件内置 fallback > 全局默认 `'md'`

## 已接入的组件（v0.3.0 范围）

13 个 stylistic 组件全部接入 `rounded` prop + useResolvedRounded：

| 组件 | 默认 fallback | 备注 |
|---|---|---|
| `KunCard` | provider `md` | 主容器 |
| `KunButton` | provider `md` | |
| `KunModal` | **`lg`**（built-in） | 大表面更适合 lg |
| `KunInput` | provider `md` | |
| `KunTextarea` | provider `md` | |
| `KunSelect` | provider `md` | trigger + 下拉一致 |
| `KunPopover` | **`lg`**（built-in） | 浮层 |
| `KunTooltip` | provider `md` | |
| `KunDatePicker` | provider `md` | trigger + 下拉一致 |
| `KunTagInput` | provider `md` | 主容器（chip 内部固定 `md`） |
| `KunUpload` | **`lg`**（built-in） | 大 drop zone |
| `KunProgress` | provider `md` | 进度条 bar |
| `KunInfo` | **`lg`**（built-in） | 信息卡片 |

## 形状本质组件（不参与统一）

这些组件圆角是语义而非样式选择，**忽略** `useKunUIConfig`：

| 组件 | 形状 | 理由 |
|---|---|---|
| `KunAvatar` | `rounded-full` | 头像永远是圆 |
| `KunChip` | `rounded-full` | 胶囊标签是 chip 的定义 |
| `KunBadge` dot/count | `rounded-full` | 红点 / 数字徽章是圆 |
| `KunSwitch` thumb / Slider thumb | `rounded-full` | 控件 thumb 是圆 |
| `KunTab` `pills` variant | `rounded-full` | 这个 variant 的视觉定义 |
| `KunTab` 其他 variant | hardcoded `md` | 5 个 variant 各有独立视觉规范，统一意义不大 |

## 用法示例

### 项目级全局调整

在每个消费 app 的入口（`apps/web/app/app.vue` 或顶层 layout）：

```vue
<script setup lang="ts">
import { provideKunUIConfig } from '@kun/ui/composables/useKunUIConfig'

// 整个 oauth admin 后台用更紧凑的 sm
provideKunUIConfig({ rounded: 'sm' })
</script>
```

```vue
<!-- 或者 wiki 用更圆润的 lg -->
<script setup>
provideKunUIConfig({ rounded: 'lg' })
</script>
```

### 子树局部覆盖

```vue
<template>
  <!-- 顶层 provider 已经设 md，这里覆盖成 none -->
  <KunUIScope :rounded="'none'">
    <DataTable />
  </KunUIScope>
</template>

<script setup>
// 简单的内联 provider
import { provideKunUIConfig } from '@kun/ui/composables/useKunUIConfig'
provideKunUIConfig({ rounded: 'none' })
</script>
```

### 单个组件覆盖

```vue
<KunCard rounded="full">圆角卡片</KunCard>
<KunModal rounded="sm">紧凑模态框</KunModal>
<KunInput rounded="none">直角输入框</KunInput>
```

### 全局微调具体档位的 px 值

不改桶位语义，只调某档实际半径：

```css
/* 在你 app 自己的 css 里 */
:root {
  --radius-kun-md: 0.375rem;  /* 默认 0.5rem → 改成 0.375rem */
  --radius-kun-lg: 1rem;       /* 默认 0.75rem → 改成 1rem */
}
```

所有用 `rounded-kun-md` 的组件立即跟着变。

## 设计取舍

### 为什么不直接用 Tailwind 默认 `rounded-*`

Tailwind 默认的 `rounded-sm / rounded-md / rounded-lg` 是**独立**的 utility，改一个不影响另一个。我们想要"全局拖一个旋钮整体调圆角"，必须把所有组件指向**同一组**变量。

### 为什么不全靠 CSS variable，还要 Vue provider

CSS variable 调的是**具体档位的 px 值**（"md 是多大"）。Vue provider 调的是**组件用哪个档位**（"卡片默认用 md 还是 lg"）。两件不同的事，所以两层。

### 为什么形状本质组件不参与

如果 provider 设 `rounded: 'none'`，理论上头像、chip 也该变方。但视觉/语义上头像必须圆、chip 必须胶囊，跟着改会破坏组件的核心识别度。这些组件**忽略 provider** 是有意的。

### 为什么 Modal/Popover/Upload 有 built-in `'lg'` fallback

大尺寸的表面（modal / drop zone）用 lg 看着更舒服；小尺寸的（input / button）用 md 更精致。这种"每个组件自己有一个合理默认"通过 `useResolvedRounded(propFn, fallback)` 的第二个参数表达。

provider 只是"你没传 prop 也没设 fallback 时的全局默认" —— 这个默认本身是 'md'。

## kungal / moyu 同步说明

如果你们 fork 了 KunUI 副本，要拿到这个系统：

1. **同步文件**：
   ```
   packages/ui/app/styles/tailwindcss.css                  (加 5 个 --radius-kun-* token)
   packages/ui/app/composables/useKunUIConfig.ts           (新文件)
   packages/ui/app/components/kun/ui/rounded.ts            (新文件)
   ```

2. **同步 13 个改造过的组件**（详见 §"已接入的组件"列表）

3. **可选**：在你们 app 顶层加 `provideKunUIConfig({ rounded: 'xxx' })` 调整风格

4. **完全不调**：什么都不做，组件视觉跟 v0.2 完全一致（fallback 默认 'md' = `rounded-md` ≈ 0.5rem，与之前硬编码值一致）

## 测试

无运行时 breaking。所有现有调用零修改，视觉零差异（fallback 默认 'md' 对应 0.5rem，与原硬编码 `rounded-md` 完全相同）。

Build 验证：
- `pnpm -F web exec nuxt build` ✅
- `pnpm -F wiki exec nuxt build` ✅
