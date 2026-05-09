# ID 统一：算法与覆盖范围

> step 7 的全部细节。这是整个迁移的技术核心。

## 1. 问题陈述

迁移前：

- kungal `user.id`：1..67373
- moyu `user.id`：1..21286（**与 kungal 重叠**）

迁移后想要：

- OAuth `users.id`：1..N（合并去重后的总用户数）
- kungal `user.id` == moyu `user.id` == OAuth `users.id`（**三库完全对齐**）

如果只在 OAuth 端建好新 ID（1..N）但不动 kungal/moyu，那 kungal 业务表里 `galgame.user_id = 5` 还是指向 kungal 旧的 user 5（已经被合并走的那个旧记录）。**所有 FK 失效。**

所以脚本必须：

1. 在 OAuth 端按时序生成新 ID（step 4）
2. 拿这个映射回去改 kungal 库的 `user.id` + 所有 FK 列（step 7 的一半）
3. 同样改 moyu 库（step 7 的另一半）

## 2. 时序 ID 分配（step 4）

合并去重后的所有用户按 `created_at` 升序排列，依次分配 ID：

```
created_at 最早 → id=1
次早            → id=2
...
最晚            → id=N
```

为什么是时序：

- kungal 用户群体在意"我的 ID 多少"
- 时序 ID 让"老用户拿到小 ID"在跨站后仍成立
- 不可能重复（按时间戳唯一）

实现：合并完之后 `sort.Slice(allMerged, byCreatedAt)`，然后 `nextID := maxExistingID + 1` 起算（支持库里已经有人的情况）。

## 3. 两阶段 offset 算法

### 为什么需要

直接 UPDATE 会撞 PK：

```
old_id=5 → new_id=2     ✓ (UPDATE user SET id=2 WHERE id=5)
old_id=2 → new_id=8     ← UPDATE 时 PK=2 已经被刚改完的 5→2 占了
```

### 解法：先全平移到非冲突区间

```
const offset = 100_000_000  // 远大于实际用户数（< 100k）

-- Pass 1：所有目标 ID 加 offset
UPDATE "user"   SET id = id + 100000000 FROM _id_map WHERE id = old_id;
UPDATE galgame  SET user_id = user_id + 100000000 FROM _id_map WHERE user_id = old_id;
... (51 张表 / 18 张表)

-- Pass 2：从 +offset 区间映射到最终 new_id
UPDATE "user"   SET id = m.new_id FROM _id_map m WHERE id = m.old_id + 100000000;
UPDATE galgame  SET user_id = m.new_id FROM _id_map m WHERE user_id = m.old_id + 100000000;
... (51 张表 / 18 张表)
```

`+100M` 区间不可能跟实际 ID 撞（kungal/moyu 用户都不到 100k），所以 Pass 1 之后所有目标行都在"安全区"，Pass 2 写入最终值时 PK 永远空着可用。

### 为什么所有用户都得加进映射

即使某用户的 `old_id == new_id`，也必须放进 `_id_map` 走两轮平移：

```
- userA: old=3, new=3   (没在 _id_map 里 → Pass 1 不动 → 仍占 id=3)
- userB: old=5, new=3   (在 _id_map 里 → Pass 1 → 100000005 → Pass 2 想写 id=3 时撞 userA)
```

所以 `sourceToNewID` 包含全部迁移过的用户，不论 ID 是否变化。

### 为什么 DISABLE TRIGGER 而不是 SET CONSTRAINTS DEFERRED

PostgreSQL 的 `SET CONSTRAINTS ALL DEFERRED` 只对 `DEFERRABLE` 声明的约束生效。Prisma 默认生成的约束都不是 `DEFERRABLE`，所以这条路走不通。

`ALTER TABLE … DISABLE TRIGGER ALL` 整体禁用 FK 约束触发器：

- 范围是事务内有效（事务结束自动恢复）
- 不需要超级用户权限（只需表 owner，与 `SET session_replication_role = 'replica'` 不同）
- rollback 自动恢复触发器状态

## 4. step 7 全过程的事务结构

每个源库一个事务：

```
BEGIN
  ALTER TABLE … DISABLE TRIGGER ALL    (kungal 30+ 张表 / moyu 15+ 张表)
  CREATE TEMP TABLE _id_map (
    old_id INT PRIMARY KEY,
    new_id INT NOT NULL
  ) ON COMMIT DROP
  INSERT INTO _id_map VALUES …          (灌入 sourceToNewID 全部映射)

  -- Pass 1
  UPDATE 51/18 个 FK 列 SET col += 100M FROM _id_map WHERE col = old_id
  UPDATE "user"        SET id  += 100M FROM _id_map WHERE id  = old_id

  -- Pass 2
  UPDATE 51/18 个 FK 列 SET col = new_id FROM _id_map WHERE col = old_id + 100M
  UPDATE "user"        SET id  = new_id FROM _id_map WHERE id  = old_id + 100M

  -- 特殊处理
  remapChatRoomNames()                  (kungal 私聊房间名 "uid1-uid2" 重算)
  rewriteMentionsInContent()            (moyu patch_comment.content 里的 mention URL 重写)

  -- 序列重置
  SELECT setval(pg_get_serial_sequence('"user"', 'id'), MAX(id))

  ALTER TABLE … ENABLE TRIGGER ALL      (defer 内执行；rollback 自动 undo)
COMMIT
```

任一步失败 → 整体 ROLLBACK，源库回到执行前状态。

## 5. kungal 重映射的 51 个 FK 列

```
chat_room.last_message_sender_id
chat_room_participant.user_id, chat_room_admin.user_id
chat_message.sender_id, chat_message.receiver_id
chat_message_read_by.user_id, chat_message_reaction.user_id
doc_article.author_id

galgame.user_id
galgame_rating.user_id, galgame_rating_like.user_id
galgame_rating_comment.user_id, galgame_rating_comment.target_user_id
galgame_comment.user_id, galgame_comment.target_user_id
galgame_comment_like.user_id, galgame_contributor.user_id
galgame_like.user_id, galgame_favorite.user_id
galgame_history.user_id, galgame_link.user_id, galgame_pr.user_id
galgame_resource.user_id, galgame_resource_like.user_id
galgame_toolset.user_id, galgame_toolset_contributor.user_id
galgame_toolset_practicality.user_id, galgame_toolset_resource.user_id
galgame_toolset_comment.user_id
galgame_website.user_id, galgame_website_comment.user_id
galgame_website_like.user_id, galgame_website_favorite.user_id

message.sender_id, message.receiver_id
system_message.user_id

topic.user_id
topic_comment.user_id, topic_comment.target_user_id, topic_comment_like.user_id
topic_poll.user_id, topic_poll_vote.user_id
topic_reply.user_id, topic_reply_like.user_id, topic_reply_dislike.user_id
topic_upvote.user_id, topic_like.user_id, topic_dislike.user_id, topic_favorite.user_id

todo.user_id, update_log.user_id, unmoe.user_id

user_friend.user_id, user_friend.friend_id
user_follow.follower_id, user_follow.followed_id

oauth_account.user_id
```

完整清单审核过 `prisma/kungal/*.prisma` 里所有 `references: [user.id]`，**100% 覆盖**。

## 6. moyu 重映射的 18 个 FK 列

```
chat_member.user_id
chat_message.sender_id, chat_message.deleted_by_id
chat_message_seen.user_id, chat_message_reaction.user_id

patch.user_id, patch_resource.user_id, patch_comment.user_id

admin_log.user_id

user_follow_relation.follower_id, user_follow_relation.following_id
user_message.sender_id, user_message.recipient_id

user_patch_favorite_relation.user_id
user_patch_contribute_relation.user_id
user_patch_comment_like_relation.user_id
user_patch_resource_like_relation.user_id

oauth_account.user_id
```

同样审核过 `prisma/moyu/*.prisma`，**100% 覆盖**。

## 7. 特殊处理 1：chat_room.name（kungal）

kungal 的私聊房间用一种特殊的 `name` 命名约定：`"<uid_min>-<uid_max>"`（升序）。比如 user 5 跟 user 30 的私聊房间 name = `"5-30"`。

user_id 重映射后这串必须重算：

```sql
UPDATE chat_room SET name =
  LEAST(m1.new_id, m2.new_id) || '-' || GREATEST(m1.new_id, m2.new_id)
FROM _id_map m1, _id_map m2
WHERE SPLIT_PART(name, '-', 1)::int = m1.old_id + 100000000
  AND SPLIT_PART(name, '-', 2)::int = m2.old_id + 100000000
  AND type = 'private';
```

只动 `type='private'` 房间。群聊房间名格式不一样，不参与。

## 8. 特殊处理 2：patch_comment.content 里的 mention URL（moyu）

moyu 评论内容里有 mention：

```markdown
[@鲲](/user/30/resource) 你好
```

URL 里的 `30` 是 moyu 的旧 user_id。重映射后必须改成对齐后的新 ID。

格式严格固定：`[@<显示名>](/user/<id>/<path>)`。Regex：`/user/(\d+)/`。

### 为什么这里也需要两阶段 offset

同一行可能引用多个用户，他们的 ID 之间可能存在循环依赖：

```
mapping: { 5→2, 2→8 }
content: "[@A](/user/5/x) 和 [@B](/user/2/y)"
```

朴素单趟会先把 `5` 改成 `2`，然后扫到刚改完的"`2`"会再改成 `8` —— A 的 mention 被搞坏。

两阶段做法：

- Pass 1：把 `/user/<old_id>/` 改成 `/user/<old_id + 100M>/`（regex 替换，跟 _id_map 走查找）
- Pass 2：把 `/user/<old_id + 100M>/` 改成 `/user/<new_id>/`

跟 FK remap 用同一 transaction、同一 `_id_map` 表、同一 offset。整体原子。

显示名 `<鲲>` 是写入时的快照，不重写 —— 这是所有社交平台对 mention 的标准行为。

## 9. 序列重置

step 7 末尾：

```sql
SELECT setval(pg_get_serial_sequence('"user"', 'id'),
              (SELECT COALESCE(MAX(id), 1) FROM "user"));
```

防止下一次 INSERT 撞已有 ID（INSERT 不显式给 id 时会从序列取下一个值）。

## 10. 这些事 step 7 不做

- 不动头像 URL（`https://image.kungal.com/avatar/user_30/avatar.webp` 里的 `30` 不会被重写）。这部分留给未来的 image_service 迁移，由它通过 `user_migrations` 表反查原 ID 后重新下载。
- 不动 kungal 的 topic / topic_comment / topic_reply / galgame_comment / chat_message / doc_article 内容字段。原因：当前确认 kungal 这些字段**没有** `[@x](/user/<id>/...)` 格式的 mention。如果后来发现有，加一行 `mentionField` 即可（已经有现成的代码框架）。

## 11. 复杂度

- kungal 50 张表 × 平均行数 → 实测 step 7 在 70k 用户、30M 行级 FK 总量下大约 **8-15 分钟**
- moyu 15 张表 × 平均行数 → 实测大约 **2-5 分钟**
- 整个 step 7 期间 kungal/moyu 必须不能有写入（trigger 关掉了）
