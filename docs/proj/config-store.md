# 配置中心（环境变量与密钥）

moyu 的做法与 kungal 2026-11-18 的 `470cdbde3` / `62f8163b7` 相同，也跟随 infra #533/#535。moyu 用自己的表，不读 infra 或 kungal 的表。

## 模型

- `pkg/config.Load` 在启动时读每个变量一次，取值顺序是 **环境变量 > 配置中心 > 代码默认**。
- 配置中心是 `kungalgame_patch` 库里的 `config_secret` 表（迁移 060）：每个变量一行，用 `KUN_CONFIG_KEK` 做 AES-256-GCM 加密。`config_secret_audit` 记录每次写入、删除和导入；密钥只记 HMAC 指纹和末四位。
- 有两类变量不能存进配置中心：启动接线（`KUN_DATABASE_URL`、`KUN_CONFIG_KEK`）和拓扑变量（`*_HOST`、`*_PORT`、`*_ADDR`、`KUN_SERVER_MODE`）。这两类留在 compose 里。
- 后台 `/admin/setting` 的「环境变量与密钥」页（需要 `setting.manage`）会列出：
  - API 读过的每个变量，以及它的值来自哪一层；
  - 面板值和存储值一致、可以从面板删掉的变量（`panel_redundant`）；
  - 面板值覆盖了存储值的变量（`panel_overrides_store`）；
  - 存储值改过、要重启才生效的变量（`restart_pending`）；
  - 存着但没有代码读的变量（`not_read`）；
  - 容器环境里有、但代码不读的变量（幽灵变量）。
- `KUN_CONFIG_KEK` 不设置时，配置中心关闭：既不读取，也不能写入，行为和以前完全一样。

## 生产切换步骤

1. 合并部署这一版。迁移 060 会自动跑；`KUN_CONFIG_KEK` 还没设置，所以什么都不变。
2. 在 moyu 的 Dokploy Environment 里加 `KUN_CONFIG_KEK`（用 `openssl rand -base64 32` 生成）。这个值设一次就不要再换，换了之后已经存进去的值全部打不开。设完点 Deploy。
3. 先演练导入，只打印计划，不写入：

   ```sh
   sudo docker inspect kun-visual-novel-patch-ndxhtp-moyu-api-1 | \
     sudo docker compose -f docker-compose.prod.yml --profile jobs run --rm -T tools config-secrets import
   ```

   导入的是 API 读取的、名字像密钥的变量；没有代码读的变量不会导入。要额外导入非密钥变量，用 `-keys A,B` 指定。确认计划无误后，加上 `-apply` 再跑一次。
4. 打开 `/admin/setting` →「环境变量与密钥」。这时每个导入的变量都应标着「面板值可删除」。
5. 从 Dokploy 面板删掉 `OAUTH_CLIENT_SECRET`、`KUN_NEXTMOE_API_KEY`、`KUN_STORE_API_KEY`、`KUN_TRUST_CALLBACK_SECRET`、`KUN_BOT_SUBMIT_KEY`，然后重新部署。compose 里的 `${VAR:-}` 会展开成空串，空串等于没设，于是值改从配置中心取。最后，面板里只剩 `POSTGRES_PASSWORD` 和 `KUN_CONFIG_KEK`。
6. 收尾：删掉 compose 里标着 Transitional 的这 5 行 `${VAR:-}`，同时把 `KUN_CONFIG_KEK` 改成 `:?` 必填。之后 API 的 compose 只剩 `KUN_SERVER_MODE`、`KUN_DATABASE_URL`、`KUN_CONFIG_KEK`、`KUN_ACTIVITY_PUSH_ENABLED` 四行，web 一行都没有。其余值都是代码里的生产默认值，由 `TestProdDefaultsAreTheDeployment` 钉住。

## 回滚

- 要让某个变量临时回到面板取值：在面板里重新设置它。环境变量优先于配置中心，所以不用删存储值。
- 迁移 060 的 down 会删掉两张表。执行之前，先确认所有值都已经回到面板。

## 跨仓注意

infra 的 dev 快照脚本 `scripts/dev-snapshot/scrub/kungalgame_patch.sql` 需要清空 `config_secret` 和 `config_secret_audit`，写法同 infra #544 对 kungalgame 的处理。否则 dev 快照会带上用生产 KEK 加密的生产密钥。这一步要在 infra 仓里改。
