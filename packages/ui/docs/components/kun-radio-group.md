# KunRadioGroup

KunRadioGroup 是 v0.4.0 新增的 form primitive，补齐 KunUI 此前缺失的 radio 缺口。提供 `classic`（圆形按钮 + label）和 `card`（矩形卡片）两种 variant，含完整 WAI-ARIA radio pattern（roving tabindex + arrow-key 立即激活）。

## v0.4.0 — KunRadioGroup（新组件，classic + card 两种 variant）（§11.4）

**问题**：KunUI 完全缺 radio primitive。消费者 fallback 到 KunSelect（多一次点击 + 隐藏 affordance + 移动端体验差）或自己 `<input type="radio">`（破坏统一视觉）。

**决策**：

| 候选方案 | 取舍 |
|---|---|
| 加 KunRadioGroup（独立组件） | ✅ 做 |
| 给 KunTab 加 radio variant | ❌ 拒绝 |

为什么不复用 Tab：ARIA 角色不同（`tablist/tab` vs `radiogroup/radio`），键盘交互模型也不同（Tab 的 ←→ 切换 view，Radio 的 ←→ 移焦并激活）。强行复用让 Tab 变成"既切 view 又传 form value"的混合体，违反单一职责。**真正想要"胶囊状互斥选择器"视觉的场景，pills variant 已经能用 v-model 当 form value 用**，本来就没缺口。

**两种 variant**：

| variant | 长相 | 适用 |
|---|---|---|
| `classic` | 圆形单选按钮 + label（+ 可选 description） | 标准 form 场景 |
| `card` | 矩形卡片（带 border + 选中淡色 tint） | 大目标点击、选项含较多信息 |

**关键设计**：

1. **泛型 `T extends string \| number`**，与 Select 同款 readonly options：
   ```ts
   export interface KunRadioOption<T extends KunRadioValue = KunRadioValue> {
     value: T; label: string
     description?: string  // 可选副文本
     disabled?: boolean
   }
   ```

2. **完整 ARIA**：`role="radiogroup"` + `aria-labelledby` / `aria-label` + 每项 `role="radio" aria-checked aria-disabled`

3. **Roving tabindex**：整组只有一个 `tabindex=0`（优先 selected，否则第一个非禁用项），其余 `tabindex=-1` —— 符合 WAI-ARIA radio pattern

4. **键盘**：↑↓←→ 移焦并**立即激活**（per ARIA spec，与 listbox/menu 的"仅移焦"不同）+ Space/Enter 激活；自动跳过 disabled 项

5. **方向**：`orientation: 'vertical' | 'horizontal'`

6. **颜色 / 尺寸**：复用 KunUI 标准 `color` × `size` 矩阵；圆角 prop 仅 card variant 生效（classic 的 indicator 永远是圆）

**用法**：

```vue
<KunRadioGroup
  v-model="role"
  :options="[
    { value: 'admin', label: '管理员', description: '完整权限' },
    { value: 'user', label: '普通用户' },
    { value: 'guest', label: '访客', disabled: true }
  ]"
  variant="card"
  color="primary"
  label="选择角色"
/>
```

### 相关：新增静态颜色映射 `kunSoftBgClasses`（§11.5）

为 RadioGroup card variant 的"选中淡色背景"加了 `bg-{color}/5` 静态表（同 JIT-safety 规则：keys 必须字面量）：

```ts
// ui/variants.ts
export const kunSoftBgClasses: Record<KunUIColor, string> = {
  default: 'bg-default/5',
  primary: 'bg-primary/5',
  // ...
}
```
