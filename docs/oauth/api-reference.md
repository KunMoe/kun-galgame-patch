# KUN OAuth API 参考

所有 API 基础路径：`/api/v1`

所有响应格式：

```json
{
  "code": 0,       // 0=成功, 非零=错误码
  "message": "成功",
  "data": { ... }  // 成功时有数据，失败时无
}
```

---

## OAuth 2.0 端点

### POST /oauth/token

用授权码或刷新令牌换取 access token。

**请求体（授权码模式）**：

```json
{
  "grant_type": "authorization_code",
  "code": "64位hex授权码",
  "redirect_uri": "https://www.kungal.com/auth/callback",
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "code_verifier": "PKCE验证器（如果authorize时使用了code_challenge）"
}
```

**请求体（刷新令牌模式）**：

```json
{
  "grant_type": "refresh_token",
  "refresh_token": "eyJhbGc...",
  "client_id": "your-client-id",
  "client_secret": "your-client-secret"
}
```

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "access_token": "eyJhbGc...",
    "token_type": "Bearer",
    "expires_in": 900,
    "refresh_token": "eyJhbGc...",
    "scope": "openid profile"
  }
}
```

| 字段 | 说明 |
|------|------|
| access_token | JWT，有效期 15 分钟 |
| token_type | 固定 "Bearer" |
| expires_in | 900 秒（15 分钟） |
| refresh_token | JWT，有效期 7 天。每次刷新会轮换 |
| scope | 可选，回显授权时的 scope |

---

### GET /oauth/authorize

获取授权码。用户必须已登录（带 Bearer Token）。

**查询参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| client_id | 是 | OAuth 客户端 ID |
| redirect_uri | 是 | 回调地址，必须与注册时一致 |
| response_type | 是 | 固定 `code` |
| state | 是 | 随机字符串，防 CSRF |
| scope | 否 | 权限范围，空格分隔 |
| code_challenge | 否 | PKCE code challenge |
| code_challenge_method | 否 | `S256`（默认）或 `plain` |

**成功响应**：HTTP 302 重定向到 `redirect_uri?code=xxx&state=xxx`

**授权码有效期**：10 分钟，一次性使用

---

### GET /oauth/userinfo

获取当前登录用户信息。

**请求头**：`Authorization: Bearer <access_token>`

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "id": 12345,
    "sub": "550e8400-e29b-41d4-a716-446655440000",
    "name": "KUN",
    "email": "kun@kungal.com",
    "picture": "https://...",
    "roles": ["user", "admin"],
    "updated_at": 1234567890
  }
}
```

| 字段 | 说明 |
|------|------|
| id | 用户整数 ID（= OAuth `users.id`，与 kungal/moyu 业务表的 `user_id` 外键对齐） |
| sub | 用户 UUID（OIDC 标准的 subject），与 `id` 标识同一用户，调用方任选其一 |
| name | 用户名（仅 `profile` scope 或空 scope 时返回） |
| email | 邮箱（仅 `email` scope 或空 scope 时返回） |
| picture | 头像 URL（仅 `profile` scope 或空 scope 时返回，可能为空） |
| roles | 角色名称数组，与 JWT `roles` claim 一致 |
| updated_at | 最后更新时间（Unix 时间戳） |

**关于 scope 与字段过滤**：

`id`、`sub`、`roles` 始终返回（不被 scope 过滤）—— 因为这三项已经在 JWT 里，调用方既然能用这个 JWT 调 /userinfo，就已经拿到了这些信息，再隐藏没有意义。`name`、`email`、`picture` 按 OIDC 标准受 `profile` / `email` scope 控制。

> **跨服务接入提示**：kungal/moyu/galgame_wiki 后端处理 OAuth callback 时，应该在登录环节就拿 `id` 入库（作为本地 user 表的主键 / 外键），不要只存 `sub` —— 后续业务表关联、`/users/batch` 批量回拉、SDK 缓存键，全部基于 `id` 整数键。

---

### POST /oauth/revoke

吊销令牌。遵循 RFC 7009，无论成功失败都返回 200。

**请求体**：

```json
{
  "token": "要吊销的 refresh_token"
}
```

---

## 跨服务批量查询

### GET /users/batch

跨服务批量获取用户公开资料。专为 kungal / moyu / galgame_wiki 等下游业务使用 ——
这些服务不在本地缓存 `users.name` / `users.avatar`，渲染时按 `user_id` 列表回拉。

**鉴权**：OAuth Client Basic Auth（`Authorization: Basic base64(client_id:client_secret)`）。
不是终端用户 JWT。任何已注册的 OAuth Client 都可以调用。

**查询参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| ids | 是 | 1..100 个用户 ID（OAuth 用户表主键），逗号分隔，如 `?ids=1,2,3` |

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "users": [
      {
        "id": 1,
        "uuid": "9e00220a-8079-4e81-8e98-49e26ce23edc",
        "name": "kun",
        "avatar": "https://image.kungal.com/avatar/user_1/avatar.webp",
        "avatar_image_hash": "abc123...",
        "bio": "KUN IS THE CUTEST!",
        "status": 0,
        "roles": ["admin"]
      }
    ],
    "not_found": [9999]
  }
}
```

| 字段 | 说明 |
|------|------|
| users[].id | 用户 ID（与 kungal/moyu 中 `*_user_id` 外键对齐） |
| users[].uuid | 用户 UUID |
| users[].name | 用户名 |
| users[].avatar | 头像 URL（可能为空字符串） |
| users[].avatar_image_hash | 头像 image_service 哈希（可空） |
| users[].bio | 个人简介 |
| users[].status | 0=正常；非 0 时调用方应隐藏或脱敏渲染 |
| users[].roles | 角色名称数组，如 `["admin"]` |
| not_found | 请求中存在但 OAuth 库里查不到的 ID 列表 |

**错误响应**：

| HTTP | code | 触发条件 |
|------|------|----------|
| 400  | 9    | `ids` 为空或包含非数字 |
| 400  | 9    | `ids` 个数超过 100 |
| 401  | 10001/15001/15009 | Basic Auth 缺失/格式错/client_id 不存在/secret 错误 |

**注意**：响应中**不包含** `email`、`moemoepoint`、`created_at` 等隐私字段。
若调用方需要邮箱（如发邮件通知），应该走专门的 RPC 而不是渲染管线。

**Go SDK**：参见 `pkg/userclient`，已内置 TTL 缓存、负缓存、singleflight 合并、自动分片：

```go
cli := userclient.New(userclient.Config{
    BaseURL:      "https://oauth.kungal.com/api/v1",
    ClientID:     "kungal-backend",
    ClientSecret: "...",
    CacheTTL:     10 * time.Minute,
})
users, err := cli.Users(ctx, []uint{1, 2, 3, 4})
// users[1].Name, users[1].Avatar...
```

---

### GET /users/search

按用户名搜索用户，case-insensitive 子串匹配。结果按相关度排序：精确匹配 > 前缀匹配 > 子串匹配，每一档内按字母升序。

适用场景：@提及自动补全、用户搜索框、管理后台用户检索。

**鉴权**：与 `/users/batch` 相同（OAuth Client Basic Auth）。

**查询参数**：

| 参数 | 必填 | 说明 |
|------|------|------|
| q | 是 | 搜索关键词，trim 后 1..50 字符。`%` `_` `\` 等 LIKE 通配符按字面匹配（已转义） |
| limit | 否 | 返回条数，默认 20，封顶 50 |

**成功响应**：

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "users": [
      { "id": 2, "uuid": "...", "name": "鲲", "avatar": "...", "bio": "...", "status": 0, "roles": ["admin"] },
      { "id": 79063, "uuid": "...", "name": "鲲1", ... },
      { "id": 38359, "uuid": "...", "name": "鲲114514", ... }
    ]
  }
}
```

**错误响应**：

| HTTP | code | 触发条件 |
|------|------|----------|
| 400  | 9    | `q` 为空或缺失 |
| 400  | 9    | `q` 超过 50 字符 |
| 400  | 9    | `limit` 不是正整数 |
| 401  | 10001/15001/15009 | Basic Auth 缺失/格式错/凭证错 |

**Go SDK**：

```go
// limit=0 用服务器默认（20）；limit>50 服务器会自动降到 50
users, err := cli.Search(ctx, "kun", 10)
for _, u := range users {
    fmt.Println(u.ID, u.Name)
}
```

> **注意**：`Search()` **不缓存**结果（query 空间无界、结果易变）。需要前端逐键自动补全的场景请在调用方做 debounce（推荐 200–300ms）。

---

## JWT Access Token Claims

```json
{
  "sub": "用户UUID",
  "email": "邮箱",
  "name": "用户名",
  "roles": ["user", "admin"],
  "site_id": 0,
  "exp": 1700000000,
  "iat": 1699999100,
  "nbf": 1699999100
}
```

签名算法：HS256

---

## 错误码速查

### OAuth 错误 (15xxx)

| Code | 消息 | 说明 |
|------|------|------|
| 15001 | 无效的客户端 | client_id 不存在 |
| 15002 | 无效的回调地址 | redirect_uri 未注册 |
| 15003 | 无效的授权码 | code 已过期/已使用/不存在 |
| 15004 | 无效的代码验证器 | PKCE code_verifier 不匹配 |
| 15005 | 无效的授权类型 | grant_type 不支持 |
| 15006 | 无效的权限范围 | scope 不被支持 |
| 15007 | 访问被拒绝 | 用户拒绝授权 |

### 认证错误 (10xxx)

| Code | 消息 | 说明 |
|------|------|------|
| 10001 | 未授权 | 未提供 Bearer Token |
| 10002 | 无效的令牌 | Token 格式错误或签名无效 |
| 10003 | 令牌已过期 | Token 已过期，需要刷新或重新登录 |
| 10005 | 用户不存在 | UUID 对应的用户不存在 |

### 通用错误

| Code | 消息 |
|------|------|
| 1 | 请求格式错误 |
| 7 | 参数验证失败 |
| 10 | 操作失败 |
