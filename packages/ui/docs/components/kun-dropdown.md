# KunDropdown

click 触发的操作菜单 —— 标准 WAI-ARIA *menu button* 模式（按钮 → 弹出一组可点操作项）。v0.6.2 新增。

## 定位：它和其它浮层/菜单组件的边界

| 组件 | 触发 | 语义 | 场景 |
|---|---|---|---|
| **KunDropdown** | **click / 键盘** | `role="menu"` + `menuitem` | **一组可点的操作项**（编辑 / 删除 / 复制…）|
| KunPopover | click | `role="dialog"` | 通用浮层，任意内容 |
| KunContextMenu | 右键 / 点位 | item 列表（无 menu a11y）| 右键菜单、在坐标点弹出 |
| KunSelect | click | 表单 listbox | 表单单/多选取值 |
| KunTooltip | hover | `role="tooltip"` | 文字提示 |

## 为什么独立组件，而不是 `KunPopover` 加 `trigger='hover'/'menu'`

详细取舍见 [changelog/v0.6.2.md](../changelog/v0.6.2.md)。两句话：

1. **Popover 太封闭，compose 不出合规 menu** —— 内容容器 `role="dialog"` 写死、open 状态内部私有（不 emit、不暴露、自管 Esc/outside-click），menu 需要的 `role="menu"` + `aria-expanded` 驱动 + 焦点管理都接不进去。
2. 故 KunDropdown **直接 wrap `@floating-ui/vue`**（与 Popover 同套定位配方），自带交互 + a11y；**不动 Popover、不抽公共基座**（YAGNI，没有第三个浮层组件来指导抽象，真长出来再抽）。item 模型复用 `KunContextMenuItem`，视觉复用 `kunVariantClasses('light', color)`。

**没有 hover 触发**：刻意省略。hover 菜单不是 WAI-ARIA 模式（触屏无 hover、键盘/读屏够不着），本组件用例是"可点操作项"。

## 用法

```vue
<script setup lang="ts">
import type { KunDropdownItem } from '#layers/ui/.../dropdown/type' // 或直接用 KunContextMenuItem
const items: KunDropdownItem[] = [
  { key: 'edit', label: '编辑', icon: 'lucide:pencil' },
  { key: 'archive', label: '归档', icon: 'lucide:archive', disabled: true },
  { key: 'del', label: '删除', icon: 'lucide:trash-2', color: 'danger' },
]
const onSelect = (item: KunDropdownItem) => { /* 按 item.key 分发 */ }
</script>

<template>
  <KunDropdown :items="items" @select="onSelect">
    <template #trigger>
      <KunButton variant="flat">操作菜单</KunButton>
    </template>
  </KunDropdown>
</template>
```

`KunDropdownItem` 就是 `KunContextMenuItem` 的别名（一处定义，两组件共用）：`{ key; label; icon?; color?; disabled? }`。

## API

| Prop | 类型 | 默认 | 说明 |
|---|---|---|---|
| `items` | `KunDropdownItem[]` | `[]` | 菜单项；空时点击不打开 |
| `position` | `Placement` | `'bottom-start'` | 锚定方向，自动 flip/shift 进视口 |
| `triggerClass` | `string` | `''` | 触发器外层 class |
| `menuClass` | `string` | `''` | 菜单容器 class |
| `minWidth` | `number` | `192` | 菜单最小宽度（px）|
| `disabled` | `boolean` | `false` | 禁用整个触发器 |

- Emits：`select(item)` / `open()` / `close()`
- Expose：`open()` / `close()` / `toggle()`（命令式控制）

## 触发器约定

与 `KunPopover` 一致：把内容（或 `<KunButton>`）放进 `#trigger`，组件用 `div[role=button]` 包裹（不是真 `<button>`，避免 button 套 button）。menu button 的 ARIA（`aria-haspopup="menu"` / `aria-expanded` / `aria-controls`）挂在这个包裹层上。

## 键盘与无障碍（WAI-ARIA menu button）

| 操作 | 行为 |
|---|---|
| Click / Enter / Space / ↓ | 打开（↓/Enter/Space 同时聚焦首项）|
| ↑（触发器上）| 打开并聚焦末项 |
| ↑ / ↓ | 启用项间循环移动（roving tabindex，**跳过 disabled**）|
| Home / End | 首 / 末启用项 |
| Enter / Space（项上）| 触发 `select` → 关闭 → 焦点归位触发器 |
| Esc / Tab | 关闭 → 焦点归位触发器 |
| 点击外部 | 关闭 |

`role="menu"` 容器 / `role="menuitem"` 项 / disabled 项标 `aria-disabled` 且导航跳过。light 变体只定义 `hover:`，键盘焦点不触发 hover，故为 7 个语义色各补 `focus:` 高亮，键盘 roving 与鼠标 hover 视觉一致。

## 已验证

- `vue-tsc` 零报错；auto-import 名 `KunDropdown`。
- Playwright 实测：打开 / ↓ 导航跳过 disabled / Enter 选中并关闭归位 / Esc 关闭归位 / danger 红 / disabled 置灰 / 图标 / 定位，全部正确，走项目色系。

> 开发期编辑 layer（packages/ui）文件后，apps 的 Vite **客户端 bundle 可能短暂缓存旧版**（SSR 用新版 → 一次性 hydration mismatch 告警）。**重启 dev server 即清除**，非组件问题。
