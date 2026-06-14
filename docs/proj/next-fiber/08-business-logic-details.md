# 业务逻辑详解

本文档详细记录 `apps/next-server/` 中的核心业务逻辑，确保迁移到 Go 时不遗漏任何细节。

## 1. 萌萌点（Moemoepoint）系统

萌萌点是站内积分，用于激励用户参与。

### 获取途径

| 操作 | 萌萌点变化 | 触发条件 |
|------|-----------|---------|
| 注册 | +3 | 新用户注册 |
| 每日签到 | +0~7（随机） | 每日首次签到 |
| 创建补丁 | +3 | 发布新补丁 |
| 上传资源 | +3 | 发布新资源 |
| 被评论 | +1 | 他人评论你的补丁 |
| 评论被点赞 | +1 | 他人点赞你的评论 |
| 补丁被收藏 | +1 | 他人收藏你的补丁 |
| 资源被点赞 | +1 | 他人点赞你的资源 |

### 扣除途径

| 操作 | 萌萌点变化 | 触发条件 |
|------|-----------|---------|
| 修改用户名 | -30 | 每次修改 |
| 删除资源 | -3 | 删除自己的资源 |
| 取消点赞评论 | -1 | 取消对他人评论的点赞 |
| 取消收藏补丁 | -1 | 取消收藏他人补丁 |
| 取消点赞资源 | -1 | 取消点赞他人资源 |

### 实现注意

- 萌萌点变化是给**被操作者**而非操作者（收藏补丁时，补丁创建者获得 +1）
- 防止自赞：点赞自己的评论/资源/补丁不触发萌萌点变化
- 签到随机数范围：`Math.floor(Math.random() * 8)` → 0-7

---

## 2. 消息通知系统

### 消息类型

```typescript
type MessageType =
  | 'mention'               // @提及
  | 'comment'               // 评论你的补丁
  | 'likeComment'           // 点赞你的评论
  | 'favorite'              // 收藏你的补丁
  | 'likeResource'          // 点赞你的资源
  | 'patchResourceCreate'   // 你收藏的补丁有新资源
  | 'patchResourceUpdate'   // 你点赞的资源有更新
  | 'follow'                // 关注你
  | 'apply'                 // 创作者申请结果
```

### 消息创建规则

**去重消息**（`createDedupMessage`）：相同 sender + recipient + type + link 只保留一条：
- `likeComment`：A 点赞 B 的评论，link 相同只记录一次
- `favorite`：A 收藏 B 的补丁，link 相同只记录一次
- `likeResource`：A 点赞 B 的资源
- `follow`：A 关注 B
- `comment`：A 评论 B 的补丁下的某个评论（link 指向具体评论）

**普通消息**（`createMessage`）：
- `mention`：@提及不去重（不同评论中的提及是不同事件）
- `patchResourceCreate`：通知所有收藏该补丁的用户
- `patchResourceUpdate`：通知所有点赞该资源的用户
- `apply`：管理员操作结果

### @提及检测

从评论 Markdown 内容中提取提及：
```
匹配格式: [@username](/user/{uid}/resource)
正则: /\[@([^\]]+)\]\(\/user\/(\d+)\/resource\)/g
```

提取到用户 ID 后，为每个被提及的用户创建 mention 消息，content 为评论前 233 字符。

---

## 3. 补丁资源管理

### 资源存储类型

资源的 `storage` 字段标识存储方式：
- `s3`：文件存储在 S3
- 其他值（如 `mega`, `onedrive`, `baidu` 等）：外部链接

### 创建资源流程（D10）

前端已通过 presigned URL 直传完成文件到 S3，调用此端点只做 DB 落盘：

```
1. 验证用户已登录且有权限
2. 如果是 S3 资源：
   a. 前端传来 s3_key（完整 S3 对象键）
   b. 服务端 HeadObject(s3_key) 确认对象存在、size 匹配、在限额内
      - 不通过 → DeleteObject(s3_key) 清理 + 返回错误
3. 创建 patch_resource 记录（blake3 = ""，s3_key 填充）（事务内）
4. 更新 patch 的聚合字段（type, language, platform 去重合并）
5. 创建 contributor 关系（如果是新贡献者）
6. 给上传者 +3 萌萌点
7. 扣减 daily_upload_size
8. 通知所有收藏该补丁的用户（patchResourceCreate 消息）
9. 更新 patch.resource_count +1
10. 更新 patch.resource_update_time
```

### 更新资源流程（D10）

```
1. 验证是资源创建者
2. 如果更换了文件：
   a. 前端用新 s3_key 完成直传
   b. 服务端 HeadObject 验证
   c. DeleteObject(old s3_key)
   d. patch_resource.s3_key 更新为新 key
3. 更新 patch_resource 记录
4. 重新计算 patch 的聚合字段（type, language, platform）
5. 通知所有点赞该资源的用户（patchResourceUpdate 消息）
```

### 删除资源流程（D10）

```
1. 验证是资源创建者
2. 如果是 S3 资源：DeleteObject(s3_key)  ← 直接用 DB 里的 s3_key，不再拼路径
3. 删除 patch_resource 记录
4. 重新计算 patch 的 type 聚合字段
5. 给删除者 -3 萌萌点
6. 更新 patch.resource_count -1
```

### 补丁聚合字段更新

补丁的 `type`, `language`, `platform` 是所有资源值的去重合集：

```typescript
// 创建/更新资源后
const allResources = await prisma.patch_resource.findMany({
  where: { patch_id: patchId },
  select: { type: true, language: true, platform: true }
})

const mergedType = [...new Set(allResources.flatMap(r => r.type))]
const mergedLanguage = [...new Set(allResources.flatMap(r => r.language))]
const mergedPlatform = [...new Set(allResources.flatMap(r => r.platform))]

await prisma.patch.update({
  where: { id: patchId },
  data: { type: mergedType, language: mergedLanguage, platform: mergedPlatform }
})
```

---

## 4. 补丁评论系统

### 评论嵌套

- 支持一层嵌套：`parent_id` 指向父评论
- 顶层评论：`parent_id = null`
- 回复：`parent_id = 父评论ID`

### 评论创建流程

```
1. 验证用户已登录
2. 如果管理后台开启了评论验证：验证 CAPTCHA
3. 创建 patch_comment 记录
4. 检测 @提及 → 创建 mention 消息
5. 如果是回复（parent_id 存在）：给父评论作者发 comment 消息
6. 给补丁创建者 +1 萌萌点
7. 更新 patch.comment_count +1
```

### 评论删除流程

```
1. 验证是评论作者或管理员（role >= 3）
2. 统计该评论及其所有子评论数量
3. 删除评论（数据库级联删除子评论）
4. 更新 patch.comment_count -= 删除数量
```

### 评论点赞

```
1. 检查是否已点赞（查 user_patch_comment_like_relation）
2. 如果已点赞 → 取消点赞（删除关系，like_count -1，被赞者 -1 萌萌点）
3. 如果未点赞 → 点赞（创建关系，like_count +1，被赞者 +1 萌萌点）
4. 防止自赞（user_id == comment.user_id 时不操作萌萌点）
5. 点赞时发去重消息 likeComment
```

---

## 5. 关注系统

### 关注流程

```
1. 验证不能关注自己
2. 创建 user_follow_relation（follower_id=我, following_id=目标）
3. 目标用户 follower_count +1
4. 我的 following_count +1
5. 发去重消息 follow
```

### 取消关注

```
1. 删除 user_follow_relation
2. 目标用户 follower_count -1
3. 我的 following_count -1
```

### 关注状态查询

获取用户列表时，如果当前用户已登录，需要返回每个用户相对于当前用户的关注状态（`isFollowed`）。

---

## 6. 每日限额系统

### 每日图片限额

- `daily_image_count`：今日已上传图片数
- 限额通过环境变量 `KUN_PATCH_USER_DAILY_UPLOAD_IMAGE_LIMIT` 配置
- 上传头像、横幅、用户图片时 +1
- 每日午夜重置为 0

### 每日上传限额

- `daily_upload_size`：今日已上传资源大小（字节）
- 每次上传分块合并后累加
- 不同角色可能有不同限额
- 每日午夜重置为 0

### 每日签到

- `daily_check_in`：0=未签到，1=已签到
- 签到获取 0-7 随机萌萌点
- 每日午夜重置为 0

---

## 7. 管理后台

### 管理操作日志

所有管理操作创建 `admin_log` 记录：
```typescript
await prisma.admin_log.create({
  data: {
    type: 'deleteComment',
    content: JSON.stringify({ commentId, reason }),
    user_id: adminUid
  }
})
```

### 用户封禁

删除用户（role >= 4 才可操作）：
```
1. 将用户邮箱和 IP 存入 Redis（永久封禁）
2. 删除用户所有 S3 资源文件
3. 删除用户记录（数据库级联删除关联数据）
4. 删除用户 Token
5. 创建管理日志
```

### 管理统计

```typescript
// GET /api/admin/stats?days=7
{
  newUser: number,           // 新注册用户数
  newActiveUser: number,     // 新活跃用户数（有登录时间记录）
  newGalgame: number,        // 新创建补丁数
  newPatchResource: number,  // 新上传资源数
  newComment: number         // 新评论数
}

// GET /api/admin/stats/sum
{
  userCount: number,
  galgameCount: number,
  patchResourceCount: number,
  patchCommentCount: number
}
```

---

## 8. 创作者申请

### 申请条件

- 当前角色 < 2（非创作者）
- 已上传至少 3 个资源
- 没有待审核的申请

### 申请流程

```
1. 检查申请条件
2. 创建 user_message（type='apply', status=0, content=申请信息）
3. 管理员在后台审核
4. 批准：设置 user.role=2, message.status=2, 发 apply 消息
5. 拒绝：message.status=3, 发 apply 消息（含拒绝原因）
```

---

## 9. 补丁创建与同步

### 创建补丁流程

```
1. 检查管理员是否开启了 "仅创作者创建"
2. 解析 FormData（banner 文件 + 其他字段）
3. 上传 banner 到 S3（1920x1080 + 460x259 缩略图）
4. 解析 alias 字符串为数组
5. 创建 patch 记录
6. 批量创建 patch_alias 记录
7. 创建 contributor 关系
8. 给创建者 +3 萌萌点, +1 daily_image_count
9. 触发 VNDB 同步（如果提供了 vndb_id）
```

### VNDB 同步（edit/sync）

> **变更（2026-04-21）**：VNDB 同步仅保留**标签**和**公司**两类本地数据的同步；封面、截图、角色、人物、发售信息全部由 Galgame Wiki Service 统一管理，本项目不再复制落盘。

当补丁有 `vndb_id` 时，本地只同步以下数据：

```
1. 标签（patch_tag + patch_tag_relation）
2. 公司（patch_company + patch_company_relation）
```

~~封面图（patch_cover）~~、~~截图（patch_screenshot）~~、~~角色（patch_char 等）~~、~~人物/声优（patch_person 等）~~、~~发售信息（patch_release）~~ — 这些前端直接/通过代理向 Galgame Wiki 查询，以 `patch.vndb_id` 为键。

同步前会清理本地的 tag/company 关系，然后重新导入。

---

## 10. 聊天系统业务逻辑

### 私聊

- 链接格式：`{min(uid1,uid2)}-{max(uid1,uid2)}`
- 首次消息时自动创建 chat_room + chat_member
- 房间类型：`PRIVATE`

### 群聊

- 仅 role >= 4 可创建
- 创建者角色为 `OWNER`，其他成员为 `MEMBER`
- 支持通过链接加入

### 消息操作（D9：全部 REST，无实时）

- 发送消息：`POST /chat/room/:link/message`。创建 chat_message + markdown→HTML 渲染 + 更新 chat_room.last_message_time
- 编辑消息：`PUT /chat/message/:id`。保存旧内容到 chat_message_edit_history + 更新 content + status='EDITED'
- 删除消息：`DELETE /chat/message/:id`。仅发送者或 role >= 3 可删除 + 标记 status='DELETED' + deleted_at + deleted_by_id
- 表情回应：`POST /chat/message/:id/reaction`。toggle（相同 user+message+emoji 唯一约束）
- 已读标记：`PUT /chat/room/:link/seen`。批量插入 chat_message_seen（user+message 唯一约束）

**废弃**：打字指示、在线状态、实时推送。前端 3s 轮询 `GET /chat/room/:link/message?after=lastMsgId` 只拉新消息；老消息的编辑/删除/表情变化**不同步**，刷新页面才看到（详见 09 D9 Q11=C）。

---

## 11. 首页数据

```
GET /api/home 返回：
{
  galgameCards: 最新 12 个补丁（含 user, counts, 标签, 封面）
  resources: 最新 6 个资源（含 user, patch 基本信息）
  comments: 最新 6 条评论（含 user, patch_id, 截取内容）
}
```

所有数据遵循 NSFW 过滤。

---

## 12. Hikari 外部 API

### CORS 白名单

```
hikarinagi.com, *.hikarinagi.com
shionlib.com, *.shionlib.com
touchgal.io, *.touchgal.io
touchgal.net, *.touchgal.net
localhost:*
```

### 限流

- 10000 次/分钟/IP+Origin 组合
- 使用 Redis 计数器实现

### 响应格式

返回补丁信息 + 资源列表，但**不包含 S3 下载链接**（安全考虑）：
- 资源的 `content` 字段被清空
- 仅返回外部链接类型的资源信息
