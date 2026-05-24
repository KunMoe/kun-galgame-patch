# 用户注册

返回 [README](./README.md)

> 🔒 **重要：注册是身份层操作。下游 kungal / moyu / wiki 不应在自己前端做注册表单**——和[改邮箱 / 改密码一样](./02-user-profile.md#身份操作-vs-展示操作)。本文档描述统一的"跳转到 OAuth 注册 → 自动回跳并登录"流程。

## 整体设计

```
┌──────────┐                                   ┌─────────────────────┐
│ Kungal   │  ① 用户点"注册" → window.location  │ oauth.kungal.com    │
│ /moyu /  │ ──────────────────────────────►   │ /auth/register      │
│ wiki     │   ?redirect=<encoded(authorize)>   │ ?redirect=...       │
└──────────┘                                   └────────┬────────────┘
     ▲                                                  │
     │                                                  │ ② 填表 → POST /api/v1/auth/register
     │                                                  │    (后端发 access_token + refresh cookie)
     │                                                  ▼
     │                                          ┌─────────────────────┐
     │                                          │ oauth.kungal.com    │
     │                                          │ /oauth/authorize    │
     │                                          │ ?client_id=...      │
     │ ⑤ /auth/callback?code=...               │ &state=...&PKCE=... │
     │ ← exchange → 本站 session 创建            └────────┬────────────┘
     │                                                   │
     │                                                   │ ③ 用户已登录（注册时拿到了 access_token）
     │                                                   │   + client.auto_consent === true
     │                                                   │   ⇒ Container.vue 不渲染同意 UI，
     │                                                   │     直接 POST /oauth/authorize/consent
     │                                                   ▼
     │                                          ┌─────────────────────┐
     │                                          │ oauth backend       │
     │                                          │ issues code         │
     └──────────────────────────────────────────│ 302 → redirect_uri  │
                       ④                       └─────────────────────┘
```

整条链路是**注册 + OAuth code 流程合一**：用户在 OAuth 注册成功后，OAuth web 复用 `?redirect=` 直接跳到 `/oauth/authorize`，由于 client 是第一方（`auto_consent=true`），同意页跳过，code 立刻发回 kungal，kungal 用现成的 OAuth callback 流程完成本站登录。**用户感知是"点注册 → 填表单 → 回到原站点已登录"**，中间 OAuth 域名的存在被淡化到一闪而过。

## 为什么不在 kungal/moyu 自己前端做注册

和[身份层政策](./02-user-profile.md#身份操作-vs-展示操作)一致：

- **唯一身份写入入口**：用户表只能从 OAuth 这边写。N 个下游各自实现注册 → N 套验证码 / 限流 / 反爬 / 邮箱去重逻辑 → N 个攻击面
- **未来收益自动化**：将来加 passkey / magic link / 第三方登录 / 异地通知，只改 OAuth 一处，所有下游零代码受益
- **和登录对齐**：登录已经走 OAuth Authorization Code + PKCE（kungal / moyu 已迁移），注册同样走 OAuth 是自然的对称——一个登录入口 + 一个注册入口都在身份提供方
- **政策一致性**：改密码 / 改邮箱必须在 OAuth profile，注册当然也应该在 OAuth

下游唯一要做的是"注册"按钮的跳转 URL —— 和登录按钮共享同一段 PKCE 生成代码即可。

---

## 端点

### POST /auth/register

创建新用户并立即发放 token（**注册即登录**）。

**请求体**：

```json
{
  "name": "kun",
  "email": "kun@kungal.com",
  "password": "secret123"
}
```

| 字段 | 类型 | 约束 |
|---|---|---|
| name | string | 2..17 字符；全局唯一 |
| email | string | 合法邮箱格式；全局唯一 |
| password | string | 6..100 字符 |

**成功响应**：返回访问令牌 + 用户资料 + 刷新令牌（写 httpOnly cookie）。

```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "access_token": "eyJhbGc...",
    "user": {
      "uuid": "...",
      "name": "kun",
      "email": "kun@kungal.com",
      "avatar": "",
      "bio": "",
      "moemoepoint": 0,
      "status": 0,
      "roles": [],
      "created_at": "2026-05-23T08:00:00Z"
    }
  }
}
```

`refresh_token` 写在 httpOnly cookie 里（`Path=/api/v1/auth`，7 天），调用方不需要也不应处理。

**错误响应**：

| HTTP | code | 触发条件 |
|------|------|----------|
| 400  | 1    | JSON 格式错误 |
| 400  | 7    | 字段约束未通过 |
| 400  | 10006 | 邮箱已被注册 |
| 400  | 10007 | 用户名已被使用 |

**调用方**：**只有 oauth.kungal.com 自己的前端应该直接调这个端点**。下游 kungal / moyu / wiki 应该走"跳转到 oauth.kungal.com/auth/register"的模式（见下方"下游接入"）。

---

### GET /oauth/client-info

公开元数据查询。无鉴权。供前端在 `/oauth/authorize` 页面**判断是否跳过同意 UI** 时调用。

**查询参数**：

| 参数 | 必填 | 说明 |
|---|---|---|
| client_id | ✓ | OAuth client ID |

**成功响应**：

```json
{
  "code": 0,
  "data": {
    "id": "4ed9bc99ec0a789a4796b83e22bd84c5",
    "name": "鲲 Galgame 论坛",
    "auto_consent": true,
    "site_domain": "www.kungal.com"
  }
}
```

| 字段 | 说明 |
|---|---|
| id | client_id（回显） |
| name | 展示名 |
| auto_consent | 第一方 client 标志；为 true 时前端**跳过同意页**，直接 POST `/oauth/authorize/consent` |
| site_domain | 关联 site 的 domain（可空），用于展示"将跳转回 X" |

> **不返回**：`secret`、`redirect_uris`、`scopes` 等敏感 / 实现细节。这个端点只为前端判定 UI 行为服务。

**错误响应**：

| HTTP | code | 触发条件 |
|---|---|---|
| 400 | 2 | 缺 client_id |
| 404 | 15001 | client_id 不存在 |

---

## 下游接入

### 1. 注册按钮（kungal / moyu / 任何下游）

和**登录按钮共用同一段 PKCE 代码**，只把目标路径从 `/oauth/authorize` 换成 `/auth/register?redirect=<encoded(/oauth/authorize?...)>`。

```ts
// kungal/apps/web/app/components/register/Register.vue (示意)
const handleOAuthRegister = async () => {
  const codeVerifier = generateCodeVerifier()
  const codeChallenge = await generateCodeChallenge(codeVerifier)
  const state = generateState()

  sessionStorage.setItem('oauth_code_verifier', codeVerifier)
  sessionStorage.setItem('oauth_state', state)

  const authorizeParams = new URLSearchParams({
    client_id: config.public.oauthClientId,
    redirect_uri: config.public.oauthRedirectUri,
    response_type: 'code',
    scope: 'openid profile',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256'
  })

  // 注册成功后 OAuth web 会跳到这里
  const authorizeUrl = `${config.public.oauthServerUrl}/oauth/authorize?${authorizeParams}`

  // OAuth 注册页 URL；redirect 参数让注册完后串到 authorize 流程
  const registerUrl = `${config.public.oauthWebUrl}/auth/register?redirect=${encodeURIComponent(authorizeUrl)}`

  window.location.href = registerUrl
}
```

注意 `oauthWebUrl` 是前端域名（开发 `:9420` / 生产 `oauth.kungal.com`），`oauthServerUrl` 是 API 域名——两者可能不同。详见 [oauth-integration-guide.md §1.3](./oauth-integration-guide.md#13-oauth-server-地址)。

### 2. 用户感知的完整时间线

| 步 | 用户看到的 URL | 时间 | 用户感知 |
|---|---|---|---|
| 1 | `www.kungal.com/login` 点击"注册" | 0 ms | 点击 |
| 2 | `oauth.kungal.com/auth/register?redirect=...` | ~200 ms | "跳到了账号注册页" |
| 3 | 同上，填表 | 用户自主时间 | 填邮箱 + 密码 + 用户名 |
| 4 | `oauth.kungal.com/oauth/authorize?...` | ~100 ms（注册返回后立即跳） | **白屏一闪**（auto_consent 不渲染 UI） |
| 5 | `www.kungal.com/auth/callback?code=...` | ~150 ms | **白屏一闪**（kungal 在交换 token） |
| 6 | `www.kungal.com/` (或 redirect_uri 配的路径) | — | "我已经登录了" |

第 4 步和第 5 步加起来一般在 300 ms 以下，用户感知就是"注册完成后回到原站点已登录"。

### 3. 已注册用户访问 `/auth/register` 的处理

用户已登录的状态下访问 `oauth.kungal.com/auth/register?redirect=...`，OAuth web 应当：
- 如果 `redirect` 参数存在 → 立即 `window.location.href = redirect`（推进 OAuth code 流程）
- 否则 → 跳 `/profile`（账号管理页）

**绝不应该**让已登录用户看到一个空注册表单——会引发"我已经登录了为什么让我再注册一次"的困惑。

---

## auto_consent 字段语义

`oauth_clients.auto_consent` boolean，默认 `false`。**为 true 表示这个 client 在 `/oauth/authorize` 流程中跳过用户同意 UI**——前端不渲染"该应用将获得以下权限"卡片，直接 POST `/oauth/authorize/consent`。

**何时设 true**：
- ✅ **第一方 client**（owner 是 OAuth 平台自己，比如 kungal / moyu / wiki / AI / sticker）
- ❌ 第三方接入应用
- ❌ 任何不在你直接控制下的 client

**安全模型**：auto_consent 不是降低安全等级，是承认"用户已经在使用 kungal，不需要再问一次 kungal 是否可以读他的 OAuth 资料"。这是 SSO 的标准做法——Google 内部应用之间也不会重复问同意。

**当前的第一方列表**（auto_consent=true）：

| client_id | name | site |
|---|---|---|
| 4ed9bc99ec0a789a4796b83e22bd84c5 | 鲲 Galgame 论坛 | www.kungal.com |
| df3ff6008d740bfacbe46aa8cf483cf2 | 鲲 Galgame 补丁 | www.moyu.moe |
| 53e9b5ea70bfc4e4d0700a9f7b8818e8 | 鲲 Galgame Wiki | wiki.kungal.com |
| df46a4cfa71ac919b7b43d63238e2311 | 鲲 Galgame AI | ai.kungal.com |
| 2d8d48a141a3340b43ae206b73cdaa37 | 鲲 Galgame 表情包 | sticker.kungal.com |

如果将来接入第三方应用（比如某社区合作伙伴），新建的 client 默认 `auto_consent=false`，会渲染同意页让用户明确授权——这是 OAuth 协议的正确语义。

---

## 未来扩展（L2+）

**L2（未来）**：在 `/auth/register` 和 `/auth/login` 页面加 "Continue with Google / GitHub / Apple" 按钮，走标准 OIDC federation。**所有下游零代码自动支持**——这是 OAuth 集中架构最大的红利。

**L3+（待定）**：passkey / magic link / identifier-first flow 等，都在 OAuth 单点实现，下游不感知。

---

## 不在范围内（明确排除）

- ❌ 邀请码 / 内测注册——目前不限制
- ❌ 手机号注册——目前邮箱-only
- ❌ 用户名 vs 邮箱选择——目前 name + email 都必填
- ❌ 二次邮箱验证后才能登录——注册即登录，邮箱验证留给后续防滥用迭代
- ❌ ToS / 隐私政策点击确认——目前没有，加的话只在 OAuth web 这一处加

这些都是 L2+ 议题；L1 只做"注册流程从 legacy 完全迁移到 OAuth 托管"。
