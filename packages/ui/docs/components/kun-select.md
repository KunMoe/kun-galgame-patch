# KunSelect

KunSelect 的设计与跨版本演化。原型期就是 KunUI 的核心 form primitive，v0.4.0 把 options 改成 generic + readonly，v0.4.7 修了长 label 横向溢出（width 方向的 flex+truncate 经典坑），v0.4.9 修了 dropdown maxHeight 不生效（height 方向的同款 bug）。

## Original design (v0.0.1+ — §3.5 inventory)

最初版本的 KunSelect 设计已经覆盖了"触发按钮 + 浮层 dropdown + options 列表"的基本形态，但 §3.5 列出几个明显缺口（这些后续都在 v0.2.x / v0.4.x 修了）：

- 缺键盘上下/回车选中
- 缺搜索过滤（项目里 series/tag 选择都需要）
- 缺 multi 模式（tag_ids / official_ids 编辑场景需要）
- `bg-white dark:bg-black`（§1.3 颜色泄漏）
- dropdown 在 Modal 内会被压住（z-10 < Modal 的 z-1007），需要 Teleport
- `defineModel`（§1.8）

v0.2.0 浮层引擎统一时（§9），KunSelect 接入了 `@floating-ui/vue` 的 `size()` middleware：

- 自动让下拉宽度 = 触发按钮宽度（旧版用 `w-full` 但不准）
- 自动 `maxHeight = Math.min(240, availableHeight - 8)` —— 视口空间不足时自动缩小，列表内部滚动
- Modal 内的 Select 不再被 Modal 的 z-1007 压住（Teleport + `z-50`）

## v0.4.0 — generic + readonly options（§11.1）

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

## v0.4.7 — Select dropdown 选项溢出（width direction）（§19 second half）

### 症状

`<KunSelect>` 的选项里如果有比 trigger button 宽的 label（典型的：长 URL / 长中文标题 / 长 SHA hash），下拉里这些 label **横向溢出 dropdown 边界**，可能盖住相邻 UI，也可能产生横向滚动条。

### 根因 — flex + truncate 的经典坑

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

### 修复 — `min-w-0 flex-1` 标准修法

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

### 同时收紧 `<ul>` overflow

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

## v0.4.9 — Select dropdown maxHeight 真溢出（height direction）（§21）

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
KUN_OAUTH=/home/kun/Desktop/code/website/kun-galgame-infra
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue \
   /home/kun/Desktop/code/website/kun-galgame-patch-next/packages/ui/app/components/kun/select/Select.vue
```

（我刚才已经帮你 cp 了一次到 moyu 本地仓做验证，所以你那边的 dev server 现在应该已经热重载到正确状态。Git 也会显示这次 sync 是 modified —— 别 revert）
