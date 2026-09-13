# 认证系统迁移

## 当前方案

- 邮箱 + 密码注册/登录
- bcrypt 加密密码
- 自签 JWT 双 Token：Access Token (60min) + Refresh Token (30d)
- Refresh Token 存 Cookie (`kungalgame-moemoe-refresh-token`) 和 Redis (`refreshToken:{uid}`)
- `getCookieTokenInfo()` 每次请求从 Cookie 取 Token → `jwt.verify()` → Redis 校验

## 目标方案

- 接入 KUN Galgame OAuth (Authorization Code + PKCE)
- Go 后端自建 Session（Redis 存储），OAuth 仅用于登录/注册
- 用户登录后 Go 签发 `kun_session` Cookie，后续请求验证此 Session

## OAuth 集成流程

```
前端                           Go 后端                         OAuth 服务器
 │                              │                                │
 │  点击登录 → 生成 PKCE        │                                │
 │  → 跳转 OAuth /authorize     │                                │
 │────────────────────────────────────────────────────────────── >│
 │                              │                                │
 │  ← 回调带 code              │                                │
 │  POST /api/auth/oauth/callback                                │
 │─────────────────────────────>│                                │
 │                              │  POST /oauth/token (code换token)│
 │                              │──────────────────────────────->│
 │                              │<── access_token + refresh_token│
 │                              │                                │
 │                              │  GET /oauth/userinfo           │
 │                              │──────────────────────────────->│
 │                              │<── sub, name, email, picture   │
 │                              │                                │
 │                              │  查找/创建本地用户              │
 │                              │  创建 Redis Session             │
 │  Set-Cookie: kun_session     │                                │
 │<─────────────────────────────│                                │
```

## OAuth 端点

| 端点 | 用途 |
|------|------|
| `GET /oauth/authorize` | 获取授权码（用户须已登录 OAuth） |
| `POST /oauth/token` | code 换 token / refresh token |
| `GET /oauth/userinfo` | 获取用户信息（Bearer token） |
| `POST /oauth/revoke` | 吊销 token（登出） |

- 生产环境：`https://account.nextmoe.com/api/v1`
- 开发环境：`http://127.0.0.1:9277/api/v1`

## Session 结构（Redis）

```
Key: session:{64位随机hex}
TTL: 7天

Value: {
  "uid": 1,
  "sub": "uuid-from-oauth",
  "name": "username",
  "email": "user@example.com",
  "role": 1,
  "oauth_access_token": "...",
  "oauth_refresh_token": "...",
  "oauth_expires_at": 1234567890
}
```

## 老用户迁移策略

现有用户有 bcrypt 密码但无 OAuth 账号，需新建 `oauth_account` 表：

```sql
CREATE TABLE oauth_account (
  id         SERIAL PRIMARY KEY,
  user_id    INT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  provider   VARCHAR(50) NOT NULL DEFAULT 'kun-oauth',
  sub        VARCHAR(255) NOT NULL UNIQUE,  -- OAuth UUID
  created    TIMESTAMP DEFAULT NOW(),
  updated    TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_oauth_account_user_id ON oauth_account(user_id);
```

迁移逻辑（在 `AuthService.findOrCreateUser` 中）：
1. 用 OAuth `sub` 查 `oauth_account` → 找到则直接返回关联用户
2. 用 OAuth `email` 查 `user` 表 → 找到则自动关联（创建 `oauth_account` 记录）
3. 都没找到 → 创建新用户 + 关联

## 中间件变更

| 当前 | 目标 |
|------|------|
| `getCookieTokenInfo(event)` | `middleware.Auth(rdb, oauthCfg)` |
| `userInfo.uid` | `middleware.GetUser(c).UID` |
| `userInfo.role` | `middleware.GetUser(c).Role` |
| 无可选认证 | `middleware.OptionalAuth(rdb, oauthCfg)` |

## 前端改动

- 登录页：邮箱密码表单 → OAuth 跳转按钮
- Cookie 名：`kungalgame-moemoe-refresh-token` → `kun_session`
- `responseHandler.ts`：错误码 205 的处理逻辑不变，仅 Cookie 名需更新
- 注册页：可移除或改为引导到 OAuth 注册
- 忘记密码：通过 OAuth 系统处理
