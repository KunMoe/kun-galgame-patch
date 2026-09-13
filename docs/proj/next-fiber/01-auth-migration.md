# 认证系统迁移

## 当前方案

- 邮箱 + 密码注册/登录，用户名或邮箱均可登录
- Argon2id 加密密码（参数：t=2, m=8192, p=3），salt:hash 格式存储
- 自签 JWT 单 Token，30 天过期
- Token 存 HTTP-only Cookie（`kun-galgame-patch-moe-token`）和 Redis（`access:token:{uid}`）
- 每次请求从 Cookie 取 Token → `jwt.verify()` → Redis 校验 Token 是否仍有效
- 注册需要 CAPTCHA 验证（白发角色图片选择）+ 邮箱验证码
- 角色体系：1=普通用户, 2=创作者(publisher), 3=管理员, 4=超级管理员

## 目标方案

- 接入 KUN OAuth（Authorization Code + PKCE）
- Go 后端自建 Redis Session，OAuth 仅用于登录/注册
- 用户登录后 Go 签发 `kun_session` Cookie，后续请求验证此 Session
- 保留邮箱验证码功能（用于修改邮箱等场景）
- CAPTCHA 验证改为可配置（管理后台可开关评论验证）

## OAuth 集成流程

```
前端                           Go 后端                         OAuth 服务器
 │                              │                                │
 │  点击登录 → 生成 PKCE        │                                │
 │  (code_verifier + challenge) │                                │
 │  → 跳转 OAuth /authorize     │                                │
 │────────────────────────────────────────────────────────────── >│
 │                              │                                │
 │  ← 回调带 code              │                                │
 │  POST /api/auth/oauth/callback                                │
 │  (code + code_verifier)      │                                │
 │─────────────────────────────>│                                │
 │                              │  POST /oauth/token             │
 │                              │  (code + code_verifier 换 token)│
 │                              │──────────────────────────────->│
 │                              │<── access_token + refresh_token│
 │                              │                                │
 │                              │  GET /oauth/userinfo           │
 │                              │  (Bearer access_token)         │
 │                              │──────────────────────────────->│
 │                              │<── sub, name, email, avatar    │
 │                              │                                │
 │                              │  查找/创建本地用户              │
 │                              │  创建 Redis Session             │
 │  Set-Cookie: kun_session     │                                │
 │<─────────────────────────────│                                │
```

## OAuth 端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/oauth/authorize` | GET | 获取授权码（用户须已登录 OAuth） |
| `/oauth/token` | POST | code 换 token / refresh token |
| `/oauth/userinfo` | GET | 获取用户信息（Bearer token） |
| `/oauth/revoke` | POST | 吊销 token（登出时调用） |

环境配置：
- 生产环境：`https://account.nextmoe.com/api/v1`
- 开发环境：`http://127.0.0.1:9277/api/v1`

## Session 结构（Redis）

```
Key:    session:{64位随机hex}
TTL:    7 天
Cookie: kun_session（HttpOnly, SameSite=Lax, Secure）

Value (JSON):
{
  "uid":                  1,
  "sub":                  "uuid-from-oauth",
  "name":                 "username",
  "email":                "user@example.com",
  "role":                 1,
  "oauth_access_token":   "...",
  "oauth_refresh_token":  "...",
  "oauth_expires_at":     1234567890
}
```

Session 刷新策略：
- 每次请求检查 `oauth_expires_at`，如果 OAuth token 即将过期（< 5 分钟），用 refresh_token 刷新
- Session 本身 7 天后过期，用户需重新 OAuth 登录
- 登出时同时清除 Redis Session 和调用 OAuth `/revoke`

## 老用户迁移策略

现有用户有 Argon2id 密码但无 OAuth 账号。迁移通过 `oauth_account` 表实现：

```sql
-- 已在 Prisma schema 中定义
CREATE TABLE oauth_account (
  id       SERIAL PRIMARY KEY,
  user_id  INT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  provider VARCHAR(50) NOT NULL DEFAULT 'kun-oauth',
  sub      VARCHAR(255) NOT NULL UNIQUE,
  created  TIMESTAMP DEFAULT NOW(),
  updated  TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_oauth_account_user_id ON oauth_account(user_id);
```

`AuthService.findOrCreateUser` 中的迁移逻辑：

```
1. 用 OAuth sub 查 oauth_account → 找到则直接返回关联用户
2. 用 OAuth email 查 user 表 → 找到则自动创建 oauth_account 关联记录
3. 都没找到 → 创建新 user + oauth_account
```

注意：老用户的密码字段保留但不再用于登录验证。

## 需要迁移的认证相关端点

### 移除的端点（由 OAuth 替代）

| 当前端点 | 说明 |
|---------|------|
| POST `/auth/login` | 邮箱/用户名 + 密码登录 → 改为 OAuth |
| POST `/auth/register` | 注册新用户 → 改为 OAuth |
| POST `/auth/send-register-code` | 注册验证码 → OAuth 自带注册 |
| POST `/auth/captcha`（生成） | CAPTCHA 图片生成 → OAuth 自带 |
| POST `/auth/captcha`（验证） | CAPTCHA 验证 → OAuth 自带 |

### 新增的端点

| Go 端点 | 方法 | 说明 |
|---------|------|------|
| `/api/auth/oauth/callback` | POST | OAuth 回调，接收 code + code_verifier |
| `/api/auth/logout` | POST | 清除 Session + 吊销 OAuth Token |
| `/api/auth/me` | GET | 获取当前登录用户信息（替代 `/user/status`） |

### 保留但需适配的端点

| 当前端点 | Go 端点 | 说明 |
|---------|---------|------|
| POST `/forgot/one` | POST `/api/auth/forgot/send-code` | 忘记密码发送验证码 |
| POST `/forgot/two` | POST `/api/auth/forgot/reset` | 验证码 + 新密码重置 |
| POST `/user/setting/send-reset-email-code` | POST `/api/auth/email/send-code` | 修改邮箱验证码 |
| POST `/user/setting/email` | PUT `/api/user/email` | 修改邮箱（需验证码） |

注意：忘记密码功能保留，因为 OAuth 密码重置后本地密码不会自动同步。

## 中间件变更

| 当前 | 目标 | 说明 |
|------|------|------|
| `verifyHeaderCookie(req)` | `middleware.Auth(c)` | 必须登录 |
| - | `middleware.OptionalAuth(c)` | 可选登录（如获取收藏状态） |
| `payload.uid` | `middleware.GetUID(c)` | 获取当前用户 ID |
| `payload.role` | `middleware.GetRole(c)` | 获取当前用户角色 |
| `role >= 3` | `middleware.RequireRole(3)` | 管理员权限检查 |
| `role >= 4` | `middleware.RequireRole(4)` | 超级管理员权限检查 |
| 无 | `middleware.RateLimit(...)` | 请求限流 |

## 管理后台设置的认证相关配置

以下配置通过管理后台控制，存储在 Redis 中：

| 配置 | Redis Key | 说明 |
|------|-----------|------|
| 评论验证 | `admin:enable_comment_verify` | 开启后发评论需要 CAPTCHA |
| 仅创作者创建 | `admin:enable_only_creator_create` | 开启后仅 role >= 2 可创建补丁 |
| 禁止注册 | `admin:disable_register` | 紧急情况下关闭注册 |

这些配置在 Go 端通过 Redis 读取，无需改动逻辑。

## 前端改动清单

1. **登录页** → 替换为 OAuth 跳转按钮（生成 PKCE → 跳转 OAuth authorize URL）
2. **注册页** → 替换为 OAuth 跳转按钮或引导到 OAuth 注册页
3. **回调页** → 新增 `/auth/callback` 页面，接收 OAuth code 并调用后端
4. **Cookie 名** → `kun-galgame-patch-moe-token` → `kun_session`
5. **错误处理** → 响应格式变为 `{code, message, data}`，session 过期错误码从 HTTP body 变为 `code` 字段
6. **忘记密码页** → 保留，但调用端点路径变更
7. **用户设置中的修改密码** → 考虑移除（密码由 OAuth 管理）
