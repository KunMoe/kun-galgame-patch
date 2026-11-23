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

## 生产切换（2026-11-23 已完成）

1. `#145` 上线，迁移 060 自动执行；`KUN_CONFIG_KEK` 写进 Dokploy 面板。
2. 用 `config-secrets import` 从运行中的容器导入 5 个密钥：`OAUTH_CLIENT_SECRET`、`KUN_NEXTMOE_API_KEY`、`KUN_STORE_API_KEY`、`KUN_TRUST_CALLBACK_SECRET`、`KUN_BOT_SUBMIT_KEY`。再跑一次演练，5 个都报 `same`，说明存储值和运行值一致。
3. 面板只留 `POSTGRES_PASSWORD` 和 `KUN_CONFIG_KEK`；compose 删掉 5 行过渡用的 `${VAR:-}`，并把 `KUN_CONFIG_KEK` 改成 `:?` 必填。

现在 API 的 compose 只有 `KUN_SERVER_MODE`、`KUN_DATABASE_URL`、`KUN_CONFIG_KEK`、`KUN_ACTIVITY_PUSH_ENABLED` 四行，web 一行都没有。其余值都是代码里的生产默认值，由 `TestProdDefaultsAreTheDeployment` 钉住。

## 以后怎么改

- **改密钥或新增密钥**：在 `/admin/setting`「环境变量与密钥」里写入，然后重启 moyu-api（在 Dokploy 点 Deploy）。
- **再从容器导入**：用下面的命令，先演练，确认后加 `-apply`。

  ```sh
  cd /etc/dokploy/compose/kun-visual-novel-patch-ndxhtp/code
  sudo docker inspect kun-visual-novel-patch-ndxhtp-moyu-api-1 | \
    sudo docker compose -p kun-visual-novel-patch-ndxhtp -f docker-compose.prod.yml \
    --profile jobs run --rm -T tools config-secrets import
  ```

  它只导入 API 读取的、名字像密钥的变量；要额外导入非密钥变量，用 `-keys A,B` 指定。

## 回滚

- 要让某个变量临时回到面板取值：在面板里设置它，并在 compose 里加一行 `${VAR}`。面板变量只有经过 compose 里的一行才能到达容器；环境变量优先于配置中心，所以不用删存储值。
- 迁移 060 的 down 会删掉两张表。执行之前，先确认所有值都已经回到面板。

## 跨仓注意

infra 的 dev 快照脚本 `scripts/dev-snapshot/scrub/kungalgame_patch.sql` 需要清空 `config_secret` 和 `config_secret_audit`，写法同 infra #544 对 kungalgame 的处理。否则 dev 快照会带上用生产 KEK 加密的生产密钥。这一步要在 infra 仓里改。
