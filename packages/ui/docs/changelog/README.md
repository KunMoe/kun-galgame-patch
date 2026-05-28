# Changelog

每个版本的完整变更记录。按时间倒序：

| 版本 | 日期 | 主题 | 严重度 / 性质 |
|---|---|---|---|
| [v0.6.0](./v0.6.0.md) | 2026-05-27 | 图片查看器重写 + KunTooltip 精简 + `--kun-background-blur` glass-blur token 修复；新增 `KunLightboxGallery` / `KunLightboxGalleryItem` 声明式包装 | 重大重构 + 2 个新组件 + 修复 |
| [v0.5.2](./v0.5.2.md) | 2026-05-22 | KunDrawer 响应式默认（桌面右、手机底） | 新增 |
| [v0.5.1](./v0.5.1.md) | 2026-05-22 | KunDrawer：边缘锚定的浮层 | 新增组件 |
| [v0.5.0](./v0.5.0.md) | 2026-05-21 | KunImage 加载骨架屏 | 新增功能 |
| [v0.4.9](./v0.4.9.md) | 2026-05-21 | KunSelect dropdown maxHeight 真溢出修复（height 维度同款 flex 坑） | bug fix |
| [v0.4.8](./v0.4.8.md) | 2026-05-21 | z-token utility 终于真的生效（Tailwind v4 不自动从 `--z-*` 生成 utility） | 🔴🔴 critical hotfix |
| [v0.4.7](./v0.4.7.md) | 2026-05-21 | z-index 数值升档 + Select 长 label 横向溢出 | bug fix |
| [v0.4.6](./v0.4.6.md) | 2026-05-21 | KunImage 5 个 prop 透传 + layer 层注册 `none` provider | 新增 API |
| [v0.4.5](./v0.4.5.md) | 2026-05-21 | z-index design token 系统：浮层不再被 Modal 压住 | 新增 + bug fix |
| [v0.4.4](./v0.4.4.md) | 2026-05-21 | `useKunMessage` 裸 render() 漏 appContext —— 一连串崩溃的真根因 | 🔴🔴 critical hotfix |
| [v0.4.3](./v0.4.3.md) | 2026-05-21 | `getRandomSticker` 双层修复：useState + 客户端 Map 缓存 | 🔴 critical hotfix |
| [v0.4.2](./v0.4.2.md) | 2026-05-21 | KunFileInput：补齐文件交互三层 | 新增组件 |
| [v0.4.1](./v0.4.1.md) | 2026-05-21 | 浮层"从角落飞来"动画错位修复（floating-ui transform: false） | bug fix |
| [v0.4.0](./v0.4.0.md) | 2026-05-21 | Primitives + Ergonomics 批次：KunSelect generic / Textarea expose / useFilePicker / KunRadioGroup | 4 项 |
| [v0.3.1](./v0.3.1.md) | 2026-05-21 | KunTab solid/light indicator 4px Y 错位修复 | bug fix |
| [v0.3.0](./v0.3.0.md) | 2026-05-21 | 统一 rounded token 系统（3 层覆盖） | 新增 + 重构 |
| [v0.2.2](./v0.2.2.md) | 2026-05-21 | KunUser `uid` 回滚回 `id`（v0.2.1 误改方向反转） | 修正回滚 |
| [v0.2.1](./v0.2.1.md) | 2026-05-21 | 下游 moyu 报回的 4 个 bug 修复（Null.vue 路径 / Button info 色 / Select defineModel / KunUser 字段名） | bug fix |
| [v0.2.0](./v0.2.0.md) | 2026-05-21 | 浮层引擎统一（@floating-ui/vue）+ a11y 工具链（focus-trap / a11y lint / vue-tsc） | 重大重构 |
| [v0.1.1](./v0.1.1.md) | 2026-05-21 | 复审加固（pr-review-toolkit 4 agent，6 🔴 + 8 🟠 全修） | 加固 |
| [v0.1.0](./v0.1.0.md) | 2026-05-20 | 首次大改：variant×color 单一来源 / Tab 重做 / KunChip + KunTagInput / KunBadge dot-count 重做 | 重大重构（含 7 处 API breaking） |

## 严重度标记

- 🔴 critical bug fix（产线影响）
- 🔴🔴 root-cause fix that resolved a cascade of seemingly-unrelated bugs（修复一处解决一连串看似无关的崩溃）
