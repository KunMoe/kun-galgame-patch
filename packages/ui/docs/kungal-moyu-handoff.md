# KunUI v0.0.1 → v0.1.1 交付清单

> 给 kungal / moyu 的 KunUI 同步文档。本文是 **纯增量**：列出你需要做什么，不讲历史。
> 设计动机、复审过程见同目录 `improvement-plan.md`。

---

## 0. TL;DR

| 维度 | 数字 |
|---|---|
| 新组件 | 2（`KunChip`、`KunTagInput`）+ 1 新版（`KunBadge` 重做为 dot/count）|
| 重命名 | 5（Modal v-model 修拼写、Badge→Chip、`KunIconLucide*` 删 5 个用 `KunIcon name="lucide:..."` 替代、`alert/Info.vue`→`alert/Loli.vue`、`alert/loli.ts`→`alert/loliAssets.ts`）|
| 删除 | 6（5 个 KunIconLucide* + `animation/GlassShatter.vue` 注释壳）|
| API 破坏 | 7 处（Modal / Card / Badge→Chip / Brand / Link / Tab variant 默认值 / Tab `hasScrollbar`→`scrollable`）|
| 新增 composable | 1（`useBodyScrollLock`，跨实例共享 body scroll 锁）|
| 新增静态 util | 1（`ui/variants.ts`，replaces dynamic `bg-${color}` template strings）|
| Bug 修复（你自动受益） | 11（DatePicker getMonth、Modal 嵌套 scroll lock、Avatar null 崩溃、Pagination 偷键、TagInput 长按 Backspace 等，无需你做事）|

### 下游强制必做（30 分钟工作量）

1. 复制 6 个新/重命名的源文件
2. 删 6 个 retired 文件
3. 跑 7 条 sed 改 call site
4. 跑 `nuxt build` 验证

---

## 1. 文件复制清单（从本仓 `packages/ui/app/components/kun/` 到你的 `components/kun/` 或等价目录）

### 1.1 新增文件（全文复制）

```
packages/ui/app/components/kun/Chip.vue                ← 新组件
packages/ui/app/components/kun/Badge.vue               ← 全文重写（旧 Badge 现叫 Chip；新 Badge 是 dot/count overlay）
packages/ui/app/components/kun/ui/type.d.ts            ← 加了 'info' 第 7 色
packages/ui/app/components/kun/ui/variants.ts          ← 新文件，七变体×七色静态 map
packages/ui/app/components/kun/tab/Tab.vue             ← 全文重写
packages/ui/app/components/kun/tab/type.d.ts           ← 变体改 5 种
packages/ui/app/components/kun/tag-input/TagInput.vue  ← 新组件
packages/ui/app/components/kun/tag-input/type.d.ts     ← 新类型
packages/ui/app/composables/useBodyScrollLock.ts       ← 新 composable（Modal 依赖）
packages/ui/app/components/kun/Popover.vue             ← v0.2.0 全文重写（@floating-ui/vue）
packages/ui/app/components/kun/tooltip/Tooltip.vue     ← v0.2.0 全文重写（+ 箭头 + delay）
packages/ui/app/components/kun/select/Select.vue       ← v0.2.0 全文重写（+ size middleware）
packages/ui/app/components/kun/date-picker/Picker.vue  ← v0.2.0 浮层引擎换 @floating-ui/vue
packages/ui/app/components/kun/Modal.vue               ← v0.2.0 加 focus-trap
```

### 1.1.5 v0.2.0 新增 npm 依赖（必装）

在你的项目根（或 `@kun/ui` 副本所在子包）：

```bash
pnpm add @floating-ui/vue @vueuse/integrations focus-trap
pnpm add -D vue-tsc eslint-plugin-vuejs-accessibility
```

不装的话上面 5 个 v0.2.0 文件 import 会全报错。

### 1.2 重命名 / 改名（先重命名再覆盖内容）

```
alert/Info.vue            →  alert/Loli.vue           （文件改名 + 内容沿用）
alert/loli.ts             →  alert/loliAssets.ts     （文件改名，避免 Nuxt 自动 import 命名冲突）
```

### 1.3 删除文件

```
animation/GlassShatter.vue                  ← 整文件删
icon/LucideAlertTriangle.vue                ← 删
icon/LucideCheckCircle2.vue                 ← 删
icon/LucideInfo.vue                         ← 删
icon/LucideX.vue                            ← 删
icon/LucideXCircle.vue                      ← 删
```

### 1.4 整改更新（修改 props/template/logic 但保留文件名）

| 文件 | 改动要点 |
|---|---|
| `Null.vue` | v0.2.1：import 路径 `../utils/` → `../../utils/`（其实 Nuxt 自动 import 兜底了，但 vue-tsc 严格模式 TS2307）|
| `button/Button.vue` | v0.2.1：**删本地 colorVariants 表，改 import `kunVariantClasses`**（不是"在本地表里给每个 variant 追加 `info: '...'`"——后者会把 v0.1.0 §1.2 刚消灭的"4 份重复 variant×color 表"问题原样引回。Button 是当时唯一漏改的组件，本次顺带收口）。**未来再加新色（第 8 色）时不要给任何组件加本地表，统一改 `ui/variants.ts` + `ui/type.d.ts` 两处**。|
| `select/Select.vue` | v0.2.1：`defineModel<string \| number>({ required: true })` —— 回调签名收紧不含 undefined |
| `shared/user.d.ts` | v0.2.2：保持 `KunUser.id`（v0.2.1 误改为 uid 已回滚 —— DB 列名 `id` 是真理之源；JWT claim/URL `uid` 只是 auth/transport 标签，不向上传播到数据/UI 层）|
| `Modal.vue` | `modalValue` → `modelValue`；scroll-lock 改 import composable；`v-model` 现在直接生效 |
| `Card.vue` | `isPressable` → `clickable`/`href` 拆分；移除 `href: '/'` 默认 |
| `Brand.vue` | 接受 `name` / `iconSrc` / `iconAlt` / `iconClass` / `badge` / `badgeColor` / `to` / `nameClass` props |
| `Input.vue` | `size: string` → `KunUISize`；红色硬编码 → `danger`；`defineModel` |
| `Textarea.vue` | `defineModel`；`focus:ring-primary-500` → `focus:ring-primary`；placeholder 默认 `''` |
| `Select.vue` | `defineModel`；type 删 `modelValue`；下拉用 `bg-content1` |
| `Switch.vue` | `defineModel` |
| `CheckBox.vue` | `defineModel`；加 `info` 色 |
| `Rating.vue` | `defineModel` |
| `Popover.vue` | `bg-content1` 替代 `bg-white dark:bg-black` |
| `Tooltip.vue` | 同上 |
| `DatePicker.vue` | `getMonth() + 1`；`bg-content1` |
| `Pagination.vue` | `<input>` 用 `border-default-200`；ArrowLeft/Right 加 ARIA role 黑名单不偷 focused widget 的键 |
| `Progress.vue` | gradient variant 改静态 map；圆环 stroke 改 `strokeColorClasses`（text-*）|
| `Avatar.vue` | `user` 类型允许 null/undefined；模板用 `user?.avatar` 防崩 |
| `avatar/type.d.ts` | `user: KunUser \| null \| undefined` |
| `Link.vue` / `link/type.d.ts` | 删 `tag` prop（无意义，NuxtLink 自动 fallback 到 `<a>`）|
| `Divider.vue` | 加 `info` 色 |
| `alert/MessageItem.vue` | 删 5 个 Lucide* import；图标改 `KunIcon name="lucide:circle-check"` 等 canonical 名 |
| `alert/Loli.vue` | import 路径改 `./loliAssets`（文件改名跟着改）|
| `useKunLoliInfo.ts` | import 路径同上 |

---

## 2. Call-site 替换（你的 `apps/` 下要跑的 sed）

### 2.1 Modal v-model 修拼写（最高频，约 20 处）

```bash
find apps -name '*.vue' -not -path '*/node_modules/*' -print0 | xargs -0 sed -i \
  -e 's/v-model:modal-value=/v-model=/g' \
  -e 's/:modal-value=/:model-value=/g' \
  -e 's/@update:modal-value=/@update:model-value=/g'
```

### 2.2 Badge → Chip 重命名

```bash
find apps -name '*.vue' -o -name '*.ts' | xargs grep -l 'KunBadge\b' \
  | xargs sed -i 's/KunBadge\b/KunChip/g'
```

**注意**：如果你确实想用新的 dot/count Badge（数量徽章），那是另一回事 —— 新 `<KunBadge variant="count" :count="..." color="danger">` 是完全不同的语义。

### 2.3 Card `isPressable` → `clickable` / `href`

```bash
grep -rn 'is-pressable\|isPressable' apps --include='*.vue'
```

逐处手动判断意图：
- 你确实想要它**导航**：改成 `:href="..."`
- 你确实想要它响应 `@click`：改成 `clickable` 并自己处理 `@click`
- 之前可能依赖了"`isPressable=true` 默认跳首页"的隐式行为 —— 这是个 bug，现在不会再发生

### 2.4 KunIconLucide* → KunIcon

```bash
find apps -name '*.vue' | xargs sed -i \
  -e 's/<KunIconLucideAlertTriangle/<KunIcon name="lucide:triangle-alert"/g' \
  -e 's/<KunIconLucideCheckCircle2/<KunIcon name="lucide:circle-check"/g' \
  -e 's/<KunIconLucideInfo/<KunIcon name="lucide:info"/g' \
  -e 's/<KunIconLucideXCircle/<KunIcon name="lucide:circle-x"/g' \
  -e 's/<KunIconLucideX/<KunIcon name="lucide:x"/g'
```

**注意**：这是单标签替换，闭合标签 `</KunIconLucideX>` 等也要相应处理，但实践中 KunIcon 通常自闭合。请 grep 完手动收尾。

### 2.5 Brand 不再硬编码 `kungal.titleShort`

如果你之前用 `<KunBrand />` 无参，现在要显式传 props：

```vue
<!-- 旧 -->
<KunBrand />

<!-- 新（kungal）-->
<KunBrand name="鲲 Galgame" badge="论坛" />

<!-- 新（moyu）-->
<KunBrand name="Hikari" badge="Patch" badge-color="secondary" />
```

### 2.6 Link `tag` prop 已删

```bash
grep -rn '<KunLink.*tag=' apps --include='*.vue'
```

逐处删 `tag` 属性。功能不变：NuxtLink 对外链自动 fallback 到 `<a>`。

### 2.7 KunUser 字段名澄清（v0.2.2 决定）

**结论**：KunUser 保留 `id`，**不**改 `uid`。如果你的项目里把用户对象的主键字段叫 `uid`，你这边要 rename 回 `id` 来对齐 @kun/ui。

**为什么**：

- 真理之源是 DB（Prisma user.id 列）
- 上游链：DB `id` → Go DTO `json:"id"`（apps/api/.../oauth_dto.go 注释明确把它列为 FK 不变量）→ nitro response 类型 `id` → KunUser `id`
- 你看到的 `uid` 出现在：
  - JWT claim 名字（OAuth 内部）
  - URL 路由参数 `[id]`（routing 层 label，v0.3.0 之前曾用 `[uid]`）
  - 这两处是 **auth/transport 层的本地标签**，跟传到数据层的字段名不该耦合

**moyu/kungal 这边怎么改**：

如果你们的 nitro response 类型、composable 返回的 user 对象等用了 `uid: number` 字段，全部 rename 为 `id: number`：

```bash
# 在你们仓里找 user 对象的 uid 字段（不是 user.uuid 这种是字符串、function param 不算）
grep -rn '\.uid\b\| uid:' apps --include='*.ts' --include='*.vue'

# 谨慎的手动改 —— 自动化容易误伤
```

模板里现在写的 `:user="{ uid: u.id, ... }"` 这种转换层全部可以删掉，改回 `:user="u"` 直接传。

### 2.8 Tab variant 默认值改了

- v0.0.1：variant 默认 `'solid'`，仅 `solid` / `underlined` 两种
- v0.1.1：variant 默认 `'underlined'`，5 种 `underlined` / `solid` / `bordered` / `light` / `pills`

如果你想保留旧外观：

```bash
grep -rn '<KunTab' apps --include='*.vue'
# 逐处确认：若没显式传 variant，且你想要旧的 solid 填充感，加上 variant="solid"
```

其他改动：
- `hasScrollbar` → `scrollable`（语义反过来：原来默认隐藏滚动条，现在默认不可滚动，需要时 `scrollable`）
- 删了 `radius` prop（用容器外层 className 控制圆角）
- 新增 `orientation`（`'horizontal'` 默认，`'vertical'` 垂直布局）

---

## 3. 颜色系统迁移（如果你的代码里有 Tailwind 原色硬编码）

KunUI v0.1.x 强制语义色。如果你的项目里有：

```bash
grep -rn 'border-red\|text-red\|bg-red\|focus:.*ring-red\|focus:.*border-red' apps --include='*.vue'
grep -rn 'bg-white dark:bg-black\|bg-black dark:bg-white' apps --include='*.vue'
```

按以下规则替换：

| 旧（Tailwind 原色） | 新（KunUI 语义色） |
|---|---|
| `text-red-{N}` / `border-red-{N}` | `text-danger` / `border-danger`（或具体阶 `text-danger-600` 等）|
| `bg-white dark:bg-black` | `bg-content1`（弹层）或 `bg-background`（页面级）|
| `text-blue-{N}` | `text-primary` |
| `text-green-{N}` | `text-success` |
| `bg-gray-{N}` | `bg-default-{N}` |

这一步 KunUI 不强制，但 CLAUDE.md 里写了"不使用 Tailwind 固有颜色"，对齐成本低。

---

## 4. 新组件用法速查

### 4.1 `<KunChip>`（= 旧 KunBadge）

```vue
<KunChip color="primary" variant="flat" size="md">校园</KunChip>
```

API 完全不变（除了组件名）。

### 4.2 `<KunBadge>`（新 dot/count overlay）

```vue
<!-- 消息数 -->
<KunBadge variant="count" :count="unreadCount" :max="99">
  <KunIcon name="lucide:bell" />
</KunBadge>

<!-- 在线红点 -->
<KunBadge variant="dot" color="success">
  <KunAvatar :user="user" />
</KunBadge>
```

### 4.3 `<KunTagInput>`（新组件）

```vue
<KunTagInput
  v-model="aliases"
  label="游戏别名"
  placeholder="输入后按回车添加"
  helper-text="最多 17 个"
  :max-tags="17"
  :max-tag-length="100"
  color="primary"
  size="md"
  variant="bordered"
/>
```

完整 spec 见 `improvement-plan.md` §5（粘贴 4 种分隔符自动拆 / IME composition 屏蔽 Enter / 长按 Backspace 不会删光 / chip 键盘 ← Backspace 删 / `tag` slot 自定义 chip）。

### 4.4 `<KunTab>`（5 variant + 滑动指示器 + 键盘导航）

```vue
<KunTab
  v-model="active"
  :items="[
    { value: 'a', textValue: '简介' },
    { value: 'b', textValue: '资源', icon: 'lucide:package' },
  ]"
  variant="underlined"
  color="primary"
  size="md"
/>
```

键盘：← / → 或 ↑ / ↓（视 orientation）切换；Home / End 跳首末；Enter / Space 确认（manual mode）。

### 4.5 `useBodyScrollLock` composable

如果你**自己**实现了 Modal 类组件（不是用 `<KunModal>`），现在可以直接接 KunUI 单例：

```ts
import { useBodyScrollLock } from '@kun/ui/composables/useBodyScrollLock'

const { lock, unlock } = useBodyScrollLock()
let locked = false
const applyLock = (should: boolean) => {
  if (should && !locked) { lock(); locked = true }
  else if (!should && locked) { unlock(); locked = false }
}
```

跨组件实例共享同一个 refcount，嵌套 overlay 不会误解锁。

### 4.6 `kunVariantClasses` / `kunBgClasses` / `kunTextClasses` / `kunBorderClasses` / `kunRingClasses`

如果你**自己**做 variant×color 组件（不用 KunButton/KunChip），共用这套静态 map 即可，免得 Tailwind JIT 因为动态拼接漏类：

```ts
import { kunVariantClasses, kunBgClasses } from '@kun/ui/components/kun/ui/variants'

const cls = kunVariantClasses('solid', 'primary')  // → 'bg-primary text-white'
const bg  = kunBgClasses['danger']                 // → 'bg-danger'
```

**关键约束**：永远不要 `` `bg-${color}` `` 这种 runtime 拼接，Tailwind JIT 在 prod build 下会扫不到类。

---

## 5. 自动继承的 bug 修复（你什么都不用做，只是知道一下）

| Bug | 之前行为 | 现在行为 |
|---|---|---|
| Modal 嵌套 scroll-lock | 内层关闭 → 外层 body 滚动被错误解锁 | 单例 refcount，全关闭后才解锁 |
| DatePicker month 显示 0-indexed | 12 月显示成 "11" | 正确显示 |
| Avatar 收到 null user | 直接 throw | 渲染 fallback sticker |
| Pagination ← / → 偷键 | 在搜索框 / KunTab / KunSelect popover 内按方向键也会翻页 | 自动跳过 INPUT/TEXTAREA/SELECT + ARIA role (tab/option/menuitem/slider/spinbutton/combobox/tree) |
| TagInput 长按 Backspace | 删光所有 tag（一次 keydown arm，OS repeat 立刻命中 delete）| 加 `e.repeat` 跳过 repeat |
| Tab 动态类拼接 | prod JIT 下颜色完全不出（latent bug） | 全静态 map |
| Progress gradient variant | prod JIT 下不出色 | 静态 gradient map |
| Progress 圆环 stroke | `<circle stroke="currentColor" class="bg-primary">` 用了 bg 不是 text-color，stroke 走 inherited | 加 `strokeColorClasses` text-* 系列 |
| Lucide 图标用了 deprecated alias 名 | `check-circle-2` 在 lucide upstream 已 deprecated（iconify 还有 alias） | 切到 canonical `circle-check`/`circle-x`/`triangle-alert` |
| Modal HMR scroll lock 残留 | dev 热重载后 `body { overflow: hidden }` 永久卡死 | `import.meta.hot.dispose()` 重置 |
| Tab `setTabRef` 静默丢非 HTMLElement | fragment 等异常 ref 静默丢失 | dev `console.warn` |
| TagInput chipRefs 残留 stale | 删 tag 后数组没缩，stale 节点引用 | 自动 truncate |

---

## 6. 类型 / 工具新增

| 新增 | 路径 | 用途 |
|---|---|---|
| `KunUIColor` 加 `info` | `ui/type.d.ts` | 7 色（多了青色），所有 `Record<KunUIColor, X>` 都要补 |
| `kunVariantClasses(variant, color)` | `ui/variants.ts` | 7 variant × 7 color = 49 个静态类组合 |
| `kunBgClasses` / `kunTextClasses` / `kunBorderClasses` / `kunRingClasses` | 同上 | 细分单维 map |
| `useBodyScrollLock` | `composables/useBodyScrollLock.ts` | 跨实例共享 body scroll 锁 |
| `KunTabVariant` 五枚举 | `tab/type.d.ts` | underlined / solid / bordered / light / pills |
| `KunTabOrientation` | 同上 | 'horizontal' \| 'vertical' |
| `KunTagInputProps` / `KunTagInputInvalidReason` | `tag-input/type.d.ts` | 全套 TagInput 配置 |

---

## 7. 你需要 confirm 的几个判断

把交付前的最后几个开放问题列出来 —— 我们的判断 + 备选。

| 问题 | 我们的判断 | 备选 |
|---|---|---|
| `info` 色已加入 `KunUIColor`，你的 `tailwindcss.css` 是否需要补 info 50-950 调色板？ | **要**（@kun/ui 自带的 `app/styles/tailwindcss.css` 已经有，但若你项目 override 了主题，需自查） | 如果你不用 `info`，可以不加，但要知道下游用户拿到 KunUI 后用 `<KunChip color="info">` 会渲染失败 |
| Tab variant 默认从 'solid' 改 'underlined'，你的页面 50+ 个 Tab 调用点要不要全加 `variant="solid"` 保持旧外观？ | **不**，underlined 更现代、视觉更轻，建议接受 | 强烈想保留旧观感的话 grep 全加 `variant="solid"` |
| Badge → Chip rename 之后，旧 KunBadge 用例你打算切到新的 dot/count overlay 还是只做改名？ | **只做改名**到 KunChip（语义不变）；如果业务上确实需要数量徽章再用新 KunBadge | 一次性切到新 Badge dot/count 风险大 |
| TagInput 用什么默认色？我们默认 `primary` | 多数场景 OK；admin 表单可考虑 `default` 让 chip 更低调 | 自定义全靠 prop |

---

## 8. 验证 checklist

迁移完毕后跑：

```bash
# 1. 类型检查
pnpm typecheck     # 或者你项目里等价命令

# 2. 完整 build
pnpm -F web build  # / wiki / api 视你项目
pnpm -F api build

# 3. 残留扫描
grep -rn 'KunBadge\b\|v-model:modal-value\|isPressable\|KunIconLucide' apps --include='*.vue'
# 期望：仅在 vendored 注释里出现，apps 业务代码应为零

grep -rn 'bg-${\|text-${\|border-${\|after:bg-${' packages --include='*.vue'
# 期望：零（动态 Tailwind 类是 latent bug）

grep -rn 'border-red\|text-red\|bg-white dark:bg-black' apps --include='*.vue'
# 期望：零（或经判断保留的少量 intentional）
```

### 烟雾测试（手工跑 dev server 验证的关键交互）

| 场景 | 步骤 | 期望 |
|---|---|---|
| Modal v-model | 任意页面打开任意 Modal | 正常打开关闭，无报错 |
| Modal 嵌套 | 打开 Modal A → 在 A 内打开 Modal B → 关 B | A 仍然开着，body 仍然 lock |
| Tab 切换 | 任意 Tab 容器，点不同 tab | 底部滑动线 / 填色块从旧 tab 滑到新 tab，~250ms |
| Tab 键盘 | Tab focus 后按 ← / → | 切换并保持焦点；Home / End 跳首末 |
| TagInput Enter | 输入 "tag1" 按 Enter | tag 加入；输入框清空 |
| TagInput 长按 Backspace | 输入框空，按住 Backspace 不松 | 不删任何 tag |
| TagInput 粘贴 | 粘贴 "a,b,c；d\ne" | 拆出 5 个 tag |
| TagInput IME | 中文输入法打字时按 Enter 选词 | 选词正常，不会触发"添加 tag" |
| Pagination 与输入框 | focus 任意 `<input>` 后按 ← → | 不翻页 |
| Avatar 收到 null user | mock 一个 `user=null` 的页面 | 渲染随机 sticker，不崩 |
| DatePicker 月份 | 切到 12 月 | 标题显示 "/12" 不是 "/11" |
| frontend dark mode toggle | 切换主题 | 所有 KunChip / KunBadge / KunTab / KunInput 颜色都自动切换，无残留浅色 |

---

## 9. 联系 / 反馈

迁移过程中遇到的：
- API 不一致：开 issue 到 @kun/ui 仓
- KunUI 新 bug：同上
- 设计意图不清楚：先读 `improvement-plan.md` 对应章节，然后还不清楚再问

**Pull-the-trigger 顺序建议**：
1. 单独一个 PR 做"文件复制 + 删除 + rename"（机械改动，零业务变化）
2. 第二 PR 跑所有 sed（call-site 改名）
3. 第三 PR 修任何 sed 没覆盖的边角（Brand props、Tab variant、Card clickable/href 拆分这种语义判断）
4. 最后一个 PR 跑烟雾测试 checklist，签收

---

## 10. v0.3.1 + v0.4.0 增量同步（2026-05-21）

两个增量批次，**全部非破坏性**，下游零修改即可继续工作；想用新功能时按需采纳。详细背景见 `improvement-plan.md` §10 / §11。

### 10.1 v0.3.1 hotfix — KunTab solid/light indicator 错位

只需要同步**一个文件**：

```bash
KUN_OAUTH=/path/to/kun-oauth-admin   # 改成你本地路径
cp $KUN_OAUTH/packages/ui/app/components/kun/tab/Tab.vue \
   ./components/kun/tab/Tab.vue       # 改成你 KunUI 副本的对应位置
```

修复内容：`updateIndicator` 拆成 underlined（单轴 translate）vs panel（双轴 translate）两个分支，补偿 solid/light variant 容器 `p-1` 引入的 4px Y 偏移。

**消费侧**：零修改。无 API 变化。

### 10.2 v0.4.0 batch — Primitives + Ergonomics（4 项）

```bash
KUN_OAUTH=/path/to/kun-oauth-admin

# 改造的 5 个文件（覆盖）
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue       ./components/kun/select/Select.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/select/type.d.ts        ./components/kun/select/type.d.ts
cp $KUN_OAUTH/packages/ui/app/components/kun/Textarea.vue            ./components/kun/Textarea.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/Input.vue               ./components/kun/Input.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/ui/variants.ts          ./components/kun/ui/variants.ts

# 新增的 3 个文件
mkdir -p ./components/kun/radio-group
cp $KUN_OAUTH/packages/ui/app/components/kun/radio-group/RadioGroup.vue ./components/kun/radio-group/
cp $KUN_OAUTH/packages/ui/app/components/kun/radio-group/type.d.ts     ./components/kun/radio-group/
cp $KUN_OAUTH/packages/ui/app/composables/useFilePicker.ts             ./composables/useFilePicker.ts
```

### 10.3 各项要点

#### #1 KunSelect generic + readonly

调用方零修改即可继续工作。**想要 union narrowing 时**：

```ts
// 原写法（仍可用）
const opts = [{ label: 'A', value: 'a' }, { label: 'B', value: 'b' }]
const v = ref<string>('a')
<KunSelect :options="opts" v-model="v" />

// 升级写法（推荐）
const opts = [
  { label: 'A', value: 'a' },
  { label: 'B', value: 'b' }
] as const
const v = ref<'a' | 'b'>('a')
<KunSelect :options="opts" v-model="v" />
// TS 自动校验 v 只能是 'a' | 'b'
```

#### #2 KunTextarea / KunInput expose

新增方法：`focus / blur / select / insertAtCaret` + 兜底 ref（`textareaRef` / `inputRef`）。chat 表情插入典型用法：

```vue
<script setup>
const taRef = ref()
const insertEmoji = (emoji: string) => taRef.value?.insertAtCaret(emoji)
</script>
<KunTextarea ref="taRef" v-model="msg" />
<KunButton @click="insertEmoji('🐱')">🐱</KunButton>
```

#### #3 useFilePicker

Nuxt 4 layer 自动 import，直接用：

```vue
<script setup>
const { pickFiles, files } = useFilePicker({
  accept: '.zip,.tar.gz',
  maxSize: 100 * 1024 * 1024,
  onError: (msg) => useKunMessage(msg, 'error')
})
watch(files, ([f]) => f && handleArchive(f))
</script>
<template>
  <KunButton @click="pickFiles">选择压缩包</KunButton>
</template>
```

**和 KunUpload 的取舍**：
- 要做图片裁剪/拖拽/预览 → KunUpload
- 纯文件选择（zip / pdf / 任意 mime）→ useFilePicker

#### #4 KunRadioGroup

```vue
<!-- classic: 标准圆点 -->
<KunRadioGroup
  v-model="role"
  :options="[
    { value: 'admin', label: '管理员' },
    { value: 'user', label: '普通用户' }
  ]"
  label="选择角色"
/>

<!-- card: 卡片式（适合大目标 + 多信息） -->
<KunRadioGroup
  v-model="plan"
  variant="card"
  color="primary"
  :options="[
    { value: 'free', label: 'Free', description: '$0 / 月，10GB' },
    { value: 'pro', label: 'Pro', description: '$10 / 月，100GB' },
    { value: 'team', label: 'Team', description: '$30 / 月，无限' }
  ]"
/>
```

完整 ARIA + 键盘（↑↓←→ 移焦并激活，Space/Enter 激活，roving tabindex）。

### 10.4 验证

```bash
# 在你们 app 仓
pnpm typecheck    # 应该全过
pnpm -F web exec nuxt build    # 应该全过
```

如果旧调用方传 `options: someArray` 给 Select 报 TS 错误（"readonly 不能赋值给 mutable"），那是你们以前的代码就有错（KunSelect 不会修改 options），新版反而宽容了 readonly 输入。如果反过来报 mutable 不能赋值给 readonly，那不会发生 —— TS 允许 mutable 赋值给 readonly。

### 10.5 不需要做的事

- v0.4.0 没改任何现有 props 的语义
- v0.4.0 没有任何文件被删
- v0.4.0 没有 sed 改造（call-site 零变更）
- v0.3.1 同上

**风险等级**：极低。建议作为单独一个小 PR 合并。

合并顺序很重要 —— 中间任何 PR 都应该能独立 build pass。

---

## 11. v0.4.1 hotfix — 浮层动画"从角落飞来"修复（2026-05-21）

`KunSelect` / `KunPopover` / `KunDatePicker` 打开时弹出层从 body 左上角"飞过来"是 transform 双写竞态。@floating-ui/vue 默认通过 `transform: translate3d()` 定位，与 Vue Transition 的 `-translate-y-1` / `scale-95` 类争抢同一个 CSS 属性，挂载瞬间出现可见的"飞行"插值。

**修复**：给 3 个组件的 `useFloating()` 各加一行 `transform: false`，定位改走 `top/left`。详见 `improvement-plan.md` §12。

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue       ./components/kun/select/Select.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/Popover.vue             ./components/kun/Popover.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/date-picker/Picker.vue  ./components/kun/date-picker/Picker.vue
```

**消费侧**：零修改，无 API 变化，纯内部修复。建议直接合并。
