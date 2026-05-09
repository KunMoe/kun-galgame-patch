# 跑完之后的校验

> 怎么确认迁移真的对齐了。还有怎么反查迁移前的原 ID（给后续脚本用）。

## 1. 总数核对

```sql
-- OAuth 端
SELECT COUNT(*) FROM users;                                          -- expect: New users created
SELECT COUNT(*) FROM user_migrations;                                 -- expect: 跨站合并算两条；> COUNT(users)
SELECT COUNT(*) FROM user_site_data;                                  -- expect: COUNT(user_migrations)
SELECT COUNT(*) FROM user_roles;                                      -- expect: 跑出来的 Roles assigned 值
SELECT COUNT(*) FROM user_follow;                                     -- expect: Follows migrated 值

-- 双源用户数（跨站合并）
SELECT COUNT(*) FROM (
  SELECT user_id FROM user_migrations GROUP BY user_id HAVING COUNT(*) = 2
) m;                                                                  -- expect: Users merged 值
```

## 2. ID 对齐检查

```sql
-- 选一个能在三库找到的用户（按 name 或 email）
-- OAuth
SELECT id, name, email FROM users WHERE name = '鲲';

-- kungal
SELECT id, name, email FROM "user" WHERE name = '鲲';

-- moyu
SELECT id, name, email FROM "user" WHERE name = '鲲';
```

**期望**：三个 `id` 完全相同。

```sql
-- 抽样验证：OAuth 的 id == kungal 的 id（按相同 email 比对）
SELECT
  u.id          AS oauth_id,
  u.email       AS oauth_email,
  k.id          AS kungal_id
FROM users u
JOIN dblink('host=... dbname=kungalgame', 'SELECT id, email FROM "user"')
  AS k(id INT, email TEXT) USING (email)
WHERE u.id != k.id
LIMIT 10;
-- expect: 0 rows（即没有不对齐的）
```

如果你不想用 dblink，把数据导出到 csv 在外面比也行。

## 3. ID 时序顺序

```sql
SELECT id, name, created_at
FROM users
ORDER BY id ASC
LIMIT 10;
```

期望：`created_at` 单调递增（早注册的 id 小）。

```sql
-- 检查"创建时间倒序但 id 顺序"的违例数
SELECT COUNT(*)
FROM users a
JOIN users b ON a.id < b.id
WHERE a.created_at > b.created_at;
-- 0 → 完全单调
-- > 0 但很小 → 同秒注册的细微违例，不影响功能
```

## 4. FK 列重映射检查

抽几张表看是否引用的是新 ID：

```sql
-- kungal 端
-- 抽 1 行 galgame，看 user_id 是否能在 OAuth users 找到
SELECT g.user_id, u.name
FROM galgame g
JOIN dblink('host=... dbname=kun_oauth', 'SELECT id, name FROM users')
  AS u(id INT, name TEXT) ON u.id = g.user_id
LIMIT 5;
-- 应该返回有效行，user_id 都能 join 上
```

```sql
-- moyu 端
SELECT pr.user_id, u.name
FROM patch_resource pr
JOIN dblink('host=... dbname=kun_oauth', 'SELECT id, name FROM users')
  AS u(id INT, name TEXT) ON u.id = pr.user_id
LIMIT 5;
```

如果 join 不上，说明那一列没被 step 7 覆盖（应该列在 [03-id-unification.md](./03-id-unification.md) 里检查 / 或脚本漏了）。

## 5. 私聊房间名重写（kungal）

```sql
-- kungal 端
SELECT id, name FROM chat_room WHERE type = 'private' LIMIT 10;
```

每行的 `name` 应该是 `"<a>-<b>"` 格式（其中 a < b），且 a、b 都是新 ID。

```sql
-- 抽验：解析房间名两端 id，看是否在 OAuth users 里
WITH parsed AS (
  SELECT id,
         SPLIT_PART(name, '-', 1)::int AS uid1,
         SPLIT_PART(name, '-', 2)::int AS uid2
  FROM chat_room
  WHERE type = 'private'
)
SELECT p.id, p.uid1, p.uid2,
       (SELECT name FROM dblink('...', 'SELECT id, name FROM users') AS u(id INT, name TEXT) WHERE u.id = p.uid1) AS u1,
       (SELECT name FROM dblink('...', 'SELECT id, name FROM users') AS u(id INT, name TEXT) WHERE u.id = p.uid2) AS u2
FROM parsed p
LIMIT 5;
```

`u1`、`u2` 都该有值（不是 NULL）。

## 6. patch_comment.content 里的 mention URL（moyu）

```sql
-- moyu 端
SELECT id, content
FROM patch_comment
WHERE content ~ '/user/[0-9]+/'
LIMIT 5;
```

抽几条，把 URL 里的 `<id>` 拷贝出来去 OAuth users 查：

```sql
-- OAuth 端
SELECT id, name FROM users WHERE id IN (2, 30, 12345);  -- 用上面查出来的 id 替换
```

**期望**：所有 ID 都能找到对应人，且名字与 mention 里的 `[@<name>]` 显示名相符（或显示名是写入时的快照、可能滞后）。

## 7. 反查迁移前的原 ID（重点）

未来的迁移脚本（最典型的：image_service 头像迁移）经常需要"OAuth 用户 X 在 kungal/moyu 当时的 ID 是多少？"，因为旧的 CDN 路径里编码的是旧 ID：

```
https://image.kungal.com/avatar/user_30/avatar.webp
                              └── kungal 旧 user_id = 30
```

`user_migrations` 表就是这个反查的权威源：

```sql
-- 给定 OAuth 用户 X，查它在 moyu 当时的 ID
SELECT source_user_id
FROM user_migrations
WHERE user_id = X
  AND source_db = 'moyu';
```

```sql
-- 反向：给定 moyu 旧 ID，查它现在 OAuth 的 ID
SELECT user_id
FROM user_migrations
WHERE source_db = 'moyu'
  AND source_user_id = 30;
```

```sql
-- 跨站合并的人：哪些 OAuth 用户既来自 kungal 也来自 moyu
SELECT user_id, ARRAY_AGG(source_db) AS sources
FROM user_migrations
GROUP BY user_id
HAVING COUNT(*) = 2;
```

> 不需要在 kungal/moyu 的 user 表上加 `legacy_id` 列。`user_migrations` 是干净的、统一的、可索引的反查源。

## 8. 登录路径验证

最直接的端到端测试：

1. 找一个迁移过的用户（任意 source）
2. 用他原来在 kungal 或 moyu 的密码尝试登录新 OAuth `/api/v1/auth/login`
3. 应该登录成功
4. 第二次登录后查 `users.kungal_password` 和 `moyu_password` 应该都被清空（透明升级见 [04-password-migration.md](./04-password-migration.md)）

```sql
-- 升级是否在迁移
SELECT
  COUNT(*) FILTER (WHERE password IS NOT NULL)        AS upgraded,
  COUNT(*) FILTER (WHERE kungal_password IS NOT NULL) AS still_bcrypt,
  COUNT(*) FILTER (WHERE moyu_password   IS NOT NULL) AS still_moyu_argon
FROM users;
-- 刚迁完时 upgraded=0；随着用户登录数字会涨
```

## 9. OAuth flow 验证

跑完后用第三方站点做完整 OAuth 授权码流程：

1. 浏览器打开 `oauth.kungal.com/api/v1/oauth/authorize?...`
2. 用迁移过的用户登录
3. 走完授权码 → token 交换
4. 拿 access_token 调 `/oauth/userinfo` → 期望返回 `id`、`sub`、`name`、`roles` 等

## 10. 后续需要做的 Prisma schema 改动

跑完用户迁移后，源库还需要按 `prisma/moyu/MIGRATION_NOTES.md` 做的 schema 改动：

- 加 `oauth_account` 表（如果还没有）
- moyu 的 `text[]` 字段改 `jsonb`（GORM 兼容）
- 反归一计数字段 backfill

这些跟 user 迁移正交、按需做。
