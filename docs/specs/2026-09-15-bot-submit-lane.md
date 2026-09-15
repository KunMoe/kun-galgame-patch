# bot 投稿通道（s2s，供 letmoe-resource-forge 投汉化/修正补丁）

`/api/v1/bot/**`。`:id` 是 catalog work id（= 本站 galgame / patch 页 id）。

## 契约

- 认证：`Authorization: Bearer <KUN_BOT_SUBMIT_KEY>`，常量时间比较。与 letmoe 同一把 key、同一个 NextMoe 用户 id（`KUN_BOT_SUBMIT_USER_ID`）。
- fail-closed：两个变量缺一则路由不注册（404），并在启动日志里说明。

### 上传

`POST /bot/upload/init` → `POST /bot/upload/complete`，断点续传走 `POST /bot/upload/resume`。body 与 `/api/v1/upload/*` 完全一致，只是用 bot key 而不是会话鉴权，配额按 creator 档（单文件 5 GB / 每日 100 GB）。

站点资源只接受 `.zip` / `.rar` / `.7z`。

**没有 `/bot/upload/abort`**：`Abort` 按 uuid 删除制品且不校验归属，这不是该交给机器 key 的能力。中断的分片由制品服务的孤儿 GC 回收。

### coverage

`GET /bot/patches/:id/coverage` → `{"types": [...], "languages": [...]}`，均为该页现有资源（`status <> 2`）去重后的值。

判断「已经有汉化了吗」要同时看两个轴：`type` 里的 `manual` 同样可能是一份日文修正，光看 type 会误判。

### 投稿

`POST /bot/patches/:id/resources`，body：`name`, `size`, `artifact_uuid`, `type[]`, `language[]`, `platform[]`, `note`。`storage` 固定 `s3`。

- 长度/必填与站内 `PatchResourceCreateRequest` 同规格（`name` ≤300、`size` ≤107、`note` ≤10007、三个数组各 1–10 项）。
- artifact 必须对本站 OAuth client 存在且为 `n=ready`，**先校验再建页**：否则一次被拒的投稿会留下空补丁页 + moemoepoint + contributor 记录。
- `type` 闭集只开放 **汉化与修正**：`manual` / `ai` / `machine_polishing` / `machine` / `fix`。`crack`、`decensor`、`r18`、`mod`、`save`、`image`、`other` 一律 422 —— 这些要人工掂量。
- `language` / `platform` 走站点完整闭集。
- 页不存在时先 `CreatePatchByGalgameID` 再挂资源。

## 已知边界

bot 投稿的页面**不会**在 catalog 认领该 work。`claimOnFirstResource` 走 `POST /v2/me/claims`，需要发布者本人的 OAuth token，bot key 不是。页面在本站 `published=true`（`MarkIndexed`）正常可见，但 catalog 那边这个 work 仍未被 kungal 站点认领。要补齐得由 infra 发一个 bot 用户 token，不是代码层面能关掉的口子。

## 部署

moyu Dokploy Environment：

```
KUN_BOT_SUBMIT_KEY=<与 letmoe / forge FORGE_LETMOE_BOT_TOKEN 同值>
KUN_BOT_SUBMIT_USER_ID=<与 LETMOE_BOT_SUBMIT_USER_ID 同值>
```

`patch.user_id` 与 `patch_resource.user_id` 都有指向本地 `"user"(id)` 的外键，而本地用户行只在 OAuth 回调里补；bot 永远不走回调。启动时 `provisionBotUser` 会按 `KUN_BOT_SUBMIT_USER_ID` 幂等插入这一行，**无需手工建表数据**，但要确认启动日志里有 `bot submit lane enabled`；若看到 `provisioning the local user row failed`，投稿会全部撞 `patch_user_id_fkey`。
