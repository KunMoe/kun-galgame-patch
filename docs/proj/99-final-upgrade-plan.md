# Galgame Wiki 升级方案最终版（v1，已拍板）

> 状态：**已逐条拍板，本文档为最终设计依据，自包含**
> 日期：2026-05-18
> 实施前阅读次序：先读 `01-revision-system-design.md §1.5 不变量` → 再读本文 §2 设计哲学 → 再读 §7 完整 Revision 逻辑梳理（实施时的审计核心）。

---

## 1. 升级范围速览（三条 P0）

| # | 内容 | 状态 |
|---|---|---|
| **U1** | `released string` → `release_date date?` + `release_date_tba bool`（**一刀切**，无双发） | ✅ 实施 |
| **U2** | Galgame 图片模型升级：新增 `galgame_cover` / `galgame_screenshot` 关联表，banner 一般化（**无票选、无 override**），refping 扩展为 ping「当前 + 所有 revision/PR snapshot 中的 hash」 | ✅ 实施 |
| **U3** | Taxonomy 修订：新增**单张多态全快照表** `taxonomy_revision`，覆盖 tag/official/engine/series | ✅ 实施 |

**显式不做的范围**：见 §9。所有"未做"项均**留 schema 扩展位**，将来加都不需要破坏性迁移。

---

## 2. 设计哲学（脊柱声明）

本次升级**严格遵守** `01-revision-system-design.md §1.5` 的 6 条不变量，并对 taxonomy 新增**等价表述**：

1. **快照 = 全量可编辑状态，关联是一等公民**：标量与关联表平级，都在 Snapshot 里、都进 revision、都参与 diff/revert。
2. **唯一写入路径 = `ApplySnapshot`**：每个实体类型一个 `Apply*Snapshot(tx, id, snap)`，全仓 create/update/delete/revert 全部经它落库。
3. **可编辑字段集 ≡ 编辑 DTO 字段集**：每个 Snapshot 字段必能被 DTO 改到。防回归测试护栏。
4. **直接编辑 = 对自己做一次 snapshot overlay**：`读 cur → overlay(req) → next → ChangedKeys==空则 no-op → Apply → 重新 Take → revision`。
5. **revision snapshot 必须在写入之后重新 `Take`**：`revision.snapshot == DB == 编辑意图`三者恒等。
6. **集合语义、顺序无关**：所有 ID/字符串数组在 Snapshot 内 canonical-sort，比较一律用规范化后的快照。

**新增声明**：

- **修订一律采用全快照（不采用 `{before,after}` delta）**：revert/diff 永远正确、无需重放重建、无"后续编辑覆盖了同字段需逆序撤销"的重叠问题。存储增长（每版几 KB）对编辑低频实体完全可接受。
- **初版不求全，扩展位优先**：本次只做必须的，并为以后能力（票选 / 字段级 RBAC / sync 预览 / changed_fields 索引）**留 schema 扩展位**，不实现这些能力本身。
- **修订表合并 = 默认偏好**：taxonomy 4 个实体共用 1 张多态修订表（`taxonomy_revision`，entity 列区分），Go 层各自有强类型 Snapshot 结构。同形态共表，加第 5 种 taxonomy 实体只加 1 个 Go struct + 1 个 CHECK 枚举值，不加表。**已知代价见 §9**：放弃 DB 层 `target_id` 引用完整性、`(entity,...)` leading column 选择性弱、未来某实体长独有字段时退路收窄。这些代价被本期规模下"少 3 张表 + 加第 5 种实体成本 10×降低 + 跨实体审计查询简单"的收益盖过，但不假装它们不存在。
- **并发安全（脊柱不变量补充）**：写入 `taxonomy_revision` / `galgame_revision` 前必须 `SELECT 1 FROM <主表> WHERE id=? FOR UPDATE` 锁住对应实体主行（`galgame_tag/official/engine/series/galgame`）来串行化同一实体的并发编辑。**不能锁 `*_revision` 表**——这次是 INSERT 新行，行锁锁不到不存在的行。`NextRevision(entity, target_id)` 计算依赖此锁保证唯一序号。

---

## 3. 决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 图片：banner 方案 | **候选集 + screenshot 双关联表**，但**不做票选** | 一次定对 schema 避免上线后拆 snapshot；票选纯增量，将来加成本 ≈ 0 |
| 图片：`banner_override_hash` | **不加** | 无票选则无 override 必要；管理员要钉哪张直接改 `sort_order=0`；将来真做票选时加列即可 |
| 图片：`Source` / `SourceKey` 列 | **保留** | 零运行成本的扩展位；导入路径可顺手填；将来做 fill-only 同步无需迁移 schema |
| Image GC × revision | **refping 扩展为收集"当前 + 所有 revision/PR 快照中的 hash"**（§7.6） | 不动 refping 会让 revision-only 图被 image_service TTL 物删（>365d 未 ping → 软删 + 30d 物删），revert 出死图。扩展收集集合是几十行代码、永不丢图 |
| 是否实现孤儿扫描脚本 | **不实现** | 存储成本极低；refping 已保证活引用全集存活；以后真需要清理才单独写脚本 |
| Taxonomy 修订形态 | **单张多态全快照表 `taxonomy_revision`** | 4 个小实体（字段 2-7 个）+ admin-only 编辑，单表形态最简、扩展性最好；与 `galgame_revision` 的核心语义（全快照 + 单一 Apply 写路径 + revert）完全一致，仅物理上合表 |
| `released` 迁移节奏 | **一刀切** | 测试环境，三仓未上线，无需双发兼容 |
| 字段级 RBAC | **不做但留扩展位** | `taxonomy_revision` 预置 `UserRole` 列 + `ChangedFields text[]` 列，将来加 RBAC 时只加策略表 + service 检查，schema 不动 |

---

## 4. 升级 U1：`released` → `release_date` + `release_date_tba`

### 4.1 Schema 变更

```go
// model/galgame.go
type Galgame struct {
    // ... 其他字段
    ReleaseDate    *time.Time `gorm:"column:release_date;type:date;index" json:"release_date"` // null = unknown
    ReleaseDateTBA bool       `gorm:"column:release_date_tba;not null;default:false" json:"release_date_tba"`
    // Released string  ← DROP（迁移完成后）
}
```

### 4.2 Snapshot 变更

```go
type Snapshot struct {
    // ...
    ReleaseDate    *string `json:"release_date"` // "YYYY-MM-DD" 或 nil；用字符串避免时区/精度抖动破坏 ChangedKeys
    ReleaseDateTBA bool    `json:"release_date_tba"`
    // Released string  ← 删除
}
```

- `TakeSnapshot`：把 `*time.Time` 序列化为 `"YYYY-MM-DD"` 字符串（无时区）。`nil` → `nil` 指针。
- `ChangedKeys`：`released` key 拆为 `release_date` + `release_date_tba` 两个独立 key。
- `ApplyChanges`：同步两个字段。

### 4.3 DTO 变更（破坏性）

```go
// CreateGalgameRequest / UpdateGalgameRequest / SubmitGalgameRequest
ReleaseDate    *string `json:"release_date" validate:"omitempty,datetime=2006-01-02"`
ReleaseDateTBA *bool   `json:"release_date_tba"`
// Released *string ← 删除
```

Update 路径的 `ReleaseDate *string` 语义：
- `nil`（未提交）= 保持原样
- `&""`（空字符串）= 清空为 unknown（DB 写 null）
- `&"2024-06-15"` = 设置具体日期
- 与 `ReleaseDateTBA *bool` 独立：TBA = true 仍可有日期（"预计 2024 年某月"）

### 4.4 迁移脚本（一次性）

`cmd/migrate-galgame-released-to-date/main.go`：

```
对每行 galgame:
  raw := galgame.released
  switch:
    case raw == "" || raw == "unknown":   release_date = NULL, release_date_tba = false
    case raw == "tba":                    release_date = NULL, release_date_tba = true
    case 匹配 /^\d{4}-\d{2}-\d{2}$/:      release_date = parse(raw), release_date_tba = false
    case 匹配 /^\d{4}-\d{2}$/:            release_date = parse(raw + "-01"), release_date_tba = false
    case 匹配 /^\d{4}$/:                  release_date = parse(raw + "-01-01"), release_date_tba = false
    default:                              release_date = NULL, release_date_tba = false, 日志记录原值
```

迁移分两步：先 add 新列 + 跑 backfill；通过后下个版本 drop `released` 旧列。同时**修补所有历史 revision snapshot 的 jsonb**：`released` 字段就地拆为 `release_date`/`release_date_tba`，否则 revert 老版本会丢字段。

### 4.5 下游影响

`released` 字段从所有 galgame 响应中**消失**；新字段 `release_date` / `release_date_tba` 同步出现。`docs/integration/galgame_wiki/00-handbook-for-downstream.md §15` 需在 BREAKING 段加一条；kungal/moyu 同期发版。

---

## 5. 升级 U2：Galgame 图片模型

### 5.1 新增表（按 `image_hash` 引用 image_service，不外键跨服务）

```go
// 封面候选集（banner 的一般化）
type GalgameCover struct {
    GalgameID int       `gorm:"primaryKey;column:galgame_id"`
    ImageHash string    `gorm:"primaryKey;type:char(64);column:image_hash"`
    SortOrder int       `gorm:"not null;default:0;index"` // 越小越优先；sort_order=0 即 effective banner（唯一）
    Sexual    int16     `gorm:"not null;default:0"`        // 本作品语境下露骨度；0 = 未评定（见 §9），1+ = 已评定
    Violence  int16     `gorm:"not null;default:0"`        // 同上
    Source    string    `gorm:"size:16;default:''"`        // 'vndb'/'user'/'bangumi'，扩展位
    SourceKey string    `gorm:"size:128;default:''"`       // 同源 fill-only 幂等键，扩展位
    Created   time.Time `gorm:"autoCreateTime"`
    // 未来扩展位（不加列）：vote_score 走独立 galgame_cover_vote 表
}
func (GalgameCover) TableName() string { return "galgame_cover" }

// 迁移时必须额外创建：
//   CREATE UNIQUE INDEX idx_galgame_cover_pinned ON galgame_cover(galgame_id) WHERE sort_order = 0;
// 强制每作品最多 1 张 sort_order=0（= effective banner）。无此索引时 admin "钉新封面" 若忘记把旧 sort_order=0 降级，会出现两张并列、ORDER BY created ASC 选老的 → "钉新封面没生效"故障。
// "钉新封面"业务流程因此固定为：事务内先 UPDATE old SET sort_order=1 → UPDATE new SET sort_order=0。

// 画廊 / CG / 截图
type GalgameScreenshot struct {
    GalgameID int       `gorm:"primaryKey;column:galgame_id"`
    ImageHash string    `gorm:"primaryKey;type:char(64);column:image_hash"`
    SortOrder int       `gorm:"not null;default:0"`
    Caption   string    `gorm:"type:text;default:''"`
    Sexual    int16     `gorm:"not null;default:0"`
    Violence  int16     `gorm:"not null;default:0"`
    Source    string    `gorm:"size:16;default:''"`
    SourceKey string    `gorm:"size:128;default:''"`
    Created   time.Time `gorm:"autoCreateTime"`
}
func (GalgameScreenshot) TableName() string { return "galgame_screenshot" }
```

主键 `(galgame_id, image_hash)`：同图在同作同用途只挂一次；同图可同时是 A 作 cover、B 作 screenshot——靠不同关联表，**`images` 上不加 `kind` 列**。**不设跨服务外键**（image_service 是独立服务）；完整性靠：写入前 wiki 调 `imageclient` 确认 hash 存在 + refping 维持存活。

### 5.2 Snapshot 扩展

```go
type Snapshot struct {
    // ...
    // BannerImageHash 字段保留一段过渡期；迁移完成后删除（迁为 covers[sort_order=0]）
    Covers      []SnapshotCover      `json:"covers"`
    Screenshots []SnapshotScreenshot `json:"screenshots"`
}
type SnapshotCover struct {
    ImageHash string `json:"image_hash"`
    SortOrder int    `json:"sort_order"`
    Sexual    int16  `json:"sexual"`
    Violence  int16  `json:"violence"`
    Source    string `json:"source"`
    SourceKey string `json:"source_key"`
}
type SnapshotScreenshot struct {
    ImageHash string `json:"image_hash"`
    SortOrder int    `json:"sort_order"`
    Caption   string `json:"caption"`
    Sexual    int16  `json:"sexual"`
    Violence  int16  `json:"violence"`
    Source    string `json:"source"`
    SourceKey string `json:"source_key"`
}
```

- `TakeSnapshot`：按 `ImageHash` canonical-sort，行内 `SortOrder/Caption/Sexual/Violence/Source/SourceKey` 是数据不影响排序。
- `ChangedKeys`：`covers` / `screenshots` 两 key；行集合差 + 同 hash 的行内字段比较。
- DTO：`Covers *[]CoverInput` / `Screenshots *[]ScreenshotInput`，presence 语义（`nil`=保持，非 nil 含空=权威全量替换）——**前端编辑表单必须回传该 galgame 的全量封面/画廊**，与 `tag_ids` 同款结构性陷阱（§1.5 #5）。

### 5.3 ApplySnapshot 扩展（从 5 张关联表 → 7 张）

`repository.ApplySnapshot(tx, gid, uid, snap)` 在现有 5 张关联表清空重建之后追加：

```
6. Rebuild covers:      DELETE WHERE galgame_id=? → 逐行 Create from snap.Covers
7. Rebuild screenshots: DELETE WHERE galgame_id=? → 逐行 Create from snap.Screenshots
```

**保持清空重建**，不做 diff 优化（§1.5 #2 不变量；编辑低频；过早优化引入第二套关联写语义）。

### 5.4 Effective Banner 语义（v1）

```
effective_banner_hash = SELECT image_hash FROM galgame_cover
                        WHERE galgame_id = ? AND sort_order = 0
                        LIMIT 1
```

由 §5.1 的 `partial unique index` 保证至多一张 sort_order=0 → 无需 tie-break。无 cover 时返回空；前端用 `imageclient.MainURL(hash)` 构造 URL。**管理员强制钉某张 = 事务内先把当前 sort_order=0 那张降级 → 再把新选的设为 0**（partial unique index 会拒绝并列）。将来加票选时再扩。

### 5.5 banner_image_hash 的去留

- 第一阶段（迁移期）：`galgame.banner_image_hash` 列保留 + 现有 banner 数据迁为 `galgame_cover(sort_order=0)` 一行；新写路径同时更新两者，读路径以 cover 表为准。
- 第二阶段（验证稳定后）：drop `galgame.banner_image_hash` 列，`Snapshot.BannerImageHash` 字段移除，**修补历史 revision snapshot jsonb**（把字段拍平到 covers[0]）。
- 旧的 `Banner string` URL 字段（已在迁移中）按现有计划继续 drop。

### 5.6 NSFW 三层归属

| 层 | 字段 | 用途 |
|---|---|---|
| 文件安全审核（"图本身是否允许上架"）| `image_service.images.ReviewStatus/Labels` | 平台准入 |
| **本作品语境露骨度** | `galgame_cover/screenshot.Sexual` / `Violence` | 按图门控、列表筛选 |
| 作品级粗分级 | `galgame.content_limit` / `age_limit` | 粗粒度页面 gate |

展示策略由应用层用这三者 + 用户年龄设置一起算，**不在任何单表存"该不该显示"**。

---

## 6. 升级 U3：Taxonomy 多态全快照修订

### 6.1 单张多态全快照表

```go
type TaxonomyRevision struct {
    ID                 int             `gorm:"primaryKey;autoIncrement"`
    Entity             string          `gorm:"size:16;not null;uniqueIndex:idx_taxrev,priority:1;check:entity IN ('tag','official','engine','series')"`
    TargetID           int             `gorm:"not null;uniqueIndex:idx_taxrev,priority:2"`
    Revision           int             `gorm:"not null;uniqueIndex:idx_taxrev,priority:3"` // per (entity, target_id) 序号
    Action             string          `gorm:"size:16;not null;check:action IN ('created','updated','deleted','reverted')"`
    UserID             int             `gorm:"not null;index"`
    UserRole           int             `gorm:"not null"`                                   // 为将来 RBAC 留位
    Snapshot           datatypes.JSON  `gorm:"type:jsonb;not null"`                        // 各实体的 Snapshot Go 结构序列化
    ChangedFields      pq.StringArray  `gorm:"type:text[]"`                                // 见 §6.5 的语义约定（不再用 '*' 哨兵）
    RefCount           int             `gorm:"not null;default:0"`                         // 仅 deleted 用：删除时该实体被多少 galgame 引用
    AffectedGalgameIDs pq.Int32Array   `gorm:"column:affected_galgame_ids;type:int[]"`     // 仅 deleted 用：被解除引用的 galgame id 列表（free data，见 §7.2.3 / §7.2.4）
    Note               string          `gorm:"type:text;default:''"`
    Created            time.Time       `gorm:"autoCreateTime"`
}
func (TaxonomyRevision) TableName() string { return "taxonomy_revision" }
```

**设计要点**：
- **`(entity, target_id, revision)` 唯一索引**：每个实体的每次编辑独立序号；查询历史按 `(entity, target_id)` 走索引高效。
- **加第 5 种 taxonomy 实体**：只需在 `Entity` 的 CHECK 加一个值 + 新增对应 Go Snapshot 结构 + 实现对应 `Apply*Snapshot`，**不加表**。
- **强类型在 Go 层**：`Snapshot jsonb` 列是 union 类型，但 Go 代码读写时根据 `Entity` 值分派到对应强类型 Snapshot 结构（`TagSnapshot` / `OfficialSnapshot` / `EngineSnapshot` / `SeriesSnapshot`），编译期保证字段正确。
- **`UserRole` / `ChangedFields` 字段当前不被业务逻辑使用**，但落库——为将来字段级 RBAC 留位（届时仅加策略表 + service 层检查，schema 不变）。

### 6.2 各实体的 Snapshot 形态（Go 强类型）

每个实体一个 Go 结构体；与表的关联差异已被 Snapshot 形态抹平（在 Snapshot 层统一为字符串数组形态，物理实现差异封装在各自 Apply 函数里）：

```go
type TagSnapshot struct {
    Name        string   `json:"name"`
    Category    string   `json:"category"`     // 'content'/'sexual'/'technical'
    Description string   `json:"description"`
    Aliases     []string `json:"aliases"`      // canonical-sorted，来自 galgame_tag_alias 表
}

type OfficialSnapshot struct {
    Name        string   `json:"name"`
    Original    string   `json:"original"`
    Link        string   `json:"link"`
    Category    string   `json:"category"`     // 'company'/'individual'/'amateur'
    Lang        string   `json:"lang"`
    Description string   `json:"description"`
    Aliases     []string `json:"aliases"`      // canonical-sorted，来自 galgame_official_alias 表
}

type EngineSnapshot struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Aliases     []string `json:"aliases"`      // canonical-sorted；engine 的 alias 物理上是 jsonb 内联列，Snapshot 层仍序列化为字符串数组（与 tag/official 对齐）
}

type SeriesSnapshot struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    // 注意：NOT 含 GalgameIDs！"哪些 galgame 属于该 series" 是 galgame.series_id 的事，
    // 已在 galgame Snapshot 里。series 自身只有 Name/Description。详见 §7.5。
}
```

### 6.3 写入路径：每实体的 `Apply*Snapshot`

每个实体一个轻量函数（约 30 行），结构同构。所有 create/update/delete/revert 路径都经它落库：

```go
// 示意，仅 tag；official/engine/series 同形

func (r *TagRepository) ApplySnapshot(tx *gorm.DB, tagID int, snap *model.TagSnapshot) error {
    // 1. 标量列 Updates
    if err := tx.Model(&model.GalgameTag{}).Where("id = ?", tagID).Updates(map[string]any{
        "name":        snap.Name,
        "category":    snap.Category,
        "description": snap.Description,
    }).Error; err != nil { return err }

    // 2. 关联表（aliases）清空重建
    if err := tx.Where("galgame_tag_id = ?", tagID).Delete(&model.GalgameTagAlias{}).Error; err != nil { return err }
    for _, name := range snap.Aliases {
        if err := tx.Create(&model.GalgameTagAlias{GalgameTagID: tagID, Name: name}).Error; err != nil { return err }
    }
    return nil
}
```

各实体差异：

| 实体 | 标量列 Updates | 关联重建 |
|---|---|---|
| tag | name, category, description | 清空 `galgame_tag_alias` → 逐行 Create |
| official | name, original, link, category, lang, description | 清空 `galgame_official_alias` → 逐行 Create |
| engine | name, description, **alias (jsonb 内联列直接 Updates)** | 无独立关联表，第 2 步跳过 |
| series | name, description | 无 alias，第 2 步跳过 |

### 6.4 Action 集合（4 种）

| Action | 触发场景 | RefCount | Snapshot 内容 |
|---|---|---|---|
| `created` | 实体首次创建 | 0 | 创建后的完整状态 |
| `updated` | 任一编辑（标量 / aliases） | 0 | 编辑后的完整状态 |
| `deleted` | 实体被删除 | 删除时被多少 galgame 引用（force-purge 计数） | **删除前**的最后状态（用于"撤销删除"时重放） |
| `reverted` | 回滚到某历史版本 | 0 | 与目标 revision 完全相同的 snapshot |

**说明 `deleted` 的 Snapshot 含义**：删除时 RefCount 计算被多少 galgame 引用（继承现有 force-purge 两步流程），Snapshot 记录删除前最后状态——这样将来"撤销删除"= 取该 deleted revision 的 snapshot → INSERT 新行 → 落 created revision，对称漂亮。

### 6.5 `ChangedFields` 语义约定（避免哨兵字符串）

为将来字段级 RBAC 能写 `WHERE 'name' = ANY(changed_fields)` 而不踩坑，本次明确：

| Action | `ChangedFields` 取值 | 含义 |
|---|---|---|
| `created` | **显式列出全部 Snapshot 字段名**（如 tag: `['name','category','description','aliases']`）| 创建 = 所有字段从无到有；RBAC 检查 "能创建此实体" 应是 action-level 而非 field-level |
| `updated` | 实际变化的字段名列表（由 `Changed*Keys` 函数计算）| RBAC 检查每个字段是否被允许编辑 |
| `deleted` | **空数组 `[]`** | 删除 = 整行消失，不是某些字段被改；RBAC 检查 "能删除此实体" 应是 action-level |
| `reverted` | 目标快照与当前状态差异的字段名列表 | 同 updated |

**不再使用 `['*']` 哨兵字符串**——它无法被 RBAC 的 `field-name = ANY(changed_fields)` 查询匹配，是隐藏的语义陷阱。`created`/`deleted` 的语义由 action 列承载（action-level RBAC），`ChangedFields` 列只用于 field-level RBAC 的 `updated`/`reverted` 路径。

同款约定也适用于将来给 `galgame_revision` 加 `changed_fields` 列时。

---

## 7. ★完整 Revision 逻辑梳理（实施时审计核心）

本节按"实体类型 × 操作类型"枚举所有 revision 触发路径与 snapshot 含义。所有路径**严格遵循 §2 脊柱**——五步骤：`读 cur → overlay(req) → next → ChangedKeys 空则 no-op → Apply → 重新 Take → 落 revision`。

### 7.1 Galgame Revision 完整流程（升级后）

> 现状参考 `01-revision-system-design.md §6`。本节仅列出**升级后追加的字段进入哪些路径**。

新增 Snapshot 字段：`ReleaseDate`、`ReleaseDateTBA`、`Covers[]`、`Screenshots[]`。涉及它们的所有路径：

| 路径 | Action | 流程 |
|---|---|---|
| Create（admin POST /galgame）| `created` | 插入裸 galgame 行（仅 system 字段）拿 ID → 构造 `Snapshot{ReleaseDate, ReleaseDateTBA, Covers, Screenshots, ...}` → `ApplySnapshot(snap)` → 重新 `TakeSnapshot` → 落 `galgame_revision(rev=1, action=created)` |
| Submit（用户 POST /galgame/submit）| `claimed` | 同上，状态为 pending；snapshot 已含全部 4 个新字段 |
| Direct Edit（owner/admin PUT /galgame）| `updated` | `cur = TakeSnapshot(load)` → `overlay(req)` 应用 presence 语义到 4 个新字段 → `next` → `ChangedKeys(cur,next)` 空则 204 no-op → `ApplySnapshot(next)` → 重新 Take → revision |
| Patch Draft（用户 PATCH 自己 declined 草稿）| `edited_pending` | 同 Direct Edit，状态回 pending |
| PR Submit（其他用户 POST /galgame/:gid/pr）| 无 revision，写 `galgame_pr.snapshot` | snapshot 含全部新字段；下游必须全量回传 covers/screenshots（与 tag_ids 同款陷阱）|
| PR Merge（admin POST /pr/:id/merge）| `merged` | 按 §6.5 auto-rebase 流程：取 PR 的 snapshot 与 base/current 三方合并 → 应用 → revision |
| Revert（admin POST /galgame/:gid/revert）| `reverted` | 加载 revision N → `ApplySnapshot(rev_N.snapshot)` → 重新 Take → revision；`reverted_to=N` |
| 状态变更（admin approve/decline/ban/unban）| `approved`/`declined`/`banned`/`unbanned`/`status_changed` | 不动可编辑字段；snapshot 来自重新 Take（捕捉 status 之外的当前状态）|

**ChangedKeys 新增可能输出**：`release_date`、`release_date_tba`、`covers`、`screenshots`。

**集合语义说明**：`covers` 的 ChangedKeys 比较 = `image_hash` 集合差（added/removed）+ 同 hash 行内 6 个字段（sort_order/caption/sexual/violence/source/source_key）任一变更视作 key 命中。注意 caption 仅 screenshots 有；covers 没 caption。

### 7.2 Tag Revision 完整流程（全新）

#### 7.2.1 创建 Tag（admin POST /galgame/tag）

```
1. 校验：name 全局唯一（DB unique 约束 + 应用层预检）；category 在枚举内
2. 事务开始
3. INSERT INTO galgame_tag (name, category, description) VALUES (...)  →  得 tag_id
4. 构造 snap = TagSnapshot{Name, Category, Description, Aliases: req.Aliases canonical-sorted}
5. tagRepository.ApplySnapshot(tx, tag_id, snap)
   - tx.Updates 标量列（标量已在第 3 步插入，此处幂等 Updates）
   - DELETE FROM galgame_tag_alias WHERE galgame_tag_id = tag_id（首创时无）
   - 逐行 Create GalgameTagAlias
6. snap2 = TakeTagSnapshot(load tag from DB)  // 重新 Take，验证 snap2 == snap
7. INSERT INTO taxonomy_revision (
     entity='tag', target_id=tag_id, revision=1, action='created',
     user_id, user_role(快照当时角色),
     snapshot=snap2,
     changed_fields=['name','category','description','aliases']  // §6.5 约定：created 列出全部字段名
   )
8. 事务提交
```

#### 7.2.2 更新 Tag（admin PUT /galgame/tag/:id）

```
1. 事务开始
2. cur_tag = SELECT galgame_tag JOIN galgame_tag_alias WHERE id = ?
3. cur_snap = TakeTagSnapshot(cur_tag)
4. next_snap = overlayTag(cur_snap, req)
   - req.Name *string nil → keep；非 nil → next.Name = *req.Name
   - req.Description 同 presence 语义
   - req.Category 同
   - req.Aliases *[]string nil → keep；非 nil → next.Aliases = canonical-sort(*req.Aliases)
5. changed = ChangedTagKeys(cur_snap, next_snap)
6. if len(changed) == 0:  return 204 no-op（不产生 revision）
7. tagRepository.ApplySnapshot(tx, id, next_snap)
8. next_take = TakeTagSnapshot(load tag from DB)
9. rev_num = NextTaxonomyRevision(tx, 'tag', id)  // SELECT FOR UPDATE 取 max revision + 1
10. INSERT INTO taxonomy_revision (
      entity='tag', target_id=id, revision=rev_num, action='updated',
      user_id, user_role,
      snapshot=next_take,
      changed_fields=changed
    )
11. 事务提交
```

**关键不变量验证**：步骤 4 严格遵循 §1.5 #5 presence 语义；步骤 6 满足 §1.5 #3（直接编辑 = 对自己做 snapshot overlay）；步骤 8 满足 §1.5 #4（revision snapshot 必须在写入之后重新 Take）。

#### 7.2.3 删除 Tag（admin DELETE /galgame/tag/:id）

复用现有 **force-purge 两步流程**：

```
Step 1 (preflight GET /galgame/tag/:id/references)：
  count = SELECT COUNT(*) FROM galgame_tag_relation WHERE tag_id = ?
  返回 count + sample（前若干被引用作品）

Step 2 (DELETE /galgame/tag/:id?force=true)：
  1. 事务开始
  2. SELECT 1 FROM galgame_tag WHERE id=? FOR UPDATE  // 锁主行（§2 并发安全要求）
  3. ref_count = COUNT galgame_tag_relation WHERE tag_id = ?
  4. affected_gids = SELECT galgame_id FROM galgame_tag_relation WHERE tag_id = ?  // 记录受影响 galgame
  5. cur_snap = TakeTagSnapshot(load)  // 删除前最后状态
  6. DELETE FROM galgame_tag_relation WHERE tag_id = ?  // 级联清引用
  7. DELETE FROM galgame_tag_alias WHERE galgame_tag_id = ?
  8. DELETE FROM galgame_tag WHERE id = ?
  9. rev_num = NextTaxonomyRevision(tx, 'tag', id)
  10. INSERT INTO taxonomy_revision (
        entity='tag', target_id=id, revision=rev_num, action='deleted',
        user_id, user_role,
        snapshot=cur_snap,                         // 删除前最后状态
        changed_fields=[],                         // §6.5 约定：deleted 用空数组
        ref_count=ref_count,                       // 关键：记录被多少 galgame 引用
        affected_galgame_ids=affected_gids         // ★存盘：将来"撤销删除"UI 据此向 admin 提议恢复列表（见 §7.2.4）
      )
  11. ★对每个 affected_gid 落一条 galgame_revision (action='updated', changed_fields=['tag_ids'])：
      - 因为 galgame_tag_relation 的删除等价于这些 galgame 的"tag_ids 集合变化"
      - 不落 galgame revision 会让 galgame 历史出现"tag 莫名消失"无迹可循
      - 实现：对每个 gid 重新 TakeSnapshot 后落 galgame_revision
  12. 事务提交
```

#### 7.2.4 Revert Tag（admin POST /galgame/tag/:id/revert {revision: N}）

```
1. 事务开始
2. target_rev = SELECT * FROM taxonomy_revision WHERE entity='tag' AND target_id=? AND revision=?
3. target_snap = TagSnapshotFromJSON(target_rev.snapshot)
4. case target_rev.action:
     'created' / 'updated' / 'reverted':
       cur_exists = SELECT EXISTS galgame_tag WHERE id=?
       if cur_exists:
         tagRepository.ApplySnapshot(tx, id, target_snap)
       else:
         // tag 之前被删过；走"撤销删除"分支
         INSERT INTO galgame_tag (id, name=target_snap.Name, category=target_snap.Category, description=target_snap.Description)
         tagRepository.ApplySnapshot(tx, id, target_snap)  // 处理 aliases
     'deleted':
       // 用户要 revert 到一个"删除"事件 = 撤销该次删除
       INSERT INTO galgame_tag (id=target_id, name=target_snap.Name, ...)  // 复活
       tagRepository.ApplySnapshot(tx, id, target_snap)                    // 重建 aliases
       // ★还原 galgame_tag_relation 引用？默认 NO：撤销删除只复活 tag 本身，
       // 不自动恢复引用关系（避免悄悄改 galgame 状态）。
       // 但 target_rev.affected_galgame_ids 已存盘（§7.2.3 step 10），
       // 前端/admin UI 据此 SHOW 一份"该 tag 删除前被以下 N 部作品引用，要恢复哪些？"
       // 列表给 admin 勾选；admin 勾的每部作品走标准 galgame 编辑路径加回 tag_ids
       // （每部一条 galgame_revision）。无 affected_galgame_ids 时（老 deleted revision）
       // UI 退回"请手动恢复"提示。
5. next_take = TakeTagSnapshot(load)  // 重新 Take 验证
6. rev_num = NextTaxonomyRevision(tx, 'tag', id)
7. INSERT INTO taxonomy_revision (
     entity='tag', target_id=id, revision=rev_num, action='reverted',
     user_id, user_role,
     snapshot=next_take,
     changed_fields=ChangedTagKeys(cur or empty, next_take)
   )
8. 事务提交
```

### 7.3 Official Revision 完整流程

**完全同 §7.2 tag**，仅替换：
- `GalgameTag` → `GalgameOfficial`
- `GalgameTagAlias` → `GalgameOfficialAlias`（外键列 `galgame_official_id`）
- `GalgameTagRelation` → `GalgameOfficialRelation`
- `taxonomy_revision.entity` = `'official'`
- `TagSnapshot{Name, Category, Description, Aliases}` → `OfficialSnapshot{Name, Original, Link, Category, Lang, Description, Aliases}`

唯一额外要素：official 多 3 个字段（Original / Link / Lang），presence overlay 一并处理。删除路径同 tag 的 force-purge 两步 + 给 affected galgame 落 galgame_revision。

### 7.4 Engine Revision 完整流程

**结构同 §7.2 tag**，但有一处实现差异：

**Engine 的 aliases 是 `datatypes.JSON` 内联列**（jsonb 数组），不是独立关联表。所以 `engineRepository.ApplySnapshot`：

```go
func (r *EngineRepository) ApplySnapshot(tx *gorm.DB, engineID int, snap *model.EngineSnapshot) error {
    aliasesJSON, _ := json.Marshal(snap.Aliases)  // 已 canonical-sort
    return tx.Model(&model.GalgameEngine{}).Where("id = ?", engineID).Updates(map[string]any{
        "name":        snap.Name,
        "description": snap.Description,
        "alias":       datatypes.JSON(aliasesJSON),
    }).Error
}
```

**没有第 2 步独立 alias 表的清空重建**。但 `EngineSnapshot.Aliases []string` 在 jsonb 序列化层面与 tag/official 完全对齐（Snapshot 形态强类型一致），上层逻辑无感。

删除路径：engine 也走 force-purge 两步 + 给 affected galgame 落 galgame_revision。

### 7.5 Series Revision 完整流程（最特殊）

**series 没有 alias，且与 galgame 的关联方向反过来**：`galgame.series_id *int` 指向 series。Series 自己只有 Name + Description。

#### 7.5.1 SeriesSnapshot 故意不含 GalgameIDs

> 重要边界：现有 `SeriesRepository.Update(seriesID, updates, galgameIDs)` 在一个 API 调用里同时改了**series 自身的标量**和**多个 galgame 的 series_id**。这是两类不同实体的混合编辑。

**升级后的设计原则**：

| 操作 | 落点 |
|---|---|
| 改 series.Name / series.Description | `taxonomy_revision (entity='series', action='updated')` 一条 |
| 把某 galgame 加入 / 移出 series（改 galgame.series_id）| **每个受影响 galgame 一条 `galgame_revision`**（action='updated', changed_fields=['series_id']），**不**落 taxonomy_revision |
| 同一次 admin API 调用既改了 series 标量又改了 galgame 成员 | 既落 taxonomy_revision **也**为每个受影响 galgame 落 galgame_revision |

**Snapshot 只覆盖 series 自身**（Name + Description）—— 这与 §1.5 #1 "关联是一等公民" 不冲突，因为这里的关联**不是 series 持有的**（galgame.series_id 在 galgame 行上，已在 galgame Snapshot 的 `SeriesID` 字段里）。Series 的 "Galgame []Galgame" GORM 反向关系是查询便利，不是 series 的数据。

#### 7.5.2 实现细节

```
Series Update（admin PUT /galgame/series/:id with optional galgame_ids）：
  1. 事务开始
  2. 如果 req 改了 Name/Description：
       cur_snap = TakeSeriesSnapshot(load series)
       next_snap = overlaySeries(cur_snap, req)
       if changed_series_keys != 空:
         seriesRepository.ApplySnapshot(tx, id, next_snap)
         落 taxonomy_revision (entity='series', action='updated', snapshot=re-take, changed_fields=changed_keys)
  3. 如果 req.GalgameIDs *[]int 非 nil：
       cur_gids = SELECT id FROM galgame WHERE series_id = ?
       next_gids = canonical-sort(*req.GalgameIDs)
       to_add = next_gids - cur_gids
       to_remove = cur_gids - next_gids
       对每个 gid in to_add:    UPDATE galgame SET series_id = ? WHERE id = gid
       对每个 gid in to_remove: UPDATE galgame SET series_id = NULL WHERE id = gid
       对每个受影响 gid（to_add ∪ to_remove）:
         re_take = TakeSnapshot(load galgame)  // 重新 Take 含新 series_id
         落 galgame_revision (action='updated', snapshot=re_take, changed_fields=['series_id'])
  4. 事务提交
```

**结果**：series 自身的修订历史只关注它自己；某 galgame "什么时候加入/离开了什么 series" 是该 galgame 的修订历史的一部分。审计干净分离。

#### 7.5.3 Series 删除

```
1. 数 ref_count = COUNT galgame WHERE series_id = ?
2. affected_gids = SELECT id FROM galgame WHERE series_id = ?
3. cur_snap = TakeSeriesSnapshot(load)
4. UPDATE galgame SET series_id = NULL WHERE series_id = ?  // 解除引用
5. 对每个 affected_gid: 落 galgame_revision (action='updated', changed_fields=['series_id'])
6. DELETE FROM galgame_series WHERE id = ?
7. 落 taxonomy_revision (entity='series', action='deleted', snapshot=cur_snap, ref_count=ref_count)
```

### 7.6 Refping 完整收集集合（升级关键）

`internal/jobs.GalgameImageRefping` 升级后的 hash 收集**SQL 形态**（伪 SQL，实际用 GORM）：

```sql
-- 收集所有需要保活的 image_hash（每天一次，去重后 batch ping image_service，1000/批）

WITH all_active_hashes AS (
    -- 1. 当前活跃 galgame 的 banner（迁移过渡期还存在）
    SELECT banner_image_hash AS hash FROM galgame
      WHERE banner_image_hash IS NOT NULL AND deleted_at IS NULL
    UNION
    -- 2. 当前 galgame_cover
    SELECT image_hash AS hash FROM galgame_cover
    UNION
    -- 3. 当前 galgame_screenshot
    SELECT image_hash AS hash FROM galgame_screenshot
    UNION
    -- 4. ★所有 galgame_revision snapshot 中曾出现的 hash
    SELECT jsonb_extract_path_text(snapshot, 'banner_image_hash') AS hash
      FROM galgame_revision
      WHERE jsonb_extract_path_text(snapshot, 'banner_image_hash') IS NOT NULL
    UNION
    SELECT jsonb_array_elements(snapshot->'covers')->>'image_hash' AS hash
      FROM galgame_revision
      WHERE snapshot ? 'covers'
    UNION
    SELECT jsonb_array_elements(snapshot->'screenshots')->>'image_hash' AS hash
      FROM galgame_revision
      WHERE snapshot ? 'screenshots'
    UNION
    -- 5. 所有 galgame_pr.snapshot 中的 hash（pending PR 也算引用）
    SELECT ... 同上但 FROM galgame_pr
)
SELECT DISTINCT hash FROM all_active_hashes WHERE hash IS NOT NULL AND hash <> '';
-- 结果集 → batch=1000 调 imageclient.ReferencePing
```

**不收集**：`taxonomy_revision`（当前 schema 不含图片字段）。

**死图兜底（revert 路径）**：revert 加载 revision N 时，对其 snapshot 里的所有 image_hash，先调 image_service 探活（一次 batch query）；不存在的 hash 跳过 + 落 revision note 警告 + 给前端返回 partial-revert 标志。不阻断 revert。**这是兜底机制；正常情况下上面的收集集合保证不会出现**。

**频率**：每日 cron（已有 `cmd/galgame-image-refping`），逻辑迁入 `internal/jobs`（已是 single source of truth 模式）。

**为什么必须扩展收集集合**：image_service TTL 是**已在线运行的现有机制**（>365d 未 ping → 软删 + 30d 物删），不动 refping = revision-only 图被 GC = revert 出死图。扩展是几十行代码（多几条 UNION）保证 revert 永远不死图。**不实现"孤儿扫描脚本"**——以后真要清理才写，那时清理范围 = `image_service.images 全集` − `refping 报活集合`。

### 7.7 §1.5 不变量在 Taxonomy 上的等价表述

| 不变量 | galgame 表述 | taxonomy 等价表述 |
|---|---|---|
| #1 关联是一等公民 | tag_ids 等进 Snapshot | aliases 进 Snapshot（tag/official: 集合排序；engine: 同款字符串数组，仅落地形态不同；series: 无 aliases，不适用）|
| #2 唯一写路径 = ApplySnapshot | `repository.ApplySnapshot` | `Apply{Tag,Official,Engine,Series}Snapshot` 4 个轻量函数；create/update/delete/revert 全经它 |
| #2b 可编辑字段集 ≡ DTO 字段集 | TestEditableSnapshotFieldsAllReachable | **新增** `TestTaxonomyEditableFieldsAllReachable` 一个单测覆盖 4 个 Snapshot 结构（结构体反射） |
| #3 直接编辑 = overlay | overlayUpdate | overlay{Tag,Official,Engine,Series} 4 个 |
| #4 写后重新 Take | TakeSnapshot 再写 revision | 同款，4 个 Take 函数 |
| #5 presence 语义 | *[]int 等 | *string/*[]string 一致；DTO 必须用指针 |
| #6 集合语义 | sort.Ints | sort.Strings on Aliases；series 无集合字段 |

### 7.8 修订交互：跨实体编辑产生多条 revision 是规则不是例外

总结本节贯穿出现的模式：**修改一个实体 X 时，如果同时改了其他实体 Y 的可编辑字段（直接或间接），则 Y 也必须落自己的 revision**。本设计中具体场景：

| 触发 | 产生的 revision |
|---|---|
| Tag 删除（force-purge） | 1 条 taxonomy_revision (entity='tag', deleted) + 每个 affected galgame 1 条 galgame_revision (updated, changed=['tag_ids']) |
| Official 删除（force-purge）| 1 条 taxonomy_revision (entity='official', deleted) + 每个 affected galgame 1 条 galgame_revision |
| Engine 删除 | 1 条 taxonomy_revision (entity='engine', deleted) + 每个 affected galgame 1 条 galgame_revision |
| Series 删除 | 1 条 taxonomy_revision (entity='series', deleted) + 每个 affected galgame 1 条 galgame_revision (changed=['series_id']) |
| Series 改成员（不删 series）| **每个**受影响 galgame 1 条 galgame_revision (changed=['series_id'])；series 标量没改则不落 taxonomy_revision |
| Galgame 编辑（含改 tag_ids/series_id 等）| 1 条 galgame_revision；不动 taxonomy_revision |

**关键不变量**：每个**修订 = 一个实体的一次完整 snapshot**。跨实体的复合编辑产生**多条独立 revision**，不会出现"一条 revision 跨实体"的情况。这与 §1.5 #4 "snapshot == DB == 编辑意图" 严格一致。

---

## 8. API 接口变更速览

| 端点 | 变更 |
|---|---|
| `GET /galgame/:gid` / 列表 | 响应字段：移除 `released`、`banner`、`banner_image_hash`（迁移完成后）；新增 `release_date`、`release_date_tba`、`covers[]`、`screenshots[]`、`effective_banner_hash`（派生） |
| `POST /galgame` / `PUT /galgame/:gid` / `PATCH /galgame/submit/:gid` | DTO：同上字段调整 |
| `POST /galgame/:gid/pr` | PR snapshot 同款扩展 |
| `POST /galgame/:gid/revert` | 不变（snapshot 形态已扩展） |
| `POST /galgame/tag` `POST /galgame/official` `POST /galgame/engine` `POST /galgame/series` | **新增**端点 + 落 taxonomy_revision (created) |
| `PUT /galgame/{tag,official,engine,series}/:id` | **现有端点改造**：经对应 ApplySnapshot + 落 taxonomy_revision (updated) |
| `DELETE /galgame/{tag,official,engine,series}/:id?force=true` | **现有端点改造**：force-purge + 落 taxonomy_revision (deleted) + affected galgame_revision |
| `GET /galgame/{tag,official,engine,series}/:id/references` | 复用现有 preflight |
| `GET /galgame/{tag,official,engine,series}/:id/revisions` / `/:rev` | **新增**：浏览实体编辑历史；底层走 `WHERE entity=? AND target_id=?` |
| `POST /galgame/{tag,official,engine,series}/:id/revert {revision: N}` | **新增**：实体回滚 |

下游 (kungal/moyu) 影响：U1 (released) 是 breaking；U2 (covers/screenshots) 是 additive（旧 banner 字段过渡期保留）；U3 (taxonomy revision) 是 additive。

---

## 9. 显式不做清单 & 已知取舍/技术债

### 9.1 已知取舍（接受这次选择 β 多态修订表带来的代价）

| 取舍 | 描述 | 何时会成为问题 / 退路 |
|---|---|---|
| **丢失 DB 层 `target_id` 引用完整性** | α 可以 `tag_revision.tag_id REFERENCES galgame_tag(id)` 让 DB 拦下"target_id 指向不存在实体"的坏数据。β 的 `taxonomy_revision.target_id` 是裸 int + entity 列，无法外键到具体实体表 | 数据迁移 / 人工 SQL 修复 / 跨服务写入时坏数据不再被 DB 兜底。**缓解**：所有写入必须经 service 层（`Apply*Snapshot` 之前已通过应用层校验实体存在）；测试覆盖坏 `target_id` 写入应拒绝 |
| **`(entity, target_id, revision)` leading column 选择性弱** | entity 只有 4 个值，B-tree 索引前缀区分度低 | 本期数据规模下无感（按 entity+target_id+revision 完整扫描走索引）。规模千万行级才需要考虑改为 hash partitioning by entity |
| **schema 演化路径变窄** | 将来若某 entity 长出独有字段（类似 galgame 的 `is_minor`/`reverted_to`），三个选项：塞 jsonb / 加 nullable 列让其他 entity 永远 NULL / 拆回 α | 本期判断 taxonomy 不会长独有字段（admin-only、简单实体）；真发生时拆出该 entity 到独立 `_revision` 表（一次性迁移） |
| **查询负担转移** | β 的每个查询都要带 `WHERE entity=?`；α 一旦记住表名就直查 | 长期跟着，但 service 层封装后业务代码无感 |

### 9.2 已知技术债（登记，不本期处理）

| 技术债 | 触发条件 | 处理方案（届时）|
|---|---|---|
| **refping 集合长期单调增长** | 每天 ping 的 hash 集合 = 当前 + 所有历史 revision/PR 中曾出现过的 hash。随时间增长无界。本期规模可承受（年级 < 10 万 hash） | 超过 N 万时评估：(a) 历史快照按时间归档（>3 年的 revision snapshot 单独存）；(b) 快照内 hash 去重存储（多 revision 同 hash 只 ping 一次）；(c) 长期未引用 hash 的 hard purge 工具 |
| **per-image NSFW gating 字段当前不消费** | `galgame_cover/screenshot.Sexual/Violence` schema 已落，但 v1 应用层**不用它做展示门控**。展示仍按 `galgame.content_limit` + 用户年龄设置粗粒度判断 | 产品需要 per-image gating 时：(a) admin 编辑界面加评级 UI 让 admin 手填；(b) 应用层 gate 链改为 `per_image > galgame.content_limit > 用户设置`；(c) `Sexual=0` 仍解释为"未评定" fallback 到粗分级 |
| **changed_fields 不落 `galgame_revision`** | galgame revision 仍走"读时算 ChangedKeys"，taxonomy 已落库 | 将来给 galgame 加 RBAC 时再加列；语义复用 §6.5 约定 |

### 9.3 显式不做的功能项

均**留 schema/字段扩展位**，将来加都不需要破坏性迁移。

| 项 | 现状 | 扩展位 |
|---|---|---|
| `changed_fields` 落库到 `galgame_revision` | 不做 | taxonomy_revision 已有该列；galgame_revision 可后加 |
| 字段级权限（位掩码 / 集合 RBAC） | 不做 | `taxonomy_revision.user_role` + `changed_fields` 已就位 |
| sync 预览 + 选择性 apply | 不做 | `galgame_cover/screenshot.Source/SourceKey` 列已为 fill-only 同步预留 |
| typed 游戏关系图（续作/FD/同企划）| 不做 | 现 `series_id` 单一关系够用 |
| 多语言封面（per-language） | 不做 | `galgame_cover` schema 可后加 `language` 列 |
| 角色实体（character） | 不做 | 单独立项 |
| per-game `tag_alias`（同一 tag 在不同作品本地化别名） | 不做 | 现 `galgame_tag_relation.spoiler_level` 同款设计可后加 |
| `staffs` / `extra_info` 灵活 JSON 列 | 不做 | 易腐化为无 schema 垃圾场，明确不做 |
| 定点 undo（撤销任意一条历史编辑） | 不做 | 现 revert 已等价；快照模型让 undo 可低成本模拟 |
| 粒度化关系动作类型（ADD/REMOVE/SET/UPDATE）| 不做 | 集合语义 + canonical diff 已能表达 added/removed |
| `actor_role` 落 `galgame_revision` | 不做 | taxonomy_revision 已为 RBAC 落；galgame_revision 后加 |
| AI 内容审核 | 不做 | 现人工 status 流转够用 |
| 媒体 provenance 三元组逻辑实现 | schema 留位但不实现 | `Source/SourceKey` 列已加 |
| 跨源一致性门（合并前校验）| 不做 | sync 改造时再做 |
| fill-only 合并（导入永不覆盖人工编辑）| 不做 | 同上 |
| sync 候选 diff 预览 | 不做 | 同上 |
| hot-score 热度算法 | 不做 | 产品决策时再做 |
| Bangumi 第二数据源 | 不做 | 同上 |
| 搜索单投影一致性纪律 | 不做 | 现 Meilisearch 工程纪律够用 |
| revert dry-run 冲突预览 | 不做 | 现 revert 直接生效够用 |
| 预计算 NSFW 过滤字段 | 不做 | 现 content_limit 粗粒度够用 |
| 增量关系编辑端点（add/remove 单 tag） | 不做 | 前端继续全量水合，与 §1.5 #5 一致 |
| 票选封面 + admin override | 不做 | `galgame_cover` schema 已为票选预留（独立 `galgame_cover_vote` 表后加）；override 字段后加（trivial 迁移）|
| 孤儿图片扫描脚本 | 不做 | refping 收集集合已保证活引用全集存活；以后真要清理再写 |

---

## 10. 实施顺序（建议 PR 切分）

| PR | 范围 | 依赖 |
|---|---|---|
| **PR1** | U1：`released → release_date + release_date_tba`，含迁移脚本（一刀切，含 revision snapshot jsonb 补丁）+ 下游 handbook 更新 | 无 |
| **PR2** | U2.a：新增 `galgame_cover` / `galgame_screenshot` schema + Snapshot 扩展 + ApplySnapshot 第 6/7 张表 + DTO + handler；effective_banner_hash 派生；现有 `banner_image_hash` 数据回填为 `galgame_cover(sort_order=0)`；测试 | 无 |
| **PR3** | U2.b：refping 扩展为收集集合（含 revision/PR snapshot 内 hash）+ revert 死图兜底 + 测试 | PR2 |
| **PR4** | U3：`taxonomy_revision` 单表 + 4 个 entity 各自的 `Apply*Snapshot` / `Take*Snapshot` / `overlay*` / `ChangedKeys` + handler 改造 + revert/references/revisions 端点 + 防回归测试 | 无 |
| **PR5（验证稳定后）** | U2.c：drop `galgame.banner_image_hash` 列 + Snapshot.BannerImageHash 字段 + 修补历史 revision snapshot jsonb | PR2 上线稳定 ≥ 2 周 |

PR1 / PR2 / PR4 可并行。PR2 → PR3 → PR5 串行。

---

## 11. 防回归测试清单

每条标"新增"的测试在对应 PR 必加，作为 `01-revision-system-design.md §1.5` 不变量的护栏：

### Galgame（升级）
- ✅ 既有：`TestEditableSnapshotFieldsAllReachable`——自动覆盖新增的 ReleaseDate / ReleaseDateTBA / Covers / Screenshots（结构体反射）
- 🆕 `TestApplySnapshot_RebuildsCoversAndScreenshots`：写入两张图后 cur snapshot 应等于 input
- 🆕 `TestUpdate_ReleaseDateAndTBAOverlay`：release_date / tba 的 presence 语义（含 `&""` 清空、`&"2024-01"` 非法、`&"2024-06-15"` 合法）
- 🆕 `TestUpdate_CoversPresenceSemantics`：nil 保持 / 空数组清空 / 非空全量替换
- 🆕 `TestRevert_RestoresCoversAndScreenshots`

### Taxonomy（新增）
- 🆕 `TestTaxonomyEditableFieldsAllReachable`：一个反射单测覆盖 4 个 Snapshot 结构 vs 对应 DTO
- 🆕 `TestTaxonomyUpdate_NoOpProducesNoRevision`（按 entity 参数化跑 4 次）
- 🆕 `TestTaxonomyDelete_RecordsRefCountAndAffectedGalgameIDsAndGalgameRevisions`（参数化；含 §7.2.3 step 10 的 affected_galgame_ids 落盘断言）
- 🆕 `TestTaxonomyRevert_FromDeleted_ResurrectsButNoRelationRestore_AffectedIDsAvailableForUI`（参数化）
- 🆕 `TestSeriesUpdate_GalgameIDsProducesGalgameRevisionsNotTaxonomyRevision`
- 🆕 `TestSeriesUpdate_NameOnlyProducesTaxonomyRevisionOnly`
- 🆕 `TestChangedFieldsSemantics_CreatedListsAll_DeletedEmpty_UpdatedDiffOnly`（§6.5 约定护栏）
- 🆕 `TestTaxonomyConcurrentUpdate_SerializedByMainRowLock`（验证 §2 并发安全：两个并发 update 同一 tag 不出现重复 revision 序号）

### 图片不变量护栏
- 🆕 `TestGalgameCover_PartialUniqueIndexEnforcesOnePinned`（两次 INSERT sort_order=0 同一 galgame_id 应 DB-level 拒绝）
- 🆕 `TestPinNewBanner_FlowAtomicallyDemotesOld`（事务内先降级旧、再升新；中途回滚不留并列态）

### Refping
- 🆕 `TestRefping_IncludesCurrentCoverAndScreenshotHashes`
- 🆕 `TestRefping_IncludesRevisionSnapshotHashes`（构造一个已删图的 cover + 一条历史 revision 仍持有该 hash）
- 🆕 `TestRevert_GracefulDegradeOnMissingImage`

---

## 12. 一句话总结

本次升级在保持 `01-revision-system-design.md §1.5` 6 条不变量的前提下做三件事：

1. `released` 改为 `release_date date? + release_date_tba bool`，一刀切；
2. 图片模型升级为 banner + 多 cover + 多 screenshot 候选集，按 image_hash 引用 image_service，**effective banner = sort_order 最小的 cover**，refping 扩展为 ping「当前 + 所有 revision/PR snapshot 中的 hash」防 revert 死图；
3. tag/official/engine/series 共用 1 张 `taxonomy_revision` 多态全快照表，与 `galgame_revision` 核心语义同构（全快照 + 单一 Apply 写路径 + revert），加新 taxonomy 实体只加 Go struct + CHECK 值，不加表。

**留扩展位但不实现**的能力：票选 / banner override / 字段级 RBAC / sync 预览管线 / changed_fields 落 galgame_revision / typed 关系图 / 多语言封面 / 角色实体。这些将来加都不需要破坏性迁移核心 schema。

修订模型一律采用全快照（不采用 delta）：revert/diff 永远正确、无需重放重建。
