# 用户迁移 - 文档索引

> 把 kungal 和 moyu 的用户合并到 OAuth 单一身份源，并把三个数据库的 `user.id` 完全对齐。

## 这套文档读什么

| 文档 | 内容 | 读它如果 |
|------|------|---------|
| [01-architecture.md](./01-architecture.md) | 为什么 OAuth 是身份单一源、字段如何分布 | 第一次接触本架构 |
| [02-data-mapping.md](./02-data-mapping.md) | kungal/moyu user 表每一列在 OAuth 端的去向 | 想知道某个字段去哪了 |
| [03-id-unification.md](./03-id-unification.md) | 时序 ID + 两阶段 offset 算法 + 全部 FK 列 + mention URL 重写 | 想搞懂 step 7 的技术细节 |
| [04-password-migration.md](./04-password-migration.md) | bcrypt/argon2id 旧密码首次登录透明迁移 | 关心用户登录是否还能用 |
| [05-execution.md](./05-execution.md) | dry-run / 正式跑 / 输出格式 | 准备实操 |
| [06-recovery.md](./06-recovery.md) | 失败状态判断 + 恢复流程 | 已经跑挂了，想救场 |
| [07-verification.md](./07-verification.md) | 跑完之后怎么验证 + 反查迁移前的原始 ID | 想确认数据正确 / 后续脚本要查原 ID |
| [08-downstream-integration.md](./08-downstream-integration.md) | 业务后端怎么调 `/users/batch` `/users/search` SDK 拉用户信息 | kungal/moyu 后端开发 |

## 一句话总结

```
脚本：apps/api/cmd/migrate-users
目标：让 OAuth.users 成为唯一的身份源；让 kungal.user.id == moyu.user.id == OAuth.users.id
```

跑完之后的不变量：

- **OAuth `users` 表**：integer id + uuid，是身份的权威定义
- **kungal/moyu `user` 表**：保留，但 `user.id` 与 OAuth 完全相同；字段瘦身只剩站点特有计数器
- **所有 FK 列**：69 个 `*_user_id` 列已对齐到新 ID
- **chat_room.name**（kungal 私聊"uid1-uid2"格式）：已重算
- **patch_comment.content**（moyu mention URL）：已重写
- **password / kungal_password / moyu_password**：用户首次登录时自动从旧 hash 透明迁移到新 hash

## 必须的执行顺序

```
1. migrate-users          ← 本文档对象
2. migrate-galgame-data   ← 见 docs/galgame_wiki/
3. migrate-moyu-galgame   ← 见 docs/galgame_wiki/
```

galgame 迁移依赖 user 迁移先跑（因为 wiki 的 `galgame.user_id` 在 import 时拿的是源库的 user_id 原值，必须是已经对齐过的值）。颠倒顺序会让 galgame.user_id 指向不存在的用户。

## 跑之前必读的三件事

1. **三库都备份。** step 7 重写源库（kungal/moyu），事务级原子但跨脚本不可逆。
2. **应用必须停机。** step 7 关闭触发器，期间任何写入都会绕过 FK 检查、写入旧 ID 而不被重映射。
3. **先 dry-run。** 看用户合并/跳过/创建的计数符不符合预期。

详见 [05-execution.md](./05-execution.md)。

## 快速命令

```bash
cd apps/api

# Dry run
go run ./cmd/migrate-users \
  --kungal-dsn="host=... dbname=kungalgame   sslmode=disable" \
  --moyu-dsn="host=...   dbname=kungalgame_patch sslmode=disable" \
  --dry-run

# 实跑
go run ./cmd/migrate-users \
  --kungal-dsn="..." \
  --moyu-dsn="..."
```

## 如果跑挂了

直接读 [06-recovery.md](./06-recovery.md)。决策表大概长这样：

| 现象 | 原因 | 操作 |
|------|------|------|
| step 1-6 失败 | OAuth 库或源库连接、数据问题 | 原地重跑（OAuth 端有 idempotent skip） |
| step 7 失败 | 重映射事务回滚 | OAuth 已部分写入；恢复 OAuth 库或手动清理后重跑 |
| 跑了一半发现不对 | 各种 | 三库恢复备份，从头来 |
