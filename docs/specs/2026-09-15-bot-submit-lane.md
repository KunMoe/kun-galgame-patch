# bot 投稿通道（s2s，供 letmoe-resource-forge 投汉化/修正补丁）

`/api/v1/bot/**`。`:id` 是 catalog work id（= 本站 galgame / patch 页 id）。

## 契约

- 认证：`Authorization: Bearer <KUN_BOT_SUBMIT_KEY>`，常量时间比较。与 letmoe 同一把 key、同一个 NextMoe 用户 id（`KUN_BOT_SUBMIT_USER_ID`）。
- fail-closed：两个变量缺一则路由不注册（404），并在启动日志里说明。

### 上传

`POST /bot/upload/init` → `POST /bot/upload/complete`，断点续传走 `POST /bot/upload/resume`。body 与 `/api/v1/upload/*` 完全一致，只是用 bot key 而不是会话鉴权。配额走 bot 自己的档：单文件 20 GiB，每日 1 TiB。2026-10-14 从 creator 档（5 GB / 100 GB）调上来，因为 forge 要为 letmoe 合并下载重打包每一个可整合的补丁。制品服务对 moyu 站点另有每日配额，归 infra 管。

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

### repack（2026-10-14，letmoe「本体 + 补丁合并下载」）

forge 从 `/v2/moyu/resources/{id}/file` 取原文件，重打成确定性 STORE zip（去掉解压密码），经上面的上传接口传成 moyu 的 artifact，再挂回原资源。repack 归原资源与原作者所有，没有自己的页面、计数、通知或萌萌点。每个资源最多一个。

`PUT /bot/resources/:id/repack`，body：

```json
{"artifact_uuid":"…","source_artifact_uuid":"…","blake3":"<64 hex>","bytes":123,"file_count":42,"source_blake3":"<64 hex>"}
```

- `source_artifact_uuid` 必须等于资源**当前**的 `artifact_uuid`，否则 409：作者在 forge 干活期间换了文件，重读 `/file` 重打。过期判断按 uuid 而不是 BLAKE3，因为 moyu 自 2026-06-04 起不记 BLAKE3。
- 资源须为 `status=0` 的托管资源，否则 404。
- artifact 须是 bot 7 天内自己发起的上传、已完成、`public`、大小等于 `bytes`，且不是任何资源的原文件、不在待删队列里。否则 422。
- 同一 `artifact_uuid` 重放：200，原样返回，不改任何东西。换一个 artifact：替换旧行，旧 repack 进入待删队列。
- 返回 `{resource_id, artifact_uuid, source_artifact_uuid, blake3, bytes, file_count, source_blake3, created_at}`。

`DELETE /bot/resources/:id/repack` → `{resource_id, deleted}`；没有 repack 时 `deleted:false`，仍是 200。

**删除由数据库触发器负责**（迁移 047）。以下情况会把 repack 的 artifact 放进 `patch_resource_repack_purge`：

- repack 行被删：撤回、资源被删，包括页面或用户被删时的外键级联；
- repack 被替换；
- 资源换了文件。

API 每分钟清一次队列，失败按 1 分钟起翻倍退避、封顶 1 小时，404 视为已删。infra 收到 moyu 的 artifact 删除后，会连带删掉钉住它的 letmoe 组合。资源被禁用（1）或隐藏（2）时 repack **不删**：`/file` 此时回 404，由 letmoe 自己下线组合，资源恢复后再用同一计划重建。

### normalize（2026-10-15，托管补丁统一为 zip）

站长裁定：每个托管（`storage=s3`）资源的文件都换成 forge 重打的无解压码 zip，原文件删除。zip 里只有一个顶层目录，作者打包的每个文件都原样保留，说明文件也在内，不做垃圾文件清理。每个文件按压缩收益选择 Deflate 或 STORE。作者、上传与更新时间、更新历史、下载数、备注都不动，只换文件、清解压码。之后的新上传和替换走同一条通道：照常发布，forge 几分钟内把文件换掉。

队列不落表。一个 `status=0` 的托管资源，只要它当前的文件既没有 job、也不是 forge 产物，就是待处理。所以新上传和换文件会自动进入队列，任何写路径都不用管。job 在 `patch_resource_normalize`（迁移 050），每个源文件一行，以源文件的 `artifact_uuid` 为键。

`POST /bot/normalize-jobs/claim`，body `{"limit": 1..20, "resource_ids": [...]}`（`limit` 默认 1；`resource_ids` 可选，最多 20 个，只在这些资源里挑，供 canary 用），返回：

```json
{"items":[{"resource_id":1,"galgame_id":2,"artifact_uuid":"…","download_url":"…","filename":"汉化补丁 v1.2.rar","password":"…","bytes":123,"repack":null,"lease_expires_at":"…"}]}
```

- 租约 6 小时，每次 claim 记一次尝试。第 5 次租约过期后，job 自动变成 `failed` / `abandoned`。临时错误（网络、OOM）不要调 fail，等租约自然过期即可。
- 带 repack 的资源先发（提升，`repack` 非 null）：不用上传，直接释放原文件的空间。其余按最近编辑排序，所以新上传会插到回填前面。
  **2026-10-15 起暂停**：infra 的改名只改了它库里的名字，B2 对象的 Content-Disposition 没变，6379 和 418 下载时仍叫「(letmoe repack).zip」。修好之前 claim 不下发提升（`holdPromotions`）；已经领取的提升照常可以 complete。
- 不带 repack 的资源，每个 UTC 日最多 claim 2,400 次。每次这样的 claim 对应 forge 的一次上传 Init，而 moyu 在制品服务的配额是每天 3,000 次 Init，读者的上传也算在内。提升不上传，不计入这个上限。
- 制品服务里已经没有的源文件不会下发，直接记为 `failed` / `source_missing`。
- `filename` 是作者上传时的文件名。forge 上传产物时用它的主名加 `.zip`。

`POST /bot/normalize-jobs/:id/complete`，body：

```json
{"artifact_uuid":"…","source_artifact_uuid":"…","filename":"汉化补丁 v1.2.zip","bytes":123,"blake3":"<64 hex>","file_count":42,"has_archives":false}
```

- `artifact_uuid` 是产物：bot 7 天内自己发起的上传，或者该资源当前的 repack（提升）。它必须已完成、`public`、大小等于 `bytes`、名字以 `.zip` 结尾，且不是任何资源的文件、别的资源的 repack、待删队列里的 artifact 或别的 job 的产物。否则 422。
- 资源当前文件已不是 `source_artifact_uuid` 时回 409，并由 moyu 把产物放进待删队列，forge 不用管。
- 同一对 (artifact_uuid, source_artifact_uuid) 重放：200，原样返回。同一个源文件换一个产物：409。
- 同一事务里依次：job 记为 `done`，把被覆盖的解压码和大小存进 job 行；`artifact_uuid` 换成产物；`size` 写成 `"%.3f MB"`，与发布表单一致；`blake3` 写产物的哈希；`has_archives=false` 时清空解压码。`update_time`、`updated`、备注、计数、`patch_resource_file_history`、`patch_resource_revision` 都不碰，也不发通知。最后原文件进待删队列，但 30 天后才真正调 DELETE，之后 infra 再软删 7 天。原文件是作者唯一的副本，而 forge 包的文件名解码问题可能几周后才暴露：2026-09-29 发布的 4,178 个 forge 包里，有 1,243 个在部分语言环境下无法正常读取。
- 交换成功后，moyu 把产物改名为 `filename`（`PATCH /api/v1/artifacts/{uuid}`），改名失败时 complete 返回错误但交换已生效。forge 重放 complete 即可补上改名。改名要在制品服务里原地重写对象，最大 5 GiB，可能需要几分钟。
- 提升 repack 时，047 的触发器会在文件变更时删掉 repack 行。050 让它不再把「仍是某个资源文件」的 artifact 放进待删队列，否则刚换上的文件会被删掉。如果带 repack 的资源上传了新产物，旧 repack 照常删除，钉住它的 letmoe 组合也一并删除。

`POST /bot/normalize-jobs/:id/fail`，body `{"source_artifact_uuid":"…","code":"wrong_password","message":"…"}`：对这个文件是终态，原文件保留。只有作者换了文件，或运维删掉 job 行，才会重新下发。没有 claim 过的文件回 404，已完成的回 409。

变更流把这次换文件记为 `normalized`，不是 `file_replaced`，否则 letmoe 会重建约 300 个合并下载。`/v2/moyu/resources/{id}/file` 对 forge 产物回 `normalized: true`，此时文件本身就是合并层，`repack` 为 null，对它再 PUT repack 会回 422。

**回滚**：job 行永久保存原文件的 uuid（主键）、解压码和大小。交换后 30 天内原文件还在，回滚不需要 infra。超过 30 天但在 37 天内，先请 infra 恢复原 artifact。然后：

1. `UPDATE patch_resource SET artifact_uuid = <source_artifact_uuid>, password = <source_password>, size = <source_size>, blake3 = '' WHERE id = <resource_id>`。变更流会记一条 `file_replaced`，letmoe 据此重建。
2. 把产物放进 `patch_resource_repack_purge`。原文件的待删行不用管：清理任务不会删除任何仍是某个资源文件的 artifact，会直接丢弃这类待删行。
3. 把 job 行改成 `state='failed', code='reverted'`，这个文件就不会再被下发。

## 已知边界

bot 投稿的页面**不会**在 catalog 认领该 work。`claimOnFirstResource` 走 `POST /v2/me/claims`，需要发布者本人的 OAuth token，bot key 不是。页面在本站 `published=true`（`MarkIndexed`）正常可见，但 catalog 那边这个 work 仍未被 kungal 站点认领。要补齐得由 infra 发一个 bot 用户 token，不是代码层面能关掉的口子。

## 部署

moyu Dokploy Environment：

```
KUN_BOT_SUBMIT_KEY=<与 letmoe / forge FORGE_LETMOE_BOT_TOKEN 同值>
KUN_BOT_SUBMIT_USER_ID=<与 LETMOE_BOT_SUBMIT_USER_ID 同值>
```

`patch.user_id` 与 `patch_resource.user_id` 都有指向本地 `"user"(id)` 的外键，而本地用户行只在 OAuth 回调里补；bot 永远不走回调。启动时 `provisionBotUser` 会按 `KUN_BOT_SUBMIT_USER_ID` 幂等插入这一行，**无需手工建表数据**，但要确认启动日志里有 `bot submit lane enabled`；若看到 `provisioning the local user row failed`，投稿会全部撞 `patch_user_id_fkey`。
