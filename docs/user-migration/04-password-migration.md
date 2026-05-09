# 密码透明迁移

> 用户登录时不需要重置密码 —— 旧密码 hash 保留，首次登录成功时自动升级到新算法。

## 1. 三个密码列

OAuth `users` 表迁移后有三个密码相关列：

| 列 | 算法 | 来源 | 含义 |
|----|------|------|------|
| `password` | argon2id | 新（统一） | 新系统密码；初始为 NULL |
| `kungal_password` | bcrypt | kungal 源库的 `user.password` | 旧 kungal 密码 hash，保留 |
| `moyu_password` | argon2id (`salt_hex:hash_hex` 自定义格式) | moyu 源库的 `user.password` | 旧 moyu 密码 hash，保留 |

迁移完毕时 **每个用户**：

- `password = NULL`
- `kungal_password = <bcrypt hash>` 如果用户来自 kungal
- `moyu_password = <argon2id hash>` 如果用户来自 moyu
- 跨站合并的用户，两个都有

## 2. 登录流程

`AuthService.Login` 按下面顺序尝试，第一个成功的算法决定下一步：

```
输入：用户提供的明文密码 P

if user.password is not NULL:
    用 argon2id 验证 P 对 user.password
    成功 → 登录通过
    失败 → 拒登

else if user.kungal_password is not NULL:
    用 bcrypt 验证 P 对 user.kungal_password
    成功 → 登录通过 + 透明升级（见 §3）
    失败 → 继续尝试 moyu_password

else if user.moyu_password is not NULL:
    用 argon2id-with-salt 验证 P 对 user.moyu_password
    成功 → 登录通过 + 透明升级（见 §3）
    失败 → 拒登

else:
    拒登（"必须用邮箱重置密码"）
```

## 3. 透明升级（首次登录成功后）

旧密码验证成功的同时，后台执行：

```sql
UPDATE users SET
  password         = <argon2id(P)>,    -- 用统一算法重新 hash
  kungal_password  = NULL,             -- 清空
  moyu_password    = NULL              -- 清空
WHERE id = <userid>
```

下一次登录就走第一条分支（`password is not NULL`），不再触碰旧 hash。

**这件事每用户只发生一次**。用户感知不到 —— 还是输同一个密码就登进来了。

## 4. 跨站合并用户的密码处理

合并用户两边都保留 hash：

```
鲲: kungal 注册过（密码 P1, bcrypt hash A）+ moyu 注册过（密码 P2, argon2id hash B）
→ OAuth.users.kungal_password = A
→ OAuth.users.moyu_password   = B
```

登录时：

- 如果用户输入 P1 → 第二条分支（kungal_password）通过 → 升级为 P1 的 argon2id hash
- 如果用户输入 P2 → 第二条分支失败 → 第三条分支（moyu_password）通过 → 升级为 P2 的 argon2id hash

升级之后任意一个旧密码都能登（因为已升级到 password 列），但只能记住其中一个 —— 用户输哪个，就用哪个。

## 5. 找不到密码的情况

只有这条路：

> 用户从未在 kungal/moyu 登录过（或不记得密码）→ 没有任何 hash 可验证 → 必须走"忘记密码 → 邮箱验证 → 设置新密码"流程

迁移脚本不破坏这条路径。新密码进 `password` 列，bcrypt/argon2id 旧 hash 保持 NULL（或保持原样无害）。

## 6. 这套设计为什么靠谱

- **零密码重置压力** —— 1k+ 用户不用收"请重设密码"邮件
- **bcrypt 升级到 argon2id** —— 长期看更耐量子；短期看 cost factor 调起来更省 CPU
- **保留 hash 而不是明文** —— 不破坏密码学保证；旧 hash 作为只读 oracle 验证
- **首次登录后立即清空** —— 即使 OAuth 库被 dump，也没有"两套 hash 并存"的窗口

## 7. 有什么需要注意

### 算法版本号

`argon2id` 的参数（memory cost, time cost, parallelism）写死在代码里。如果以后调整：

- 已经升级到 password 列的用户不受影响（hash 自带参数信息）
- 还在 kungal_password / moyu_password 状态的用户登录时会按新参数重新 hash 进 password 列

### 重置密码流程

用户走"忘记密码"流程后，新密码直接覆写 `password` 列；同时清空 `kungal_password` / `moyu_password`（即使本来就是 NULL，也保持 idempotent）。

### bcrypt 慢

单个 bcrypt 验证大约 100-300ms（取决于 cost）。如果你担心首次登录潮峰：

- 实测中迁移完毕后 24h 内大约 30% 的活跃用户会触发首次登录
- 即使所有用户同一秒登录，也只是**单用户**的延迟问题，不会拖垮服务（每个登录请求独立）

## 8. 反向：从 OAuth password 回退到 kungal_password

不可能。argon2id hash 不能逆推回 bcrypt hash。一旦升级就升级了。

如果出现 OAuth `password` 列被错误清空的情况，旧 hash 仍在 `kungal_password` / `moyu_password`（前提是没被清空）—— 用户照样可以用旧密码登录。这是一份隐含的"保险"。

如果连旧 hash 也被清空了 —— 用户必须走"忘记密码"流程。
