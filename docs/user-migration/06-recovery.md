# 故障恢复

> 跑挂了怎么办。先按现象定位，再按推荐的恢复路径走。

## 1. 失败状态决策表

| 现象 | 在哪挂的 | OAuth 库状态 | kungal 库状态 | moyu 库状态 | 建议恢复路径 |
|------|----------|--------------|---------------|-------------|--------------|
| 连不上源库 | step 0 | 未动 | 未动 | 未动 | 修连接，原地重跑 |
| 缺 OAuth schema 表 | step 0 | 未动 | 未动 | 未动 | 跑 `cmd/migrate` 建 schema，再重跑 |
| 缺 site 记录 | step 0 | 未动 | 未动 | 未动 | 在 OAuth 库 INSERT site 记录，重跑 |
| 用户插入报错 | step 4 | **部分插入** | 未动 | 未动 | 见 §2 |
| follow 关系报错 | step 5 | 全部插入 | 未动 | 未动 | 一般不致命，原地重跑（idempotent） |
| 角色映射报错 | step 6 | 全部插入 + follow | 未动 | 未动 | 同上 |
| step 7 kungal 失败 | step 7（kungal 阶段） | 全部插入 | **回滚** | 未动 | 见 §3 |
| step 7 moyu 失败 | step 7（moyu 阶段） | 全部插入 | **已重映射** | **回滚** | 见 §4（最严重） |
| 跑了一半进程被杀 | 任意 | 看进度 | 看进度 | 看进度 | 见 §5 |

## 2. step 4 部分插入后失败

OAuth 端的 `users` 表已经有一部分新行，但 `user_site_data` / `user_migrations` 可能不完整。

**原地重跑**：脚本里有按 email 的去重逻辑：

```go
existingEmails := ...  // 已存在的 email
for _, m := range allMerged {
    if existingEmails[m.Email] {
        result.SkippedDuplicates++
        continue
    }
    ...
}
```

所以重跑时已插入的用户会被跳过、未插入的会补上。**安全。**

但请确认：

- step 4 的失败是"业务问题"还是"DB 损坏"？业务问题（比如某个 email 太长 / 含 NULL）原地重跑还会再撞同一行
- 如果你想完全清状态从头来：`TRUNCATE users, user_site_data, user_migrations RESTART IDENTITY CASCADE`，然后重跑

## 3. step 7 kungal 阶段失败

事务回滚 → kungal 库未动；但 OAuth 端已经有 81442 个新用户和 user_migrations 表数据。

**两条恢复路：**

### 3.1 修了 bug 后重跑（推荐）

脚本检测到 OAuth `users` 已经有这些 email → SkippedDuplicates 加满 → 跳过 step 4。然后接着跑 step 7。

但要注意：脚本用 `sourceToNewID` 内存映射来重映射。重跑时这个映射要重新构建 —— 它会从源库读 `kungal.user`、`moyu.user`，然后查 OAuth 端 `user_migrations` 表反查（`source_db, source_user_id → user_id`）。

**目前脚本的实现不会自动从 user_migrations 反查**。所以原地重跑会发现 `sourceToNewID` 是空的（因为 step 4 跳过了所有用户）→ step 7 没事干 → 跑完显示"0 users remapped"。

实际可行的解决方案：

- 改脚本：让它在 SkippedDuplicates 路径也填充 sourceToNewID（从 user_migrations 反查）
- **或者** 把 OAuth 端清空 + 重跑全程（见 3.2）

### 3.2 OAuth 清空重跑（保守）

```sql
-- 在 OAuth 库
TRUNCATE users, user_site_data, user_migrations, user_follow, user_roles RESTART IDENTITY CASCADE;
```

然后重跑全脚本。这样 sourceToNewID 重新填充、step 7 正常跑。

适用条件：尚未有真实用户在 OAuth 上注册过新账号（即只有迁移产生的用户）。

如果 OAuth 上已经有迁移之外的新用户，那不能 TRUNCATE —— 必须用 3.1 的"改脚本"方案。

## 4. step 7 moyu 阶段失败（最棘手）

最严重的状态：

- OAuth：全部插入完毕
- kungal：**已经重映射**（它的 step 7 半个事务已经 commit 了）
- moyu：事务回滚，未动

这意味着 kungal 的 user.id 已经是 OAuth 的 ID，但 moyu 还是原样。三库**部分对齐**。

**只有一个干净恢复路：从 backup 恢复 kungal 库**。

```bash
pg_restore -d kungalgame_TARGET kungal.dump
```

然后修 moyu 端的 bug，再跑全程。**OAuth 库不用动**（因为脚本对已有用户会 skip）。

> 这就是为什么 §1 的 prereq 强调"三库都备份"。step 7 一旦半成功，必须通过外部备份才能恢复。

理论上也可以"反向 remap kungal" —— 把 kungal 的 user.id 用 user_migrations 表反查回去再写回旧值。但这是很危险的脚本，没人愿意写第二份反向迁移。**用 backup 就好。**

## 5. 进程被杀（事务未提交）

PostgreSQL 的事务模型保证：如果连接断开（进程被杀、网络断、kill -9），未 commit 的事务**自动 ROLLBACK**。

所以：

- step 4 中途被杀 → 那一个用户的 transaction 回滚（每用户一个 tx）；之前 commit 过的用户保留
- step 5/6 中途被杀 → 类似（每条 follow / role 一个 tx）
- step 7 中途被杀 → 整个 step 7 回滚（每个源库一个大 tx）

按 §1 表里对应的"已成功的步骤"判断状态。例如：

- step 7 kungal 跑了 5 分钟被杀 → kungal 库的 step 7 回滚 → kungal 未动
- step 7 kungal 已 commit、moyu 跑到一半被杀 → 同 §4，需要从 kungal backup 恢复

## 6. 验证当前状态

```sql
-- OAuth 端
SELECT COUNT(*) FROM users;            -- 期望: 81442 (或预期值)
SELECT COUNT(*) FROM user_migrations;  -- 期望: 88636 (跨站合并的算两条)

-- kungal 端
SELECT MAX(id) FROM "user";            -- 应该 == OAuth users.id 最大值
SELECT id FROM "user" WHERE name = '鲲'; -- 应该等于 OAuth 端 '鲲' 的 id

-- moyu 端
SELECT id FROM "user" WHERE name = '鲲'; -- 同上，应该相等
```

如果三库三个 `id` 不一致 → 说明 step 7 没在那一边成功执行（或那一边的备份被恢复了之后忘记重跑）。

## 7. 安全网

- 三库都备份（pg_dump -Fc）
- step 7 是事务化的 —— 单步内事故安全
- 跨脚本顺序事故（kungal 成功 / moyu 失败）需要 backup 救场
- OAuth 端 `user_migrations` 表是"原 ID ↔ 新 ID"的权威账本，可用于审计和未来反查

## 8. 不该做的事

| 危险操作 | 后果 |
|----------|------|
| 跑了 step 7 之后，发现 OAuth 端 user 数据有问题，TRUNCATE OAuth `users` 表 | OAuth 端清空，但 kungal/moyu 的 user.id 已经指向那些不存在的 ID。三库脱钩。**只能从 backup 恢复 kungal/moyu**。 |
| 在 step 7 跑到一半时手动 ALTER TABLE | 数据库锁冲突，事务挂住或异常退出 |
| 不加 `--dry-run` 直接跑生产 | 错过验算合并/跳过/创建数 |
| 不停应用直接跑 step 7 | 应用写入绕过 trigger 进入旧 ID，迁移完毕后这些行成为孤儿 |
| 跑两遍同一脚本然后期待"幂等" | step 4 是幂等的，step 7 不是 —— 第二次跑 step 7 会试图把"已经是 new_id"的 id 当成 old_id 再次平移，导致灾难 |

最后这条特别关键：**step 7 不可重跑**。一旦跑过一次成功，源库的 user.id 已经是 OAuth 的 ID，再跑就是把 OAuth ID 当 kungal 旧 ID 二次重映射，全乱。

要重做迁移：先从 backup 恢复源库 + OAuth 库，再从头跑。
