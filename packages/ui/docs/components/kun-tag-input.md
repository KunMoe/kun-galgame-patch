# KunTagInput

KunTagInput 是 v0.1.0 引入的新 primitive。下方为终稿 spec（四问四答后定下来的最终 API），落地位置 `packages/ui/app/components/kun/tag-input/`。

## v0.1.0 — KunTagInput 终稿 spec（§5）

> 这是上一轮四问四答后定下来的最终 spec，落地位置 `packages/ui/app/components/kun/tag-input/`。

### 5.1 文件结构

```
packages/ui/app/components/kun/tag-input/
├── TagInput.vue
└── type.d.ts
```

### 5.2 Props

```ts
const tags = defineModel<string[]>({ default: () => [] })

interface KunTagInputProps {
  label?: string
  placeholder?: string
  helperText?: string
  error?: string

  // 限制
  maxTags?: number              // 默认 Infinity
  maxTagLength?: number         // 默认 100
  minTagLength?: number         // 默认 1

  // 去重 / 规范化
  allowDuplicates?: boolean     // 默认 false（去重，case-insensitive）
  caseSensitive?: boolean       // 默认 false（dedupe 时 lowercase 比较）
  trim?: boolean                // 默认 true
  transform?: (raw: string) => string
  validate?: (tag: string, all: string[]) => true | string

  // 分隔符
  delimiters?: string[]         // 默认 ['Enter']（键盘提交键）
  splitChars?: (string | RegExp)[]  // 默认 ['\n', ',', '，', ';']
  splitOnPaste?: boolean        // 默认 true

  // 行为
  confirmOnBlur?: boolean       // 默认 true
  respectComposition?: boolean  // 默认 true（IME composition 中 Enter 不提交）

  // 样式（沿用 KunUI）
  color?: KunUIColor            // 默认 'primary'
  size?: KunUISize              // 默认 'md'
  variant?: 'bordered' | 'flat' // 默认 'bordered'
  disabled?: boolean
  readonly?: boolean
  showCounter?: boolean         // 默认 false；显示 {tags.length}/{maxTags}
  className?: string
}
```

### 5.3 Emits

```ts
defineEmits<{
  add: [tag: string]
  remove: [tag: string, index: number]
  invalid: [reason: 'duplicate' | 'too-long' | 'too-short' | 'max-reached' | 'custom', raw: string, detail?: string]
}>()
```

### 5.4 Slots

```ts
defineSlots<{
  tag(props: { tag: string; index: number; remove: () => void }): any
}>()
```

### 5.5 行为表

| 触发 | 行为 |
|---|---|
| `Enter`（非 IME composition） | commit 当前 input；按 `splitChars` 拆，逐个 add |
| `Enter`（IME composition 中） | 不提交（compositionend 后才允许） |
| `Backspace`（input 非空） | 普通退格 |
| `Backspace`（input 空，第一次） | `canDelete` 置 true，**不删**（防误删） |
| `Backspace`（input 空，第二次） | 删最后一个 tag |
| 任意字符输入 | `canDelete` 重置 false |
| `←` / `→`（input 空） | 焦点跳到前一个 / 后一个 chip，chip 上 Enter/Delete 删 |
| paste | 按 `splitChars` 拆，逐个 add（去重/校验照走） |
| blur（`confirmOnBlur`） | pending input 当作一次 commit |
| click 容器空白 | focus input |
| `disabled` | input + chip × 都禁用 + `aria-disabled` |
| `readonly` | 显示 chips 但无 × 也无 input |

### 5.6 a11y

- 容器 `role="group"` + `aria-label={label}`
- 每个 chip `role="listitem"`
- × 按钮 `aria-label="移除标签 {tag}"`
- input `aria-describedby` 接 helper/error
- 容器 `aria-invalid` 跟随 `error`

### 5.7 视觉

- 外框沿用 Input 同款：`border border-default-200 rounded-lg`
- focus-within：`ring-2 ring-{color}/40 border-{color}`（用 §1.2 静态映射表）
- chip 沿用 KunBadge → 重命名后的 KunChip，`size` 跟随容器 `size`
- 容器 `min-h` 跟随 `size`：sm=36、md=42、lg=48
- chips wrap 后容器自动长高
- 错误态：边框 `border-danger`，下方 `text-danger text-sm`
- counter（可选）：右下角 `text-default-400 text-xs`

### 5.8 测试覆盖（Vitest）

```
✓ 加 tag（Enter）
✓ 删 tag（× 按钮 / Backspace 两段式）
✓ IME composition 期 Enter 不提交
✓ 粘贴 4 种分隔符（\n / , / ， / ;）正确拆
✓ 去重 case-insensitive
✓ 去重 case-sensitive（caseSensitive=true）
✓ maxTags 达上限 → emit invalid + 拒绝
✓ maxTagLength 超限 → emit invalid + 拒绝
✓ validate 返回 string → emit invalid + 拒绝
✓ transform 规范化（如 toLowerCase）生效
✓ confirmOnBlur=true 时 blur commit pending input
✓ confirmOnBlur=false 时 blur 丢弃
✓ tag slot 自定义渲染替换默认 chip
✓ readonly 模式无 × 无 input
✓ disabled 模式键盘和点击都失效
```
