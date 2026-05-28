# 改 Tailwind theme / utility 后必须 grep 编译产物

> 首次识别于 [v0.4.8](../changelog/v0.4.8.md) — z-token 终于真的生效（Tailwind v4 不自动从 `--z-*` 生成 utility）。
>
> 这是 v0.4.5 / v0.4.6 / v0.4.7 三轮看似"已修"的 z-index 系统实际上**utility 一直没生成**留下的元教训。比某一条具体 bug 更危险，因为发现晚（要等用户报告）+ 误导（前面三轮的 "fix" doc 看起来都对）。

## 诚实开头 —— 之前三轮"修复"都没生效

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

## 根因 — Tailwind v4 不是所有 `@theme` 变量都自动生成 utility

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

## 真正的修复 — Tailwind v4 的 `@utility` directive 显式声明

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

## 验证 — 这次真的看了输出

```bash
$ grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' apps/web/.output/public/_nuxt/*.css | sort -u
.z-kun-alert{z-index:var(--z-kun-alert)}
.z-kun-message{z-index:var(--z-kun-message)}
.z-kun-modal{z-index:var(--z-kun-modal)}
.z-kun-popover{z-index:var(--z-kun-popover)}
```

`z-kun-sticky` 没出现是因为本仓没组件用它（Tailwind v4 tree-shake 未引用的 utility），这是预期行为。其他四个**首次真实出现在编译产物里**。

## 为什么 v0.4.5/6/7 之间看起来"popover 在上方了"

完全是 DOM order 偶然性 ——

- Popover 被 Teleport 到 body 时挂载在 body 末尾，**渲染顺序晚于大部分页面元素**
- 在所有元素 z-index 都是 `auto` 的栈中，**后挂载的元素覆盖先挂载的**（CSS 默认行为）
- 所以 popover 看着像"在上方"，其实是 stacking-context 内的 painting order，根本没走 z-index

但其他组件挂载时机不同：

- `<KunMessage>` 第一次调用时挂载 MessageContainer（中等时机）
- `<KunTooltip>` 在 hover 触发时挂载，但有 `delayShow` 100ms → 挂载更晚
- Sticky topbar 是页面初始 render 一部分（早挂载），但 `position: sticky` + 浏览器对 sticky 在某些场景给一个隐式 stacking context

所以会出现"popover 看着 OK / tooltip / message 看着不 OK" 这种**完全无规律**的表现。本质是**所有这些浮层的 z-index 都没生效**，谁先谁后纯靠运气。

## 教训 — 这次最该立成铁律的事

**修改 design token / Tailwind config / CSS 系统级的东西之后，必须 grep 编译产物确认 utility 真的生成了。** 不要止于"源码里写得对"。

具体操作：

```bash
# 任何 @theme / @utility 改动之后
pnpm -F your-app exec nuxt build
grep -ho '\.your-new-utility-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css | sort -u
# 期望：能看到你声明的所有 utility，每个都 deref 到对应 CSS 变量
```

如果上面 grep 空 → 你的 utility 没被 Tailwind 生成 → class 名是个空字符串 → 视觉上什么都没发生。

> **流程规则**：修改 Tailwind theme / utility / 任何系统级 CSS 后，grep 编译产物验证 utility 真实生成。
>
> 这条流程规则比之前那 6 条普通工程规则更基础 —— 它防的是**整套修复机制被静默架空**。比某一条具体 bug 更危险，因为发现晚（要等用户报告）+ 误导（前面三轮的 "fix" doc 看起来都对）。

## 反思 — 我为什么犯这个错

- **类比泛化**：`rounded-kun-md` 工作，我以为 `z-kun-popover` 同理。没意识到 Tailwind v4 的 `@theme` 自动 utility 是**精挑的白名单**，不是"`--xxx-*` 都行"
- **没看产出**：三轮修复都只确认"build 通过 + 视觉上某个场景 OK"，没拿编译产物 grep 验证
- **错误归因**：moyu 报 "tooltip 还在下方" 时，我以为是 token 数值不够，又升一档（v0.4.7）—— 还是没生效，但因为 popover 在 DOM 末尾**碰巧好**，我以为 fix 部分生效了

这次纠错是 moyu 的复测把误归因暴露出来。**不复测 = 工程债。**
