# KunModal

KunModal 是 KunUI 最早的浮层组件（v0.0.1 起就在）。v0.1.0 修了 prop 命名（`modalValue` → `modelValue`），v0.2.0 接入 focus-trap，所有后续 drawer / dialog / lightbox 都共享同一套 mechanics。

## v0.1.0 — KunModal inventory（§3.2 + §1.6）

§3.2 列的 inventory：

- `modalValue` → `modelValue`（§1.6）
- 滚动锁直接 `document.body.style.overflow = 'hidden'`，**多个 Modal 嵌套时**内层关闭就解锁外层应该锁定的滚动。改为 `useScrollLock` (vueuse) 或自己 ref-count
- 没有 focus trap：Tab 键能漂出 Modal
- `z-1007` 硬编码 → CSS var

### §1.6 — Modal 拼写成 `modalValue`

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

## v0.2.0 — Focus trap 接入（§9）

**Modal.vue** 加 `useFocusTrap` from `@vueuse/integrations`：

- Tab / Shift+Tab 不会再漂出 Modal
- 关闭时自动 restore focus 到打开前的元素
- `escapeDeactivates: false` —— Modal 自己处理 Escape，不让 trap 抢
- `allowOutsideClick: true` —— 允许背景点击关闭（Modal 的 `isDismissable` 仍然生效）
- 嵌套 Modal：内层 activate 时外层自动 deactivate；内层 close 后焦点回外层（trap 库本身的栈语义）

Lightbox 通过 KunModal 自动继承 trap，无需单独改。

## Modal mechanics that other components reuse

KunModal 在 v0.1.0 → v0.2.0 之间累积起来的这套"阻塞浮层 mechanics"，是 KunUI 里所有阻塞类浮层（Modal / Lightbox / Drawer）共享的基础设施。新建同类组件时直接 wire-up，**零学习成本**：

| Mechanic | 实现 | 复用者 |
|---|---|---|
| **`useBodyScrollLock`** | vueuse scroll lock + ref-count 处理嵌套 | KunModal / KunDrawer |
| **`useFocusTrap`** | `@vueuse/integrations` + `focus-trap`；`escapeDeactivates: false` / `allowOutsideClick: true` / `returnFocusOnDeactivate: true` | KunModal / KunLightbox（通过 Modal）/ KunDrawer |
| **Escape 关闭** | `useEventListener('keydown')` + `isDismissable` 控制 | KunModal / KunDrawer |
| **Teleport pattern** | `<Teleport to="body">` 把浮层 portal 出去，避开父级 `overflow: hidden` 裁切 | KunModal / KunDrawer / KunPopover / KunTooltip / KunSelect / KunDatePicker |
| **z-token `z-kun-modal`** | Tailwind v4 `@utility`-generated z-index utility（v0.4.8 起真的生效） | KunModal / KunDrawer 同层 |

KunDrawer v0.5.1 就是按这张清单**全部抄一遍** Modal 的实现，只换 anchor placement 和 slide 方向。这也是 KunDrawer 在 §23 明确写"消费方零学习成本"的根因。
