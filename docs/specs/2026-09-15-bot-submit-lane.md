# bot 投稿通道（s2s，供 letmoe-resource-forge 投汉化/修正补丁）

`GET /api/v1/bot/patches/:id/coverage` 与 `POST /api/v1/bot/patches/:id/resources`。`:id` 是 catalog work id（= 本站 galgame / patch 页 id）。

## 契约

- 认证：`Authorization: Bearer <KUN_BOT_SUBMIT_KEY>`，常量时间比较。与 letmoe 同一把 key、同一个 NextMoe 用户 id（`KUN_BOT_SUBMIT_USER_ID`）。
- fail-closed：key 或 user id 未配则路由不注册（404）。
- coverage：返回该作品现有资源的 `type` 去重列表（`status <> 2`）。Forge 在打包前用它判断「已经有汉化就跳过」。
- POST body：`name`, `size`, `artifact_uuid`, `type[]`, `language[]`, `platform[]`, `note`。`storage` 固定 `s3`。artifact 必须对本站 OAuth client 为 `StatusReady`。`type`/`language`/`platform` 走站点闭集词表，缺一 422。
- `type` 含 `crack` 一律 422。破解不走 bot。
- 页不存在时先 `CreatePatchByGalgameID` 再挂资源。

## 部署

moyu Dokploy Environment：

```
KUN_BOT_SUBMIT_KEY=<与 letmoe / forge FORGE_LETMOE_BOT_TOKEN 同值>
KUN_BOT_SUBMIT_USER_ID=<与 LETMOE_BOT_SUBMIT_USER_ID 同值>
```

补丁文件的 artifact 必须用 **moyu** 的 OAuth client 上传（`artifact_site_key=moyu`）。letmoe 键下的 uuid 在这边 404。
