# KunTab

KunTab 的完整设计与跨版本演化。v0.1.0 完成 5 variant 重设计（取代旧 solid / underlined 二选一），v0.3.1 修了 solid / light variant 指示器 4px 错位的几何 bug。

## v0.1.0 — KunTab 重设计（§4）

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

## Subsequent fixes

### v0.3.1 — KunTab solid/light indicator 错位修复（2026-05-21）（§10）

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
