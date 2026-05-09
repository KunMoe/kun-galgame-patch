# KUN OAuth 接入指南

本文档面向需要接入 KUN OAuth 系统的第三方网站（如 kungal-nuxt、moyu-moe 等），提供完整的 OAuth 2.0 Authorization Code + PKCE 对接流程。

---

## 1. 前置条件

### 1.1 注册 OAuth 客户端

在 KUN OAuth 管理后台创建 OAuth 客户端，获得：

- `client_id` — 客户端标识符
- `client_secret` — 客户端密钥（保密，仅用于服务端）
- 配置的 `redirect_uris` — 允许的回调地址列表

### 1.2 OAuth Server 地址

| 环境 | Base URL |
|------|----------|
| 开发 | `http://127.0.0.1:9277/api/v1` |
| 生产 | `https://oauth.kungal.com/api/v1` |

### 1.3 端点列表

| 端点 | 方法 | 认证 | 用途 |
|------|------|------|------|
| `/oauth/authorize` | GET | 需要登录 | 获取授权码 |
| `/oauth/token` | POST | 不需要 | 用授权码/刷新令牌换取 access token |
| `/oauth/userinfo` | GET | Bearer Token | 获取用户信息 |
| `/oauth/revoke` | POST | 不需要 | 吊销令牌 |

---

## 2. 完整对接流程

### 流程概览

```
用户点击「使用 KUN 账号登录」
  ↓
客户端生成 PKCE code_verifier + code_challenge
  ↓
重定向到 OAuth Server 的 /oauth/authorize
  ↓
用户在 OAuth Server 登录（如果未登录）
  ↓
OAuth Server 重定向回 redirect_uri，带上 code 和 state
  ↓
客户端服务端用 code 换取 access_token + refresh_token
  ↓
客户端用 access_token 请求 /oauth/userinfo 获取用户信息
  ↓
完成登录
```

---

## 3. 详细步骤

### 步骤 1：生成 PKCE 参数和 state

```typescript
// 生成 code_verifier（43-128 字符的随机字符串）
const generateCodeVerifier = (): string => {
  const array = new Uint8Array(32)
  crypto.getRandomValues(array)
  return btoa(String.fromCharCode(...array))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

// 根据 verifier 生成 code_challenge (S256)
const generateCodeChallenge = async (verifier: string): Promise<string> => {
  const encoder = new TextEncoder()
  const data = encoder.encode(verifier)
  const digest = await crypto.subtle.digest('SHA-256', data)
  return btoa(String.fromCharCode(...new Uint8Array(digest)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

// 生成 state（防 CSRF）
const generateState = (): string => {
  const array = new Uint8Array(16)
  crypto.getRandomValues(array)
  return Array.from(array, (b) => b.toString(16).padStart(2, '0')).join('')
}
```

### 步骤 2：重定向到授权端点

```typescript
const codeVerifier = generateCodeVerifier()
const codeChallenge = await generateCodeChallenge(codeVerifier)
const state = generateState()

// 保存到 session（回调时需要验证）
sessionStorage.setItem('oauth_code_verifier', codeVerifier)
sessionStorage.setItem('oauth_state', state)

// 构建授权 URL
const params = new URLSearchParams({
  client_id: 'your-client-id',
  redirect_uri: 'https://www.kungal.com/auth/callback',
  response_type: 'code',
  scope: 'openid profile',
  state,
  code_challenge: codeChallenge,
  code_challenge_method: 'S256',
})

// 重定向
window.location.href = `https://oauth.kungal.com/api/v1/oauth/authorize?${params}`
```

**注意**：用户在此时会被重定向到 OAuth Server。如果用户未登录，OAuth Server 会先要求用户登录，登录成功后自动重定向回你的 `redirect_uri`。

### 步骤 3：处理回调

用户授权后，浏览器会被重定向到：

```
https://www.kungal.com/auth/callback?code=abc123...&state=xyz789...
```

在回调页面：

```typescript
// 1. 验证 state
const urlParams = new URLSearchParams(window.location.search)
const code = urlParams.get('code')
const returnedState = urlParams.get('state')
const savedState = sessionStorage.getItem('oauth_state')

if (returnedState !== savedState) {
  throw new Error('State mismatch — possible CSRF attack')
}

// 2. 取出 code_verifier
const codeVerifier = sessionStorage.getItem('oauth_code_verifier')

// 3. 清理
sessionStorage.removeItem('oauth_state')
sessionStorage.removeItem('oauth_code_verifier')
```

### 步骤 4：用授权码换取令牌（服务端执行）

**重要**：这一步应该在服务端完成，不要在浏览器中暴露 client_secret。

```typescript
// Nuxt 3/4 server route: /server/api/auth/callback.post.ts
export default defineEventHandler(async (event) => {
  const { code, code_verifier } = await readBody(event)

  const response = await $fetch('https://oauth.kungal.com/api/v1/oauth/token', {
    method: 'POST',
    body: {
      grant_type: 'authorization_code',
      code,
      redirect_uri: 'https://www.kungal.com/auth/callback',
      client_id: process.env.OAUTH_CLIENT_ID,
      client_secret: process.env.OAUTH_CLIENT_SECRET,
      code_verifier,
    },
  })

  // response 结构：
  // {
  //   "code": 0,
  //   "message": "成功",
  //   "data": {
  //     "access_token": "eyJhbGc...",
  //     "token_type": "Bearer",
  //     "expires_in": 900,
  //     "refresh_token": "eyJhbGc...",
  //     "scope": "openid profile"
  //   }
  // }

  return response.data
})
```

### 步骤 5：获取用户信息

```typescript
const userInfo = await $fetch('https://oauth.kungal.com/api/v1/oauth/userinfo', {
  headers: {
    Authorization: `Bearer ${accessToken}`,
  },
})

// 返回：
// {
//   "code": 0,
//   "message": "成功",
//   "data": {
//     "sub": "550e8400-e29b-41d4-a716-446655440000",  // 用户 UUID（唯一标识）
//     "name": "KUN",
//     "email": "kun@kungal.com",
//     "picture": "https://...",
//     "updated_at": 1234567890
//   }
// }
```

### 步骤 6：在本站创建/关联用户

```typescript
// 伪代码：在你的数据库中查找或创建用户
let localUser = await db.user.findByOAuthId('kun-oauth', userInfo.sub)

if (!localUser) {
  // 首次登录 — 创建本站用户
  localUser = await db.user.create({
    oauthProvider: 'kun-oauth',
    oauthId: userInfo.sub,    // 用 sub (UUID) 作为唯一标识
    name: userInfo.name,
    email: userInfo.email,
    avatar: userInfo.picture,
  })
} else {
  // 已有用户 — 可选更新信息
  await db.user.update(localUser.id, {
    name: userInfo.name,
    avatar: userInfo.picture,
  })
}

// 创建本站 session，设置 cookie 等
```

---

## 4. 令牌刷新

Access token 有效期 15 分钟。过期后用 refresh token 获取新的：

```typescript
const response = await $fetch('https://oauth.kungal.com/api/v1/oauth/token', {
  method: 'POST',
  body: {
    grant_type: 'refresh_token',
    refresh_token: storedRefreshToken,
    client_id: process.env.OAUTH_CLIENT_ID,
    client_secret: process.env.OAUTH_CLIENT_SECRET,
  },
})

// 返回 { code: 0, data: { access_token, refresh_token, ... } }
// 必须用新的 refresh_token 替换旧的（令牌轮换）
```

**注意**：每次刷新都会返回新的 refresh_token，旧的会立即失效（token rotation）。

---

## 5. 令牌吊销（登出）

用户在你的网站登出时，应该吊销 OAuth 令牌：

```typescript
await $fetch('https://oauth.kungal.com/api/v1/oauth/revoke', {
  method: 'POST',
  body: {
    token: storedRefreshToken,
  },
})

// 遵循 RFC 7009，无论令牌是否有效，始终返回 200 OK
```

---

## 6. JWT Access Token 结构

如果你需要在不调用 userinfo 端点的情况下解析用户信息，可以直接解码 JWT：

```json
{
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email": "kun@kungal.com",
  "name": "KUN",
  "roles": ["user", "admin"],
  "exp": 1700000000,
  "iat": 1699999100,
  "nbf": 1699999100
}
```

- **签名算法**：HS256
- **有效期**：15 分钟
- **重要**：不要在客户端验证签名（你没有 JWT secret），仅用于读取 claims。需要验证时请调用 `/oauth/userinfo`。

---

## 7. 错误处理

所有 API 响应格式：

```json
{
  "code": 0,
  "message": "成功",
  "data": { ... }
}
```

`code = 0` 表示成功，非零表示错误。

### OAuth 相关错误码

| code | 含义 | 处理方式 |
|------|------|---------|
| 15001 | 无效的客户端 | 检查 client_id 是否正确 |
| 15002 | 无效的回调地址 | 检查 redirect_uri 是否已注册 |
| 15003 | 无效的授权码 | 授权码已过期或已使用，重新走授权流程 |
| 15004 | 无效的代码验证器 | PKCE code_verifier 不匹配 |
| 15005 | 无效的授权类型 | grant_type 只支持 authorization_code 和 refresh_token |
| 10002 | 无效的令牌 | refresh_token 无效 |
| 10003 | 令牌已过期 | refresh_token 过期，需要用户重新登录 |

---

## 8. Nuxt 3/4 完整接入示例

### 8.1 环境变量

```env
# .env
OAUTH_SERVER_URL=https://oauth.kungal.com/api/v1
OAUTH_CLIENT_ID=your-client-id
OAUTH_CLIENT_SECRET=your-client-secret
OAUTH_REDIRECT_URI=https://www.kungal.com/auth/callback
```

### 8.2 登录按钮组件

```vue
<!-- components/OAuthLoginButton.vue -->
<script setup lang="ts">
const config = useRuntimeConfig()

const handleLogin = async () => {
  const codeVerifier = generateCodeVerifier()
  const codeChallenge = await generateCodeChallenge(codeVerifier)
  const state = generateState()

  // 保存到 session
  sessionStorage.setItem('oauth_code_verifier', codeVerifier)
  sessionStorage.setItem('oauth_state', state)

  const params = new URLSearchParams({
    client_id: config.public.oauthClientId,
    redirect_uri: config.public.oauthRedirectUri,
    response_type: 'code',
    scope: 'openid profile',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
  })

  window.location.href = `${config.public.oauthServerUrl}/oauth/authorize?${params}`
}
</script>

<template>
  <button @click="handleLogin">使用 KUN 账号登录</button>
</template>
```

### 8.3 回调页面

```vue
<!-- pages/auth/callback.vue -->
<script setup lang="ts">
const route = useRoute()
const router = useRouter()

onMounted(async () => {
  const code = route.query.code as string
  const state = route.query.state as string
  const savedState = sessionStorage.getItem('oauth_state')
  const codeVerifier = sessionStorage.getItem('oauth_code_verifier')

  // 清理
  sessionStorage.removeItem('oauth_state')
  sessionStorage.removeItem('oauth_code_verifier')

  if (!code || state !== savedState) {
    router.push('/auth/login?error=invalid_state')
    return
  }

  try {
    // 调用自己的服务端 API 来换取 token
    const result = await $fetch('/api/auth/oauth-callback', {
      method: 'POST',
      body: { code, code_verifier: codeVerifier },
    })

    // 服务端已设置了 session cookie，跳转到首页
    router.push('/')
  } catch {
    router.push('/auth/login?error=oauth_failed')
  }
})
</script>

<template>
  <div>正在登录...</div>
</template>
```

### 8.4 服务端回调处理

```typescript
// server/api/auth/oauth-callback.post.ts
export default defineEventHandler(async (event) => {
  const { code, code_verifier } = await readBody(event)
  const config = useRuntimeConfig()

  // 1. 用授权码换取 token
  const tokenResponse = await $fetch(`${config.oauthServerUrl}/oauth/token`, {
    method: 'POST',
    body: {
      grant_type: 'authorization_code',
      code,
      redirect_uri: config.public.oauthRedirectUri,
      client_id: config.public.oauthClientId,
      client_secret: config.oauthClientSecret,
      code_verifier,
    },
  })

  // 2. 获取用户信息
  const userInfoResp = await $fetch(`${config.oauthServerUrl}/oauth/userinfo`, {
    headers: { Authorization: `Bearer ${tokenResponse.data.access_token}` },
  })
  const userInfo = userInfoResp.data

  // 3. 在本站创建/查找用户（根据你的数据库逻辑）
  // ...

  // 4. 创建本站 session
  // ...

  // 5. 保存 OAuth refresh_token 以便后续刷新
  // ...

  return { success: true }
})
```

---

## 9. 安全注意事项

1. **client_secret 只能在服务端使用**，绝不能暴露到前端代码
2. **始终使用 PKCE**（S256 方法），即使你有 client_secret
3. **始终验证 state 参数**，防止 CSRF 攻击
4. **存储 refresh_token** 时使用 httpOnly cookie 或加密存储
5. **令牌轮换**：每次刷新后用新的 refresh_token 替换旧的
6. **CORS**：生产环境已配置 `kungal.com` 和 `moyu.moe`，其他域名需要在 OAuth Server 管理后台添加

---

## 10. 后端跨服务用户回拉（kungal / moyu / galgame_wiki）

OAuth 是单一用户身份源（single source of truth）。kungal / moyu / galgame_wiki 等业务库
**不再缓存** `users.name` / `users.avatar` 等字段，只保留 `user_id` 外键。
渲染列表时按需从 OAuth 批量拉取。

### 10.1 端点

`GET /users/batch?ids=1,2,3` — 详见 [api-reference.md#跨服务批量查询](./api-reference.md#跨服务批量查询)

- 鉴权：OAuth Client Basic Auth（不是终端用户 JWT）
- 单次最多 100 个 ID
- 响应不含 `email`、`moemoepoint` 等隐私字段
- 通过 migrate-users 后，kungal / moyu 中的 `*_user_id` 已与 OAuth `users.id` 对齐

### 10.2 Go SDK：`pkg/userclient`

apps/api 仓库提供了 `pkg/userclient`，封装了：

- TTL 缓存（默认 10 分钟）+ 负缓存（默认 1 分钟，避免反复查不存在的 ID）
- `singleflight` 合并并发的相同请求
- 自动分片（>100 个 ID 自动拆成多次请求）
- 空和重复 ID 的去重

**使用示例**：

```go
import "api/pkg/userclient"

// 服务启动时初始化一次（建议放进 DI 容器）
cli := userclient.New(userclient.Config{
    BaseURL:      "https://oauth.kungal.com/api/v1",
    ClientID:     "kungal-backend",
    ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
    CacheTTL:     10 * time.Minute,
})

// 批量取
users, err := cli.Users(ctx, []uint{1, 2, 3, 4})
// users[1].Name, users[1].Avatar...

// 单个（返回 nil 表示不存在）
u, err := cli.User(ctx, 1)

// 用户改名/换头像后，主动失效缓存
cli.Invalidate(uid)
```

### 10.3 渲染管线建议

1. **DB 查询**：业务表只 `SELECT ..., user_id FROM ...`，不 JOIN 用户表
2. **收集 ID**：把列表里所有 `user_id` 收成 `[]uint`（去重）
3. **批量回拉**：`cli.Users(ctx, ids)` 一次调用拿齐
4. **拼装**：在 service / handler 层把 user brief 注入到响应 DTO

**N+1 防护**：永远批量拉。不要在循环里 `cli.User(ctx, item.UserID)` —— 即使有缓存命中，
miss 时仍然是 N 次 HTTP 请求。

### 10.4 失效策略

OAuth 端用户改名 / 换头像 / 被封禁时，下游服务的缓存最多滞后 `CacheTTL` 时间。
对一致性要求严格的场景：

- 短 TTL（30s–2min），靠时间到期被动刷新
- 或在 OAuth 侧广播 `user.updated` 事件，下游订阅后调 `cli.Invalidate(uid)`
