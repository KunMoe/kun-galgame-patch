# Moyu 升级方案（v1，已拍板）

> 状态：**已拍板，本文档为最终设计依据，自包含**
> 日期：2026-05-18
> 阅读次序：先扫 §1 范围速览 → §2 设计哲学 → 按 §3（Wiki-coupled）与 §4（Moyu-internal）章节按 PR 切分实施。
> 配套文档：Wiki 侧最终方案见 `docs/proj/99-final-upgrade-plan.md`；Wiki API 契约见 `docs/galgame_wiki/`。

---

## 1. 范围速览

本次 moyu 升级分两条互不依赖的主线，可并行推进：

### 1.1 Wiki-coupled（下游必做，随 Wiki 发版对齐）

| # | 触发 | 内容 | 兼容性 | 优先级 |
|---|---|---|---|---|
| **W1** | Wiki U1 | `released string` → `release_date date? + release_date_tba bool` 全链路下游迁移 | **BREAKING**，与 Wiki 同期发版 | **已完成**（2026-05-18，MOYU-PR2，§3.1.5） |
| **W2** | Wiki U2 / PR5 | 消费 `covers[] / screenshots[] / effective_banner_hash`；banner 展示链切到 `effective_banner_hash → banner`；类型 + `banner_image_hash` 死字段全清；编辑表单 covers/screenshots 走 presence 全量替换 + 编辑器 UI | Additive；BREAKING 部分（`banner_image_hash` 移除）零残留 | **已完成**（2026-05-18，MOYU-PR3 + PR3b，§3.2.6 / §3.2.7）|
| **W3** | Wiki U3 | 代理新增的 taxonomy 修订端点（`/{tag\|official\|engine\|series}/:id/{revisions[/:rev],revert}`，**top-level 路径**，与 Wiki 最终实现对齐）+ 管理 UI 加修订历史/回滚入口 | Additive | **已完成**（2026-05-18，MOYU-PR4，§3.3.4）|

### 1.2 Moyu-internal（与 Wiki 解耦）

| # | 内容 | 状态结论 | 优先级 |
|---|---|---|---|
| **M1** | 列表排序 `SortField/SortOrder` SQL 注入面**安全核实** | **已审计通过**（2026-05-18，§4.1.5） | ~~P0~~ → P3（防御性加强可选） |
| **M2** | 列表分页恒加 `id DESC` 兜底，修翻页漂移 | **已完成**（2026-05-18，MOYU-PR1，§4.2.4） | ~~P1~~ → 关闭 |
| **M3** | `patch_resource` 追加式文件历史表 `patch_resource_file_history` | **已完成**（2026-05-18，MOYU-PR5，§4.3.5）| ~~P1~~ → 关闭 |
| **M4** | `GalgameEditDiffView` 升级为字符级 LCS diff（含长 intro_* 体验） | **已完成**（2026-05-18，MOYU-PR6，§4.4.4）| ~~P2~~ → 关闭 |
| **M5** | 上传 session 属主/单次/COMPLETED 不变量核实加固 | **已完成**（2026-05-18，MOYU-PR7，§4.5.4）| ~~P2~~ → 关闭 |
| **M6** | 下载链接按文件大小缩放预签名 TTL（人机校验可选） | **已完成**（2026-05-18，MOYU-PR8，§4.6.5）—— **方案大改**：架构不适用，改为对 `/link` 端点限流 | ~~P2~~ → 关闭 |
| **M7** | 待审/未落 S3 资源可见性核实（仅上传者可见） | 先核实再决定 | P3 |
| **M8** | 本地热点页缓存（若引入）必须按 `content_limit + 鉴权上下文` 维度隔离；NSFW 未命中按 not-found 而非 403 | 仅在引入缓存时一起做 | P3 |
| **M9** | DiffView 数组字段集合级折叠（aliases/tag_ids 显示"+X / −Y"） | **已完成**（2026-05-18，MOYU-PR6 合并实施，§4.4.4）| ~~P3~~ → 关闭 |
| **M10** | 资源举报双向追责（若上线举报功能） | 与举报功能产品决策同步 | 待产品 |

### 1.3 显式不做（接受现状或暂缓）

- **资源 AV 流水线 / Malware 扫描**：完整建模成本高、暂无明确威胁面；保留**"超时未审 → 自动删"原则**为将来接入扫描器时的硬性要求。
- **`patch_resource` 拆 Resource/File 两层**：当前单层（一个资源一个文件/链接）满足需求；拆两层属 schema 重构无对应业务收益。
- **后端字段级位掩码 RBAC**：Wiki 已有 PR + 审核工作流，本仓库 §15 明确"不在本地重做鉴权"；位掩码属过度设计。
- **本地缓存预先抽象 / 搜索引擎抽象**：当前无热点页缓存、搜索由 Wiki 持有，过早抽象。

---

## 2. 设计哲学

1. **下游永远不重做 Wiki 鉴权**（handbook §15.2）：W3 全程薄代理；本地 §15 已有的通用 `WikiEditProxy` 在路径镜像下天然吸收新端点（含 `?force=true` query 等），新增基本只加路由行。
2. **presence 全量替换语义**（Wiki §1.5 不变量 #5、本地已对 tag_ids 应用）**强制扩展到 covers/screenshots**：编辑/PR 表单**必须预填当前全量 covers/screenshots**，diff 后**只在集合变化时**才发送；省略字段=保持不变；空数组=显式清空。错误模式（只回传新增项）会**静默清空其余封面/截图**，与历史 tag 失效 bug 同源。
3. **修订模型对齐**：Wiki 已统一全快照修订（galgame_revision + taxonomy_revision），本仓库前端继续使用已实现的 `GalgameEditDiffView` 统一渲染 `changed_keys / snapshot` 形态。无需为 taxonomy 历史另写组件。
4. **可验证 / 可撤销**：每条改动配测试与回归护栏（M1 安全审计、M2 翻页幂等、W2 covers presence 单测、W3 taxonomy 历史/revert 端到端）。
5. **附加表/列默认 nullable + default**：避免下游迁移阻塞；新建表的迁移幂等（`IF NOT EXISTS`）。

---

## 3. Wiki-coupled 章节

### 3.1 W1：`released` → `release_date` + `release_date_tba`（**BREAKING**）

#### 3.1.1 影响面

`released` 字段从所有 galgame 响应中**消失**，替换为 `release_date`（`YYYY-MM-DD` 字符串或 null）+ `release_date_tba` bool。这是 BREAKING：所有读到 `patch.galgame.released` / 排序字段含 `released` / 列表筛选 year 的代码都要改。

#### 3.1.2 现状核实清单（实施前先 grep）

```bash
# 后端
grep -rn '"released"\|\.Released\b\|`released`' apps/api/internal
# 前端
grep -rn "released\b\|'released'\|\"released\"" apps/web/app
```

预期命中：
- 后端 enricher、Wiki client 的 `GalgameFull/GalgameHit/GalgameBrief` 结构、任何按 `released` 排序的查询参数白名单
- 前端 `shared/types/patch.d.ts` 的 `galgame.released` 字段（实际 patch.d.ts 当前未列 `released`，但 detail 页 / galgame 列表筛选若使用 Wiki `released_from/_to` 参数会涉及）
- 任何"按发售年筛选"的列表 UI

#### 3.1.3 变更清单

**后端**：
- `apps/api/internal/galgame/client/client.go`：
  - `GalgameHit` / `GalgameFull` / `GalgameBrief` 删除 `Released string`，新增 `ReleaseDate *string `json:"release_date"`` + `ReleaseDateTBA bool `json:"release_date_tba"``。
  - `SearchGalgameParams.ReleasedFrom / ReleasedTo` 保留（Wiki 搜索仍接受 `released_from/_to` 年份参数；未变更）。
- `apps/api/internal/galgame/enricher/enricher.go`：透传新字段到响应。
- 任何排序白名单含 `released` 的，替换为 `release_date`（若有）。

**前端**：
- `shared/types/patch.d.ts` 的 `GalgameCard.galgame` 增 `release_date: string | null; release_date_tba: boolean`，并在所有展示 banner 下方"发售年"/筛选下拉处改读新字段。
- 既有的 galgame 列表"年份筛选"如使用 Wiki search 的 `released_from/_to` 仍兼容。
- BREAKING 提示：发版前在 changelog 与 Telegram 群组通告。

#### 3.1.4 验收

- 详情/列表/编辑页发售日期展示正确（含 TBA 标记）
- Wiki `release_date_tba=true` 的作品在排序中按"未定"分组（与 Wiki `release_date_tba=true` 排序到末尾的语义一致）
- 旧 `released` 字段不再出现在任何响应/类型/UI

---

#### 3.1.5 实施记录（2026-05-18，MOYU-PR2）

实际改动面比初版预期小（详细 grep 显示）。最终落地 5 个文件：

| 文件 | 改动 |
|---|---|
| `internal/galgame/client/client.go` | `GalgameHit` 删 `Released string` → 加 `ReleaseDate *string + ReleaseDateTBA bool`；`GalgameBrief` 与 `GalgameFull` 各加同样 2 字段；注释补 U1 历史与 Wiki 搜索参数保留说明 |
| `internal/galgame/enricher/enricher.go:319-330` | `base.Galgame = &GalgameBrief{...}` 构造增加 `ReleaseDate / ReleaseDateTBA` 透传 |
| `shared/types/patch.d.ts` | `GalgameCard.galgame: { ... }` 增 `release_date: string \| null` + `release_date_tba: boolean`；`PatchDetail` extends GalgameCard 自动继承；附 U1 注释 |
| （未触及）`internal/common/search/search.go` 与 `client.go:182-225` | Wiki 搜索端点的 `released_from/_to` 入参 + `Sort=released_desc/_asc` 仍兼容（Wiki 内部派生 `released_year` 用于年范围 filter / sort），DTO 字段名不需改 |
| （未触及）前端 `editStore.ts` / `patch.d.ts` 顶端 D12 历史注释 | 提到旧 `released` 字段的是 D12 历史说明，非活引用，保留不动 |

**验证**：
- `go build ./...`（通过）
- `go vet ./...`（通过）
- `go test ./...`（通过，无 FAIL）
- 前端 `eslint app/shared/types/patch.d.ts` exit 0
- 最终 grep `'json:"released"'` 全项目空集 → 无 stray 旧响应字段
- 6 处 `release_date / release_date_tba` 落点正确（client.go 3 struct × 2 + enricher.go 2 + patch.d.ts 2）

**与 Wiki 同窗发版要点**：Wiki 文档已自带 BREAKING 通告（00-handbook §687），下游同期发版即可；Wiki migration cmd 已重写所有历史 `galgame_revision / galgame_pr.snapshot` 的 jsonb，**下游无快照迁移任务**。

---

### 3.2 W2：covers / screenshots 模型 + presence 全量陷阱

#### 3.2.1 数据形态变化

Wiki 响应新增：
```ts
interface PatchDetailGalgame {
  // ...
  effective_banner_hash: string  // sort_order=0 的 cover hash；空则前端 fallback
  covers: Array<{
    image_hash: string
    sort_order: number  // 0 = 当前 banner
    sexual: number; violence: number  // v1 不消费
    source: string; source_key: string  // 留扩展位，v1 不消费
  }>
  screenshots: Array<{
    image_hash: string
    sort_order: number
    caption: string
    sexual: number; violence: number
    source: string; source_key: string
  }>
}
```

`banner_image_hash` 过渡期保留；最终版本删除。`Banner string` URL 字段已按 D12/D13 移除计划继续 drop。

#### 3.2.2 前端展示改造

`apps/web/app/shared/types/patch.d.ts`：在 `GalgameCard.galgame` 与 `PatchDetail` 上追加 `effective_banner_hash`/`covers[]`/`screenshots[]`，过渡期同时保留 `banner_image_hash`（与 Wiki 一致）。

**Banner 解析顺序**：
```ts
const bannerHash = patch.galgame?.effective_banner_hash
  || patch.galgame?.banner_image_hash  // 过渡期 fallback
  || ''
const bannerUrl = bannerHash ? imageMainUrl(bannerHash) : (patch.banner || DEFAULT)
```

详情页可新增"截图/CG 画廊"区块（消费 `screenshots[]`），按 `sort_order` 升序展示；`caption` 作 alt/figcaption。**v1 不消费 `sexual/violence`**（与 Wiki §9.2 一致：粗粒度 NSFW 仍按 `content_limit` 走）。

#### 3.2.3 编辑表单 prefill + presence（强制，否则会清空封面）

`apps/web/app/pages/edit/rewrite.vue` 与 `apps/web/app/pages/patch/[id]/prs.vue` 已对 `tag_ids/official_ids/engine_ids` 实现"预填+差异判定才发送"。**covers/screenshots 必须套用同一模板**：

```ts
// rewrite.vue / prs.vue 增量
const covers = ref<SnapshotCover[]>([])
const screenshots = ref<SnapshotScreenshot[]>([])
const origCovers = ref<SnapshotCover[]>([])
const origScreenshots = ref<SnapshotScreenshot[]>([])

// 在 watch(detail) 里：
covers.value = [...(d.galgame?.covers ?? [])]
screenshots.value = [...(d.galgame?.screenshots ?? [])]
origCovers.value = covers.value.map(c => ({ ...c }))
origScreenshots.value = screenshots.value.map(s => ({ ...s }))

// buildPayload：
if (!sameCoverSet(covers.value, origCovers.value)) payload.covers = covers.value
if (!sameScreenshotSet(screenshots.value, origScreenshots.value)) payload.screenshots = screenshots.value
```

`sameCoverSet` 按 `(image_hash → row 完整字段)` 做集合 + 行内 6 字段对比（与现有 `sameIdSet` 同思路；新建 `sameRowSet` 辅助）。

**编辑器 UI（最小可用集）**：
- Covers：一个网格 + 上传按钮（走现有 `imageclient.Upload`）+ 每张可"设为当前 banner（钉到 sort_order=0）"按钮 + 删除。Wiki 端事务内会自动降级旧的 sort_order=0。
- Screenshots：网格 + 上传 + 拖拽排序（修改 `sort_order`）+ caption 输入 + 删除。
- 不实现：`sexual/violence` 评级 UI（v1 不消费）、`source/source_key` 编辑（留扩展位，不开放）。

#### 3.2.4 类型与 composable 扩展

`useGalgameEdit.ts` 的 `GalgameEditFields`：

```ts
export interface SnapshotCover { image_hash: string; sort_order: number; sexual?: number; violence?: number; source?: string; source_key?: string }
export interface SnapshotScreenshot extends SnapshotCover { caption?: string }

export interface GalgameEditFields {
  // ...
  covers?: SnapshotCover[]
  screenshots?: SnapshotScreenshot[]
  release_date?: string | null
  release_date_tba?: boolean
}
```

#### 3.2.5 验收

- 编辑表单加载时 covers/screenshots 已预填全量
- "钉为 banner"操作在 Wiki 端事务内降旧升新；前端刷新后 `effective_banner_hash` 正确
- 只改名字不动 covers/screenshots → 提交后 Wiki revision diff 不包含 covers/screenshots key
- 主动清空 covers → 提交 `covers: []` → Wiki 清空（与 tag_ids `[]` 一致）

---

#### 3.2.6 实施记录（2026-05-18，MOYU-PR3 / Phase A+B+C，编辑器 UI 分到 PR3b）

**完成范围**：后端类型 + 前端类型 + banner 展示链切换 + `banner_image_hash` 死字段全清 + composable/DiffView 类型标签更新。**未完成**：covers/screenshots 编辑器 UI（含新增的 image_service hash 上传后端端点）—— 拆为 MOYU-PR3b。

| 类别 | 文件 | 改动 |
|---|---|---|
| **A. 后端类型** | `client.go` | 新增 `CoverInput / ScreenshotInput` 共享类型；`GalgameHit / GalgameBrief / GalgameFull` 三结构各加 `EffectiveBannerHash / Covers / Screenshots`；`UpdateGalgameRequest` 加 `Covers/Screenshots` + **删除** `BannerImageHash` 字段 |
| | `submission.go` | `SubmitGalgameRequest` 加 `Covers/Screenshots` + **删除** `BannerImageHash`；`MineItem / WikiMessageGalgame` 字段从 `BannerImageHash` 重命名为 `EffectiveBannerHash` |
| | `enricher.go` | `base.Galgame` 构造时透传 `EffectiveBannerHash / Covers / Screenshots` |
| **B. 前端类型 + helper + 展示链** | `shared/utils/resolveBannerUrl.ts` | **新建**；镜像 `resolveAvatarUrl`；接受 patch 顶层 / 嵌套 galgame 两种 source shape；优先 `effective_banner_hash → banner` 两级；支持 `mini` 变体（image_service 派生 `_mini.webp` + 旧 `-mini.avif` URL 替换） |
| | `shared/types/patch.d.ts` | `galgame: { ... }` 增 `effective_banner_hash / covers[] / screenshots[]`；新增 `GalgameCoverRow / GalgameScreenshotRow` 类型 |
| | `pages/edit/{draft,create}.vue`, `pages/me/submissions.vue` | 3 个本地 interface 字段从 `banner_image_hash` 改为 `effective_banner_hash` |
| | `composables/useGalgameEdit.ts` | `GalgameEditFields` 加 `release_date / release_date_tba / covers / screenshots`；新增 export `CoverInput / ScreenshotInput` |
| | `components/galgame/edit/DiffView.vue` | `KEY_LABEL` 删 `banner_image_hash`；新增 `effective_banner_hash / covers / screenshots / release_date / release_date_tba` 中文标签 |
| | **8 处 banner 展示切换** | `PatchCard.vue / Card.vue / ranking/patch.vue / admin/galgame.vue / patch/[id].vue (×2 hero img) / user/[id]/resource.vue / resource/[id].vue` 全部从 `patch.banner.replace(...)` 改为 `resolveBannerUrl(patch, 'mini')` 或 `resolveBannerUrl(patch)` |
| **C. presence 安全（无 UI 编辑覆盖时的零风险设计）** | rewrite.vue / prs.vue | **无需改动**：当前表单不包含 covers/screenshots 字段→ payload 不发→ Wiki presence omit 保持原集合不变→ 安全。编辑器加上后才需要 prefill + sameRowSet diff（PR3b） |

**未完成项 → MOYU-PR3b（独立排期）**：
- 后端新增 `POST /api/upload/image-service` 端点：multipart 文件上传 → 转发 image_service 拿 hash → 返回 `{hash, width, height, variant_urls}`（封装 `pkg/imageclient.Upload`）
- 前端 cover 编辑器：网格 + 上传按钮（复用 PUT galgame multipart `file` 模式，Wiki 自动 PromoteCoverHash）+ 钉选/删除
- 前端 screenshot 编辑器：网格 + 上传按钮（走新 image-service 端点拿 hash，再随 `screenshots` 字段 PUT 提交）+ caption + 拖拽排序 + 删除
- rewrite.vue / prs.vue 加 origCovers / origScreenshots + sameRowSet diff + 在 buildPayload 包含 covers/screenshots（差异时）
- 详情页画廊区块（消费 `screenshots[]` 按 sort_order 展示）

**验证**（本期落地范围）：
- `go build / vet / test` 全绿
- ESLint 全部 14 个改动文件 exit 0
- `grep banner_image_hash apps/api apps/web` 全项目**零命中**（W2 死字段清理彻底）
- 8 个 banner 展示点统一走 `resolveBannerUrl`，新旧 banner 源（image_service hash / 老 URL）均可正确解析

**与 Wiki 同窗发版要点**：Wiki PR5 已删 `banner_image_hash`，moyu 本期与之同步——零回归。后续编辑功能（PR3b）不阻塞当前发版。

---

#### 3.2.7 实施记录（2026-05-18，MOYU-PR3b：W2 编辑器 UI + 详情页画廊）

补完 W2 的 D 部分。新增 8 个文件、改 2 个文件、零 break。

**A. 后端：新增 image_service 上传代理**
| 文件 | 改动 |
|---|---|
| `pkg/imageclient/client.go` **新建**（~190 行）| 通用 image_service SDK：`Config{BaseURL,CDNBase,ClientID,ClientSecret,HTTPClient}` + `New()` + `Upload(ctx, body, filename, mime, preset) (*UploadResult, error)` + `MainURL/VariantURL` URL helper；HTTP Basic 鉴权（OAuth client_id/secret，与 /users/batch 共用）；sentinel 错误 `ErrQuotaExceeded / ErrModerationRejected / ErrUnauthorized`；error envelope 映射；stdlib-only |
| `pkg/config/config.go` | 新增 `ImageServiceConfig{BaseURL, CDNBase, ClientID, ClientSecret}` + 环境变量 `KUN_IMAGE_SERVICE_BASE_URL / KUN_IMAGE_CDN_BASE / KUN_IMAGE_OAUTH_CLIENT_ID / KUN_IMAGE_OAUTH_CLIENT_SECRET`（后两者空则 app.go fall back 到 OAuth 凭据，零新配置）|
| `internal/app/app.go` | 构造 `imageclient.Client` 注入 `UploadHandler` |
| `internal/common/upload/handler.go` | `Handler` 加 `img *imageclient.Client` 字段；新增 `UploadImageService` 方法（多 part 接收 `file + preset`，10MB 上限，转发 SDK，sentinel 错误映射为 80008/60002 等 Wiki 兼容业务码透传给前端）|
| `internal/app/router.go` | 注册 `POST /api/v1/upload/image-service`（带 `auth` 中间件）|

**B. 前端：editor 组件 + composable 方法**
| 文件 | 改动 |
|---|---|
| `composables/useGalgameEdit.ts` | 新增 `uploadImageService(file, preset='topic')` 方法 + `ImageServiceUploadResult` 类型；走 `$fetch.raw` multipart |
| `shared/utils/resolveBannerUrl.ts` | 新增 export `imageServiceUrl(hash, variant?)` —— 给编辑器/画廊缩略图统一拼 CDN URL |
| `components/galgame/edit/CoversEditor.vue` **新建** | 网格展示当前 covers（按 sort_order 排序）；每张可"设为 Banner"（事务内 demote 旧 0→1）+"移除"；**不**做新建上传——新 banner 仍走 rewrite.vue 既有的 multipart `file` 流（Wiki `PromoteCoverHash` 自动 promote 到 sort_order=0），避免要求 admin 给 moyu 的 oauth_client 加 `galgame_banner` preset |
| `components/galgame/edit/ScreenshotsEditor.vue` **新建** | 网格 + 多选上传（走新 `/upload/image-service`，preset='topic'，moyu 默认已允许）+ caption 编辑 + 上/下调序 + 移除；本地 hash 去重防 Wiki PK 冲突 |
| `pages/edit/rewrite.vue` | 加 `covers / screenshots` 与 `origCovers / origScreenshots` refs；新 `sameRowSet`/`rowKey` helper；watch(detail) 预填全集；buildPayload 集合差异时才包含 covers/screenshots；template 在 Banner 段下嵌入两个 editor 组件；保留 bannerFile 老路径（multipart）作为"上传新封面"的入口 |
| `pages/patch/[id]/introduction.vue` | 新增"截图 / 画廊"区块（按 sort_order 渲染 grid，lazy load），并把底部"更多去 Wiki"提示里的"截图"字眼删掉 |

**验证**：
- `go build / vet / test` 全绿
- 6 个前端改动文件 ESLint exit 0
- 关键路径自检：rewrite 不动 covers/screenshots → buildPayload 不含 → Wiki presence omit 保持（通过）；钉新封面 → 本地 demote+promote → 整集发送 → Wiki 替换（通过）；上传截图 → image_service hash → append → 集合差异判断后发送（通过）

**部署前提（提醒 admin / ops）**：moyu 的 OAuth client 在 `kun_galgame_infra` 库的 `oauth_client` 表必须 `image_enabled=true` 且 `image_allowed_presets` 含 `'topic'`（截图用）。无需新增 `galgame_banner` preset（封面通过 Wiki multipart 流转）。环境变量未设时自动 fall back 到 OAuth 凭据，零新配置即可启动。

---

### 3.3 W3：taxonomy 修订端点代理 + 管理 UI 加历史/回滚

#### 3.3.1 新增 Wiki 端点（4 实体 × 3 路径，**top-level 路径已锁定**）

按 Wiki 04-taxonomy.md / 00-handbook §15 ADDITIVE 段实施。**路径前缀已确定为 top-level**（与 Wiki 既有 `/tag /official /engine /series` CRUD 一致；Wiki 团队评估嵌套风格 BREAKING 成本过高，最终保留 top-level）：

```
GET   /<entity>/:id/revisions          # 修订列表（分页，公开）
GET   /<entity>/:id/revisions/:rev     # 单条修订快照（公开）
POST  /<entity>/:id/revert             # body: {revision: N} （admin/moderator）
```

`<entity>` ∈ `{tag, official, engine, series}`。共 12 条新代理路由。

**preflight 不需要独立端点**：Wiki 的 `DELETE /<entity>/:id`（不带 `?force=true`）本身就是 preflight——被引用时返回 `code:7 + ref_count`，前端据此弹二次确认带 force 重试。这套两段式已在 §4.1.x 实现。

> **现有 taxonomy 代理路由零迁移**：本仓库当前 router.go 注册的 `GET /tag /official /engine /series`、`POST /tag`、`PUT /tag`、`DELETE /tag/:id`、`GET /tag/:name`、`GET /tag/search` 等全部已是 top-level，与 Wiki 文档一致。本 PR 只**新增** 12 条 revision/revert 路由，不动现有路由。`useGalgameEdit.ts` 也无需修改既有方法 URL。

#### 3.3.2 后端：仅新增 12 条 revision/revert 路由

`apps/api/internal/app/router.go` 在现有「Galgame taxonomy proxy」块**追加**：

```go
for _, e := range []string{"tag", "official", "engine", "series"} {
    api.Get("/"+e+"/:id/revisions", a.PatchHandler.WikiEditProxy)
    api.Get("/"+e+"/:id/revisions/:rev", a.PatchHandler.WikiEditProxy)
    api.Post("/"+e+"/:id/revert", auth, a.PatchHandler.WikiEditProxy)
}
```

**顺序铁律**：`/<entity>/:id/...` 子路径必须在 `/<entity>/:name`（已存在的）**之前**注册，否则 Fiber 会把 `1/revisions` 当作 `:name=1/revisions` 匹配掉。实施时把上面 3 条 × 4 实体的 12 条 **插在 `api.Get("/tag/:name", ...)` 之前**，同理 official/engine。series 路径只有 `:id` 没有 `:name`，注意 `/series/:id`、`/series/:id/revisions`、`/series/:id/revert` 三者全部 GET/POST 不同 method 或 path 末段不同，Fiber 能正确分派；新加的 `:id/revisions` 仍要在 `GET /series/:id` 之前。

`DELETE /<entity>/:id` 的两段式 `?force=true` 已支持（generic proxy 自动转发 query）。

#### 3.3.3 前端：composable 与 UI

`apps/web/app/composables/useGalgameEdit.ts` **新增** 12 个方法（4 实体 × 3）：

```ts
type TaxKind = 'tag' | 'official' | 'engine' | 'series'

const taxListRevisions = (kind: TaxKind, id: number, opts?: { page?: number; limit?: number }) =>
  api.get<WikiPage<TaxonomyRevision>>(`/${kind}/${id}/revisions${qs(opts as Q)}`)
const taxGetRevision = (kind: TaxKind, id: number, rev: number) =>
  api.get<TaxonomyRevision>(`/${kind}/${id}/revisions/${rev}`)
const taxRevert = (kind: TaxKind, id: number, revision: number) =>
  api.post<{ reverted_to: number }>(`/${kind}/${id}/revert`, { revision })
```

`TaxonomyRevision` 类型按 04-taxonomy.md §修订与回滚 的响应 shape 定义。

`apps/web/app/pages/galgame/taxonomy.vue` 在每行追加"历史"按钮 → 弹窗显示该实体修订列表 + 每条可查看快照与回滚。复用 `GalgameEditDiffView`（snapshot 同样是全量快照）。

`deleted` 状态的修订：弹窗按钮变为"恢复（撤销删除）"——后端 revert 自动 INSERT 主行 + 重建 aliases；UI 用 `affected_galgame_ids` 列出"该实体删除前被以下 N 部作品引用"，让 admin 勾选要恢复的，每个走 PUT galgame 标准编辑路径加回 tag_ids/official_ids/engine_ids/series_id（每个作品一条 `galgame_revision`）。

---

#### 3.3.4 实施记录（2026-05-18，MOYU-PR4：W3 完工）

**后端（极简，3 个文件）**
| 文件 | 改动 |
|---|---|
| `internal/app/router.go` | 在 taxonomy 块尾追加 4 行 for-loop **生成 12 条新代理路由**：`GET /<entity>/:id/revisions` `GET /<entity>/:id/revisions/:rev` `POST /<entity>/:id/revert` × 4 实体。注释解释 Fiber 按段数匹配，与已有 `/<entity>/:name` (2 段) 不冲突，顺序无关 |

零新 handler 代码——通用 `WikiEditProxy` 已能处理所有 pass-through，OriginalURL 镜像。Wiki 端鉴权（GET 公开 / revert 需 admin/moderator）由 Wiki 强制，下游零鉴权重做。

**前端 composable（3 个新方法 + 1 类型）**
| 文件 | 改动 |
|---|---|
| `composables/useGalgameEdit.ts` | 新增 `TaxonomyRevision` interface（含 `entity / target_id / revision / action / user_id / user_role / snapshot / changed_fields / ref_count / affected_galgame_ids / note / created`，所有 entity 共用一个 Go 多态表）；新增 `TaxKind` 别名 + 3 个方法 `taxListRevisions / taxGetRevision / taxRevert`（4 实体 × 3 端点 = 12 个调用面合并为 3 个签名）|

**前端 UI（taxonomy.vue 加修订历史弹窗）**
| 改动 |
|---|
| 每行追加「历史」按钮（在「编辑」「删除」前面）|
| 新增 `histOpen / histRow / histList / histPage / histTotal / histLimit` 状态 + `acting / expandedRev` 操作态 |
| `ACTION_LABEL: Record<string, {text, color: KunUIColor}>` —— `created / updated / deleted / reverted` 4 色映射；新增 `KunUIColor` 类型 import |
| `loadHistory / openHistory / doRevert / toggleExpand / fmtSnapshotValue` 函数 + `watch(histPage)` 自动重拉 |
| 弹窗模板：每条修订显示 #编号 + action badge + user/时间 + `changed_fields` + `note`；可展开「查看快照」键值对；`deleted` 行特殊红框展示 `ref_count + affected_galgame_ids` + 一行 UX 提示（恢复只复活实体本身、引用需手动加回）；按钮文案根据 action 动态切换（deleted → 「恢复」+ success 色；其他 → 「回滚到此版本」+ warning 色）；`KunPagination` 翻页 |

**验证**
- `go build / vet / test` 全绿（4 行加 12 条路由无需额外测试）
- 2 个前端改动文件 ESLint exit 0
- 关键路径自检：
  - `histRow` 切换不同实体时（用户切 tab 后再开历史）状态正确重置（通过）
  - revert 成功后既刷新 history 列表也刷新 entity 列表（被恢复的实体重新出现）（通过）
  - acting 状态防止并发点击多个 revert 按钮（通过）
  - 鉴权失败（普通用户点 revert）→ Wiki 返 403 → 透传 → useKunMessage 友好提示（通过）

**与 W2/W1 协同**：本 PR 纯 Additive，零回归。与 §3.3.3 描述的 12 端点完全对齐；既有 taxonomy CRUD 路由零迁移（top-level 已对齐）。

#### 3.3.3 前端：composable 扩展

`useGalgameEdit.ts` 新增 4 实体 × 4 方法 = 16 个方法（强类型；与已实现的 galgame revision/PR 同形）：

```ts
type TaxKind = 'tag' | 'official' | 'engine' | 'series'

const taxReferences = (kind: TaxKind, id: number) =>
  api.get<{ count: number; sample: any[] }>(`/${kind}/${id}/references`)

const taxListRevisions = (kind: TaxKind, id: number, opts?: { page?: number; limit?: number }) =>
  api.get<WikiPage<TaxonomyRevision>>(`/${kind}/${id}/revisions${qs(opts as Q)}`)

const taxGetRevision = (kind: TaxKind, id: number, rev: number) =>
  api.get<TaxonomyRevisionDetail>(`/${kind}/${id}/revisions/${rev}`)

const taxRevert = (kind: TaxKind, id: number, revision: number) =>
  api.post(`/${kind}/${id}/revert`, { revision })
```

`TaxonomyRevision` / `TaxonomyRevisionDetail` 类型按 Wiki §6.1 `taxonomy_revision` schema 定义（`action: 'created' | 'updated' | 'deleted' | 'reverted'`，`snapshot` 是 union of 4 种 entity snapshot）。

#### 3.3.4 前端：taxonomy 管理页扩展

`apps/web/app/pages/galgame/taxonomy.vue` 在每行右侧追加："编辑 / 删除"之外新增"历史"按钮 → 弹窗显示该实体的修订列表 + 每条可"查看快照"与"回滚到此版本"。

复用现有 `GalgameEditDiffView` 渲染快照差异（taxonomy_revision 的 snapshot 同样是全量快照，diff = 当前 vs 目标）。

`deleted` 状态的修订处理：弹窗按钮变为"恢复（撤销删除）"。Wiki 端 §7.2.4 实现"复活实体本身但不自动恢复引用"，UI 需告知用户"该 tag 删除前被 N 部作品引用，需手动恢复"（用 revision 上的 `affected_galgame_ids` 字段呈现列表）。

#### 3.3.5 验收

- 12 条新路由经构建 + 启动 Fiber 不冲突（`go test ./internal/app/...` + 进程启动通过）
- 在 taxonomy.vue 对一个 tag 做 PUT → 列表点"历史" → 看到 `updated` 一条 → 点回滚 → tag 字段恢复 + 新增 `reverted` 一条
- 删除一个被引用 tag（强制）→ 历史中看到 `deleted` 条带 `ref_count`/`affected_galgame_ids` → "恢复"按钮可复活实体（不恢复引用）
- 错误透传：非 admin 调 revert 见 Wiki 的 403/code

---

## 4. Moyu-internal 章节

### 4.1 M1：列表排序 SQL 注入面**安全核实**（已审计通过 · 2026-05-18）

#### 4.1.1 问题

`apps/api/internal/common/handler.go` 多处用：

```go
.Order(fmt.Sprintf("%s %s", req.SortField, req.SortOrder))
.Order(fmt.Sprintf("patch_resource.%s %s", sortField, req.SortOrder))
```

字符串拼接进 ORDER BY。**若 DTO 未对 `SortField/SortOrder` 做严格 `oneof` 白名单**，攻击者可传任意字符串注入。这是 SQL 注入面，**优先于其他升级核实**。

#### 4.1.2 实施步骤

1. **第一步：审计** —— 列出所有命中：
   ```bash
   grep -rn 'Order(fmt.Sprintf' apps/api/internal
   grep -rn 'Order(' apps/api/internal | grep -v '"created\|"id\|"updated\|RANDOM\|Expr('
   ```
2. **第二步：核实每个 `Order` 的 `SortField/SortOrder` 来源 DTO 是否有 `validate:"oneof=..."` 白名单**：
   - 命中位置（已知）：`internal/common/handler.go:152, 187, 270, 474, 529`
   - DTO 文件：`internal/patch/dto/dto.go` / `internal/common/*` 各 GET DTO
3. **第三步：未白名单的全部加上**：
   ```go
   SortField string `validate:"oneof=created updated resource_update_time view download"`
   SortOrder string `validate:"oneof=asc desc"`
   ```
4. **第四步：保险层** —— 即使 DTO 校验存在，handler 处再做一次显式白名单兜底（防 DTO 漏过 / 未来加字段忘了更新）：
   ```go
   var sortWhitelist = map[string]bool{"created": true, "updated": true, ...}
   if !sortWhitelist[req.SortField] { req.SortField = "created" }
   if req.SortOrder != "asc" && req.SortOrder != "desc" { req.SortOrder = "desc" }
   ```

#### 4.1.3 验收

- 单测：传 `SortField=";DROP TABLE patch;--"` 应返回 400（DTO 校验失败）或被兜底回默认值
- grep 确认无未白名单的 `Order(fmt.Sprintf(...SortField...))`

---

#### 4.1.4 审计结论（2026-05-18）

只读审计已完成。**全部 5 处 `Order` 字符串拼接均有上游白名单保护，无注入风险**：

| 文件:行 | 拼接表达式 | 保护来源 | 状态 |
|---|---|---|---|
| `internal/common/handler.go:152` | `fmt.Sprintf("%s %s", req.SortField, req.SortOrder)` | `galgameListRequest.SortField validate:"required,oneof=resource_update_time created view download"` + `SortOrder oneof=asc desc`（行 124-125） | 通过 |
| `internal/common/handler.go:187` | 同上 | `commentListRequest`：`SortField oneof=created like_count`（行 168-169） | 通过 |
| `internal/common/handler.go:270` | `fmt.Sprintf("patch_resource.%s %s", sortField, req.SortOrder)` | `resourceListRequest.SortField oneof=update_time created download like_count`（行 244-245）；本地变量 `sortField` 进一步带 fallback `"like_count"` | 通过 |
| `internal/common/handler.go:474` | `orderBy`（本地变量）| `switch sortBy { case "patch"/... }` 全部映射为**字面量字符串**，默认 `"u.moemoepoint DESC"` | 通过 |
| `internal/common/handler.go:529` | `fmt.Sprintf("%s DESC", column)` | `column` 来自 `switch sortBy`，全部映射为**字面量字符串**，默认 `"view"` | 通过 |

**结论**：M1 不存在线上漏洞，**降级为 P3 防御性加强（可选）**。可考虑的可选加强（不是必须）：
1. handler 层加 map fallback 兜底（保险层），防 DTO 未来加 SortField 字段忘了更新 `oneof`：

```go
var allowedSort = map[string]bool{"created":true, "resource_update_time":true, ...}
if !allowedSort[req.SortField] { req.SortField = "created" }
```

2. 抽出 `safeOrderBy(sortField, sortOrder string, whitelist map[string]bool, fallback string) string` 公共工具，统一所有 Order 拼接走它。

以上**不阻塞其他升级**，可作为 M2/M3 完成后的清理项。

---

### 4.2 M2：列表分页 `id DESC` 兜底（P1）

#### 4.2.1 问题

`Order("created DESC")` / `Order("download DESC")` / `Order("<sortField> <sortOrder>")` 等**均无 id 兜底**。同值时 PostgreSQL 返回顺序无定义 → 翻页可能出现重复/遗漏。

#### 4.2.2 实施

在所有分页 `Order` 末尾追加 `, id DESC`（或对 `patch_resource.*` 排序追加 `, patch_resource.id DESC`，避免列名歧义）。涉及文件（已知）：

| 文件 | 行 | 改动 |
|---|---|---|
| `internal/common/handler.go` | 101, 102, 103 | `Order("created DESC")` → `Order("created DESC, id DESC")` |
| `internal/common/handler.go` | 152, 187 | `Order(fmt.Sprintf("%s %s", ...))` → 末尾拼 `, id DESC` |
| `internal/common/handler.go` | 270 | 同上，前缀 `patch_resource.` |
| `internal/common/handler.go` | 311 | `Order("download DESC")` → `Order("download DESC, id DESC")` |
| `internal/common/handler.go` | 474, 529 | 同上 |
| `internal/patch/repository/repository.go` | 77, 79, 139 | 同上（`Order("created DESC")` 等） |

> RANDOM 排序（`repository.go:59`）不需要兜底。

#### 4.2.3 验收

- 单测：构造 5 行同 `created` 时间戳的数据，分两页（`limit=2`）取 → 总共应覆盖 5 行无重复
- grep `'Order(.*DESC")'`、`'Order(fmt.Sprintf'` 确认末尾都有 `id`

---

#### 4.2.4 实施记录（2026-05-18，MOYU-PR1）

最终落地 **16 个 Order 调用点 × 6 个文件**（实施时全项目复查发现初版 §4.2.2 表漏列的 5 处，已一并修复）：

| 文件 | 修改点 | 处理 |
|---|---|---|
| `internal/admin/repository/repository.go` | 37, 64, 110, 142（4 处分页 `Order("created DESC")`）| `, id DESC` 兜底；164 已有 `, id ASC` 跳过 |
| `internal/patch/repository/repository.go` | 77（评论分页）、79（回复 Preload 内）、139（资源全量）| 77/139 加 `, id DESC`；79 加 `, id ASC`；59 `RANDOM()` 跳过 |
| `internal/user/repository/repository.go` | 70/81/93/104/116（5 处用户子页：patches/resources/favorites/comments/contributions）| 全部 `, id DESC` |
| `internal/common/handler.go` | 101/102/103（home top-N）、152/187（galgame/comment 列表 Sprintf）、270（resource 列表带 `patch_resource.` 前缀）、311（推荐 top-N）、474（user ranking switch 内 4 个 orderBy 字面量统一加 `, u.id DESC`）、529（patch ranking）| 全部兜底，注释说明 |
| `internal/message/repository/repository.go` | 39（用户通知分页）| `, id DESC` |
| `internal/chat/repository/repository.go` | 49（chat 房间列表）、233（房间成员列表）| 49 加 `, chat_room.id DESC`；233 加 `, id ASC`；其余 5 处本就是 `Order("id ASC/DESC")` 已稳定，跳过 |

**验证**：
- `go build ./...`（通过）
- `go vet ./...`（通过）
- `go test ./...`（通过，无回归）
- 最终 grep `'Order('` 排除 `id (ASC|DESC)|RANDOM|orderBy)` 结果为空 → 全项目 Order 调用均已 id 兜底或为合法跳过

无单测（项目当前无 list-pagination 集成测试基础设施；改动机械、SQL 语义明确，通过 build+vet+静态扫描三重校验）。后续若需要可在 `internal/testutil` 上加 PG 容器搭配场景测试。

---

### 4.3 M3：`patch_resource` 文件历史表（P1）

#### 4.3.1 问题

`patch_resource` 是 S3 资源（`s3_key/blake3/size`）或外链。当前更新/重传**原地 mutate**，零审计——
- 用户报"下载坏了"时无法回溯何时换了什么
- 误改/恶意改无追责依据
- 即使有 `patchResourceUpdate` 消息通知收藏者，也无法告诉收藏者"换的是新文件还是改了元数据"

#### 4.3.2 新增表

```sql
-- migrations/007_patch_resource_file_history.up.sql
CREATE TABLE patch_resource_file_history (
    id              BIGSERIAL PRIMARY KEY,
    resource_id     INT NOT NULL REFERENCES patch_resource(id) ON DELETE CASCADE,
    -- 旧文件快照
    old_storage     VARCHAR(16) NOT NULL,                   -- 's3' / 'mega' / 'onedrive' / ...
    old_s3_key      VARCHAR(2048) NOT NULL DEFAULT '',      -- storage='s3' 时有效
    old_blake3      VARCHAR(128) NOT NULL DEFAULT '',
    old_size        BIGINT NOT NULL DEFAULT 0,
    old_content     TEXT NOT NULL DEFAULT '',               -- 旧外链原文（保留原始 URL/文本）
    -- 元数据
    reason          TEXT NOT NULL DEFAULT '',               -- 操作者填写"为什么换"，无强制
    actor_id        INT NOT NULL,                           -- 谁做的（admin / 资源属主）
    actor_role      INT NOT NULL DEFAULT 0,                 -- 快照角色（与 wiki 修订一致语义）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_prfh_resource ON patch_resource_file_history(resource_id, created_at DESC);
```

#### 4.3.3 写入点

`internal/patch/service/service.go` 的 `UpdateResource` 与 `DeleteResource`（保留删除前快照供后续 admin 取证）：

```go
func (s *PatchService) UpdateResource(uid int, resID int, req *dto.UpdateResourceDTO) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        var cur model.PatchResource
        if err := tx.First(&cur, resID).Error; err != nil { return err }

        // 判定"文件实质变更"才落历史（仅改 note/type/language/platform 不写历史）
        fileChanged := req.Storage != "" && (req.Storage != cur.Storage ||
                       req.S3Key != cur.S3Key || req.Content != cur.Content)
        if fileChanged {
            if err := tx.Create(&model.PatchResourceFileHistory{
                ResourceID: cur.ID,
                OldStorage: cur.Storage, OldS3Key: cur.S3Key,
                OldBlake3: cur.Blake3, OldSize: cur.Size,
                OldContent: cur.Content,
                Reason: req.Reason,  // 新增 DTO 字段，可选
                ActorID: uid, ActorRole: getRoleFromCtx(),
            }).Error; err != nil { return err }
        }
        // ... 原有 Updates 逻辑
        return nil
    })
}
```

DTO 加 `Reason string` 可选字段（最大 500 字）。

#### 4.3.4 admin 端点（最小）

```
GET /admin/resource/:id/history → 该资源的所有 history 行，倒序
```

不开放给前台用户（隐私 + 防探测）。

#### 4.3.5 验收

- 单测：先 create 资源 → update 同 s3_key（仅改 note）→ history 0 行 → update 换 s3_key → history 1 行
- 单测：delete 资源 → history 行不被级联删（验证 CASCADE 行为正确——其实应级联删；上面 schema 是 ON DELETE CASCADE 因为资源不存在了，history 也无意义；若希望保留则改 SET NULL 并加 `resource_id_nullable`。**决策：保留 CASCADE**，删除即遗忘，避免被删资源的历史残留泄露信息）

---

#### 4.3.6 实施记录（2026-05-18，MOYU-PR5：M3 完工）

**后端（6 文件）**
| 文件 | 改动 |
|---|---|
| `migrations/007_patch_resource_file_history.{up,down}.sql` **新建** | 建表 + 复合索引 `(resource_id, created_at DESC)`；`ON DELETE CASCADE` 与资源同寿；列含 old_storage/s3_key/blake3/size/content + reason + actor_id + actor_role + created_at |
| `internal/patch/model/model.go` | 新增 `PatchResourceFileHistory` struct（与 SQL 列一对一，GORM 标签含复合索引 + auto created_at） |
| `internal/patch/dto/dto.go` | `PatchResourceUpdateRequest` 新增可选 `Reason string` 字段（max 500），用于"为什么换文件"的操作者备注 |
| `internal/patch/service/service.go` | `UpdateResource` 签名扩展 `(resourceID, userID, *Resource, reason string, actorRole int)`；新增**文件变更检测**（`Storage / S3Key / Content` 任一变化即视为文件替换）；变更时在 `db.Transaction` 内**先写一条 history 行再更新主表**；纯元数据编辑跳过历史；aggregate refresh 放事务外 |
| `internal/patch/handler/handler.go` | `UpdateResource` 解析改用 `PatchResourceUpdateRequest`；从 OAuth roles 推导 `actorRole`（admin=3 / moderator=2 / user=1）；传入新增 reason + actorRole |
| `internal/admin/{repository,service,handler}.go` + `internal/app/router.go` | 新增 `GetResourceFileHistory` 三层（repository 按 `created_at DESC, id DESC` 翻页）+ admin 端点 `GET /api/v1/admin/resource/:id/history`（gated by `moderatorAuth`）|

**前端（1 文件）**
| 文件 | 改动 |
|---|---|
| `pages/admin/resource.vue` | 每行追加「历史」按钮；新增 `FileHistoryItem` interface + `ACTOR_ROLE_LABEL` 映射 + history 弹窗状态/加载/分页；弹窗逐条显示旧 storage/s3_key/blake3/size/content + 操作者+角色+时间 + 备注（斜体引用样式）|

**验证**
- `go build / vet / test` 全绿；fresh `go clean -testcache && go test ./...` 通过
- ESLint 改动文件 exit 0
- Grep 确认 `service.UpdateResource(` 唯一新调用点在 patch handler（admin 的 `AdminService.UpdateResource(id, note, adminUID)` 是同名不同语义函数，无冲突）
- 自检：
  - 仅改 note 不写 history（fileChanged=false）（通过）
  - 换 storage / s3_key / content 任一 → 1 条 history 行（事务内先写后改）（通过）
  - 删除资源 → CASCADE 同删 history（删除即遗忘语义）（通过）
  - 普通用户 / admin / moderator 编辑各自记录正确的 actor_role（通过）
  - admin UI「历史」按钮可拉取分页 + 弹窗展示（通过）

**设计取舍**
- **文件变更判定**只看 `Storage / S3Key / Content` 三个字段——size 是展示字符串（"100 MB"等），不算文件本身变化；type/language/platform 是元数据 jsonb，更不算
- **CASCADE 删除历史**：避免被删资源的历史残留泄露信息（旧 s3_key 等可能含敏感路径）；若将来需要保留可改 SET NULL + nullable resource_id
- **`Reason` 完全可选**：UI 留 placeholder 但不强制（强制会增加摩擦，admin/owner 多数情况不会填）；填了才有价值
- **actor_role 用 int**（3/2/1/0）与 Wiki 修订表语义一致；handler 层从 OAuth roles 字符串推导
- **aggregate refresh 放事务外**：避免大事务持有时间过长（counter 更新是后续读修复，非强一致）

---

### 4.4 M4：`GalgameEditDiffView` 字符级 LCS diff（P2）

#### 4.4.1 问题

当前 `apps/web/app/components/galgame/edit/DiffView.vue` 对每个 changed key 渲染**整块 old / new 文本**。`intro_*` 单语言可达 2 万字，仅改一两段时整块前后对比阅读成本极高。

#### 4.4.2 方案

- 对**短字符串**（< 200 字）保留整块对照（视觉对齐更清晰）
- 对**长字符串**（≥ 200 字）做字符级 / 行级 LCS diff：
  - 预处理：剥离 old/new 的公共前缀+后缀（`trimSharedEdges`，能把"只改中间一段"的 diff 长度降到改动量级）
  - LCS 设上限 `STRING_DIFF_DP_MAX_CELLS = 4_000_000`（约 2000×2000 字符），超限回退整块替换（避免恶意大字段拖死浏览器）
  - 行级模式：按 `\n` 切分先做行级 LCS，再对修改的行做字符级 LCS（性能与可读性最佳平衡）
- 渲染：新增行/段染绿背景、删除染红背景、未变保持原色；可点折叠未变上下文（默认展开 3 行）

#### 4.4.3 数组字段集合级折叠（合并实施 M9）

`aliases / tag_ids / official_ids / engine_ids` 等数组字段，diff 时不展示"index=0 改了 index=1 改了"，而是渲染"+ 添加: A,B / − 删除: X,Y / = 保留: …"。已知字段类型从 `Snapshot` 字段名识别（aliases / *_ids 是数组）。

#### 4.4.4 实施位置

新建 `apps/web/app/components/galgame/edit/StringDiff.vue`（字符/行级 diff）+ `apps/web/app/utils/lcs-diff.ts`（纯函数）。`DiffView.vue` 引入：

```vue
<template>
  <div v-for="k in keys" :key="k">
    <p class="title">{{ label(k) }}</p>
    <ArrayDiff v-if="isArrayKey(k)" :old="oldSnap?.[k]" :new="newSnap?.[k]" />
    <StringDiff v-else-if="isLongString(k, oldSnap?.[k], newSnap?.[k])" :old="..." :new="..." />
    <BlockDiff v-else :old="oldSnap?.[k]" :new="newSnap?.[k]" />  <!-- 现有整块对照 -->
  </div>
</template>
```

#### 4.4.5 验收

- 单测：纯函数 `diffLines("aaa\nbbb\nccc","aaa\nXXX\nccc")` 返回 `[{op:'eq',line:'aaa'},{op:'del',line:'bbb'},{op:'add',line:'XXX'},{op:'eq',line:'ccc'}]`
- 单测：8MB 大字符串 diff 不卡死（命中 cell 上限，返回回退结果）
- 视觉：在编辑/PR diff 弹窗看长 intro 改动只高亮改动段

---

#### 4.4.4 实施记录（2026-05-18，MOYU-PR6：M4 + M9 合并实施）

**新增 3 个文件**：

| 文件 | 改动 |
|---|---|
| `utils/lcs-diff.ts` **新建** | 纯函数库（无 Vue/Nuxt 依赖）：`trimSharedEdges(old, new)` 剥公共前后缀加速「改中间一段」case；`lcsDiff<T>(a, b, eq)` 通用 LCS 带 `STRING_DIFF_DP_MAX_CELLS=4_000_000` 单元上限护栏；`diffLines / diffChars`（chars 自动合并连续同 op 段减少 DOM 节点）；`setDiff(old, new)` 集合差返 `{added, removed, kept}`；溢出时返 null 让 caller 回退 |
| `components/galgame/edit/StringDiff.vue` **新建** | 行级 LCS + 共享前后缀剥离 + 修改对 (del,add) 内联字符级 diff；命中 cell 上限自动回退为传统侧拼 old/new；未变上下文默认折叠为「··· N char unchanged ···」可展开 |
| `components/galgame/edit/ArrayDiff.vue` **新建** | 集合级 +新增 / −删除 / =保留 展示；对象元素（covers/screenshots/links）调用 `display()` 紧凑摘要：cover 显 `hash:abcd…(sort:0)`、link 显 `name → url`，避免完整 JSON 噪声 |

**改 1 个文件**：

| `DiffView.vue` | 新增 `ARRAY_KEYS = {aliases, tag_ids, official_ids, engine_ids, links, covers, screenshots}` 集合；`LONG_STRING_THRESHOLD = 200`；helper `isArrayKey` / `isLongStringKey`（`intro_*` 一律视为长字符串）；模板按字段形态分派：`proposalOnly` → 单栏快照；数组 → `GalgameEditArrayDiff`；长字符串 → `GalgameEditStringDiff`；其余标量 → 现有侧拼 block。**对外 props 接口零变化**，3 处既有 caller（revisions.vue / prs.vue / taxonomy.vue 内联）零修改 |

**设计取舍**
- `STRING_DIFF_DP_MAX_CELLS = 4_000_000` 约 2000×2000 字符，覆盖正常 intro（≤20KB）；恶意大输入自动回退避免拖死浏览器
- 行级 LCS + 修改对内字符级 LCS 是性能与可读性平衡：纯字符级对 20KB 文本 DP=4×10^8 直接爆，纯行级又看不出"行内改了哪几个字"
- ArrayDiff 用对象 JSON 序列化作集合 key——对象比较的常用做法；前端展示用紧凑 `display()` 而不是完整 JSON 减少视觉噪声
- DiffView 字段形态分派优先级：proposalOnly > 数组 > 长字符串 > 标量；用 `isArrayKey` 兼容 ARRAY_KEYS 集合外但实际是数组的字段（防御性）

**验证**
- ESLint 4 个文件全 exit 0
- DiffView 对外接口零变化：grep 确认 2 个调用方 (`revisions.vue` / `prs.vue`) 无需修改；taxonomy.vue 内联渲染快照、不经 DiffView，不受影响
- 后端 zero 改动
- 手算验证算法正确：`trimSharedEdges('abc','aBc') = {prefix:'a', oldMid:'b', newMid:'B', suffix:'c'}`（通过）；`diffLines('a\nb\nc','a\nB\nc')` → `[eq a, del b, add B, eq c]`（通过）；`setDiff([1,2,3],[2,3,4])` → `{added:[4], removed:[1], kept:[2,3]}`（通过）
- 项目当前无 web 单测基础设施（无 vitest/jest）；算法正确性靠手算 + 实际渲染时的视觉验证。如未来加 vitest 可直接 unit-test lcs-diff.ts（纯函数无 DOM 依赖）

**不引入的能力**（避免过度设计）
- diff highlighting 库（如 diff-match-patch）：依赖 + 60KB，本场景手写 LCS 足够
- 拖拽分屏对比视图：现折叠式行内已足够
- Markdown 感知 diff（按段落而非按行）：边界 corner case 太多，行级在实践中够用

---

### 4.5 M5：上传 session 不变量核实加固（P2）

#### 4.5.1 问题

D10 设计要求资源仅能由 **COMPLETED + 属主 + 单次** 的 upload session 派生。需核实 `internal/common/upload/service.go` 与 `internal/patch/service/service.go` 的 `CreateResource` 路径是否严格 enforce。

#### 4.5.2 不变量

1. **属主**：`upload_session.user_id == ctx.uid`
2. **状态**：`upload_session.status == 'COMPLETED'`
3. **单次使用**：标记 session 为 `consumed`（或物理删除）后才落 `patch_resource` 行；若并发两次提交同一 sessionID，第二次必须失败
4. **匹配文件**：`patch_resource.s3_key == upload_session.s3_key`（防张冠李戴）

#### 4.5.3 实施

在 `CreateResource` 服务函数中：

```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    var sess UploadSession
    // SELECT FOR UPDATE 锁住 session，保证单次使用
    if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("id = ? AND user_id = ? AND status = 'COMPLETED' AND consumed_at IS NULL", sessionID, uid).
        First(&sess).Error; err != nil { return errors.ErrBadRequest("无效或已使用的上传会话") }

    // 标记 consumed
    sess.ConsumedAt = ptrTime(time.Now())
    if err := tx.Save(&sess).Error; err != nil { return err }

    // 创建 patch_resource，s3_key 必须来自 session
    res := model.PatchResource{
        S3Key: sess.S3Key, Blake3: sess.Blake3, Size: sess.Size,
        // ... 其他字段
    }
    return tx.Create(&res).Error
})
```

DDL 增量：`upload_session` 加 `consumed_at TIMESTAMPTZ NULL`（若不存在）+ 唯一约束 `UNIQUE(id) WHERE consumed_at IS NULL` 视情况。

#### 4.5.4 验收

- 单测：并发提交同 sessionID 两次 → 一次成功一次失败
- 单测：用别人的 sessionID → 失败
- 单测：未 COMPLETED 的 sessionID → 失败

---

#### 4.5.5 实施记录（2026-05-18，MOYU-PR7：M5 完工）

**审计结论先行**：D10 上传流**没有 upload_session 表**——s3_key 即"session 句柄"，CreateResource 只接受字符串。原 §4.5 的 4 条不变量（属主 / COMPLETED / 单次 / 匹配文件）不能套用 session 模型。改成针对实际暴露面的 3 个加固：

| 弱点 | 修法 | 文件 |
|---|---|---|
| **A. 路径前缀未校验** —— CreateResource 接受任意 s3_key（理论上可挂 `secrets/*` 等外部路径） | 在 service 入口校验 `storage='s3'` 必须 `s3_key` 以 `patch/{galgameID}/` 开头 | `internal/patch/service/service.go::CreateResource` |
| **B. 同 s3_key 可挂多 resource** —— 无去重，"单次使用"形同虚设 | DB 部分唯一索引 `idx_patch_resource_s3_key_unique ON patch_resource(s3_key) WHERE storage='s3' AND s3_key<>''`；service 捕获 unique violation 透传友好提示 | `migrations/008_patch_resource_s3_key_unique.{up,down}.sql` + service |
| **C. CompleteSmall/Multipart 配额双扣** —— 同 s3_key 调 complete 两次 → daily_upload_size 扣两次 | Redis `SetArgs NX + 24h TTL`：首次 complete 标记 `upload:complete:{s3_key}`，后续 complete 跳过配额扣减（仍返回正确 size 让客户端重试无感）| `internal/common/upload/service.go::verifyAndFinalize` + 新 `markCompleteOnce` |

**改 5 文件 + 2 新迁移**：
- `migrations/008_patch_resource_s3_key_unique.{up,down}.sql` 新建，含 prod 升级前必跑的 dedup audit SQL 注释
- `internal/common/upload/service.go` `Service` 加 `rdb *redis.Client` 字段；New 接受 rdb；新 `markCompleteOnce` 用 `SetArgs{Mode:"NX", TTL: 24h}` 与 `middleware/auth.go::refreshOAuthToken` 现有模式一致（避免 deprecated `SetNX`）；`verifyAndFinalize` 在配额扣减前调用 markCompleteOnce
- `internal/app/app.go` `uploadPkg.New(s3, db, rdb)` 注入 redis
- `internal/patch/service/service.go` `CreateResource` 入口加路径前缀校验 + import `strings`；捕获 unique violation 错误（containing `idx_patch_resource_s3_key_unique` 或 `duplicate key value`）改提示「该上传已被其它资源占用，请重新上传一次」

**验证**
- `go build / vet / test` 全绿；fresh test 9 packages ok 0 FAIL
- 静态分析：deprecation 警告已通过改用 `SetArgs` 解决
- 自检：
  - 提交带前缀 `patch/123/...` 的 s3_key → 通过路径校验（通过）
  - 提交 `secrets/foo` 等 → 入 service 立即拒绝（早于 DB）（通过）
  - 同一 s3_key 创建两次 patch_resource → 第二次 DB unique violation → 友好提示（通过）
  - 同一 s3_key 调 CompleteSmall 两次 → 首次正常扣配额，第二次 SETNX 命中跳过扣减（idempotent）（通过）
  - Redis 不可达时 markCompleteOnce 抛错，complete 失败——可接受（Redis 是已部署的核心依赖）

**不做的事**（避免过度设计）
- 不加 upload_session 表：4 个不变量改通过 1 个 DB unique + 1 个路径前缀 + 1 个 Redis SETNX 全部覆盖；新建表是更大的 schema 工程
- 不在 CreateResource 里加 S3 HeadObject 验证（多 1 个 S3 RTT 给所有创建路径，收益主要是"自坑：用户提交错 key"——损失只是该用户自己看到下载失败；保留为可选未来增强）
- 不做"属主"验证（要 session 表才能可靠做）：64 字符 s3_key 随机段实际上承担了"难以伪造"职责；如未来真的发生 key 泄露事件再考虑

**部署提醒**：
- 迁移 008 是 DB 层加 unique，prod 升级前必须先跑 dedup audit SQL（migration 顶部注释里）——若有重复，先合并/清理再上线
- Redis 不可用时 complete 失败：现有 D10 已强依赖 Redis（session），无新依赖增加

---

### 4.6 M6：下载链接按文件大小缩放预签名 TTL（P2）

#### 4.6.1 问题

当前 `GET /patch/resource/:resourceId/link` 直接返回 `s3_key/content`，无 TTL 缩放、无人机校验。大文件用户下载时间长，固定短 TTL 会断流；统一长 TTL 会被盗刷。

#### 4.6.2 方案

按 `patch_resource.size` 计算 presigned URL TTL：

| size | TTL | 备注 |
|---|---|---|
| ≤ 100 MB | 1 h | 桌面/移动端单次拉取充足 |
| ≤ 1 GB | 2 h | 一般补丁 |
| ≤ 5 GB | 4 h | 大整合包 |
| > 5 GB | 6 h | 上限 |

实施位置：`internal/common/upload/service.go` 的 presigned URL 生成处（如未拆出 helper，集中到一个 `presignedDownloadURL(s3Key string, size int64)` 函数）。

#### 4.6.3 人机校验（可选，看是否真有盗刷）

如有数据证明被盗刷：在 `GET /patch/resource/:id/link` 接入 Cloudflare Turnstile token 校验（请求头 `x-turnstile-token` → 调 `siteverify`）。前端在 `useApi().get` 前先拿 token。**默认不做，等到有信号再加**。

#### 4.6.4 验收

- 单测：1 GB 资源 link 响应 TTL 在 2h 区间
- 现有下载流程无回归

---

#### 4.6.5 实施记录（2026-05-18，MOYU-PR8：M6 —— **方案重构**）

**审计先行 — 架构与原方案不匹配**：

原计划 §4.6 假设下载走 presigned GET URL，按文件大小缩放 TTL。**实际架构**：

- `GET /patch/resource/:resourceId/link` 返回 `{storage, content, code, password}`
- `content` 是公开 S3 URL（`PublicURL = endpoint + "/" + bucket + "/" + s3_key`），由客户端在 CreateResource 时生成并持久化到 `patch_resource.content`
- 后端**不生成任何 presigned GET URL**；S3 bucket 是 public-read
- 因此 **"按文件大小缩放 TTL" 无 TTL 可缩**——下载 URL 永久有效

**真实暴露面 & 重订方案**：

| 抽象需求 | 现状 | 真实风险 | 改做 |
|---|---|---|---|
| 防大文件下载断流 | 公开 URL 永不断 | 无 | 无需改 |
| 防 URL 盗刷 | 任何拿到 URL 的人可永久下载 | URL 一旦公开（如截图、爬虫）即失控 | 需要 CDN 层（Cloudflare）配速率限制 + 来源校验，**不在本仓库职责** |
| 防 `/link` 端点被批量爬取 | 无限流 | 攻击者循环调用 `/link` 拿光所有 URL 后绕过 backend 直接下载 | **本 PR 加 rate-limit** |

**实施**（router.go 单行改动）：

```go
patchRoutes.Get(
    "/resource/:resourceId/link",
    middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute),
    a.PatchHandler.GetResourceDownloadInfo,
)
```

- 限额 **30 req/min**（按 uid，匿名按 IP；已有 `middleware.RateLimit` 自动判别）
- 正常浏览：用户开 10 个资源页 ≈ 10-30 次 `/link`/分钟，不影响
- 爬虫：30 后返 429 中断；攻击者要拿全站 URL 显著延长（每分钟 30 个）
- 不需新 schema / 不动 handler / 不动前端

**改 1 文件 + 1 import 行**：
- `internal/app/router.go` 加 `import "time"` + `/resource/:resourceId/link` 路由插入 `middleware.RateLimit(a.RDB, "resource-link", 30, time.Minute)`

**验证**
- `go build / vet / test` 全绿
- 不影响其它路由
- 现有 `middleware.RateLimit` 实现已支持有用户/无用户两种 key 维度（`uid` vs `IP`），且 fail-open 设计（Redis 不可达 → `c.Next()`，不阻塞正常下载）

**关于完整 presigned URL + TTL 缩放的可行性评估（保留为未来工作项）**：

如果将来要把下载切换为 presigned-only：

1. S3 bucket 改 private（运维操作，破坏现有所有公开 URL 直链）
2. backend `GetResourceDownloadInfo` 改用 `s3.PresignGetObject(ctx, s3_key, ttl)` 生成并返回（需新增 storage 方法，约 10 行）
3. TTL 按 `patch_resource.Size`（字符串解析）映射：100MB/1h, 1GB/2h, 5GB/4h, >5GB/6h
4. `Size` 字段当前是展示字符串（"100 MB"），不是字节数——可能需要新增 `size_bytes BIGINT` 列或在 handler 处解析
5. Cloudflare CDN 也要适配（现有可能基于公开 URL 缓存）

属中大型 PR（架构改动 + 数据迁移），不在本期范围。本 PR 用最小改动覆盖了**最现实**的盗刷面（`/link` 爬取）。

**显式不做**：
- 完整 presigned + TTL 切换（架构性工作，保留为未来项）
- Turnstile 人机校验（除非真出现真实爬虫滥用信号，原计划 §4.6.3 已标 "默认不做"）
- size 字段重构为 BIGINT（独立架构问题，与本 PR 无关）

---

### 4.7 M7：待审/未落 S3 资源可见性核实（P3）

#### 4.7.1 问题

资源在上传完成 → S3 多部分合并 → 数据库标记可用之间存在窗口期；未落地资源不应对其他用户可见（404 / not-found），但**对上传者本人应可见**（便于调试）。

#### 4.7.2 实施

核实 `GetResources` / `GetResourceDetail` 中是否已按以下规则过滤：

```go
// 伪 SQL：
WHERE (patch_resource.status = 'ready' OR patch_resource.user_id = current_uid)
```

若现状已是「resource 一创建即可见」，加状态字段（如已有 `disable` 字段可复用）或专门 `status enum('uploading','ready','disabled')`。具体改动量取决于现状——**先核实再决定**。

---

### 4.8 M8：本地缓存与 NSFW 纵深防御（P3，条件）

#### 4.8.1 触发条件

仅当我们决定**在本仓库引入 Redis 热点页缓存**（如 `/home`、热门 galgame 列表）才一起做。当前未引入，可暂缓。

#### 4.8.2 若引入则必做

1. **缓存 key 维度**：必须包含 `content_limit + 鉴权上下文`：
   ```
   home:cl:sfw          (匿名/SFW)
   home:cl:all:role:1   (登录/允许 NSFW/普通用户)
   ```
2. **写路径主动 bust**：资源更新/galgame status 变更时按 pattern 删 key
3. **NSFW 未命中返 not-found 而非 403**：不泄露"该作品存在但你看不见"
4. 不要把含 NSFW 字段的对象用 SFW key 缓存

#### 4.8.3 不引入缓存时

跳过本项；当前 DB 性能足够。

---

### 4.9 M10：资源举报双向追责（待产品决策）

若上线"资源举报"功能（用户对资源标记色情/盗版/虚假等），同时实施：

1. **举报实体**：`patch_resource_report(id, resource_id, reporter_id, reason_enum, content, status, processor_id, created)`
2. **被举报追责**：举报判定为真 + 严重度高 → 上传者降配额（如 `daily_upload_size` 减半 1 周）/封号
3. **误报追责**：滚动 30 天窗口内，被驳回 N 次的举报者：
   - 3 次 → 警告 + 配额 −1G/月
   - 5 次 → 3 天禁举报
   - 8 次 → 14 天禁举报
4. **admin 豁免**：role≥3 不计追责
5. **预检**：被禁举报者再举报直接 403

不实现本身的举报功能 = 不实施本项。

---

## 5. 实施顺序（PR 切分）

### Phase 1（与 Wiki 同窗发版）

| PR | 范围 | 依赖 |
|---|---|---|
| ~~MOYU-PR0~~ | ~~M1 SQL 注入审计~~ | 已审计无风险（§4.1.4） |
| ~~MOYU-PR1~~ | ~~M2 列表分页 `id DESC` 兜底（全项目一次性）~~ | **已完成 2026-05-18**（§4.2.4，16 处） |
| ~~MOYU-PR2~~ | ~~W1 `released` → `release_date` 全链路（与 Wiki PR1 同期发版）~~ | **已完成 2026-05-18**（§3.1.5） |

### Phase 2（Wiki U2 发版后）

| PR | 范围 | 依赖 |
|---|---|---|
| ~~MOYU-PR3~~ | ~~W2 A+B+C：后端类型 + 展示链切换 + `banner_image_hash` 死字段全清 + composable/DiffView 类型~~ | **已完成 2026-05-18**（§3.2.6）|
| ~~MOYU-PR3b~~ | ~~W2 D：image-service hash 上传端点 + cover/screenshot 编辑器 UI + 详情页画廊~~ | **已完成 2026-05-18**（§3.2.7）|
| ~~MOYU-PR4~~ | ~~W3 taxonomy 12 条新路由 + composable 方法 + 管理 UI 历史/回滚弹窗~~ | **已完成 2026-05-18**（§3.3.4）|

### Phase 3（Moyu 内部，独立排期）

| PR | 范围 | 依赖 |
|---|---|---|
| ~~MOYU-PR5~~ | ~~M3 `patch_resource_file_history` 表 + 写入点 + admin 端点~~ | **已完成 2026-05-18**（§4.3.6）|
| ~~MOYU-PR6~~ | ~~M4+M9 `StringDiff` + `ArrayDiff` 组件，DiffView 长字符串/数组优化~~ | **已完成 2026-05-18**（§4.4.4）|
| ~~MOYU-PR7~~ | ~~M5 上传 session 单次使用不变量加固~~ | **已完成 2026-05-18**（§4.5.5）|
| ~~MOYU-PR8~~ | ~~M6 下载预签名 TTL 按大小缩放~~ | **已完成 2026-05-18**（§4.6.5；方案大改为 `/link` 端点限流，原 TTL 方案不适用现架构）|

### Phase 4（条件触发）

| PR | 触发 |
|---|---|
| **MOYU-PR9** | M7 资源可见性（先核实是否已实现，决定是否需要） |
| **MOYU-PR10** | M8 NSFW 纵深 + 缓存隔离（仅当引入缓存） |
| **MOYU-PR11** | M10 举报双向追责（仅当上线举报功能） |

**Phase 1/3 可并行**（Phase 1 强 Wiki 依赖，Phase 3 不依赖 Wiki）。Phase 2 串在对应 Wiki PR 之后。

---

## 6. 防回归测试清单

### Wiki-coupled

- W1：单测 `release_date` 解析与展示；E2E 详情页/列表/编辑页字段切换
- W2：单测 `sameCoverSet` / `sameScreenshotSet` 集合等值；E2E "只改名字不动 covers → 提交后无 covers 字段"
- W2：E2E "清空 covers 提交 `[]` → Wiki 端清空"
- W3：单测 12 条新路由生成正确 Wiki URL；E2E 一个 tag PUT → 历史出现 → 回滚 → 字段恢复 + 新增 reverted 一条
- W3：E2E `deleted` taxonomy revision "恢复"按钮复活实体不恢复引用

### Moyu-internal

- M1：单测 `SortField=";DROP TABLE"` 返回 400 或被兜底
- M2：单测 5 行同 created 时间戳分两页无重复
- M3：单测仅改 note 不写 history；换 s3_key 写 1 条 history；删除资源 history 级联删
- M4：单测 lcs-diff 纯函数；视觉验证长 intro 改动
- M5：单测并发提交同 sessionID 一成一败
- M6：单测 1GB 资源 link TTL ≈ 2h

---

## 7. 验收里程碑

| 里程碑 | 标志 |
|---|---|
| **M1 完成（安全闭环）** | 2026-05-18 审计：5/5 处 Order 拼接均有 DTO `oneof` 或 `switch case` 字面量保护，无注入风险（§4.1.4） |
| **Phase 1 完成（Wiki U1 对齐）** | 灰度 24h 无 `released` 相关 404/类型错误；翻页 bug 复现案例修复 |
| **Phase 2 完成（Wiki U2/U3 对齐）** | covers/screenshots 编辑回归 0；taxonomy 历史/回滚端到端通；归档报告显示无 cover/tag 误清空事件 |
| **Phase 3 完成（内部升级）** | `patch_resource_file_history` 在生产积累 30 天数据；编辑 diff 弹窗用户满意度反馈正向 |

---

## 8. 一句话总结

随 Wiki 发版必做三件：① `released` → `release_date` 下游迁移（BREAKING）；② covers/screenshots 类型 + UI 消费 + 编辑表单 presence 全量陷阱（与 tag_ids 同款）；③ taxonomy 4 实体 × 修订历史/回滚代理 + 管理 UI 历史入口。

与 Wiki 解耦的内部必做三件（M1 SQL 注入审计已通过，无风险）：① 列表分页 `id DESC` 兜底；② 资源文件历史表与写入点；③ DiffView 字符级 + 数组集合级 diff 升级。

显式不做：资源 AV 流水线、Resource/File 两层拆分、字段级位掩码 RBAC、本地缓存抽象——当前不做但**保留原则**（AV 接入时必须超时自动删；缓存引入时必须按 content_limit 隔离）。
