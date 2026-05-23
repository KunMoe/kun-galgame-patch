# Token 与错误码

返回 [README](./README.md)

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

所有错误的响应体格式都是：

```json
{ "code": <int>, "message": "<中文消息>" }
```

`code = 0` 表示成功；其余按下面分类查阅。

### OAuth 错误 (15xxx)

| Code | HTTP | 消息 | 说明 |
|------|------|------|------|
| 15001 | 400 | 无效的客户端 | client_id 不存在 |
| 15002 | 400 | 无效的回调地址 | redirect_uri 未注册 |
| 15003 | 400 | 无效的授权码 | code 已过期 / 已使用 / 不存在 |
| 15004 | 400 | 无效的代码验证器 | PKCE code_verifier 不匹配 |
| 15005 | 400 | 无效的授权类型 | client 的 `grants` 列里没有当前 grant_type — **常见：admin 创建 client 时漏勾 `refresh_token`** |
| 15006 | 400 | 无效的权限范围 | 请求的 scope 不在 client 的 `allowed_scopes` 内 |
| 15007 | 400 | 访问被拒绝 | 用户拒绝授权 |
| 15008 | 400 | 无效的 client secret | confidential client 没传或填错 client_secret |
| 15009 | 400 | 需要 PKCE | public client 没传 code_verifier |

### 认证错误 (10xxx)

| Code | HTTP | 消息 | 说明 |
|------|------|------|------|
| 10001 | 401 | 未授权 | 未提供 Bearer Token |
| 10002 | 401 | 无效的令牌 | Token 格式错误 / 签名无效 / **refresh 时 client_id 与签发时的不匹配** |
| 10003 | 401 | 令牌已过期 | access_token 或 refresh_token 已过期，需要刷新或重新登录 |
| 10005 | 401 | 用户不存在 | UUID 对应的用户不存在（账号被硬删等罕见情况） |
| 10007 | 400 | 用户名已存在 | PATCH /auth/me 改 name 时与其他用户重复 |
| **10014** | **403** | **账号已封禁** | 用户被 admin 封号 — **前端应跳错误页（"账号被封禁"）而非登录页**，让用户再登也是同样的 403 |

### 通用错误

| Code | 消息 | 触发场景示例 |
|------|------|----|
| 1 | 请求格式错误 | JSON 语法错 / multipart 解析失败 / 上游服务（如 image_service）报错 |
| 7 | 参数验证失败 | 字段长度 / 格式校验未通过 |
| 8 | 缺少必要参数 | 必填字段没传（如 multipart 缺 `file`） |
| 9 | 参数无效 | 类型不对 / 超出范围（如 `ids` 个数 >100、`limit` 不是正整数） |
| 10 | 操作失败 | 一般是 DB 写入失败、外部服务异常等内部错误 |
