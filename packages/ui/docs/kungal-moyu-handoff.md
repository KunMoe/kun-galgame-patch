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

---

## 12. v0.4.2 — KunFileInput：moyu 7 处 `<input type="file">` 收口（2026-05-21）

针对 moyu 反馈的"还有 7 处 native `<input type="file">`"，本批次给出**分层文件 API**的中层组件 KunFileInput，专门覆盖"按钮 + v-model + 文件"这种 80% 场景。详细设计见 `improvement-plan.md` §13。

### 同步文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
mkdir -p ./components/kun/file-input
cp $KUN_OAUTH/packages/ui/app/components/kun/file-input/FileInput.vue  ./components/kun/file-input/
cp $KUN_OAUTH/packages/ui/app/components/kun/file-input/type.d.ts      ./components/kun/file-input/
```

只新增 2 个文件，零现有文件改动。

### 三层 API 速查（决定用哪个）

| 场景 | 用哪个 | 例 |
|---|---|---|
| 图片裁剪 / 拖放 / 预览 | `KunUpload`（已有） | 用户头像、封面图必裁切场景 |
| **按钮 + v-model 选文件** | **`KunFileInput`（新）** | banner、截图、PR patch 包等 |
| 任意元素当触发器 / SSR 安全的程序化选择 | `useFilePicker`（v0.4.0） | 点 KunCard 触发选文件 |

### moyu 的 7 处迁移示例

#### A. banner / 截图编辑页 × 3

```vue
<!-- 之前 -->
<input
  ref="bannerInputRef"
  type="file"
  accept="image/jpeg,image/png,image/webp"
  class="file:bg-content2 file:..."
  @change="onPickBanner"
/>

<!-- 之后 -->
<KunFileInput
  v-model="bannerFile"
  accept="image/jpeg,image/png,image/webp"
  :max-size="10 * 1024 * 1024"
  trigger-text="选择 banner"
  trigger-icon="lucide:image-up"
  hint="JPEG / PNG / WebP，最大 10 MB"
  @error-pick="(msg) => useKunMessage(msg, 'warn')"
/>
<!-- bannerFile: ref<File | null> -->
```

#### B. 截图批量上传（ScreenshotsEditor）

```vue
<KunFileInput
  v-model:files="screenshotFiles"
  multiple
  accept="image/*"
  :max-size="5 * 1024 * 1024"
  trigger-text="批量添加截图"
  trigger-icon="lucide:images"
  trigger-variant="bordered"
  @error-pick="(msg) => useKunMessage(msg, 'warn')"
/>
<!-- screenshotFiles: ref<File[]> -->
```

#### C. patch PR banner

跟 A 同款，复用即可。

#### D. patch Hash 校验文件选择（这种"选了文件→读取算 hash→不上传"的场景）

```vue
<KunFileInput
  v-model="hashCheckFile"
  accept=".zip,.7z,.rar"
  trigger-text="选择 patch 包计算 hash"
  trigger-icon="lucide:file-archive"
  show-file-name
/>

<script setup>
const hashCheckFile = ref<File | null>(null)
watch(hashCheckFile, async (f) => {
  if (!f) return
  const hash = await sha256(await f.arrayBuffer())
  computedHash.value = hash
  // 注意：v-model 不发请求，hash 完才发
})
</script>
```

#### E. 头像 cropper bridge（需要把选到的 File 喂给 vue-advanced-cropper）

这个场景"选文件 + 弹出 cropper"其实就是 `KunUpload` 的设计目标 —— **优先用 KunUpload**。如果坚持自己控制 cropper UI（比如多张连续裁切），就用 `useFilePicker` composable 自行调度：

```ts
const { pickFiles, files } = useFilePicker({ accept: 'image/*', maxSize: 5*1024*1024 })
watch(files, ([f]) => f && openCropper(f))
```

### 自定义触发器（slot 模式）

任何"想用 KunCard / KunChip / 自定义 div 触发选文件"的场景，用 default slot：

```vue
<KunFileInput v-model="patchFile" accept=".zip" v-slot="{ pick, fileName }">
  <KunCard clickable @click="pick">
    <div class="flex items-center gap-3 p-4">
      <Icon name="lucide:file-archive" class="text-primary size-8" />
      <div>
        <p class="font-medium">{{ fileName ?? '点击选择 patch 包' }}</p>
        <p class="text-default-400 text-xs">支持 zip / 7z / rar</p>
      </div>
    </div>
  </KunCard>
</KunFileInput>
```

slot props：
- `pick: () => void` — 调用就打开文件对话框
- `fileName: string | null` — 已选文件名（多文件模式下是"已选 N 个文件"）
- `disabled: boolean` — 透传

### 设计要点速记

1. **`v-model` (默认) = File | null，`v-model:files` = File[]**（启用 `multiple` 时用 files）
2. **`@change` 始终发数组**（单文件 = `[f]`，多文件 = `[f1, f2, ...]`），逻辑同构
3. **用户取消保留旧选择**（与 native input 行为对齐）
4. **`@error-pick`** 在 maxSize 超限时触发，message 可以直接转给 `useKunMessage`
5. **`show-file-name` 默认 true**（显示在按钮旁），不想显示就 `:show-file-name="false"`

### 验证 checklist（同步完后跑一遍）

```bash
pnpm -F your-app exec nuxt build
pnpm -F your-app exec nuxt typecheck

# 残留扫描：迁完后应该是 0
grep -rn '<input.*type="file"' apps --include='*.vue'
```

如果 grep 还有命中，对照上面 A-E 五种场景判断该用哪一层；除了 KunUpload 内部实现，没有合理留存。

**风险等级**：极低 —— 纯新增组件，无依赖、无 API 变更、无破坏性改动。建议作为单独一个小 PR 合并。

---

## 13. v0.4.3 hotfix 🔴 — `getRandomSticker` 双层修复（2026-05-21）

**严重度 critical**。两层 bug：

1. **崩页面**：computed / watcher 重算路径上渲染 `<KunAvatar :user>` / `<KunNull description>` → refresh 后整个页面崩，`Cannot read properties of null (reading '$nuxt')`
2. **水合不一致**：F5 #2+ 之后 fallback sticker 与 server 渲染的不匹配（**第一版修复引入的**新 bug —— 见教训）

详细分析见 `improvement-plan.md` §14。

### 修复 — 只动一个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/utils/getRandomSticker.ts ./utils/getRandomSticker.ts
```

如果你的 fork 里有 **`apps/web/app/utils/getRandomSticker.ts`** 或类似的重复实现（layer 引入 KunUI 之前的旧拷贝），**直接删掉**：layer 里的版本会接管，重复实现会触发 nuxt `prepare` 的 dup warning。

```bash
find apps -name 'getRandomSticker.*' -not -path '*/node_modules/*'
# 如果有命中且 layer 已经导出，删掉
```

### 新实现的核心结构

```ts
// Client-only cache. Server stays null → useState 的 per-request 路径不被短路。
const clientCache = import.meta.client
  ? new Map<string, Ref<string>>()
  : null

export const getRandomSticker = (id: string): Ref<string> => {
  // 1. Client reactive 重算路径：cache hit → 直接返
  if (clientCache?.has(key)) return clientCache.get(key)!

  // 2. setup / SSR 路径：useState 走 payload
  const nuxtApp = tryUseNuxtApp()
  const stickerUrl = nuxtApp
    ? useState<string>(key, makeUrl)
    : ref(makeUrl())   // 3. Client 重入 + 新 id 路径：plain ref fallback

  if (clientCache) clientCache.set(key, stickerUrl)
  return stickerUrl
}
```

四条执行路径覆盖矩阵：

| 路径 | 行为 |
|---|---|
| SSR pass | `clientCache=null`，走 `useState` → 入 payload |
| Client 首次挂载（cache miss + 有 nuxtApp） | 走 `useState` → 读 payload，与 server 一致 |
| Client reactive 重算（cache hit） | 直接返 cached ref，不调 Nuxt composable，不崩 |
| Client refresh 后新 id（cache miss + 无 nuxtApp） | fallback `ref(makeUrl())`，无水合配对要保护 |

### 消费侧迁移

**零修改**。`getRandomSticker(id)` 依然返回 `Ref<string>`，`.value` 拿 URL 的调用点全部继续工作。

### 验证

```bash
pnpm prepare         # 应无 dup warning
pnpm typecheck
pnpm test            # 如果有 vitest
pnpm -F your-app exec nuxt build
```

运行时复现：
- 数据列表页（含头像或 Null 兜底），触发 refresh / reload → **之前会炸的现在不再炸**
- F5 多次刷新 → **水合警告不再出现**

### 关于 fallback 路径上的水合配对

修复后**只剩一种**残留：client 第一次见到 refresh 后新出现的 id 时，走 plain `ref()` fallback。但这个 id **根本没经过 SSR**，所以没有"参照真值"可对比，不存在 hydration mismatch。

### 给 KunUI 立两条工程规则（v0.4.3 起生效）

#### 规则 1：layer utils 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门

> 任何依赖 Nuxt context 的 composable（`useState` / `useFetch` / `useAsyncData` / `useRoute` / `useRuntimeConfig` 等）调用前，要主动审视"是否可能从 reactive effect 重入路径触发"。如果可能，**必须 `tryUseNuxtApp()` 守门 + plain Vue 原语 fallback**。同族表现：`LinkDetailModal.vue::watch(open)` 用 `nuxtApp.runWithContext(...)` 包 `kunFetch`。

#### 规则 2：layer utils 里**永远不要加模块级 mutable 缓存而不区分 server / client**

> Nuxt SSR 进程长驻，模块作用域变量跨请求泄漏，会把 `useState` / `useFetch` 这种 per-request scoped 机制短路 → **只在 F5 #2+ 出现的水合 / payload 不一致 bug**。难诊断，产线常见。
>
> 安全模式：
> ```ts
> const cache = import.meta.client ? new Map() : null
> ```
> 或者挂到 `nuxtApp.payload._xxxCache` 之类的请求作用域里。
>
> **静态只读常量（如 `KEY_OWNING_ROLES = new Set([...])` 这种 lookup table）不在此约束内** —— 不写入就没有泄漏。

### 教训 — 修一个 bug 引一个新 bug 的反面教材

我第一版修复只把 `useState` 换成 plain `ref()` + 模块级 Map，**没把"模块级 Map 在 Node SSR 长驻进程里的寿命"作为独立维度审视**，结果引入了"只在 F5 #2+ 出现的水合不一致"新 bug。

以后改 SSR 代码时拿这四个轴过一遍：

```
1. 在 setup 顶层调用 → OK?
2. 在 reactive effect 重入路径调用 → OK?
3. 在 SSR 进程的 N 个请求间共享状态 → OK?
4. 在 CSR hydration 时 server / client 状态对得上 → OK?
```

只通过 1+2 不够，必须 1234 全通过。

### 给 kungal 的 patch notes

> ▎ **v0.4.3 patch — `getRandomSticker` 修复 reactive effect 崩溃 + SSR 水合不一致**
> ▎ 1) 避开 `useState` 在 microtask 重算里调用导致的 null context 崩溃；2) cache 只在 client 端启用，避免 SSR 模块级缓存跨请求泄漏破坏 payload 序列化。
> ▎ 下游消费方零迁移成本（API 不变）。

**风险等级**：极低 —— 只改一个文件，API 不变。建议作为独立 PR，标记为 hotfix 优先合。

---

## 14. v0.4.4 hotfix 🔴🔴 — `useKunMessage` 一行 fix 终结整个调试链（2026-05-21）

**最高优先级 hotfix**。moyu 揪到的"统一根因"，意外解释了 v0.4.3 之前一连串看似不相关的 `$nuxt null` 崩溃报告。详细侦探故事见 `improvement-plan.md` §15。

### 这一个修复消除哪些症状

| 症状 | 之前以为的原因 | 真实原因 |
|---|---|---|
| 编辑 modal 关闭报 `$nuxt` | close handler 调度问题 | **关闭时 `useKunMessage('成功', 'success')` 首次挂载 MessageContainer 失败** |
| `kunFetch` 在 watch 里报 `useRuntimeConfig` null | watch microtask 丢 Nuxt context | **上面那次失败把 Nuxt 实例腐化了** |
| 后续任意操作 `$nuxt` 一片 null | 多个独立 bug | **同上连锁反应** |

如果你之前为绕这些 bug 加过 `nuxtApp.runWithContext(() => useKunMessage(...))` / `nextTick + setTimeout` / 其他 workaround，**装上 v0.4.4 后可以删掉** —— 留着无害，删了更干净。

### 同步 — 1 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/composables/useKunMessage.ts ./composables/useKunMessage.ts
```

### 修复内容

`initializeContainer()` 里 `render(vNode, container)` 之前补一步：

```ts
const vNode = h(MessageContainer)

// 必须 graft Nuxt vueApp._context 到 vNode 上，否则 MessageContainer
// 子树（含 <KunIcon> → <NuxtIcon>）拿不到 Nuxt 实例 → 崩 + 腐化
const nuxtApp = tryUseNuxtApp()
if (nuxtApp?.vueApp) {
  vNode.appContext = nuxtApp.vueApp._context
}

render(vNode, containerRef)
```

### 消费侧迁移

**零修改**。`useKunMessage(msg, type)` API 完全不变。

### 验证

```bash
pnpm prepare
pnpm typecheck
pnpm test
pnpm -F your-app exec nuxt build
```

运行时复现：
- 任何会触发 `useKunMessage` 首次调用的流程（编辑提交成功、登录失败、API error toast）→ **不再炸**
- success / error message **真的可见**（之前 MessageContainer 挂载失败，message 永远没出现过）
- 后续 Avatar / kunFetch / useFetch 都正常

### 提醒：v0.4.3 + v0.4.4 强烈建议一起合

v0.4.3 修复了 `getRandomSticker` 在 reactive recompute 路径上的崩溃（一个真独立 bug）。v0.4.4 修复了 `useKunMessage` 这个**会连锁腐化整个 Nuxt 实例的更深层 bug**。

两个都修了之后，**KunUI v0.1.x 起所有"`$nuxt` null"系列报告都应该消失**。如果还有残留，那是新 bug，请用同样的调试方法（按时间顺序找最早触发的失败点，而不是按 stack trace 当前报错位置）继续挖。

### KunUI 三铁律（看上面 v0.4.3 + 这一节后，应该写进你们 fork 的开发规约）

| # | 规则 |
|---|---|
| 1 | layer util 调 Nuxt composable 必须 `tryUseNuxtApp()` 守门 + plain Vue 原语 fallback |
| 2 | layer util module-level mutable cache 必须 `import.meta.client ? new Map() : null` |
| 3 | 命令式 `render(vnode, container)` 必须 graft `nuxtApp.vueApp._context` 到 vNode.appContext |

违反任何一条 → 等着踩 `$nuxt null` 的坑。

**风险等级**：极低 —— 一个文件 + 一行 graft，API 不变。**优先级最高**，建议作为独立 PR 立即合并。

---

## 15. v0.4.5 — z-index 设计 token 系统：浮层不再被 Modal 压住（2026-05-21）

**症状**：KunModal 里打开 KunSelect / KunPopover / KunDatePicker / KunTooltip，下拉视觉上**沉到 Modal 下面**，看不见。

**根因**：v0.1.x 起 9 个浮层组件各自硬编码 magic number z-index：Modal 是 z-1007、popover 类全是 z-50 → Modal **永远盖住** popover。Teleport to body 没有改变 z-index 竞争关系。

详细分析见 `improvement-plan.md` §16。

### 同步 — 10 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin

# token 定义（关键 —— 必须先同步这个）
cp $KUN_OAUTH/packages/ui/app/styles/tailwindcss.css ./styles/tailwindcss.css

# 9 个 component 文件
cp $KUN_OAUTH/packages/ui/app/components/kun/Modal.vue                       ./components/kun/Modal.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/Popover.vue                     ./components/kun/Popover.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue               ./components/kun/select/Select.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/date-picker/Picker.vue          ./components/kun/date-picker/Picker.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/tooltip/Tooltip.vue             ./components/kun/tooltip/Tooltip.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/context-menu/ContextMenu.vue    ./components/kun/context-menu/ContextMenu.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/alert/Alert.vue                 ./components/kun/alert/Alert.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/alert/Loli.vue                  ./components/kun/alert/Loli.vue
cp $KUN_OAUTH/packages/ui/app/components/kun/alert/MessageContainer.vue      ./components/kun/alert/MessageContainer.vue
```

如果你 fork 后改过 `tailwindcss.css`（加了自己的 design tokens），**不要直接覆盖** —— 把 v0.4.5 加的这段插进你的 `@theme` 块即可：

```css
@theme {
  /* ... 你既有的 token ... */
  --z-kun-sticky: 30;
  --z-kun-modal: 1000;
  --z-kun-popover: 1500;
  --z-kun-alert: 2000;
  --z-kun-message: 9000;
}
```

### 设计层级速查

```
z-kun-sticky (30) < z-kun-modal (1000) < z-kun-popover (1500) < z-kun-alert (2000) < z-kun-message (9000)
```

| token | 谁用 | 何时该用 |
|---|---|---|
| `z-kun-sticky` | 自定义 sticky header / 滚动阴影 | 局部 sticky / 容器内浮层 |
| `z-kun-modal` | `KunModal` | 阻塞容器 |
| `z-kun-popover` | `KunPopover` / `KunSelect` / `KunDatePicker` / `KunTooltip` / `KunContextMenu` | 任何 Teleport 出去的浮层 |
| `z-kun-alert` | `KunAlert`（确认对话框） | 比 Modal 更高一级的阻塞 dialog |
| `z-kun-message` | `KunMessage`（toast） / `KunLoliInfo` | 永远在最上层 |

### 消费侧

**零修改**。所有 KunUI 组件依然按之前用法工作。如果你自己有 `class="z-50"` 之类硬编码 Modal 边沿的 z-index，建议改成 token，避免将来 token 调整后行为漂移：

```bash
# 在你们仓里 grep 一下自己写的 z-index 硬编码
grep -rn 'z-\[\|class="[^"]*z-[0-9]\+' apps --include='*.vue'
```

业务代码里少量 `z-10` / `z-20` 这种局部相对值是 OK 的，**只要不是 fixed / absolute 跨容器的全局浮层即可**。

### 验证

```bash
pnpm -F your-app exec nuxt build
```

运行时复现：
- 任何场景"在 KunModal 里打开 KunSelect / KunPopover / KunDatePicker / KunTooltip" → **浮层正确出现在 Modal 之上**
- KunAlert 确认对话框打开时盖住 KunPopover（如果同时存在）
- KunMessage toast 永远可见，不会被任何 modal / popover 盖住

### 与 Modal-stack 第三方 widget 协作

如果你接入了某个写死 `z-index: 9998` 的 vendor 库，可以在你 app 的 `:root` 里覆盖 token：

```css
:root {
  --z-kun-modal: 9998;
  --z-kun-popover: 10500;  /* KunSelect 仍然高于 vendor modal */
}
```

这是 token 系统比 magic number 优越的关键 —— 跨库 z-index 协作通过修改变量解决，不用挨个改组件。

**风险等级**：低 —— token 引入是新增 CSS 变量，组件改动只是把硬编码值换 token。**视觉无差异**（除了"在 Modal 内打开 popover" 这一个 bug 修复场景）。建议作为独立 PR 合并。

---

## 16. v0.4.6 — KunImage 5 个 prop 透传 + `none` provider 在 layer 层注册（2026-05-21）

来自 moyu 调查 `/about` 页卡顿的副产品：KunImage 缺 `provider` / `densities` / `sizes` / `fetchpriority` / `decoding` 几个常用 NuxtImg pass-through prop，导致优化场景被迫用裸 `<NuxtImg>`。详细背景见 `improvement-plan.md` §17。

### 同步 — 2 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/components/kun/image/Image.vue ./components/kun/image/Image.vue
cp $KUN_OAUTH/packages/ui/nuxt.config.ts                     ./nuxt.config.ts
```

注意第二个 —— 如果你 fork 了 KunUI layer 的 `nuxt.config.ts` 并自己改过，**别整个覆盖**，把 `image.providers.none` 这一段插进你的版本即可：

```ts
image: {
  providers: {
    none: { name: 'none', provider: '@nuxt/image/runtime/providers/none' }
  }
}
```

### 你之前在自己 app 的 nuxt.config 加的 `none` provider 可以删了

moyu 之前在自己仓的 `apps/web/nuxt.config.ts` 加过：

```ts
image: {
  providers: {
    none: { ... }
  }
}
```

**v0.4.6 之后这段可以从 app 层删掉** —— KunUI layer 已经全局注册，下游 inherit。如果想保留 app 层覆盖（比如要重新定义自己的 provider）也无害，nuxt 会 merge。

### 用 KunImage 的最佳实践 cheat sheet

#### 静态预优化图（author 时已 AVIF / WebP）—— 跳过 IPX

```vue
<KunImage
  :src="post.banner"
  provider="none"
  loading="lazy"
  :width="512" :height="288"
  class-name="h-full w-full object-cover"
/>
```

适用场景：about / blog post banner、固定品牌图、已经手动压过的图

#### LCP 元素（首屏最大图，详情页 banner）

```vue
<KunImage
  :src="banner"
  provider="none"
  loading="eager"
  fetchpriority="high"
  :width="1200" :height="400"
/>
```

适用场景：博客详情页头图、产品详情页主图

#### 需要 runtime resize 的图（用户上传、变体尺寸）

```vue
<KunImage
  :src="user.avatar"
  loading="lazy"
  :width="64" :height="64"
  densities="1x 2x"
/>
```

适用场景：用户头像、galgame banner、缩略图

#### 列表里的次要图（非 LCP，可以慢点）

```vue
<KunImage
  :src="thumb"
  loading="lazy"
  decoding="async"
  :width="200" :height="120"
/>
```

`decoding="async"` 让浏览器在主线程外解码，避免阻塞滚动。

### 验证

```bash
pnpm -F your-app exec nuxt build
# 应该都通过
```

运行时验证：把 `/about` 或类似含多 banner 的页面打开 Chrome DevTools Network，确认：

1. **预优化 banner 的 URL 没有 `/_ipx/` 前缀**（说明 provider="none" 生效）
2. **第二次访问同一张图直接 304 / from cache**（没走 IPX 5 分钟缓存）
3. **layout shift 看 Web Vitals 接近 0**（width/height 预留生效）
4. **LCP < 2.5s**（fetchpriority="high" 生效）

### 反思 —— sharp 不在前端运行，但卡是真的

moyu 这次最值得学的不是技术修复，是**调试方法**：

用户报告"sharp 在前端运行所以卡"。**moyu 没顺着错假设修**，先用 build artifact 直接证伪（`.output/public/_nuxt/*.js` 里 0 sharp），然后重新定义问题（"卡是真的，但成因是 IPX 冷启动 + layout shift + 重复 transcode"），再去修真根因。

下次遇到 KunUI / 其他 layer 的性能 / 崩溃 bug 报告，第一步**永远是查证假设**：
- 报"前端 X 在跑" → 看 client bundle 有没有 X
- 报"内存泄漏" → DevTools Memory profile 找具体保留引用
- 报"重渲染太多" → Vue DevTools 看 component re-render count

把假设当作待验证的命题而不是事实，是 senior 调试的核心姿态。

**风险等级**：极低 —— Image.vue 加可选 prop，nuxt.config 加 provider 注册。无 API 破坏。建议作为独立 PR 合并。

---

## 17. 迁移陷阱 —— 孤儿 store + `runWithContext` 过度防御（2026-05-21）

不是组件改动，是给下游写迁移 / 调试时的两条**必读警告**。moyu 这次踩坑后总结的，避免其他 fork 重蹈。

### 陷阱 1 🔴 —— 老 alert store 改用 KunUI 后变孤儿，按钮"无任何反应"

**症状**：把"删除"、"举报"这类需要确认的按钮点下去，**屏幕零反应**、控制台无报错、network 无请求。

**根因**：下游 app 原本有自己的 alert store，比如：

```ts
// apps/web/app/store/temp/components/message.ts  ← moyu 旧实现
export const useComponentMessageStore = defineStore('message', () => {
  const showAlert = ref(false)
  const alertTitle = ref('')
  const alertMessage = ref('')
  // ... 一堆 ref

  const alert = (title?: string, message?: string, showCancel?: boolean) => {
    showAlert.value = true
    alertTitle.value = title ?? ''
    // ... 等用户点确认 / 取消，返回 Promise<boolean>
  }
  return { showAlert, alert, /* ... */ }
})
```

老实现里有个 `<OldAlertComponent>` 监听这些 ref 渲染弹窗 + 处理点击 + resolve promise。

**迁到 KunUI 时**，开发者通常这样做：

1. 把 `<OldAlertComponent>` 删了，换成 `<KunAlert>`（KunUI layer 全局挂载）
2. 改了 ~50% 调用点用 `useKunAlert(...)`
3. 剩下 ~50% 调用点仍然写 `const message = useComponentMessageStore(); await message.alert(...)`

**结果**：第 3 类调用点变孤儿 —— `showAlert.value = true` 改了 ref，但**没人监听这个 ref**（老组件被删了），promise 永不 resolve，`await` 卡死。代码看起来在跑，UI 完全静默。比报错更难调试。

### 修复 — 桥接而不是"全部 sed"

最稳的修法是**让老 store 的 `alert` 内部 delegate 给 `useKunAlert`**，所有调用点零修改：

```ts
// apps/web/app/store/temp/components/message.ts
import { useKunAlert } from '#imports'

export const useComponentMessageStore = defineStore('message', () => {
  // ... 其他保留字段（isShowCapture / codeSalt 等保留不动，
  // 避免破坏 capture 等其他流程）

  // alert 改成 useKunAlert 的薄包装，签名照旧
  const alert = (
    title?: string,
    message?: string,
    showCancel?: boolean
  ): Promise<boolean> =>
    useKunAlert({
      title,
      message,
      showCancel: showCancel ?? true
    })

  return { /* ...其他字段..., alert */ }
})
```

收益：
- 10+ 文件的调用点 `await message.alert(...)` **零修改**
- UI 由 layer 的 `<KunAlert>` 渲染（全局挂载，肯定可见）
- promise 由 `useKunAlertState().handleConfirm/handleCancel` 正常 resolve
- 老 store 的其他字段（如 capture / salt）保留不动，不破坏其他流程

**反例（不推荐）**：开 sed 把所有 `useComponentMessageStore().alert(...)` 全部改成 `useKunAlert(...)`。这样会：
- 改动面广，回归风险大
- 容易漏改（动态调用、间接引用）
- 漏一处就静默 hang，比批量改组件 import 难发现

### 排查清单 — 怀疑撞上"孤儿 store"时怎么验证

```bash
# 1. 找你们仓里所有自定义 alert 系统
grep -rn 'defineStore.*alert\|defineStore.*message' apps --include='*.ts'

# 2. 看 alert 函数的实现 —— 是否还在用 ref + watch UI 组件？
#    那个 UI 组件还存在吗？

# 3. 老 store 的 alert 调用点（迁完后应该 0 或全部是桥接）
grep -rn 'message\.alert\|.alert(' apps --include='*.vue' | grep -v 'console.alert\|window.alert'
```

如果第 3 步还有命中，又确认那个 store 的 alert 已经没人监听 → 你正在踩这个坑。

### 陷阱 2 ⚠️ —— `runWithContext` 不是万能药，过度包装是反模式

之前在 v0.4.4 §15（improvement-plan）里我隐含说过"layer util 调 Nuxt composable 必须 `tryUseNuxtApp` 守门 + 准备 fallback"。这条**仍然对**，但很多人由此推论"凡是涉及 await / async / 跨 tick 都应该 `nuxtApp.runWithContext(...)` 包一下" —— **这是过度防御**。

moyu 复盘他们之前的修法：

> 之前我对 click 路径包 `runWithContext` 是过度防御。**可以保留**（几个闭包成本几乎为零），**也可以删**，行为一致。

#### `runWithContext` 真正必须的两个场景

```
✓ 必须：1. Vue 的 watch / watchEffect 回调里
              （脱离当前组件 instance 的微任务）
✓ 必须：2. render(vNode, container) 裸 mount 的 vNode 子树
              （没有任何 instance binding，例如 useKunMessage 里
               mount MessageContainer —— 该问题已在 v0.4.4 修过）

✗ 不必要：3. @click / @input 等 Vue 事件处理器
              （Vue 3 的 withCtx 包装，执行期 getCurrentInstance() 不为 null）
✗ 不必要：4. setup() 顶层（同步路径）
✗ 不必要：5. onMounted / onUnmounted 等生命周期 hook（Vue 已经在 hook 里
              保住 instance）
✗ 大多不必要：6. await kunFetch(...) 之后继续访问 Nuxt composable
              （Nuxt 3 对常见 await 路径——await Promise + Vue 调度——
              有内部 patch 保 context；codebase 里大量 await kunFetch 没包
              runWithContext 也工作）
```

#### 为什么 `@click` 不需要 `runWithContext`

`tryUseNuxtApp()` 的查找顺序大致是：

```
1. nuxtAppCtx.tryUse()      ← Nuxt 自己维护的 AsyncLocalStorage
2. getCurrentInstance()
     .appContext.app.$nuxt  ← Vue 当前实例的 app context
```

Vue 3 的 `@click` 处理器有 `withCtx` 包装，**执行期 `getCurrentInstance()` 不为 null**，所以路径 2 永远命中。`runWithContext` 是为了路径 1 失败时也能强制注入 —— **路径 2 已经够用的话就不需要 1**。

#### 当 `$nuxt null` 仍然发生时怎么诊断（升级版）

之前 §13 / §14 的规则"按时间排序找最早失败" + 这次 moyu 的补充：

1. **先排除"孤儿 store"**：UI 完全静默、按钮无反应 → 不是 Nuxt context 问题，是 promise 永不 resolve；grep 看是否有老 alert / dialog store 没桥接
2. **再排除"render() 漏 graft appContext"**：见 v0.4.4 §15 / §13 修法
3. **再排除"watch / watchEffect 微任务里调 Nuxt composable"**：用 `nuxtApp.runWithContext` 包
4. **最后才考虑 `@click` 路径的 context 注入**：99% 的时候不需要管，Vue 已经处理

按这个顺序排查能省去很多绕弯路 —— 不要一上来就在所有 await 点撒 `runWithContext`，是徒劳且让代码变脏。

### 这次修正的 KunUI 三铁律

§13 / §14 末尾给过 KunUI 三铁律 + §15 加了第四条。结合这次反思，规则 1 的措辞要更准：

| # | 旧措辞 | 修正后 |
|---|---|---|
| 1 | "layer util 调 Nuxt composable 必须 `tryUseNuxtApp` 守门 + plain Vue 原语 fallback" | 同左，**适用范围仅限**：watch/watchEffect 微任务、render() 裸 mount。普通 setup / 事件处理 / 生命周期 hook **不需要** |

加上 moyu 这次补的两条：

| # | 规则 |
|---|---|
| 5 | 下游迁移 KunUI 状态类 composable（useKunAlert / useKunMessage / useKunDisclosure 等）时，**保留老 store 的接口、内部 delegate**，避免孤儿 store 静默 hang |
| 6 | `runWithContext` 不是"撒着用更安全"的护身符，只在 watch / 裸 render() 这两个**真有 instance 漂移**的场景用 |

### 验证 — 给 moyu 自己 fork 的 checklist（你已经做过，对其他 fork 也适用）

```bash
# 1. 老 alert / message / dialog store 都迁了桥接
grep -rn 'defineStore.*alert\|defineStore.*dialog' apps --include='*.ts'

# 2. 没有遗留的 await yourStore.alert(...) 调用直接拿不到 promise resolution
grep -rn 'await.*\.alert(\|await.*\.dialog(' apps --include='*.vue' --include='*.ts'

# 3. 没有过度防御的 runWithContext 在 click handler 上
grep -rn 'nuxtApp\.runWithContext' apps --include='*.vue' | wc -l
# 不需要为 0 但如果几十个就说明过度撒了，可以走查一遍删 click 路径上的
```

**风险等级**：本节是**文档警告**，不引入代码改动。其他下游迁移时请把 §17 这一节当 checklist 跑一遍。

---

## 18. v0.4.7 — z-token 升档 + Select 选项溢出（2026-05-21）

moyu 又报了两个 UI bug，本节修。详细背景见 `improvement-plan.md` §19。

### bug 1 — tooltip/popover 还是被 topbar 盖住

v0.4.5 的 z-kun-popover = 1500 没考虑 legacy app 的 sticky header 用 z-9999 这种 nuclear z-index。**整体升档到 9000-9999 区间**：

```diff
- --z-kun-modal: 1000;     --z-kun-popover: 1500;
- --z-kun-alert: 2000;     --z-kun-message: 9000;
+ --z-kun-modal: 9000;     --z-kun-popover: 9300;
+ --z-kun-alert: 9700;     --z-kun-message: 9999;
```

相对层级不变。只升档不重构。

### bug 2 — Select dropdown 长 label 溢出

`<KunSelect>` 选项 label 超过 trigger 宽度时**横向溢出 dropdown**。根因是 flex + truncate 经典坑：flex item 默认 `min-width: auto` = `min-content`，truncate 不生效。修法是给文本 span 加 `min-w-0`。

### 同步 — 2 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/styles/tailwindcss.css            ./styles/tailwindcss.css
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue  ./components/kun/select/Select.vue
```

或者只改 token（如果你 fork 改过 tailwindcss.css）：把 `--z-kun-*` 五个 token 的值升档即可。

### 消费侧

**零修改**。所有 `<KunSelect>` / `<KunPopover>` / `<KunTooltip>` 调用点无需改动。

### 如果你 app 侧 topbar 还是盖住浮层

罕见情况下 app 侧用了 z-99999 / z-2147483647 之类极端值（来自某些 vendor template）。这时有两种解决：

**A. 推荐 — 把 app 侧 z-index 降下来**

KunUI 升档到 9999 已经是合理上限。app 侧 navbar 不该比浮层更高。grep 找出违例：

```bash
grep -rn 'z-\[1[0-9]\{4,\}\]\|z-index:[0-9]\{5,\}' apps --include='*.vue' --include='*.css'
```

把命中的值降到 z-50 / z-100 量级。

**B. 凑合 — 把 KunUI token 推到更高**

如果不想动 app 侧（暂时遗留代码），在你 app 的 css 里覆盖 KunUI token：

```css
:root {
  --z-kun-modal: 999000;
  --z-kun-popover: 999300;
  --z-kun-alert: 999700;
  --z-kun-message: 999999;
}
```

但这是俗气补丁，建议作为临时方案。

### "flex + truncate 不生效" —— 你 app 业务代码也建议自查一遍

这次发现的修法是 KunSelect 内部的，但同款坑可能藏在你们 app 的业务代码里。grep 自查：

```bash
grep -rn "flex.*truncate\|truncate.*flex" apps --include='*.vue'
```

每一处命中都检查：truncate 的 span / div 是不是 flex item？是的话有没有 `min-w-0`？没有的话长内容会撑爆容器。

最佳实践：**任何 flex 里的 truncate span 默认加 `min-w-0 flex-1`**。

### 验证

```bash
pnpm -F your-app exec nuxt build
```

视觉手测：
- 主页 hover topbar 元素 → tooltip / popover **盖住所有页面其他元素**
- 给 KunSelect 喂一组超长 label 的 options → 选项 **被 ellipsis 截断**，dropdown 宽度等于 trigger
- 不论在 KunModal 内还是页面 root，KunSelect / KunPopover / KunDatePicker / KunTooltip 都在最上层

**风险等级**：低 —— token 数值升档不改相对层级，Select 选项溢出修复零 API 变化。建议作为独立 PR 合并。

---

## 19. v0.4.8 hotfix 🔴🔴 — z-index utility 真的生效（之前三轮没生效）

**重要**：v0.4.5 / v0.4.6 / v0.4.7 三轮的 z-index "修复"**实际上从未生效**。我把 `--z-kun-*` token 加到 `@theme`，假设 Tailwind v4 会像 `--radius-kun-*` 那样自动生成 utility —— **错了**。Tailwind v4 只对特定命名空间（`--color` / `--radius` / `--spacing` / `--font` 等）自动生成 utility，`--z-*` 不在白名单。所以组件里写的 `class="z-kun-popover"` 是空 class，元素拿默认 `z-auto`。

之前看着"popover 在上方了" 完全是 DOM order 偶然，不是 z-index 生效。

详细诊断 + 真修复见 `improvement-plan.md` §20。

### 同步 — 1 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/styles/tailwindcss.css ./styles/tailwindcss.css
```

或者只加我新增的 5 个 `@utility` 块（放在 `@theme` 关闭花括号外、`@layer base` 之前）：

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

### 同步后必须 rebuild + grep 验证

```bash
# 重新 build
pnpm -F your-app exec nuxt build

# 必须验证 utility 真的进了输出 CSS
grep -ho '\.z-kun-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css | sort -u

# 期望看到 4 个（z-kun-sticky 没用到的话不出现是预期）：
# .z-kun-alert{z-index:var(--z-kun-alert)}
# .z-kun-message{z-index:var(--z-kun-message)}
# .z-kun-modal{z-index:var(--z-kun-modal)}
# .z-kun-popover{z-index:var(--z-kun-popover)}

# 如果输出空 → @utility 块没写对，再核查
```

### 视觉手测（同步 + rebuild 后）

| 场景 | 期望 |
|---|---|
| Hover topbar 元素 → tooltip 出现 | tooltip **在所有页面元素之上**，包括 sticky topbar 自己 |
| 点击触发 popover | popover 同上 |
| Modal 内打开 Select / DatePicker | 浮层在 Modal **之上**（不被 Modal 内容盖住） |
| useKunMessage 触发 toast | toast 在最高层，永远可见 |
| 长 label 的 Select option | label 被 ellipsis 截断（v0.4.7 已修，本次会一并奏效） |

### 关于 moyu 报的"Select 还溢出"

很可能是同一个 z-index bug 的视觉副作用：dropdown 没拿到正确 z-index 时，被某个 z-index 更高的元素切掉一部分，看起来像选项"溢出"到了别的地方。**z-index 修好后这个症状大概率自动消失**。如果同步 v0.4.8 + rebuild 后 Select 仍有视觉异常，请截图或具体描述（哪个 Select、什么 viewport 宽度、option 内容），我再深挖。

### 反思 — 给所有下游的诚实警告

我之前三轮 handoff（§15 / §16 / §18）都说"z-index 升档了，浮层会在上面"。**实际效果是 0**。这不是把数值改大没改大的事 —— 是整套 utility 根本没生成。

这个错误暴露了一个我（和 KunUI maintain）必须强化的流程：

> **修改 Tailwind theme / utility / 任何系统级 CSS 之后，必须 grep 编译产物确认 utility 真的生成了。不要止于"源码里写得对"或者"build 通过"。**

具体 grep 命令：

```bash
grep -ho '\.YOUR-PREFIX-[a-z]*\s*{[^}]*}' .output/public/_nuxt/*.css | sort -u
```

如果空 → 你的 utility 没被 Tailwind 生成 → 视觉上什么都没发生。这条流程规则比任何具体 bug 修复都更基础，**KunUI 工程规则 §20 把它立成了铁律**。

**风险等级**：低 —— 加 5 个 `@utility` 块，无破坏。**但优先级最高**：不同步这次，前面三轮所有 z-index "fix" 都是空头支票。建议立即合并。

---

## 20. v0.4.9 — KunSelect dropdown 真·溢出修复（height 维度同款 flex 坑）

v0.4.7 修了 "Select 长 label 横向溢出"，但 **height 维度的同款 bug** 一直在没人发现：dropdown 选项内容超过 maxHeight (240px) 时，**视觉上叠在下层 UI 之上**，看起来像 "Select 溢出容器"。

### 真根因

`floating-ui` 的 `size()` middleware 把 `max-height: 240px` 写在 **outer floating div** 上，但 outer div 默认 `overflow: visible`。`<ul>` 按自然 content height (~360px) 渲染，溢出 outer 边界 → 画在 outer 之外。同时 `<ul>` 自己 height 是 auto (= content)，所以 `<ul>` 的 `overflow-y-auto` **scroll 永远不触发**。

### 修复 — outer flex column + ul `min-h-0 flex-1`

详细原理见 `improvement-plan.md` §21。机制跟 v0.4.7 的 `min-w-0` 完全对称，只是从 width 维度搬到 height 维度。

### 同步 — 1 个文件

```bash
KUN_OAUTH=/path/to/kun-oauth-admin
cp $KUN_OAUTH/packages/ui/app/components/kun/select/Select.vue ./components/kun/select/Select.vue
```

### 验证（Playwright DOM probe，已在 moyu 本地 dev server 验过）

```
修复前：outer.height=240, overflow:visible, ul.height=360 (溢出 120px), 滚动失败
修复后：outer.height=240, overflow:hidden, ul.height=230 (受约束), 滚动: scrollTop 0→100 OK
```

视觉验证：dropdown 高度 cap 在 240px，超过 6 选项需要内部滚动；卡片网格完全可见、不被 dropdown 覆盖。

### KunUI 工程规则 6 修订（合并 v0.4.7 + v0.4.9 的同源 bug）

| 旧 (v0.4.7) | 新 (v0.4.9) |
|---|---|
| flex 容器里的 truncate 必须搭配 `min-w-0` | flex 容器里**任何要受父级尺寸约束的子元素**都要搭配对应的 `min-w-0` / `min-h-0`。truncate / overflow-auto / max-height 等约束都隐含这个前提 |

更通用的版本，覆盖 width 和 height 两个轴。

### 关于"用户报告环境一致性"的元教训

这次第一轮我跑去 moyu.moe 生产部署测，发现是 HeroUI 后写了一大段"不是 KunSelect"的报告 —— 全是误诊。生产部署是旧 Next.js 项目，moyu 测试是 `127.0.0.1:6969` 本地 Nuxt dev。

之后规则更明确：

> **用户报视觉 bug，第一步永远先 Playwright 看一下 DOM 是不是 KunUI 渲染的**（grep `react-aria` / `data-slot` / `kun-` prefix）。不是的话直接 short-circuit "这不在 KunUI scope"，不要浪费时间。

**风险等级**：低 —— 4 个 class 加在 Select.vue，无 API 变化。建议合并。
