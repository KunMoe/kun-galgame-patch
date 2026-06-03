![kun-galgame-patch-next](./apps/web/public/kungalgame-trans.webp)

**Contact us：[Telegram](https://t.me/kungalgame) | [Discord](https://discord.com/invite/5F4FS2cXhX)**

图片来源于游戏 [方舟指令](https://apps.qoo-app.com/en/app/9593) 中的 `鲲`

# 鲲 Galgame 补丁

## 网站简介

鲲 Galgame 补丁是一个开源, 免费, 零门槛, 纯手写, 最先进的 Galgame 补丁资源下载站, 她为全体 Galgame 玩家提供 Galgame 补丁资源下载服务

下面是它的 GitHub 开源地址, 可以给我们点一个 star 支持哦

[https://github.com/KUN1007/kun-galgame-patch-next](https://github.com/KUN1007/kun-galgame-patch-next)

它是一个由开源社区驱动的, 属于 [KUN Visual Novel Website Cluster](https://nav.kungal.org), 遵从 [OpenGal](https://github.com/opengal) 原则的, 完全免费的, 非营利性的补丁资源下载网站

该网站的所有补丁资源由全体 Galgame 玩家提供, 免费, 无门槛的下载

**该网站不接受任何盗版 Galgame 本体资源, 以及 R18 补丁等 NSFW(not safe for work) 资源**

## 技术架构

> 本仓库已从早期的 Next.js 全栈架构迁移到 **Go (后端) + Nuxt 4 (前端)** 的 pnpm monorepo。下文描述的是当前版本的架构。

本项目是一个 **pnpm workspace monorepo**（`packageManager: pnpm@10.12.1`），由三个包组成：

| 包 | 名称 | 技术栈 | 说明 |
| --- | --- | --- | --- |
| `apps/api` | `@apps/api` | **Go 1.26 · Fiber v2 · GORM (pgx) · PostgreSQL · Redis** | 后端 API 服务，`/api/v1` 前缀，开发端口 `5214` |
| `apps/web` | `@apps/web` | **Nuxt 4 · Vue 3 (`<script setup>`) · Pinia · Tailwind 4 · Zod** | 前端 SSR 应用，开发端口 `6969` |
| `packages/ui` | `@kun/ui` | **Nuxt Layer** | 共享 UI 组件库（被 `apps/web` 以 layer 形式 `extends`） |

### 后端 `apps/api`

- **Go + Fiber v2** Web 框架，**GORM + pgx** 访问 PostgreSQL，**go-redis** 管理会话。
- 采用 5 层模块结构 `model → dto → repository → service → handler`，模块按业务域划分：`auth` / `patch` / `user` / `chat` / `message` / `admin` / `galgame` / `about` / `setting` 等。
- 纯 Go 图像处理（`disintegration/imaging` + `golang.org/x/image`，**无 CGO**），Markdown 渲染用 `goldmark`，定时任务用 `robfig/cron`，对象存储用 `minio-go`。
- 多个可执行命令位于 `apps/api/cmd/`：`server`（HTTP 服务）、`migrate`（SQL 迁移）以及若干一次性数据迁移/回填工具。
- SQL 迁移文件位于 `apps/api/migrations/`（`*.up.sql` / `*.down.sql` 成对）。

### 前端 `apps/web`

- **Nuxt 4 SSR**，仅通过 `useApi()` composable（对 `$fetch` 的薄封装）与 Go 后端通信，依赖 `moyu_session` httpOnly cookie 鉴权，SSR 阶段转发 cookie。
- 富文本编辑基于 **Milkdown** + **CodeMirror**，Markdown 解析/渲染走 **unified / remark / rehype**，公式用 **KaTeX**。
- 浏览器面向的公开配置（API base、OAuth 地址等）在**运行时**从 `NUXT_PUBLIC_*` 环境变量读取，因此同一份构建产物可在任意环境复用。

### 依赖的外部服务（不在本仓库内）

本项目是 [`kun-galgame-infra`](https://nav.kungal.org) 基础设施（infra）的**下游应用**，自身不拥有任何有状态服务，运行时按服务名连接 infra 提供的上游：

| 服务 | 默认端口 | 职责 |
| --- | --- | --- |
| KUN OAuth | `9277` | 身份认证唯一数据源（Authorization Code + PKCE） |
| Galgame Wiki Service | `9280` | 全部 Galgame 元数据（名称/简介/标签/会社…）与搜索的唯一数据源 |
| Image Service | `9278` | 内容寻址图床（上传返回 hash，URL 由 hash 推导） |
| PostgreSQL / Redis / S3(MinIO) | — | 数据库 / 会话缓存 / 对象存储 |

## 本地开发

### 环境要求

- **Node.js 22+** 与 **pnpm 10**（`corepack enable` 即可获得正确版本）
- **Go 1.26+**
- 可访问的 **PostgreSQL** 与 **Redis**，以及上述 OAuth / Wiki / Image 上游服务

### 安装与启动

```bash
# 安装前端依赖（pnpm workspace）
pnpm install

# 准备环境变量：分别复制并填写 apps/api 与 apps/web 的 .env
#   apps/api 至少需要 KUN_DATABASE_URL、Redis、OAuth、Image Service 等配置
#   apps/web 的公开配置见 nuxt.config.ts 中的 runtimeConfig.public

# 同时启动前后端（前端 :6969，后端 :5214）
pnpm dev

# 或单独启动
pnpm dev:web     # 仅 Nuxt 前端
pnpm dev:api     # 仅 Go 后端（air 热重载）
```

### 常用脚本（根目录）

```bash
pnpm build        # 构建 api + web
pnpm build:api    # 仅构建 Go 二进制
pnpm build:web    # 仅构建 Nuxt 产物

pnpm lint         # 前端 ESLint
pnpm typecheck    # 前端 nuxt typecheck (vue-tsc)
pnpm vet          # 后端 go vet
pnpm test:api     # 后端 go test ./...
pnpm format       # 前后端格式化（prettier / gofmt）
```

数据库迁移在 `apps/api` 内执行：构建并运行 `migrate` 命令（容器化部署见下文 `docker compose run --rm migrate`）。

## 部署

本项目使用 **Docker 多阶段构建**部署，所有镜像定义在 `docker/`，编排文件在仓库根目录：

- `docker/go.Dockerfile` — 参数化（`ARG CMD`）构建 Go 二进制，产物为 **distroless static (nonroot)** 镜像（约 45 MB）；健康检查复用二进制自带的 `healthcheck` 子命令（distroless 无 shell）。
- `docker/nuxt.Dockerfile` — 参数化（`ARG APP`）构建 Nuxt，运行阶段为 `node:22-slim` + 自包含的 `.output`（Nitro server）。
- `docker-compose.yml` — 开发/本机编排：`api` + `web` + `profiles: jobs` 的一次性 `migrate` 任务，加入 infra 的外部网络以解析 `postgres` / `oauth` / `galgame` / `image` 等服务名。
- `docker-compose.prod.yml` — 生产编排（GHCR 镜像 + Dokploy + Traefik）。

```bash
# 先确保基础设施 infra（postgres/redis/oauth/...）已启动且网络存在
cp docker/api.env.example docker/api.env   # 填写密钥（*.env 已 gitignore）
cp docker/web.env.example docker/web.env

docker compose up -d --build       # 启动 api + web
docker compose run --rm migrate    # 按需执行 SQL 迁移
```

详细的部署约定、infra 网络对接、图床 env 对齐说明见 [`docker/README.md`](./docker/README.md)。

### 持续集成

`.github/workflows/build.yml` 在推送到 `master` 时构建 `moyu-api` / `moyu-migrate` / `moyu-web` 三个镜像并推送到 **GHCR**，随后触发 Dokploy 重新部署。

## 网站原则

### 开源

本网站目前遵循 [AGPL-3.0 开源协议](https://www.gnu.org/licenses/gpl-3.0.en.html), 完全开源于 [GitHub](https://github.com/KUN1007/kun-galgame-patch-next)

本网站最大的目的是, 帮助全体 Galgame 玩家提供存档, 错误修正补丁等必要资源, 提供零门槛的获取渠道

以及对现代 Web 开发技术的研究, 例如 Go, Fiber, GORM, PostgreSQL, Redis, Nuxt 4, Vue 3, Pinia, Tailwind CSS 4, Zod, Milkdown, CodeMirror, Unified.js, S3 Object Storage, OAuth 2.0 (PKCE), Docker, distroless 镜像等等

对以上 Web 开发技术感兴趣的朋友们, 可以加入本文末尾的 Telegram 开发群组

### 免费

本网站永远不会出现任何付费下载, 付费积分制等行为, 我们是一个社区驱动的开源组织, 抵制一切收费行为

### 零门槛

一个好的互联网应当是开放的, 作为广大开源社区的一个角落, 我们向往开源, 合作, 友好, 透明化, 去中心化的社区环境

您不会看到诸如 `登录以下载`, `回复以下载`, `支付积分以下载` 等等封闭的环境

### 纯手写

我们的代码依旧是由一群热爱开源, 热爱开发技术, 热爱 Galgame 故事内涵的人们自己编写的, 而不是套用诸多现成的网站模板

网站的一笔一划, 都是我们手写的代码, 希望可以给您带来良好的体验！

我们向我们用到的开源工具, 以及使用我们网站的朋友们, 致以衷心的感谢!

### 最先进

1. 我们没有使用任何现成的网站模板, 而是使用我们上面提到的现代 Web 开发技术栈自行编写

2. 我们采用合作式编辑的方式编辑网站介绍, 尊重全体用户的思想, 该编辑方式是我们首创的

3. 我们采用 S3 对象存储以保证您的下载, 提供优质的, 免登录的, 无门槛的下载方式

## 加入 / 联系我们

您可以通过以下渠道联系到我们

### 鲲 Galgame 论坛 （推荐）

[www.kungal.com](https://www.kungal.com)

鲲 Galgame 论坛是世界上最萌的 Galgame 论坛, 提供同样先进的社区服务与企业级论坛业务支持, 本网站的一切原则鲲 Galgame 论坛均满足

您可以在鲲 Galgame 论坛的 `其它` 分区 -> `补丁网站` 分类发布新话题进行反馈, 例如反馈本网站的 BUG, 让您感觉到困惑的地方, 以及您觉得可以改进的地方, 您想要的功能, 请尽情反馈!

### Telegram （推荐）

如果您有 Telegram 账号, 可以加入

[https://t.me/kungalgame](https://t.me/kungalgame)

这是我们的社区群组, 群组内有大量活跃的, 热爱 Galgame 的朋友们, 以及我们的个别开发者, 他们都可以给您良好的支持!

### GitHub Issue （推荐）

除了以上的方式之外, 如果您对开源事业较为感兴趣, 您可以前往我们 GitHub 开源仓库的 Issue

[https://github.com/KUN1007/kun-galgame-patch-next/issues/new](https://github.com/KUN1007/kun-galgame-patch-next/issues/new)

在这里编写您的 Issue 并提交, 以创建一个新的 Issue, 这是要获得我们优先支持最有效的方式

## 关注我们

除此之外, 您还可以在下面的社交平台关注我们

[Twitter / X](https://twitter.com/kungalgame)

[Discord 服务器](https://discord.com/invite/5F4FS2cXhX)

[YouTube 频道](https://youtube.com/@kungalgame)

[Bilibili (中国大陆支持)](https://space.bilibili.com/1748455574)

## 开发联系

如果有对 Web 开发技术 (Go, Node.js, Nuxt, Next.js, SvelteKit, SolidStart 等) 感兴趣的朋友们, 可以加入我们的 Telegram 开发群组: [https://t.me/KUNForum](https://t.me/KUNForum)

## 开源声明 / 开源协议

本项目遵从 `AGPL-3.0` 开源协议
