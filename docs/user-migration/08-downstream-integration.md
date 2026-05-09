# 下游服务接入 OAuth 用户系统

> 给 kungal / moyu / galgame_wiki 后端开发看的接入指南。**核心原则**：迁移完毕后，所有用户展示字段（name、avatar、bio）都从 OAuth 拉取，本站不再持久化这些字段。

## 1. 背景与决策

迁移之前：每个站点有自己的 `user` 表，包含 name / avatar / bio 等字段。

迁移之后：

- OAuth `users` 表是**唯一的**身份字段持有者
- 站点的 user 表保留，但**只剩站点特有字段**（daily_check_in、moemoepoint、role、last_login_time 等）
- name / avatar / bio / status / email 这些都不在站点本地存

为什么这么设计：详见 [01-architecture.md](./01-architecture.md) 第 5 节"替代方案对比"。

## 2. 渲染管线的改造模式

### 改造前（站点本地有完整 user 表）

```sql
-- 渲染评论列表
SELECT
  c.id, c.content, c.created_at,
  u.id, u.name, u.avatar, u.bio
FROM galgame_comment c
JOIN "user" u ON u.id = c.user_id
WHERE c.galgame_id = ?;
```

### 改造后（OAuth 是唯一名 / 头像源）

```sql
-- 步骤 1: SELECT 只取 user_id
SELECT
  c.id, c.content, c.created_at, c.user_id
FROM galgame_comment c
WHERE c.galgame_id = ?;
```

```go
// 步骤 2: 收集所有 user_id（去重）
ids := []uint{}
for _, c := range comments {
    ids = append(ids, c.UserID)
}

// 步骤 3: 一次批量回拉
users, err := userClient.Users(ctx, ids)
// users[uid] → *UserBrief{ID, UUID, Name, Avatar, Bio, Status, Roles, ...}

// 步骤 4: 在响应 DTO 里拼装
type CommentResponse struct {
    ID        uint        `json:"id"`
    Content   string      `json:"content"`
    CreatedAt time.Time   `json:"created_at"`
    User      *UserBrief  `json:"user"`
}

resp := make([]CommentResponse, 0, len(comments))
for _, c := range comments {
    resp = append(resp, CommentResponse{
        ID:        c.ID,
        Content:   c.Content,
        CreatedAt: c.CreatedAt,
        User:      users[c.UserID],
    })
}
```

**关键**：DB 查询不再 JOIN user 表（也根本没法 JOIN，那些字段不存在了）。展示字段全部走 SDK。

## 3. OAuth 用户 API

### 3.1 `GET /users/batch` —— 批量拉用户 brief

适用场景：渲染列表、需要把一组 user_id 解析成展示字段。

```http
GET /api/v1/users/batch?ids=1,2,3
Authorization: Basic base64(client_id:client_secret)
```

响应：

```json
{
  "code": 0,
  "data": {
    "users": [
      { "id": 1, "uuid": "...", "name": "kun", "avatar": "...", "bio": "...", "status": 0, "roles": ["admin"] }
    ],
    "not_found": [99]
  }
}
```

- 单次最多 100 个 ID
- 鉴权是 OAuth Client Basic Auth（client_id + client_secret），不是终端用户 JWT —— 这是服务到服务调用
- 响应**不含** email / moemoepoint / created_at 等隐私 / 非展示字段

### 3.2 `GET /users/search` —— 按名字搜索

适用场景：@ 提及自动补全、用户搜索框、管理后台用户查找。

```http
GET /api/v1/users/search?q=kun&limit=10
Authorization: Basic base64(client_id:client_secret)
```

响应（按相关度排序：精确 > 前缀 > 子串，每档内字母序）：

```json
{
  "code": 0,
  "data": {
    "users": [
      { "id": 1, "uuid": "...", "name": "kun", ... },
      { "id": 5894, "uuid": "...", "name": "kun123", ... }
    ]
  }
}
```

- `q`：1..50 字符；`%` `_` `\` 等 LIKE 通配符按字面匹配（已转义）
- `limit`：默认 20，封顶 50
- 同样 Basic Auth
- 同样不含隐私字段

### 3.3 `GET /oauth/userinfo` —— 当前登录用户信息

适用场景：OAuth callback 中拿用户身份；前端 /me 端点。

```http
GET /api/v1/oauth/userinfo
Authorization: Bearer <access_token>
```

响应：

```json
{
  "code": 0,
  "data": {
    "id": 12345,
    "sub": "550e8400-e29b-41d4-a716-446655440000",
    "name": "KUN",
    "email": "kun@kungal.com",
    "picture": "...",
    "roles": ["user", "admin"],
    "updated_at": 1234567890
  }
}
```

- `id`：integer，与 OAuth `users.id` 一致 —— **本地 user 表的 PK 应该用这个**
- `sub`：UUID，OIDC 标准的 subject
- `id` 和 `sub` 都标识同一用户，业务后端任选其一
- `roles` 与 JWT roles claim 一致
- name / email / picture 受 OIDC scope 控制；`id` / `sub` / `roles` 始终返回

### 3.4 `PATCH /auth/me` —— 自助修改 name / avatar / bio

适用场景：站点要让用户改 name / avatar / bio，但这些字段在 OAuth 端持有，所以最终修改都要落到 OAuth。

```http
PATCH /api/v1/auth/me
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "newname",            // 可选，2..17 字符，全局唯一
  "avatar": "https://...",       // 可选，≤255 字符（legacy URL）
  "avatar_image_hash": "abc...", // 可选，≤64 字符（image_service 哈希，优先于 avatar）
  "bio": "..."                   // 可选，≤107 字符
}
```

字段都是**指针语义**：没传则不动，传了就改（含传空字符串 = 清空）。

**注意**：

- 鉴权用**终端用户 JWT**（Bearer access_token），**不是** OAuth Client Basic Auth
- 改 email / password 不在这里 —— 走专门的两步验证流程（`/auth/email/send-code` + `/auth/email` / `/auth/password`）

**两种典型用法**：

```
方法 A（推荐）：跳转到 OAuth 前端
  站点点"改头像" → 302 redirect → oauth.kungal.com/profile

方法 B：站点代理
  PATCH /api/user/avatar （站点 endpoint）
    ↓
  站点后端调 PATCH https://oauth.kungal.com/api/v1/auth/me
    带上当前用户的 access_token
    ↓
  OAuth 改完，站点返回成功 + 主动失效本地的用户缓存（如果有）
```

注意：PATCH /auth/me 用**终端用户 JWT**（access_token），跟批量/搜索端点用的 OAuth Client Basic Auth 不一样。不能复用同一个 client。

## 4. 客户端实现指南

> **OAuth 这边不提供 SDK 代码**。API 文档（`docs/integration/oauth/api-reference.md`）是契约的唯一来源。每个 consumer 服务自己实现一个薄客户端 —— 通常是 30-150 行 Go 代码，按工作负载需要的复杂度选实现层级。

### 4.1 为什么自己写

- **API 是契约，不是 SDK 代码**。HTTP + JSON 任何语言都能调；强行共享 Go 代码会把 Go-only 实现细节绑进契约。
- **每个服务的负载特征不同**：高并发渲染需要 singleflight，单线程脚本不需要；批量大需要分片，每次只查一两个不需要。让 consumer 按需要选层级，比一刀切的"功能齐全 SDK"更合身。
- **代码不跨仓库飘移**。SDK 复制到 N 个仓库之后，API 演化时这 N 份代码各自漂；自己写的客户端，只针对自己业务的本仓库代码，演化清晰。

### 4.2 L1：最小实现（30-50 行，无缓存）

适用：脚本工具、低 QPS 的后台任务、原型验证、快速接入。

```go
// userclient.go - L1 最小实现
package userclient

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"
)

type Brief struct {
    ID     uint     `json:"id"`
    UUID   string   `json:"uuid"`
    Name   string   `json:"name"`
    Avatar string   `json:"avatar"`
    Bio    string   `json:"bio"`
    Status int      `json:"status"`
    Roles  []string `json:"roles"`
}

type Client struct {
    baseURL    string
    authHeader string
    http       *http.Client
}

func New(baseURL, clientID, clientSecret string) *Client {
    creds := clientID + ":" + clientSecret
    return &Client{
        baseURL:    strings.TrimRight(baseURL, "/"),
        authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte(creds)),
        http:       &http.Client{Timeout: 5 * time.Second},
    }
}

func (c *Client) Users(ctx context.Context, ids []uint) (map[uint]*Brief, error) {
    if len(ids) == 0 {
        return map[uint]*Brief{}, nil
    }
    parts := make([]string, len(ids))
    for i, id := range ids {
        parts[i] = strconv.FormatUint(uint64(id), 10)
    }
    url := fmt.Sprintf("%s/users/batch?ids=%s", c.baseURL, strings.Join(parts, ","))
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", c.authHeader)
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("oauth: status=%d", resp.StatusCode)
    }
    var env struct {
        Code int `json:"code"`
        Data struct {
            Users []Brief `json:"users"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
        return nil, err
    }
    if env.Code != 0 {
        return nil, fmt.Errorf("oauth: code=%d", env.Code)
    }
    out := make(map[uint]*Brief, len(env.Data.Users))
    for i := range env.Data.Users {
        u := env.Data.Users[i]
        out[u.ID] = &u
    }
    return out, nil
}
```

用法：

```go
cli := userclient.New(
    "https://oauth.kungal.com/api/v1",
    os.Getenv("OAUTH_CLIENT_ID"),
    os.Getenv("OAUTH_CLIENT_SECRET"),
)
users, err := cli.Users(ctx, []uint{1, 2, 3})
```

**什么时候要升级到 L2**：每秒 > 几十次相同 ID 的查询 / 评论列表渲染会拿到大量重复 user_id / OAuth 端响应延迟开始影响 P99。

### 4.3 L2：加 TTL 缓存（+30 行）

在 L1 基础上加进程内 TTL 缓存。命中后省一次 HTTP 往返。

关键改动：

```go
type cacheEntry struct {
    brief   *Brief
    expires time.Time
}

type Client struct {
    // ... L1 fields
    cache    sync.Map      // uint → cacheEntry
    cacheTTL time.Duration // e.g. 10 * time.Minute
}

func (c *Client) Users(ctx context.Context, ids []uint) (map[uint]*Brief, error) {
    out := make(map[uint]*Brief, len(ids))
    missing := make([]uint, 0, len(ids))
    now := time.Now()

    // 1. 命中缓存的直接放进 out
    for _, id := range ids {
        if v, ok := c.cache.Load(id); ok {
            e := v.(cacheEntry)
            if e.expires.After(now) {
                out[id] = e.brief
                continue
            }
        }
        missing = append(missing, id)
    }

    if len(missing) == 0 {
        return out, nil
    }

    // 2. 缺失的去 API 拉
    fetched, err := c.fetchBatch(ctx, missing)
    if err != nil {
        return out, err
    }

    // 3. 写入缓存 + 合并到 out
    expires := now.Add(c.cacheTTL)
    for id, brief := range fetched {
        c.cache.Store(id, cacheEntry{brief: brief, expires: expires})
        out[id] = brief
    }
    return out, nil
}

// fetchBatch 是把 L1 的 Users 重命名为 fetchBatch，纯走 API 不查缓存
```

**主动失效**：用户改资料后调 `cli.Invalidate(uid)`，删 cache.Store 那条。

**TTL 怎么选**：

| 场景 | TTL 建议 |
|------|---------|
| name/avatar/bio 展示（容忍少量滞后） | 5–10 分钟 |
| status（封号判断需要快） | 30 秒–1 分钟 |
| 内部脚本 / 任务（数据不能错） | 0（不缓存）或 30 秒 |

如果不同字段需要不同 TTL，最简做法是 `status` 的判断不走这个 cache，而是在权限决策时直接解 JWT roles claim（即时、无 RPC）。

**什么时候要升级到 L3**：高并发场景下，缓存 miss 时同一个 user_id 被多个 goroutine 同时查询，重复打 OAuth；或者一次请求里要查 >100 个 user_id（API 单次上限）。

### 4.4 L3：加 singleflight + 负缓存 + 分片（+50 行）

适用：高并发 HTTP 服务（论坛页面、社区评论流）。

#### singleflight：合并并发的相同请求

10 个 goroutine 同时 cache miss 查 user_id=42 → 不要发 10 次 HTTP，只发 1 次。

```go
import "golang.org/x/sync/singleflight"

type Client struct {
    // ...
    sf singleflight.Group
}

func (c *Client) fetchBatch(ctx context.Context, ids []uint) (map[uint]*Brief, error) {
    key := singleflightKey(ids) // 把 sorted ids 拼成 string
    v, err, _ := c.sf.Do(key, func() (any, error) {
        // ... 实际 HTTP 调用
    })
    if err != nil { return nil, err }
    return v.(map[uint]*Brief), nil
}

func singleflightKey(ids []uint) string {
    cp := append([]uint(nil), ids...)
    sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
    parts := make([]string, len(cp))
    for i, id := range cp {
        parts[i] = strconv.FormatUint(uint64(id), 10)
    }
    return strings.Join(parts, ",")
}
```

#### 负缓存：避免反复查不存在的 ID

如果传一个不存在的 ID（比如评论指向的用户被删了），API 会把它放进 `not_found` 数组。下次再问还是不存在 —— 短 TTL 缓存它的"不存在"状态，避免反复 RPC。

```go
type Client struct {
    // ...
    notFound        sync.Map      // uint → time.Time (until)
    notFoundTTL     time.Duration // 推荐 1 分钟
}

// 在 Users() 的 1. 检查缓存阶段加：
if v, ok := c.notFound.Load(id); ok {
    if v.(time.Time).After(now) {
        continue // 跳过这个 id，不再请求
    }
}

// 在写入缓存阶段加：把 API 返回的 not_found 数组每个 id 加进 notFound
```

#### 自动分片：>100 个 ID 拆成多次请求

API 单次最多 100 个 ID。consumer 不应该让上层调用方关心这个限制。

```go
func chunk(ids []uint, n int) [][]uint {
    out := make([][]uint, 0, (len(ids)+n-1)/n)
    for i := 0; i < len(ids); i += n {
        end := i + n
        if end > len(ids) { end = len(ids) }
        out = append(out, ids[i:end])
    }
    return out
}

// 在 Users() 实际调 API 时：
for _, batch := range chunk(missing, 100) {
    fetched, err := c.fetchBatch(ctx, batch)
    if err != nil { return out, err }
    // 写缓存 + 合并到 out
}
```

#### 输入去重

重复 ID 应该折叠成一次：

```go
func dedupe(ids []uint) []uint {
    seen := make(map[uint]struct{}, len(ids))
    out := make([]uint, 0, len(ids))
    for _, id := range ids {
        if _, ok := seen[id]; ok { continue }
        seen[id] = struct{}{}
        out = append(out, id)
    }
    return out
}
```

### 4.5 决定哪一级

| 工作负载 | 推荐层级 |
|---------|---------|
| 一次性脚本、低频后台任务 | L1（无缓存） |
| 中频后端服务（每分钟数十–数百次 user 查询） | L2（TTL 缓存） |
| 高并发 HTTP 服务（评论流、社区主页） | L3（全套：singleflight + 负缓存 + 分片） |
| 单机 dev 环境调试 | L1 即可 |

升级路径是单调的：L1 → L2 → L3，每步增量可控。**不要从一开始就上 L3** —— 没并发的场景里 singleflight 是死代码。

### 4.6 不在客户端做的事

这些**不应该**进 consumer 的 OAuth 客户端，避免责任错位：

| 事 | 为什么不做 |
|----|-----------|
| 把 user brief 写到本地 user 表 | 那是 dual-write 反模式，参见 [01-architecture.md §5](./01-architecture.md) |
| 鉴权（"这个 user 是 admin 吗"） | 用 JWT roles claim 即可，不用查 OAuth |
| 用户列表分页 | OAuth 没暴露这个（也不该暴露）；管理后台直接用 OAuth admin 自己 |
| 修改用户字段 | PATCH /auth/me 走终端用户 token 路径，是另一套客户端 |

### 4.7 测试策略

不需要打真 OAuth。用 `httptest.NewServer` 模拟一个返回固定 JSON 的 HTTP server，测客户端的：

- 缓存命中
- 缓存过期
- 不存在 ID 的负缓存
- 并发请求合并（用 `time.Sleep` 让 server 慢）
- 大 ID 列表的分片

参考实现里这些测试加起来约 250 行；按需写。

---

## 5. 站点 user 表瘦身建议

迁移完毕后，站点 user 表可以删掉这些列（逐步即可，不必一蹴而就）：

| 列 | 是否可删 |
|----|---------|
| name | ✓ 可以删（OAuth 提供） |
| email | ✓ 可以删（OAuth 提供） |
| password | ✓ 必须删（OAuth 是身份源；本地保留是安全风险） |
| avatar | ✓ 可以删（OAuth 提供） |
| bio | ✓ 可以删（OAuth 提供） |
| role | ✓ 可以删，但要先确认所有权限判断都改走 OAuth roles claim 或 `/users/batch` 返回的 roles 字段 |
| status | ✓ 可以删（OAuth 提供） |
| moemoepoint | ✗ **保留**（OAuth 也有但被认为是 OAuth-端总积分；站点的 moemoepoint 是站点行为分） |
| daily_check_in / daily_image_count | ✗ **保留**（站点功能特有） |
| daily_toolset_upload_count（kungal）/ daily_upload_size（moyu） | ✗ **保留** |
| last_login_time | ✗ **保留** |
| ip | ✗ **保留**（站点最近会话指纹） |
| follower_count / following_count（moyu）| 由你定 —— 反归一计数，可删可留 |

**建议路径**：

1. 改代码：渲染层全部走 OAuth 客户端拿 name/avatar/bio
2. 上线，观察一个迭代（确保没有遗漏的引用）
3. 一次性 ALTER TABLE DROP 那些列

## 6. 缓存与一致性策略

> 这一节是高层策略；具体实现见 [§4 客户端实现指南](#4-客户端实现指南)。

### 6.1 默认延迟模型

进程内 TTL 缓存（10 分钟，consumer 可调）意味着：

- 用户改了 name → 下游服务最多滞后 TTL 才能看到
- 头像换了 → 同上
- 封号了 → 同上

对**展示字段**（name / avatar / bio）—— 这种延迟一般可以接受。

对**强一致字段**（status / roles）—— 你可能需要更短的 TTL，或者绕过缓存。

### 6.2 强一致字段的处理

| 字段 | 推荐做法 |
|------|---------|
| `roles`（鉴权决策） | **解 JWT roles claim**，不查 OAuth。JWT 是这次请求自带的，roles 是 claim 的一部分，零 RPC 零延迟 |
| `status`（封号判断） | 短 TTL（30 秒）；或在敏感操作前不读缓存直接调 `/users/batch` |

### 6.3 主动失效

如果你的服务发出了 mutation（比如代理 PATCH /auth/me 之后），主动失效本地缓存的对应 ID。

更常见的失效来自 OAuth 端 —— 用户在 OAuth 自己改了名。**当前 OAuth 没有事件 broadcast**；如果业务对 name 漂移敏感，要么短 TTL 兜，要么 OAuth 后续加 webhook（暂未规划）。

### 6.4 N+1 防护

永远批量拉。**不要**在循环里调用单个 user 的接口：

```go
// ❌ N+1 反例
for _, item := range items {
    user, _ := cli.User(ctx, item.UserID)  // 每个 item 一次 RPC
    ...
}

// ✓ 批量
ids := make([]uint, 0, len(items))
for _, item := range items {
    ids = append(ids, item.UserID)
}
users, _ := cli.Users(ctx, ids)            // 一次 RPC
for _, item := range items {
    user := users[item.UserID]
    ...
}
```

即使有缓存，单调用在 miss 时仍是 N 次 HTTP；批量调用是 1 次。客户端实现至少要在 API 层暴露 `Users(ids []uint)` 形式（L1 已经有了）。

## 7. 终端用户登录的接入

这一步在你做用户登录回调时做：

```
用户 → 点击 "用 KUN 账号登录"
  ↓
你的站点 → 重定向到 /oauth/authorize
  ↓
用户在 OAuth 登录（如未登录）
  ↓
OAuth 重定向回你的 redirect_uri，带 code
  ↓
你的服务端 → /oauth/token 用 code 换 access_token
  ↓
你的服务端 → /oauth/userinfo 拿用户 id / sub / name / email / roles
  ↓
你的服务端：
  - 在本站 user 表查 id（应该等于 userinfo.id）
  - 不存在 → INSERT 新行（user.id = userinfo.id）
  - 创建 session、设 cookie
```

**关键点**：

- 本站 user 表的 `id` 列必须接受 OAuth 给的 integer ID（即不要 autoincrement，要从 OAuth 取值）
- 或者：禁用 autoincrement，每次 INSERT 显式给 ID
- 或者：保留 autoincrement 但忽略它，用 OAuth ID 写入 —— 然后定期把 sequence reset 到 max

### 7.1 不需要 `oauth_account` 中间表

教科书式 OAuth 集成里通常会建一张 `oauth_account(provider, sub, user_id)` 中间表，把 OAuth 的 UUID（`sub`）映射到本地 `user.id`。

**在我们这套架构里这张表是多余的**，应该**删除**（如果 prisma schema 里有的话）。原因：

| 这张表能解决的问题 | 在我们架构里的实际情况 |
|------------------|---------------------|
| 一个本地用户绑定多个 OAuth provider（Google + GitHub + ...） | 只有一个 KUN OAuth；不会接其他 provider |
| 本地 `user.id` 与 OAuth 的 ID 独立 | **我们故意做了对齐**（migrate-users step 7） |
| 解绑某个 provider 不删本地用户 | 唯一 provider，"解绑" = 注销账号 |

`/oauth/userinfo` 现在直接返回 integer `id`，登录回调按 id 直查 / 直插本地 user 表即可，**不需要任何 sub → id 的间接查询**。

```go
// ✓ 推荐：直接用 userinfo.id
var u User
if err := db.First(&u, info.ID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
    u = User{ID: info.ID, /* 站点特有字段初始化 */}
    db.Create(&u)
}

// ✗ 不要：通过 oauth_account 转一层
var oa OAuthAccount
db.Where("sub = ?", info.Sub).First(&oa)
db.First(&u, oa.UserID)  // 多余的 indirection
```

**删除步骤**（moyu / kungal 都适用）：

1. 删除 prisma schema 里的 `model oauth_account`
2. 删除关联逻辑（`FindOAuthAccountBySub` / `CreateOAuthAccount` 等）
3. 跑 prisma migrate（DROP TABLE）
4. migrate-users 脚本能正确处理"表不存在"的情况（[03-id-unification.md §3.5](./03-id-unification.md) 的 remap 列表里包含 oauth_account.user_id，但脚本会先查 `pg_tables` 过滤），所以删表时机不影响迁移

详见 `docs/integration/oauth/oauth-integration-guide.md`。

## 8. 调试与排错

### 8.1 客户端日志

客户端实现里在出错路径 wrap 一层日志，方便后续排查：

```go
users, err := cli.Users(ctx, ids)
if err != nil {
    slog.Error("oauth users batch failed", "ids", ids, "err", err)
    // 决定：返回错误 / 用 fallback 渲染（user_id 数字直接显示）/ 静默
}
```

### 8.2 OAuth 端不可用怎么办

短期：缓存里有的还能用（TTL 内）。TTL 过期、OAuth 仍不可用 → 客户端返回 error。

中长期建议：在你的渲染层加 graceful degradation —— 拿不到 user brief 时仍然渲染 user_id（数字 + 默认头像），不要让整个页面 500。

### 8.3 验证 OAuth Client 凭证

```bash
curl -sS -w "\n%{http_code}\n" \
  'https://oauth.kungal.com/api/v1/users/batch?ids=1' \
  -H "Authorization: Basic $(echo -n 'YOUR_CLIENT_ID:YOUR_CLIENT_SECRET' | base64)"
```

期望 200 + 用户 brief。如果 401，检查：

- client_id 是否在 OAuth 端注册了
- client_secret 是否正确
- 该 client 是否启用

### 8.4 客户端单元测试

不需要打真 OAuth，用 `httptest.NewServer` 模拟：

```go
func TestUsers_CacheHit(t *testing.T) {
    var hits atomic.Int32
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        hits.Add(1)
        _, _ = w.Write([]byte(`{"code":0,"data":{"users":[{"id":1,"name":"alice"}]}}`))
    }))
    defer srv.Close()

    cli := New(srv.URL, "x", "y")
    _, _ = cli.Users(context.Background(), []uint{1})
    _, _ = cli.Users(context.Background(), []uint{1})
    if hits.Load() != 1 {
        t.Errorf("expected 1 server hit, got %d", hits.Load())
    }
}
```

L2/L3 实现的关键测试：缓存命中、缓存过期、并发合并、不存在 ID 的负缓存、>100 ID 的分片。

## 9. 参考文档

- [api-reference.md](../../integration/oauth/api-reference.md) —— 完整 OAuth API 参考（含 `/users/batch` `/users/search` `/oauth/userinfo`）
- [oauth-integration-guide.md](../../integration/oauth/oauth-integration-guide.md) —— 完整 OAuth 接入指南（含 PKCE、token 轮换、安全注意事项）
