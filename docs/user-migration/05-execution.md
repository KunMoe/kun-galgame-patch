# 执行：怎么跑

## 1. 前置条件

### 1.1 OAuth target 库 schema 已就绪

```bash
cd apps/api
go run ./cmd/migrate
```

会建好：

- `users` / `user_roles` / `roles` / `user_site_data` / `user_migrations`
- `sites` 表（必须有 `www.kungal.com` 和 `www.moyu.moe` 两条 site 记录）
- `oauth_clients` / `oauth_sessions` / 其他 OAuth 协议表

### 1.2 源库可达

kungal 和 moyu 两个 PostgreSQL 实例的连接 DSN：

```
host=...  port=5432  user=...  password=...  dbname=kungalgame   sslmode=disable
host=...  port=5432  user=...  password=...  dbname=kungalgame_patch  sslmode=disable
```

源库可以是只读快照、生产副本、或生产本身（生产本身请先停机 + 备份）。

### 1.3 三库都备份

step 7 重写源库（kungal/moyu），事务级原子但跨脚本不可逆。

```bash
pg_dump -Fc kungalgame      > kungal.dump
pg_dump -Fc kungalgame_patch > moyu.dump
pg_dump -Fc kun_oauth        > oauth.dump
```

### 1.4 应用必须停机

step 7 关闭 trigger。期间任何写入会绕过 FK 检查、写入旧 ID 而不被重映射，造成数据漂移。

实操方法：

- 停 kungal / moyu 站点的应用进程
- 或把数据库设置成 readonly
- 或在应用层加只读维护开关

### 1.5 源库邮箱建议先去重

合并按 email 做。如果 kungal 内部就有重复 email（一人注册了两次），脚本会**保留最早**那条、跳过其他。这虽然安全，但你可能想先在源库手动合并干净，避免 SkippedDuplicates > 0。

## 2. Dry run

正式跑之前**强烈建议**先 dry-run。它读源库、计算合并、打印将要发生的事，但**不写任何数据**：

```bash
cd apps/api

go run ./cmd/migrate-users \
  --kungal-dsn="host=localhost port=5432 user=postgres password=xxx dbname=kungalgame      sslmode=disable" \
  --moyu-dsn="host=localhost port=5432 user=postgres password=xxx dbname=kungalgame_patch sslmode=disable" \
  --dry-run
```

dry-run 输出示例：

```
INFO Connected to database name=kungal
INFO Connected to database name=moyu
INFO DRY RUN MODE - No changes will be made
INFO Site IDs resolved kungal=1 moyu=2
INFO Fetched kungal users count=67373
INFO Fetched moyu users count=21286
INFO Merged and sorted users total=81442 merged_count=7194
INFO Found existing users in target count=0

==================================================
Migration Results (DRY RUN)
==================================================
Kungal users total:    67373
Moyu users total:      21286
--------------------------------------------------
New users created:     81442
Users merged:          7194         (跨站邮箱相同)
Site data created:     88636
Follows migrated:      2278
Follows skipped:       0
Roles assigned:        57
Skipped (existing):    0
Errors:                0
==================================================
```

检查这些值是不是符合预期：

- `New users created` 应等于 `kungal_total + moyu_total - merged_count - skipped_intra_dups`
- `Users merged` = 跨站同邮箱的用户数
- `Errors: 0` —— 任何 >0 的 errors 都要先调查

## 3. 正式跑

去掉 `--dry-run`：

```bash
cd apps/api

go run ./cmd/migrate-users \
  --kungal-dsn="host=localhost port=5432 user=postgres password=xxx dbname=kungalgame      sslmode=disable" \
  --moyu-dsn="host=localhost port=5432 user=postgres password=xxx dbname=kungalgame_patch sslmode=disable"
```

预期时间：

| 步骤 | 时间（70k 用户、30M FK 行） |
|------|-----------------------------|
| step 1-3 读取 + 合并 | 5-15 秒 |
| step 4 OAuth 端插入用户 | 30-60 秒（每 1000 行打一行 progress） |
| step 5 follow 关系 | 5-10 秒 |
| step 6 角色映射 | <5 秒 |
| step 7 kungal 重映射 | 8-15 分钟 |
| step 7 moyu 重映射 | 2-5 分钟 |
| **总计** | **15-25 分钟** |

如果你看到 step 7 跑了 30 分钟还没动静，那就有问题（详见 [06-recovery.md](./06-recovery.md)）。

## 4. 输出格式

跑完之后打印的 summary：

```
==================================================
Migration Results
==================================================
Kungal users total:    67373
Moyu users total:      21286
--------------------------------------------------
New users created:     81442      ← 实际新建的 OAuth users 行数
Users merged:          7194       ← 跨站邮箱相同被合并的人数
Site data created:     88636      ← user_site_data 行数（每用户 × 每站点 = 1 行）
Follows migrated:      2278       ← 从 moyu 迁过来的 follow 关系
Follows skipped:       0          ← 因为找不到映射跳过的 follow（应为 0）
Roles assigned:        57         ← admin + moderator 总数
Skipped (existing):    0          ← 在 OAuth 库已存在（按 email）跳过的人数
Errors:                0          ← 任何错误（应为 0）
==================================================
```

期间日志：

- 每 1000 用户打一条 progress（INFO）
- step 7 每张表打一条 `remap pass1` / `remap pass2`
- 任何错误打 ERROR + stack 后继续（**Errors > 0 必查**）

## 5. 跑完之后

继续按顺序跑 galgame 迁移：

```
2. migrate-galgame-data    (kungal → wiki)
3. migrate-moyu-galgame    (moyu → wiki + moyu patch_id remap)
```

详见 `docs/galgame_wiki/02-moyu-migration-design.md`。

然后做后置校验：详见 [07-verification.md](./07-verification.md)。

## 6. 跑完之后能不能让应用上来

**不能立刻。** 因为：

1. galgame 迁移还没跑（它依赖 user 迁移先跑完）
2. kungal/moyu 应用必须更新代码到"从 OAuth 拉用户信息"的版本（详见 [08-downstream-integration.md](./08-downstream-integration.md)）

如果只是想做个 staging 验证：

- user 迁移完后可以先用 OAuth 端 API（`/users/batch`）验证身份能正确解析
- 然后跑 galgame 迁移
- 最后再让站点上线

## 7. 跑挂了怎么办

读 [06-recovery.md](./06-recovery.md)。

简短决策：

- step 1-6 失败 → 一般可以原地修复后重跑（OAuth 端有按 email 的 idempotent skip）
- step 7 失败 → 由于事务原子性，源库不会半动；OAuth 库已有部分插入，需要决定是清掉重来还是从 backup 起
