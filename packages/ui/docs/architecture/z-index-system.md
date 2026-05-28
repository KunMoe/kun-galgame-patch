# KunUI z-index 设计系统

> 跨三个版本（[v0.4.5](../changelog/v0.4.5.md) → [v0.4.7](../changelog/v0.4.7.md) → [v0.4.8](../changelog/v0.4.8.md)）演进的浮层 z-index token 体系。
>
> **当前状态（v0.4.8+）**：5 层 token + Tailwind v4 `@utility` 显式声明，**真正生效**。
>
> 前两轮（v0.4.5 用 1000-9000 区间、v0.4.7 升档到 9000-9999）虽然源码看着对，但 `@theme --z-*` 在 Tailwind v4 不自动生成 utility，三轮"修复"实际上 utility 一直没生成。v0.4.8 加 `@utility` directive 才真的工作。详见 [verify-build-artifact.md](../lessons/verify-build-artifact.md)。

## 当前层级（v0.4.8 起）

```
sticky (30) < modal (9000) < popover (9300) < alert (9700) < message (9999)
```

| Token | 值 | 用途 | 典型组件 |
|---|---|---|---|
| `--z-kun-sticky` | 30 | 文档流内的 sticky / 局部固定元素 | sticky header / floating button（页面内） |
| `--z-kun-modal` | 9000 | 全局阻塞容器（dialog） | KunModal |
| `--z-kun-popover` | 9300 | 用户主动触发的浮层 | KunPopover / KunSelect / KunDatePicker / KunTooltip / KunContextMenu |
| `--z-kun-alert` | 9700 | 阻塞确认对话框 | KunAlert |
| `--z-kun-message` | 9999 | 通知 toast | KunMessage / KunLoli |

**核心层级规则**：

> 一个浮层永远在"触发它的层"之上。

- popover 高于 modal → 在 Modal 内打开 Select 时下拉在 Modal **之上** ✓
- alert 高于 popover → KunAlert 确认对话框打开时永远盖住普通 popover ✓
- message 最高 → toast 总能被看见，即便在 alert / modal 之上 ✓

## CSS 声明（packages/ui/app/styles/tailwindcss.css）

### `@theme` 块声明 token 默认值

```css
@theme {
  /* ... 既有 color / radius tokens ... */
  --z-kun-sticky: 30;
  --z-kun-modal: 9000;
  --z-kun-popover: 9300;
  --z-kun-alert: 9700;
  --z-kun-message: 9999;
}
```

### `@utility` 块**必须**显式生成 utility 类

Tailwind v4 不自动从 `--z-*` 生成 utility（只覆盖 `--color/--radius/--spacing/--font/--text/--shadow/--animate/--ease/--blur/--breakpoint` 等白名单命名空间）。要让 `class="z-kun-popover"` 真的生效，**必须**加 `@utility` directive：

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

`@theme` 里的 `--z-kun-*` 变量声明**保留**（用来定义默认值 + 让消费方覆盖），`@utility` 块**新增**（让 utility 类真正生成）。两者一起才能正常工作。

## 验证 — 必须 grep 编译产物

```bash
$ pnpm -F web exec nuxt build
$ grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' apps/web/.output/public/_nuxt/*.css | sort -u
.z-kun-alert{z-index:var(--z-kun-alert)}
.z-kun-message{z-index:var(--z-kun-message)}
.z-kun-modal{z-index:var(--z-kun-modal)}
.z-kun-popover{z-index:var(--z-kun-popover)}
```

`z-kun-sticky` 不出现是因为本仓没组件用它（Tailwind v4 tree-shake 未引用的 utility），这是预期行为。**如果 grep 空 → utility 没生成 → 视觉上什么都没发生**。

## 9 个组件的迁移

| 组件 | 旧 | 新 | 备注 |
|---|---|---|---|
| `Modal.vue` | `z-1007` | `z-kun-modal` | |
| `Popover.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix**（之前压在 Modal 下方） |
| `select/Select.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `date-picker/Picker.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `tooltip/Tooltip.vue` | `z-50` 🔴 | `z-kun-popover` | **bug fix** |
| `context-menu/ContextMenu.vue` | `z-[1100]` | `z-kun-popover` | 语义对齐 |
| `alert/Alert.vue` | `z-2000` | `z-kun-alert` | |
| `alert/Loli.vue` | `z-2000` | `z-kun-message` | toast 性质，应在 message 层 |
| `alert/MessageContainer.vue` | `z-[7777]` | `z-kun-message` | |

## 故意不动的 z-index

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

## 消费侧覆盖

下游 app 如果需要与第三方 widget z-index 对接（比如某个 vendor 库的 modal 写死 z-9999），可以在自己的 `:root` 里覆盖：

```css
:root {
  --z-kun-modal: 9998;     /* 让 KunModal 比那个 vendor modal 低 1 */
  --z-kun-popover: 10500;  /* KunSelect 仍然高于 vendor modal */
}
```

整套 token 化的本质是**让全局浮层栈成为可配置的 design system，而不是 9 个文件里 9 个 magic number**。

---

## 三轮演进的反思

### v0.4.5 — 初版（1000-9000 区间）

最初推上日程是因为用户报"Select 在 Modal 内被压住"。v0.1.x 起 z-index 一直是各组件自己拍脑袋写的硬编码值：Modal 在 `z-1007`，四个 popover 类组件在 `z-50`。Teleport 让 popover 离开 Modal 的 DOM 子树，**但没改变 z-index 排序** —— body 下两个并列子树 z-50 vs z-1007，Modal 视觉上完全压住 popover。

初版 token：

```css
--z-kun-sticky: 30;
--z-kun-modal: 1000;
--z-kun-popover: 1500;
--z-kun-alert: 2000;
--z-kun-message: 9000;
```

**反思**：v0.1.0 §1.9 "其他小型一致性问题"里早就列过 "z-index 散落硬编码"这条，标"低优"留到后面。视觉层级 bug 在没人实际把 Select 放进 Modal 之前是看不见的，但放进去那天就是 critical。**design system 类的一致性 issue 即使当前没有可见报错，也应该作为"在产线之前先做完"的功课**。

### v0.4.7 — 整体升档到 9000-9999

moyu 报"主页 hover topbar 元素 → tooltip / popover 出现在其他元素下方"。复盘 v0.4.5：

- 老项目里的 navbar / topbar 不少用 `z-9999` 或 `z-[9999]`（来自不知道哪个旧 boilerplate）
- 第三方组件库（部分 admin template）默认 header 用 `z-1000` 或更高
- 我们 v0.4.5 的 popover @ 1500 在这些场景**仍然被压**

修复：整体抬到 9000-9999 区间，相对层级保持不变。

**为什么不直接所有都 z-99999**：

- 跟 vendor 库的 toast / modal 协作变难（很多 vendor 写死 z-9999，你 z-99999 会盖住他们的）
- 调试时数值太大不直观
- 浏览器对 z-index 整数大小没有上限，但视觉上"9999 与 99999"用户无法区分

9000-9999 是"足够大到赢过 99% legacy 代码，但不至于把 vendor 库也碾过去"的平衡点。

### 语义边界

> **抬升 z-index 不是"用魔法数字硬刚"，是承认 KunUI 浮层在视觉栈中天然属于"最高优先级 UI"。**

弹出层之所以叫弹出层，就是因为**用户主动触发**它们时，期望它们盖住一切。

### v0.4.8 — `@utility` directive 让 utility 终于真的生效

诚实开头：v0.4.5 / v0.4.6 / v0.4.7 **三轮"修复"实际上从未应用**。

直到 grep 编译产物：

```bash
grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css
# (空 —— 一个匹配都没有)
```

`rounded-kun-*` utility 正常生成（因为 `--radius-*` 在 Tailwind v4 自动 utility 白名单里）；`z-kun-*` 一个都没生成（`--z-*` 不在白名单）。三轮所谓的 z-index 修复，组件里写的 `class="z-kun-popover"` 等同于 `class="some-undefined-class"` —— 浏览器忽略，元素拿默认 `z-auto`。

修复：加 5 个 `@utility` 显式声明（见上文 CSS 声明部分）。

**为什么 v0.4.5/6/7 之间看起来"popover 在上方了"**：完全是 DOM order 偶然性。Popover 被 Teleport 到 body 时挂载在 body 末尾，**渲染顺序晚于大部分页面元素** —— 在所有元素 z-index 都是 `auto` 的栈中，**后挂载的元素覆盖先挂载的**（CSS 默认行为）。所以 popover 看着像"在上方"，其实是 painting order，根本没走 z-index。

其他组件挂载时机不同：

- `<KunMessage>` 第一次调用时挂载（中等时机）
- `<KunTooltip>` 在 hover 触发时挂载 + `delayShow` 100ms → 挂载更晚
- Sticky topbar 是页面初始 render（早挂载），但 `position: sticky` 在某些场景给隐式 stacking context

所以会出现"popover 看着 OK / tooltip / message 看着不 OK" 这种**完全无规律**的表现。本质是所有这些浮层的 z-index 都没生效，谁先谁后纯靠运气。

---

## 跨三轮的元教训

1. **design token 类的一致性 issue 应该在产线之前先做完**（v0.4.5 反思 — §1.9 TODO 早识别但被低估优先级）
2. **数值要赢过真实 legacy 代码**，不只赢过理想假设（v0.4.7 反思 — 9000-9999 平衡点）
3. **修改 design token / Tailwind config / CSS 系统级的东西后必须 grep 编译产物**确认 utility 真的生成（v0.4.8 反思 — 详见 [verify-build-artifact.md](../lessons/verify-build-artifact.md)）

第 3 条比前两条更基础 —— 它防的是**整套修复机制被静默架空**。三轮 z-token 演进里，前两轮的"看着改对了"全部是错觉，直到 v0.4.8 才真正落地。
