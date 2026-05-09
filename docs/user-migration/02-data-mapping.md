# 字段映射

每一列从源数据库到 OAuth target 的去向。

## 1. kungal.user → OAuth

| kungal.user 列 | OAuth 端 | 备注 |
|---------------|---------|------|
| `id` | `users.id` | step 7 重映射；新值按 `created_at` 时序分配 |
| `name` | `users.name` | 名字撞了 → append `_1` `_2` |
| `email` | `users.email` | 全小写、去空白后用作合并键 |
| `password` | `users.kungal_password` | bcrypt hash 保留；`users.password` 设 NULL |
| `avatar` | `users.avatar` | 直接复制（跨站合并时 kungal 优先） |
| `bio` | `users.bio` | 同上 |
| `role` | `user_site_data.role` | 按 kungal site_id 独立保留 |
| `status` | `users.status` + `user_site_data.status` | 双写 |
| `moemoepoint` | `users.moemoepoint` | 跨站合并时与 moyu 求和 |
| `ip` | `users.ip` | 直接复制 |
| `daily_check_in` | `user_site_data.daily_check_in` | 按站点独立 |
| `daily_image_count` | `user_site_data.daily_image_count` | 按站点独立 |
| `daily_toolset_upload_count` | `user_site_data.extra` (JSON) | 站点特有，进 JSON |
| `created` | `users.created_at` | **决定时序 ID 顺序** |
| `updated` | `users.updated_at` | 直接复制 |

15 列全覆盖。

## 2. moyu.user → OAuth

| moyu.user 列 | OAuth 端 | 备注 |
|--------------|---------|------|
| `id` | `users.id` | step 7 重映射 |
| `name` | `users.name` | 跨站合并时 kungal 优先 |
| `email` | `users.email` | 用作合并键 |
| `password` | `users.moyu_password` | argon2id (`salt_hex:hash_hex`) 保留 |
| `avatar` | `users.avatar` | kungal 无值时用 moyu |
| `bio` | `users.bio` | kungal 无值时用 moyu |
| `role` | `user_site_data.role` | 按 moyu site_id 独立保留 |
| `status` | `users.status` + `user_site_data.status` | 双写 |
| `moemoepoint` | `users.moemoepoint` | 跨站合并时与 kungal 求和 |
| `ip` | `users.ip` | 直接复制 |
| `daily_check_in` | `user_site_data.daily_check_in` | 按站点独立 |
| `daily_image_count` | `user_site_data.daily_image_count` | 按站点独立 |
| `daily_upload_size` | `user_site_data.extra.daily_upload_size` | 站点特有，进 JSON |
| `last_login_time` | `user_site_data.extra.last_login_time` | 站点特有，进 JSON |
| `follower_count` | — | 反归一计数；迁移后从 `user_follow` 表关系重新计算 |
| `following_count` | — | 同上 |
| `created` | `users.created_at` | 决定时序 ID 顺序（与 kungal 比谁早） |
| `updated` | `users.updated_at` | 直接复制 |

16/18 列覆盖，2 列（反归一计数）按设计跳过。

## 3. 跨站邮箱合并

按 `LOWER(TRIM(email))` 合并：

```
kungal.user[id=5,  email=Kun@kungal.com] ─┐
                                          ├─ 合并 ─→ OAuth.users[id=2, email=kun@kungal.com]
moyu.user  [id=30, email=kun@kungal.com] ─┘                       (新分配的整数 ID)
```

合并规则：

| 字段 | 规则 |
|------|------|
| name / avatar / bio | **kungal 优先**（kungal 有就用 kungal 的）；kungal 无值时用 moyu 的 |
| email | 用 lowercased + trimmed 形式 |
| moemoepoint | **两站求和** |
| created_at | **取较早** |
| password | kungal_password + moyu_password **都保留** —— 登录时按用户记得的密码自动选验证算法 |
| status | 用 kungal 的（kungal 优先） |

## 4. 站内重复邮箱

如果 kungal 内部就有重复邮箱（一个用户在 kungal 注册了两次）：

- 保留 `created_at` **最早**那条
- 其他跳过，计入 `SkippedDuplicates` 统计

moyu 同理。

## 5. 用户名去重

OAuth `users.name` 有 unique 约束。合并后若两个用户撞名（比如 kungal 用户 A 和 moyu 用户 B 都叫 "kun"，但邮箱不同→不合并→两条独立记录）：

- 按合并顺序：先来的占用原名
- 后来的 `name` append `_1` `_2` …

合并顺序 = chronological（按 created_at），所以**老用户优先占名**。

## 6. user_migrations 审计表

每个被迁移的用户，每个源库**写一条审计记录**到 OAuth 的 `user_migrations` 表：

```
UserID       uint    -- OAuth users.id（新值）
UserUUID     string  -- 同上的 uuid
SourceDB     string  -- "kungal" 或 "moyu"
SourceUserID uint    -- 该用户在源库的原始 ID（重映射前）
SourceEmail  string  -- 该用户在源库登记的邮箱
MergedFrom   *string -- 若两站合并，记另一边来源；否则 NULL
```

- 跨站合并的用户 → 写两条（一条 `kungal`、一条 `moyu`，user_id 相同）
- 仅 kungal 来的用户 → 写一条 `kungal`
- 仅 moyu 来的用户 → 写一条 `moyu`

这张表是**反查迁移前 ID 的权威来源**。详见 [07-verification.md](./07-verification.md)。

## 7. 角色映射

站点的 `role` 整数值会同时映射到 OAuth 的全局角色（用于 JWT roles claim）：

| 来源 | 站点 role | 对应 OAuth role |
|------|----------|----------------|
| kungal | 3 | admin |
| kungal | 2 | moderator |
| moyu | 4 | admin (super admin) |
| moyu | 3 | moderator |
| 其他 | — | 默认 user |

跨站取**最高权限**：如果同一用户在 kungal 是 admin、在 moyu 是 user，那 OAuth 上他是 admin。

写入 `user_roles` 多对多表，`ON CONFLICT DO NOTHING` 幂等。

## 8. 哪些字段对应跳过、不被迁移

明确不迁移：

- moyu `user.follower_count` / `following_count`（反归一计数；迁移后由 follow 表关系重算）
- kungal `user_follow` 表自身（kungal 内部关注关系；不跨站迁移到 OAuth follow 表，因为 OAuth 这边已经从 moyu follow 表迁了一份）

详见 [03-id-unification.md](./03-id-unification.md) 的 step 5（仅迁 moyu follow）。
