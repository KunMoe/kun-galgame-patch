# 数据库 Schema 主权

> 本文档说明 moyu (`kungalgame_patch`) 数据库 schema 的归属、FK 行为约定，以及 Go 代码层如何引用它们。

## TL;DR

- **真理之源**：`apps/api/migrations/*.sql` —— Go 仓库内显式 SQL migrations。
- **GORM AutoMigrate**：**不用**。Go API 启动不重建表，只通过 `cmd/migrate` 跑 SQL。
- **历史**：老 Nitro + Prisma 时代的 schema 已 `pg_dump -s` 进 `000_baseline.up.sql`，后续增量在 `001-NNN_*.sql`。Prisma 已从仓库移除。
- **GORM model 上的 `constraint:OnDelete:X` tag** 是**文档注释**，运行时无效。真正起作用的是数据库里的 FK。

## 为什么会有 baseline

老路径：`reset_all.sh` → 从 `kungalgame_patch_backup.dump`（Prisma 时代的备份）恢复 → 跑 `cmd/migrate-oauth-prep` → 跑 `cmd/migrate` 应用增量 001-NNN。这条路径在生产环境工作良好，但有两个隐患：

1. **跨环境不一致**：dev / CI / DR 若没有 `kungalgame_patch_backup.dump`，从空库起步跑 `cmd/migrate` 拿不到任何 CREATE TABLE / FK，只能得到 9 个增量的"空中楼阁"。
2. **代码与现实脱钩**：Go 代码里看不出 FK 约束 / CASCADE 行为，因为 GORM model 没声明这些。审计 / 安全 / 新人 onboarding 时只能依赖"幽灵约束"——它们在 prod 真实存在，但仓库里不可见。

`000_baseline.up.sql` 解决这两件事：它把当前 prod schema 的完整结构（22 表、19 序列、19 索引、56 约束含 32 FK）以幂等 SQL 形式入库。新环境 `cmd/migrate` 一发命令即可拿到完整 schema；prod 环境只需手动 `INSERT INTO _migrations (name) VALUES ('000_baseline')` 标记跳过。

## FK 行为分布

| 行为 | 数量 | 含义 |
|---|---|---|
| `ON DELETE CASCADE` | 28 | 默认。删父行时级联删子行，PG 自动处理。**Go 代码不需要也不应该手动级联** |
| `ON DELETE SET NULL` | 2 | 见下 |
| `ON DELETE RESTRICT` | 2 | 见下 |

### 4 个非 CASCADE 特例

| 表 | 字段 → 引用 | 行为 | 业务原因 |
|---|---|---|---|
| `chat_message` | `deleted_by_id → user.id` | SET NULL | mod/admin 用户被删时，被删消息的 deleted_by 指针清空，保留软删记录 |
| `chat_message` | `reply_to_id → chat_message.id` | SET NULL | 被引用的消息被删时，引用方仍存在但失去 "回复了哪条" 的指向 |
| `patch` | `user_id → user.id` | RESTRICT | 用户名下有 patch 时阻止删用户（防失主） |
| `user_follow_relation` | `following_id → user.id` | RESTRICT | 被关注关系阻止删用户（注意：`follower_id` 是 CASCADE，**不对称**） |

## GORM `constraint:OnDelete:X` tag 约定

- **CASCADE = 默认，model 上不标**（标了会稀释真正的特例）
- **特例必标 + 长注释**（说明为什么不是 CASCADE，谁会被影响）
- **tag 不参与运行时**：GORM 只在 AutoMigrate 时解析这个 tag，本项目不跑 AutoMigrate，所以 tag 纯粹是给读 model 的人看的文档。真正生效的是数据库里的 FK 定义，源头是 `000_baseline.up.sql`。

举例：
```go
// chat/model/model.go
type ChatMessage struct {
    ...
    DeletedByID *int `gorm:"constraint:OnDelete:SET NULL" json:"deleted_by_id"`
    ReplyToID   *int `gorm:"constraint:OnDelete:SET NULL" json:"reply_to_id"`
}

// patch/model/model.go
type Patch struct {
    ...
    UserID int `gorm:"not null;constraint:OnDelete:RESTRICT" json:"user_id"`
}

// user/model/model.go
type UserFollowRelation struct {
    FollowerID  int `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"` // CASCADE
    FollowingID int `gorm:"uniqueIndex:idx_follow;not null;constraint:OnDelete:RESTRICT" json:"following_id"`
}
```

## Go 代码依赖 FK CASCADE 时的 4 条规则

1. **CASCADE 不维护 denormalized count**。PG 级联删除子表行时，父表 / 兄弟表的 `*_count` 字段（如 `patch.resource_count`、`user.follower_count`）**不会自动减**。任何会引发级联的删除路径，service 层必须在事务内手动 update count。
2. **CASCADE 不触发应用层 hook / notification**。如 `notifyFavoritedUsers` 这类副作用要由 service 显式调用，不能寄望 DB 一并搞定。
3. **CASCADE 不清理 S3 / 外部资源**。`patch_resource.s3_key` 指的 B2 对象在 row 被 CASCADE 删除时**不会被一起删**。这种场景必须走 `DeleteResource` service path（带 best-effort `s3.DeleteObject`），不能裸 SQL DELETE。
4. **`*_count` 字段读时不可信任**（如果跨开发周期发生过 CASCADE 删除而没维护 count）。修复方式是写一条 reconcile cron 定期重算，或在删除路径上手动 update。

## 改 schema 的标准流程

1. 在 `apps/api/migrations/` 下新增 `NNN_descriptive_name.up.sql` + `.down.sql`，编号紧接现有最后一个。
2. SQL 写**幂等**：`CREATE TABLE IF NOT EXISTS`、`ALTER TABLE ... ADD COLUMN IF NOT EXISTS`、`DO $$ BEGIN ... EXCEPTION WHEN duplicate_object ... END $$;`。
3. 如果改 FK 行为（特别是从 CASCADE 改成 SET NULL/RESTRICT 或反之）：在对应 GORM model 上同步加 / 改 `constraint:OnDelete:X` doc tag + 长注释，并更新本文 §FK 行为分布。
4. 跑 `go run ./cmd/migrate -yes` 验证；fresh DB 跑 `reset_all.sh` 之外的纯空库验证。
5. 部署到 prod 之前在 staging 跑一遍 `cmd/migrate`，看 NOTICE 行无 ERROR。

## 跟现有 runbook 的关系

完整 runbook 见上层文档（部署流程）。这里只说 baseline 涉及到的两条路径：

- **Prisma-restore 路径**（`reset_all.sh` + `kungalgame_patch_backup.dump`）：runbook 跑 `cmd/migrate-oauth-prep` → `cmd/migrate`。`migrate-oauth-prep` 写完自己的 marker 后会顺手 `INSERT '000_baseline' INTO _migrations`，所以后续 `cmd/migrate` 跳过 baseline，直接增量 001-009。**必须跳过** —— Prisma-era schema 里没有 `s3_key` / `galgame_id` 列（这些是 002/004 才加的），baseline 的 `CREATE INDEX IF NOT EXISTS` 不预校验列存在，会在 prisma 库上炸。

- **Fresh DB 路径**（dev / CI / DR，无 Prisma 备份）：直接 `cmd/migrate -yes`，baseline 自动跑（建 post-009 schema）→ 001-009 全部被各自的 IF EXISTS / DO EXCEPTION 守护跳过 → 得到跟 prod 完全一致的 schema。

两条路径的"自动登记 baseline marker"是 `migrate-oauth-prep` 的副作用，operator 不需要手动 SQL。如果某天有人在 oauth-prep 之外的场景从 prisma 备份起步（比如重做 oauth-prep 已经跑过的库），手动登记仍是 escape hatch：
```bash
psql -d kungalgame_patch -c \
  "INSERT INTO _migrations (name) VALUES ('000_baseline') ON CONFLICT DO NOTHING;"
```
