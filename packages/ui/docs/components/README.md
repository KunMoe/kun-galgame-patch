# Components

单组件的 spec / API / 设计取舍演变。每个文件聚焦一个 KunUI 组件，把它跨版本的设计决策、API 演化、关键修复合并展示。

## 组件文档清单

| 组件 | 文件 | 关键里程碑 |
|---|---|---|
| `KunTab` | [kun-tab.md](./kun-tab.md) | §4 v0.1.0 重设计 5 variant；v0.3.1 修 solid 指示器 4px 错位 |
| `KunTagInput` | [kun-tag-input.md](./kun-tag-input.md) | §5 v0.1.0 终稿 spec |
| `KunSelect` | [kun-select.md](./kun-select.md) | v0.4.0 generic + readonly options；v0.4.7 长 label 横向溢出；v0.4.9 dropdown maxHeight 真溢出 |
| `KunModal` | [kun-modal.md](./kun-modal.md) | v0.0.1 起，v0.2.0 接入 focus-trap，所有 drawer / dialog 都基于它 |
| `KunImage` | [kun-image.md](./kun-image.md) | v0.4.6 加 5 个 prop 透传；v0.5.0 加加载 skeleton |
| `KunRadioGroup` | [kun-radio-group.md](./kun-radio-group.md) | v0.4.0 新增（classic + card 两 variant，含 ARIA） |
| `KunFileInput` | [kun-file-input.md](./kun-file-input.md) | v0.4.2 新增，与 useFilePicker + KunUpload 形成三层文件交互 API |
| `KunDrawer` | [kun-drawer.md](./kun-drawer.md) | v0.5.1 新增；v0.5.2 加 responsive（默认桌面右、手机底） |
| `KunLightbox` + `KunLightboxGallery` + `KunLightboxGalleryItem` | [kun-lightbox.md](./kun-lightbox.md) | v0.6.0 全面重写：`<dialog>` 底座 + view-transitions + 滑动切图 + 左/右旋按钮 + 工具条按截图重排 + 手势 bug 全清 + dark mode 修复；新增声明式 Gallery/Item 子组件 |

## 没单独立文件的组件

下面这些组件直接看源码 + JSDoc + props 类型即可（设计上相对直接，没有跨版本演化或复杂取舍）：

- KunButton / KunChip / KunBadge / KunCard / KunInput / KunTextarea / KunCheckBox / KunSwitch / KunSlider / KunAvatar / KunPopover / KunTooltip / KunDatePicker / KunUpload / KunProgress / KunInfo / KunAlert / KunPagination / KunContextMenu / KunHeader / KunBrand / KunLink / KunDivider / KunIcon / KunCopy

这些组件在 [changelog/](../changelog/) 各版本文件里有具体改动记录。
