# 架构：OAuth 作为身份单一源

> 写这个迁移的根本原因不是"清理多个站点的用户表"，而是把"用户身份"从业务表里**彻底剥离**。

## 1. 迁移之前：双独立用户表

```
                             ┌──────────────┐
   kungalgame.com  ──────────│ kungal.user  │── kungal.user.id  (1..67373)
                             └──────────────┘
                             ┌──────────────┐
   moyu.moe        ──────────│ moyu.user    │── moyu.user.id    (1..21286)
                             └──────────────┘
```

两个独立的 user 表，**ID 空间重叠但语义不同**。kungal 的 user 5 跟 moyu 的 user 5 是完全不同的人。同一邮箱可能两边各注册了一次，账号不互通。

任何一个跨站功能（OAuth 登录、统一头像、跨站 @ 提及…）都要先解决"这两个 user 5 是不是同一个人"的问题。

## 2. 迁移之后：单一身份源 + 站点本地特有数据

```
   ┌─────────────────────────────────────────────┐
   │  OAuth (oauth.kungal.com)                   │
   │                                             │
   │  users                ← 身份单一源（SoT）   │
   │   - id          int (PK)                    │
   │   - uuid        text (sub claim)            │
   │   - name        text                        │
   │   - email       text                        │
   │   - avatar      text                        │
   │   - bio         text                        │
   │   - status      int                         │
   │   - moemoepoint int                         │
   │   - kungal_password text  (legacy bcrypt)   │
   │   - moyu_password   text  (legacy argon2id) │
   │   - password        text  (新 argon2id)     │
   │                                             │
   │  user_roles      ← 全局权限（admin/moderator） │
   │  user_site_data  ← 每站独立的站点元数据     │
   │  user_migrations ← 迁移审计：原 ID ↔ 新 ID │
   └─────────────────────────────────────────────┘
              ▲                       ▲
              │                       │
   ┌──────────┴────────┐ ┌────────────┴──────────┐
   │ kungal.user       │ │ moyu.user             │
   │  - id (= OAuth)   │ │  - id (= OAuth)       │
   │  - daily_check_in │ │  - daily_check_in     │
   │  - daily_image_count│  - daily_image_count  │
   │  - daily_toolset_…│ │  - daily_upload_size  │
   │  - moemoepoint    │ │  - moemoepoint        │
   │  - role           │ │  - role               │
   │  - status         │ │  - status             │
   │  - last_login_time│ │  - last_login_time    │
   │                   │ │                       │
   │  (站点特有计数器/ │ │  (站点特有计数器/     │
   │   行为状态)       │ │   行为状态)           │
   └───────────────────┘ └───────────────────────┘
```

**核心不变量**：三个 `user.id` 是同一个整数。OAuth 的 `users.id`、kungal 的 `user.id`、moyu 的 `user.id` 完全相同（迁移脚本 step 7 强制对齐）。

## 3. 字段归属表

| 字段类别 | 例子 | 归属 | 理由 |
|---------|------|------|------|
| **登录身份** | name, email, password | OAuth | 登录决策必须在身份提供方做 |
| **跨站展示** | avatar, bio | OAuth | 同一用户跨站看到统一头像/简介 |
| **强一致状态** | status（封号）, roles | OAuth | 一处封禁全站生效；权限不分站点 |
| **跨站计分** | moemoepoint | OAuth（合并求和） | 设计上视为统一身份的总积分 |
| **站点特有计数器** | daily_check_in, daily_image_count | 站点本地 | 每站独立活跃度，互不影响 |
| **站点特有功能** | daily_toolset_upload_count（kungal 工具集）, daily_upload_size（moyu 上传） | 站点本地 | 仅在该站点的功能里有意义 |
| **会话指纹** | ip, last_login_time | 站点本地（各站独记一份） | 每站独立的最近登录痕迹 |

详细的字段对照表见 [02-data-mapping.md](./02-data-mapping.md)。

## 4. 下游服务的消费模式

kungal/moyu/galgame_wiki 的后端**不再 SELECT 自己的 user 表来获取展示字段**（name、avatar、bio）。这些字段不在站点本地存。

需要展示用户时：

| 场景 | OAuth 端点 |
|------|-----------|
| 渲染评论列表（拿到一组 user_id，要批量解析为 brief） | `GET /users/batch?ids=1,2,3` |
| @ 提及自动补全 | `GET /users/search?q=kun` |
| 用户登录回调 | `GET /oauth/userinfo` |
| 单个用户查询 | 同 batch（传单元素 slice 即可） |

OAuth 这边**不发布 SDK 代码** —— API 是契约，每个 consumer 自己实现一个薄客户端。30 行可起步，按工作负载需要加 TTL 缓存 / singleflight / 分片。完整实现指南、可复用 Go 参考代码、各级升级标准见 [08-downstream-integration.md §4](./08-downstream-integration.md#4-客户端实现指南)。

## 5. 替代方案的对比（为什么不走另一条路）

### 方案 A：dual-write 同步

> "用户改 name 时，OAuth 同时写 OAuth.users + kungal.user + moyu.user"

**否决**。这是分布式系统经典反模式。失败模式：

- OAuth commit 成功、kungal sync 失败 → 三库永久漂移，需要对账 job 兜底
- name 改重了（kungal 已经有人占了）→ 部分成功无法干净回滚
- 头像 URL 缓存延迟 → 旧 URL 仍渲染
- 永远要维护异步重试 + 死信队列 + 周期对账
- 每加一个共享字段都要三处 schema + 同步逻辑

### 方案 B：完全删除站点 user 表

> "kungal/moyu 不要 user 表，所有业务表的 user_id 直接是 OAuth 的"

**也否决**。这其实是"完全 SoT"理论极限，但落地成本高：

- 站点特有字段（daily_*、moemoepoint、role）必须有地方放 —— 没了 user 表得新建一张
- 既有代码大量 JOIN user 表，要改的地方比想象多
- 没什么显著好处 —— 反正 user.id 已经对齐了，留个 user 表当本站特有数据的容器没坏处

### 方案 C：单一身份源 + 站点 user 表瘦身（采用）

- OAuth 持有所有"身份字段"（name / email / avatar / bio / status / role / 跨站积分）
- 站点 user 表保留，**只剩站点特有字段**（daily_*、last_login_time、role 副本）
- 站点 `user.id` 与 OAuth `users.id` 完全对齐（这次迁移的核心交付物）
- 渲染需要的展示字段，调用 `/users/batch` 等 API 即时拉取

**这是当前实现。** 它在"完全 SoT 的整洁性"和"既有代码的迁移成本"之间取得了平衡。

## 6. 这个架构的边界与未来

**当前能做的**：

- ✓ 改 name/avatar/bio/email/password —— 在 OAuth 改一处全站生效
- ✓ 封号 —— OAuth status 改了，kungal/moyu 拉到的 brief 里就是封禁状态
- ✓ 跨站 @ 提及 —— 只要把 user_id 存对了就能跨站解析名字

**当前还需要协调的**：

- 头像 URL 仍然是绝对编码（`https://image.kungal.com/avatar/user_30/...`），用户改头像需要 image_service 迁移之后才能彻底解决（详见 image_service 相关文档）

**长期演进**：

- kungal/moyu 的 user 表里站点特有字段如果以后能搬到 OAuth `user_site_data` 表（已经预留），可以进一步把 user 表删掉
- 当前不做，因为站点本地查 user_site_data 等于跨库 RPC，没有性能优势
