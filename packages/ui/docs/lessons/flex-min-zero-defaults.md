# flex item 的 min-width/min-height: auto 默认值是隐藏地雷

> 首次识别于 [v0.4.7](../changelog/v0.4.7.md) — KunSelect dropdown 选项 horizontal 溢出修复（`min-w-0`）。
> 同款 bug 换到 height 维度在 [v0.4.9](../changelog/v0.4.9.md) 再次出现（`min-h-0`）。两次修复机制完全对称。

## CSS 规则提醒

> **flex item 的 `min-width: auto` / `min-height: auto` 默认值 = `min-content`**，意味着 flex item 无法被父级的尺寸约束压缩到内容自然尺寸之下。要让父级 max-* 约束真正传递到 flex item，必须 `min-w-0` / `min-h-0` 显式解锁。

这是 CSS 历史上一个有争议的默认（CSS WG 在 2017 年讨论过改默认但因为兼容性放弃）。**对所有"在 flex 容器里要做 truncate / 自适应宽度 / 受 max-height 约束"的 span / div**，先加 `min-w-0` / `min-h-0` 是安全网。

## v0.4.7 — width 维度的坑（truncate）

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

### 元教训

`truncate` 在非 flex 上下文（普通 block）工作得很自然，导致大家形成"truncate 是一个原子级 utility，写了就能用"的错觉。但 **flex 上下文里 `min-width: auto` 默认值是 flex item 的隐藏地雷**：

- truncate 不生效
- `width: X` 在 flex item 上也不生效（被 min-content 顶起来）
- `flex-shrink: 1` 写了也不收缩
- 直到加 `min-w-0` 才解锁

## v0.4.9 — height 维度的坑（max-height + overflow-auto）

### 真根因 — `size()` middleware 的 maxHeight 写到 outer div，但 outer div `overflow: visible`

KunSelect dropdown 用 floating-ui 的 `size()` middleware 把 `maxHeight: 240` 写在外层 floating div 上，DOM probe 结果：

| 元素 | 关键属性 |
|---|---|
| outer floating div | `style="max-height: 240px"`、`overflow: visible`、height 实际 **240px** |
| inner `<ul>` | `overflow-y-auto`、scrollHeight 360px、`clientHeight = scrollHeight` → **不溢出自己**，**scroll 不触发** |

外层 div `overflow: visible` 时，UL 按自然 content height (360px) 渲染，溢出外层的 240px 边界**直接画在外层之外**。UL 本身没溢出（height: auto = content），所以 `overflow-y-auto` 的 scroll 永远不触发。

视觉表现：dropdown 看起来"撑爆"了它的 240px 限制，多出的 120px **叠加在下层卡片上**。

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

`min-h-0` 是关键 —— 跟 v0.4.7 给 truncate span 加的 `min-w-0` **完全同款 bug，只是换到 height 维度**。

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

## v0.4.7 → v0.4.9 修法相互呼应

| 版本 | 维度 | flex 容器 | 被约束元素 | 解锁 prop |
|---|---|---|---|---|
| v0.4.7 | **width** | `<li class="flex">` | `<span class="truncate">` (label) | `min-w-0` |
| v0.4.9 | **height** | outer floating div `<div class="flex flex-col">` | `<ul>` (option list) | `min-h-0` |

两次修复机制完全对称。

## 工程规则

> **flex 容器里任何要受父级尺寸约束的子元素**都要搭配对应的 `min-w-0` / `min-h-0`。truncate / overflow-auto / max-height 等约束都隐含这个前提。

具体场景清单：

- 父级是 `flex` 或 `flex-col` 且子元素要 `truncate` → 子元素加 `min-w-0`
- 父级是 `flex-col` 且子元素 `overflow-y-auto` 要受父级 max-height 约束 → 子元素加 `min-h-0`
- 父级是 `flex` 且子元素 `overflow-x-auto` 要受父级 max-width 约束 → 子元素加 `min-w-0`
- 任何 `flex-1` 的子元素其实都隐含需要对应的 `min-*-0`（除非内容尺寸天然就小于可分配空间）

## 全仓审计

```bash
# 这种组合是潜在 bug 点
grep -rn "flex.*truncate\|truncate.*flex" packages/ui/app/components --include="*.vue"
```

v0.4.7 跑过一遍。结果只有 Select.vue 这一处有真实问题（其他出现 flex + truncate 组合的，要么 truncate 在正确层级，要么宽度由父级显式约束）。

下次新增组件如果用到 truncate / overflow-auto + max-height 组合，**默认加对应 `min-w-0` / `min-h-0`**，纳入 KunUI lint 检查清单。
