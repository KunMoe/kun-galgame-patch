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
| **W1** | Wiki U1 | `released string` → `release_date date? + release_date_tba bool` 全链路下游迁移 | **BREAKING**，与 Wiki 同期发版 | P0 |
| **W2** | Wiki U2 | 消费 `covers[] / screenshots[] / effective_banner_hash`；编辑/PR 表单 covers/screenshots 走 presence 全量替换（同 tag_ids 同款陷阱） | Additive（过渡期 `banner_image_hash` 仍存在；最终 drop） | P1 |
| **W3** | Wiki U3 | 代理新增的 taxonomy 修订端点（`/galgame/{tag\|official\|engine\|series}/:id/{references,revisions[/:rev],revert}`，嵌套风格已锁定）+ 管理 UI 加修订历史/回滚入口 | Additive | P1 |

### 1.2 Moyu-internal（与 Wiki 解耦）

| # | 内容 | 状态结论 | 优先级 |
|---|---|---|---|
| **M1** | 列表排序 `SortField/SortOrder` SQL 注入面**安全核实** | 🟢 **已审计通过**（2026-05-18，§4.1.5） | ~~P0~~ → P3（防御性加强可选） |
| **M2** | 列表分页恒加 `id DESC` 兜底，修翻页漂移 | ✅ 必做 | P1 |
| **M3** | `patch_resource` 追加式文件历史表 `patch_resource_file_history` | ✅ 必做 | P1 |
| **M4** | `GalgameEditDiffView` 升级为字符级 LCS diff（含长 intro_* 体验） | ✅ 必做 | P2 |
| **M5** | 上传 session 属主/单次/COMPLETED 不变量核实加固 | ✅ 必做 | P2 |
| **M6** | 下载链接按文件大小缩放预签名 TTL（人机校验可选） | ✅ 必做 | P2 |
| **M7** | 待审/未落 S3 资源可见性核实（仅上传者可见） | ⚠️ 先核实再决定 | P3 |
| **M8** | 本地热点页缓存（若引入）必须按 `content_limit + 鉴权上下文` 维度隔离；NSFW 未命中按 not-found 而非 403 | ⚠️ 仅在引入缓存时一起做 | P3 |
| **M9** | DiffView 数组字段集合级折叠（aliases/tag_ids 显示"+X / −Y"） | ⚠️ 小增强，与 M4 同期可并入 | P3 |
| **M10** | 资源举报双向追责（若上线举报功能） | ⚠️ 与举报功能产品决策同步 | 待产品 |

### 1.3 显式不做（接受现状或暂缓）

- ❌ **资源 AV 流水线 / Malware 扫描**：完整建模成本高、暂无明确威胁面；保留**"超时未审 → 自动删"原则**为将来接入扫描器时的硬性要求。
- ❌ **`patch_resource` 拆 Resource/File 两层**：当前单层（一个资源一个文件/链接）满足需求；拆两层属 schema 重构无对应业务收益。
- ❌ **后端字段级位掩码 RBAC**：Wiki 已有 PR + 审核工作流，本仓库 §15 明确"不在本地重做鉴权"；位掩码属过度设计。
- ❌ **本地缓存预先抽象 / 搜索引擎抽象**：当前无热点页缓存、搜索由 Wiki 持有，过早抽象。

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

### 3.3 W3：taxonomy 修订端点代理 + 管理 UI 加历史/回滚

#### 3.3.1 新增 Wiki 端点（4 实体 × 3+1 路径，**已锁定 `/galgame/<entity>/...` 嵌套风格**）

按 Wiki 99-final §8 实施。**路径前缀已确定**：所有 taxonomy 端点（含原有 CRUD 与本次新增的 references/revisions/revert）统一改为 `/galgame/<entity>/...` 嵌套形式：

```
GET    /galgame/<entity>/:id/references          # preflight：返回引用数 + sample
GET    /galgame/<entity>/:id/revisions           # 修订列表（分页）
GET    /galgame/<entity>/:id/revisions/:rev      # 单条修订快照
POST   /galgame/<entity>/:id/revert              # body: {revision: N}
```

`<entity>` ∈ `{tag, official, engine, series}`。

> ⚠️ **同步迁移现有 taxonomy 代理路由**：本仓库当前已注册的 `GET /tag /official /engine /series`、`POST /tag`、`PUT /tag`、`DELETE /tag/:id`、`GET /tag/:name` 等（router.go 内 §「Galgame taxonomy proxy」块）需**一次性全部迁到 `/galgame/<entity>/...` 前缀**，与 Wiki 对齐。这是 BREAKING for any frontend code that still calls old paths——同 PR 内 `useGalgameEdit.ts` 所有 taxonomy 方法的 URL 一并改掉。`WikiEditProxy` 路径镜像让后端改动近似只是 route 字符串调整。

#### 3.3.2 后端：路由迁移（全部 `/galgame/<entity>/...`）

`apps/api/internal/app/router.go` 的「Galgame taxonomy proxy」整块**重写**为 `/galgame/<entity>/...` 前缀。**全量重写示意**：

```go
for _, e := range []string{"tag", "official", "engine", "series"} {
    base := "/galgame/" + e

    // ── 现有 CRUD 迁前缀 ──
    api.Get(base, a.PatchHandler.WikiEditProxy)
    api.Post(base, auth, a.PatchHandler.WikiEditProxy)
    api.Put(base, auth, a.PatchHandler.WikiEditProxy)
    api.Delete(base+"/:id", auth, a.PatchHandler.WikiEditProxy)

    // entity-specific 子路径（在 :name 之前注册）
    if e == "tag" {
        api.Get(base+"/search", a.PatchHandler.WikiEditProxy)
        api.Get(base+"/multi", a.PatchHandler.WikiEditProxy)
    }
    if e == "official" {
        api.Get(base+"/search", a.PatchHandler.WikiEditProxy)
    }
    if e == "series" {
        api.Get(base+"/search", a.PatchHandler.WikiEditProxy)
        api.Post(base+"/modal", auth, a.PatchHandler.WikiEditProxy)
    }

    // ── 本期新增 ──
    api.Get(base+"/:id/references", a.PatchHandler.WikiEditProxy)
    api.Get(base+"/:id/revisions", a.PatchHandler.WikiEditProxy)
    api.Get(base+"/:id/revisions/:rev", a.PatchHandler.WikiEditProxy)
    api.Post(base+"/:id/revert", auth, a.PatchHandler.WikiEditProxy)

    // ── :name 兜底（必须最后注册，让 :id 子路径优先匹配）──
    if e != "series" { // series 用 :id 没有 :name
        api.Get(base+"/:name", a.PatchHandler.WikiEditProxy)
    } else {
        api.Get(base+"/:id", a.PatchHandler.WikiEditProxy)
    }
}
```

**顺序铁律**：`base+"/:id/..."` 子路径必须在 `base+"/:name"`（或 `:id` 兜底 GET）**之前**注册，否则 Fiber 会把 `1/revisions` 当 `:name=1/revisions` 匹配掉。

`DELETE /galgame/<entity>/:id` 的两段式 `?force=true` 已支持（generic proxy 自动转发 query）。

#### 3.3.2bis 前端同步迁 URL

`apps/web/app/composables/useGalgameEdit.ts` 内**所有** taxonomy 方法的 URL 前缀同改：`/tag/...` → `/galgame/tag/...`（official/engine/series 同）。受影响方法（已知）：`tagSearch / createTag / updateTag / deleteTag / officialSearch / createOfficial / updateOfficial / deleteOfficial / engineList / createEngine / updateEngine / deleteEngine / seriesList / seriesSearch / seriesDetail / createSeries / seriesModal / updateSeries / deleteSeries`。`pages/galgame/taxonomy.vue` 与 `TaxonomyPicker.vue` 无需改（它们经 composable 间接调用）。

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

### 4.1 M1：列表排序 SQL 注入面**安全核实**（🟢 已审计通过 · 2026-05-18）

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
| `internal/common/handler.go:152` | `fmt.Sprintf("%s %s", req.SortField, req.SortOrder)` | `galgameListRequest.SortField validate:"required,oneof=resource_update_time created view download"` + `SortOrder oneof=asc desc`（行 124-125） | ✅ |
| `internal/common/handler.go:187` | 同上 | `commentListRequest`：`SortField oneof=created like_count`（行 168-169） | ✅ |
| `internal/common/handler.go:270` | `fmt.Sprintf("patch_resource.%s %s", sortField, req.SortOrder)` | `resourceListRequest.SortField oneof=update_time created download like_count`（行 244-245）；本地变量 `sortField` 进一步带 fallback `"like_count"` | ✅ |
| `internal/common/handler.go:474` | `orderBy`（本地变量）| `switch sortBy { case "patch"/... }` 全部映射为**字面量字符串**，默认 `"u.moemoepoint DESC"` | ✅ |
| `internal/common/handler.go:529` | `fmt.Sprintf("%s DESC", column)` | `column` 来自 `switch sortBy`，全部映射为**字面量字符串**，默认 `"view"` | ✅ |

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
| ~~MOYU-PR0~~ | ~~M1 SQL 注入审计~~ | 🟢 已审计无风险（§4.1.4） |
| **MOYU-PR1** | M2 列表分页 `id DESC` 兜底（全项目一次性） | 无 |
| **MOYU-PR2** | W1 `released` → `release_date` 全链路（与 Wiki PR1 同期发版） | Wiki PR1 |

### Phase 2（Wiki U2 发版后）

| PR | 范围 | 依赖 |
|---|---|---|
| **MOYU-PR3** | W2 covers/screenshots 类型与展示；编辑/PR 表单 prefill + presence | Wiki PR2 |
| **MOYU-PR4** | W3 taxonomy 12 条新路由 + composable 方法 + 管理 UI 历史/回滚弹窗 | Wiki PR4 |

### Phase 3（Moyu 内部，独立排期）

| PR | 范围 | 依赖 |
|---|---|---|
| **MOYU-PR5** | M3 `patch_resource_file_history` 表 + 写入点 + admin 端点 | 无 |
| **MOYU-PR6** | M4+M9 `StringDiff` + `ArrayDiff` 组件，DiffView 长字符串/数组优化 | 无 |
| **MOYU-PR7** | M5 上传 session 单次使用不变量加固 | 无 |
| **MOYU-PR8** | M6 下载预签名 TTL 按大小缩放 | 无 |

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
| **M1 完成（安全闭环）** | ✅ 2026-05-18 审计：5/5 处 Order 拼接均有 DTO `oneof` 或 `switch case` 字面量保护，无注入风险（§4.1.4） |
| **Phase 1 完成（Wiki U1 对齐）** | 灰度 24h 无 `released` 相关 404/类型错误；翻页 bug 复现案例修复 |
| **Phase 2 完成（Wiki U2/U3 对齐）** | covers/screenshots 编辑回归 0；taxonomy 历史/回滚端到端通；归档报告显示无 cover/tag 误清空事件 |
| **Phase 3 完成（内部升级）** | `patch_resource_file_history` 在生产积累 30 天数据；编辑 diff 弹窗用户满意度反馈正向 |

---

## 8. 一句话总结

随 Wiki 发版必做三件：① `released` → `release_date` 下游迁移（BREAKING）；② covers/screenshots 类型 + UI 消费 + 编辑表单 presence 全量陷阱（与 tag_ids 同款）；③ taxonomy 4 实体 × 修订历史/回滚代理 + 管理 UI 历史入口。

与 Wiki 解耦的内部必做三件（M1 SQL 注入审计已 ✅ 通过，无风险）：① 列表分页 `id DESC` 兜底；② 资源文件历史表与写入点；③ DiffView 字符级 + 数组集合级 diff 升级。

显式不做：资源 AV 流水线、Resource/File 两层拆分、字段级位掩码 RBAC、本地缓存抽象——当前不做但**保留原则**（AV 接入时必须超时自动删；缓存引入时必须按 content_limit 隔离）。
