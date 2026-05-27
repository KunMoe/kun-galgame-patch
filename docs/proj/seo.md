# SEO 与可索引性

> 本文档说明 moyu 前端每个页面的 SEO 状态、JSON-LD 注入情况、被禁掉 SEO 的页面与原因，以及 SEO 与 NSFW 过滤的耦合关系。**任何 SEO 相关改动都必须先读本文档**，因为搜索引擎一次错误索引（特别是把站点判定为 adult content）会造成持久的自然流量损失。

## TL;DR

- **两条路径**：可索引页用 `useKunSeoMeta({...})` 注入完整 SEO；不可索引页用 `useKunDisableSeo(title)` 强制 `noindex,nofollow` + 清空所有 og/twitter/JSON-LD 字段。
- **JSON-LD 默认**由 `nuxt-schema-org` 模块自动注入 `WebSite + WebPage + Organization`。`useKunDisableSeo` 会把它们替换为空壳（`host: '', path: ''`）保证不被爬虫拾取。
- **NSFW 必须禁 SEO**：所有承载 NSFW 内容（或可能承载）的页面，渲染时都要走 disable 分支 — SEO 是给搜索引擎看的，**爬虫拿到 NSFW = 站点被打成 adult**。
- **404 / 加载失败也禁 SEO**：缺数据时显示的 stub 页不该被索引（避免大量 thin pages 被索引）。
- **56 个 pages**：15 个开 SEO，31 个禁 SEO，10 个不声明（redirect 中间件 / 嵌套子 tab 由父 layout 处理）。

---

## 1. 全局 SEO 基础设施

### 1.1 站点元信息源

`apps/web/app/config/moyu-moe.ts` 是站点级 SEO 单一真理源：

| 字段 | 用途 |
|---|---|
| `title` / `titleShort` / `template` | 站名 + page title 模板（`'%s - 鲲 Galgame 补丁'`） |
| `description` | 默认 description（首页用） |
| `keywords[]` | 全局关键词数组，被 `useKunSeoMeta` toString 后注入每个页面 |
| `canonical` | `https://www.moyu.moe` 作为 canonical 主域 |
| `author[]` / `creator` / `publisher` | schema.org Organization 数据 |
| `domain.main` / `imageBed` / `storage` | URL 生成基础 |
| `og.title` / `og.description` / `og.image` | Open Graph 默认值 |

修改任何站名 / 关键词 / 主域 → 改这一处。**不要**在 page 内 hard-code 站名。

### 1.2 useKunSeoMeta — 可索引页的入口

`apps/web/app/composables/useKunSeoMeta.ts` 接受 `{title, description, ogType?, ogImage?}`，自动注入：

| 字段 | 注入逻辑 |
|---|---|
| `title` | `${input.title} - 鲲 Galgame 补丁`（自动拼模板） |
| `description` | 透传 `input.description` |
| `keywords` | `kunMoyuMoe.keywords.toString()` 全局值（不可 per-page 覆盖） |
| `og:url` | `${kunMoyuMoe.domain.main}${route.path}` — 当前页路径 |
| `og:type` | `input.ogType || 'website'` |
| `og:title` / `og:description` | 复用 title / description |
| `og:image` / `og:image:alt` | `input.ogImage || kunMoyuMoe.images[0].url || '/kungalgame.webp'` |
| `twitter:card` | `summary_large_image` |
| `twitter:title` / `description` / `image` | 镜像 og.* |
| `<link rel="canonical">` | `${domain.main}${route.path}` |

设计原则：
- **keywords 是全局的**：单页无法添加 page-specific keyword。SEO 多样性靠 title + description 文本。
- **canonical 用当前 path**：分页 `?page=2` 不被 canonical 化（页码本身就是不同内容）。
- **og:type=article 用于详情页**：列表 / 工具页保持 `website`。

### 1.3 useKunDisableSeo — 不可索引页的入口

`apps/web/app/composables/useKunDisableSeo.ts` 接受 `title: string`（仅用于浏览器标签显示），强制注入：

```html
<meta name="robots" content="noindex, nofollow">
<meta name="title" content="">
<!-- og:* / twitter:* 全部空字符串或 undefined -->
```

且 schema-org 也被替换为空壳：

```ts
useSchemaOrg([defineOrganization({}), defineWebSite({}), defineWebPage()])
// templateParams.schemaOrg = { host: '', path: '', inLanguage: 'zh-Hans' }
```

→ `nuxt-schema-org` 输出的 JSON-LD 是结构正确但内容全空的占位符 — 爬虫无法从中提取任何信息。

**为什么不直接不调 useSchemaOrg？** 因为 `nuxt-schema-org` 模块默认会自动注入 WebSite/WebPage schema 基于全局站点配置。不显式覆盖的话，禁 SEO 的页面仍会有一份"站名 + 当前 URL"的 JSON-LD —— 这虽然不会让 NSFW 内容暴露，但会让 noindex 页面被搜索引擎"认知到存在"。显式空壳 = 彻底切断信息流。

### 1.4 JSON-LD (Schema.org) — 模块层默认注入

`nuxt.config.ts` 启用 `nuxt-schema-org` 模块。

| 行为 | 默认页 (useKunSeoMeta) | 禁 SEO 页 (useKunDisableSeo) |
|---|---|---|
| `Organization` schema | 自动从全局生成（站名 / logo / sameAs 等） | 空壳 |
| `WebSite` schema | 自动从全局生成（站名 / URL / search action） | 空壳 |
| `WebPage` schema | 自动基于 useHead/useSeoMeta meta 生成（title / description / url） | 空壳 |
| `Article` / `BreadcrumbList` 等富类型 | **当前未在任何 page 显式声明** —— 详见 §5 |
| 全局 `templateParams.schemaOrg` | `{ host, path, inLanguage }` 由 nuxt-schema-org 自动填充 | `{ host: '', path: '', inLanguage: 'zh-Hans' }` |

→ 当前所有可索引页都拿到自动生成的 `Organization + WebSite + WebPage` 三件套，**没有任何 page 注入富文本类型**（如 `Article` 详情页、`BreadcrumbList` 面包屑、`SearchAction` 站内搜索）。这是 §6 "未来改进项" 的一项。

### 1.5 robots.txt + sitemap

- `apps/web/public/robots.txt` 当前为 `User-Agent: *` + `Disallow:` —— 即允许爬一切。
- **没有 sitemap.xml**。爬虫只能通过链接发现页面。

→ 这意味着 `noindex` 元标签是阻止索引的**唯一手段**，所以 `useKunDisableSeo` 的正确性至关重要。**生成 sitemap.xml 列出 SFW 详情页 URL** 是后续优化项（详见 §6）。

---

## 2. SEO ↔ NSFW 过滤的耦合关系

这是最关键的设计交互。**NSFW 的设计目标是给搜索引擎看的**，下游若干处的"应当 SEO / 应当不 SEO"决策直接由 NSFW 状态决定。完整的 NSFW 协议见 [`docs/galgame_wiki/00-handbook-for-downstream.md` §16](../galgame_wiki/00-handbook-for-downstream.md) 和 [`docs/proj/`](.) 内的实施记录。

### 2.1 决策矩阵

**重要原则**：登录态对 NSFW 可见性的影响**仅作用于 patch / resource 详情页**，不影响列表页。这是产品规则 — "页面上的各种游戏列表只有用户打开显示全部内容才会显示"。useApi 内部通过 `/^\/(patch|resource)\/\d+/` 正则匹配 route.path 区分详情与列表。

| 页面类型 | 用户身份 | NSFW 模式（cookie） | useApi 发出的 content_limit | 后端返回 | SEO 决策 |
|---|---|---|---|---|---|
| Patch / Resource 详情 | 匿名 | sfw | sfw | 404 | useKunDisableSeo（NSFW 确认 placeholder） |
| Patch / Resource 详情 | 匿名 + 已 ack | sfw | **all**（ack 命中） | 200 + NSFW 数据 | useKunDisableSeo（数据可见但不索引） |
| Patch / Resource 详情 | 匿名 | nsfw/all | 透传 cookie 值 | 200 + NSFW 数据 | useKunDisableSeo |
| Patch / Resource 详情 | 登录 | sfw | **all**（detail-route + 登录绕过） | 200 + 任意数据 | 视 content_limit：SFW → useKunSeoMeta；NSFW → useKunDisableSeo |
| Patch / Resource 详情 | 登录 | nsfw/all | 透传 cookie 值 | 200 + 任意数据 | 同上 |
| 列表页（home / galgame / resource / comment / ranking） | 匿名 | sfw | sfw | SFW 子集（后端 filter） | useKunSeoMeta |
| 列表页 | **登录** | sfw | **sfw**（登录不绕过列表！） | SFW 子集 | useKunSeoMeta |
| 列表页 | 任意 | nsfw/all | 透传 | 含 NSFW | useKunSeoMeta — 但 NSFW 模式开启的用户视图，搜索引擎拿不到此 cookie 状态 |
| User 主页 / 子 tab（/user/:id/*） | * | sfw | sfw（不在 detail-route 范围） | SFW 子集 | useKunSeoMeta（loaded）/ useKunDisableSeo（404） |
| Tag / Official / Engine / Series 详情 | * | sfw | sfw（不在 detail-route 范围） | wiki 端默认 sfw filter | useKunSeoMeta（loaded）/ useKunDisableSeo（404） |
| 搜索页 | * | * | n/a（后端 hard-code all） | 全集（含 NSFW） | useKunDisableSeo（避免动态 URL + NSFW 暴露） |

> **登录用户 ≠ 看到所有 NSFW**：登录只是让"直接打开 NSFW 详情 URL"不需要点确认按钮。要在列表页看到 NSFW 仍然必须从顶栏切换 NSFW 模式 (cookie `kunNsfwEnable` 改成 `all` 或 `nsfw`)。这是产品定的边界，也避免登录用户的列表页突然多出大量 NSFW 卡片造成不适。

### 2.2 "拿到数据但仍禁 SEO" 的几种情况

- **NSFW patch 详情 + 已登录**：用户能正常看到完整页面，但 `useKunDisableSeo` 阻止搜索引擎索引该 URL。是否 NSFW 由 `patch.content_limit === 'sfw'` 判断（字段来自 wiki，由 enricher restamp 到 GalgameCard）。
- **NSFW patch 详情 + 匿名 + ack**：用户点过"我已知晓"按钮，cookie 记录了 `nsfwAckedIds`。用户能看，但页面仍 noindex。
- **真 404**：详情页拿不到数据（patch 不存在 / id 错），不论 NSFW 与否都禁 SEO（避免 thin 404 stub 被索引）。

### 2.3 字段来源（D12 后必须从 wiki 取）

D12 (2026-04-21) 后所有 galgame 元数据归 wiki。SEO 用到的字段映射：

| 字段 | 数据源 | 在 moyu 哪里可拿 |
|---|---|---|
| `patch.name` (KunLanguage 对象) | wiki | enricher.applyGalgame 写入 GalgameCard.name |
| `patch.banner` (URL) | wiki | enricher.applyGalgame 写入 GalgameCard.banner |
| `patch.content_limit` (sfw/nsfw) | wiki | enricher.applyGalgame 写入 GalgameCard.content_limit |
| `patch.galgame.intro_*` | wiki | EnrichPatchDetail 才有（GalgameCard 不含） |
| `user.name` / `user.avatar` / `user.bio` | OAuth | user.handler.GetUserInfo 直接转发 |
| `tag.name` / `tag.galgame_count` | wiki | tag detail 端点直接返回 |
| `official.name` / `official.galgame_count` | wiki | official detail 端点直接返回 |

**已废弃字段（不要再 SEO 引用）**：
- `patch.introduction` / `patch.alias` / `patch.engine` / `patch.original_language` — 全部移到 wiki，本地表无字段。如果旧代码还在引用，会拿到 undefined / 空字符串。

本次审计已确认所有 SEO 调用站点的字段引用均为 D12 后正确版本（参见 §3 逐页表中的"字段来源"列）。

---

## 3. 逐页 SEO 清单

> 表格按路由路径排序。"状态" 一列：
> - 🟢 **SEO** — 调用 `useKunSeoMeta` 生成完整 SEO meta + JSON-LD
> - 🔴 **NO SEO** — 调用 `useKunDisableSeo`，注入 `noindex,nofollow` + 空 JSON-LD
> - ⚪ **N/A** — 不声明（redirect 中间件 / 嵌套子路由由父 layout 处理）
>
> "JSON-LD" 一列：当前所有页面只有 nuxt-schema-org 自动生成的 `Organization + WebSite + WebPage` 三件套；详情页 / 文章页 / 面包屑等富类型暂未注入（见 §6）。所以列出的是"自动 schema 是否含真实内容"。

### 3.1 首页与公开浏览

| 路径 | 状态 | JSON-LD | Title | Description (摘要) | 字段来源 / 备注 |
|---|---|---|---|---|---|
| `/` | 🟢 SEO | ✅ 实质内容 | `首页 - ...` | 站点定位（开源 / 免费 / 多平台补丁下载） | 静态文案 |
| `/galgame` | 🟢 SEO | ✅ 实质内容 | `Galgame 列表 - ...` | 收录 + 排序 + 多平台筛选关键词 | 静态文案 |
| `/comment` | 🟢 SEO | ✅ 实质内容 | `最新评论 - ...` | 全站评论流引导 | 后端已 NSFW filter，payload 安全 |
| `/resource` | 🟢 SEO | ✅ 实质内容 | `最新补丁资源 - ...` | 多平台 / 翻译类型关键词 | 后端已 NSFW filter，payload 安全 |
| `/ranking` | ⚪ N/A | — | — | — | redirect → `/ranking/user` |
| `/ranking/patch` | 🟢 SEO | ✅ 实质内容 | `Galgame 补丁排行榜 - ...` | 浏览量 / 下载量 / 收藏数排序 | 后端已 NSFW filter |
| `/ranking/user` | 🟢 SEO | ✅ 实质内容 | `用户排行榜 - ...` | 萌萌点 / 贡献排名 | 仅用户信息，无 NSFW 风险 |
| `/friend-link` | 🟢 SEO | ✅ 实质内容 | `友情链接 - ...` | 同好站点导航 | 静态文案 |
| `/check-hash` | 🟢 SEO | ✅ 实质内容 | `BLAKE3 文件校验工具 - ...` | 在线 BLAKE3 校验工具说明 | 静态文案 |

### 3.2 详情页（条件式：数据状态决定 SEO 开关）

| 路径 | 状态 | JSON-LD | 触发分支 |
|---|---|---|---|
| `/patch/[id]` | 🟢 / 🔴 (条件) | 条件 | `patch.value && patch.value.content_limit === 'sfw'` → 🟢 SEO（title=patch.name，desc 含"中文补丁 / 汉化补丁 / AI 翻译补丁"长尾，og:image=banner）<br>其他全部分支 → 🔴 NO SEO（NSFW patch 实际可见但 noindex / 404 stub / NSFW 确认 placeholder） |
| `/patch/[id]/index` | ⚪ N/A | — | 嵌套子，父 layout `patch/[id].vue` 已处理 |
| `/patch/[id]/introduction` | ⚪ N/A | — | 同上 |
| `/patch/[id]/comment` | ⚪ N/A | — | 同上 |
| `/patch/[id]/resource` | ⚪ N/A | — | 同上 |
| `/patch/[id]/revisions` | ⚪ N/A | — | 同上 |
| `/patch/[id]/prs` | ⚪ N/A | — | 同上 |
| `/resource/[id]` | 🟢 / 🔴 (条件) | 条件 | `detail && resource && owningPatch && owningPatch.content_limit === 'sfw'` → 🟢 SEO（title 为 composed 长尾："{游戏名}{平台}{语言}{模型}{类型}资源下载"；desc 用 note_html 文本；og:image=banner）<br>其他 → 🔴 NO SEO |
| `/tag/[id]` | 🟢 / 🔴 (条件) | 条件 | `tag.value` 存在 → 🟢（title=`标签 · ${name}`，desc 含 galgame_count）<br>null → 🔴 |
| `/official/[id]` | 🟢 / 🔴 (条件) | 条件 | 同 tag pattern |
| `/user/[id]` | 🟢 / 🔴 (条件) | 条件 | `user.value && user.value.name` → 🟢（title=`${name} 的主页`，desc=`bio` 或 fallback 文案，og:image=avatar）<br>null → 🔴 |
| `/user/[id]/index` | ⚪ N/A | — | 嵌套子，父 layout `user/[id].vue` |
| `/user/[id]/galgame` | ⚪ N/A | — | 同上 |
| `/user/[id]/resource` | ⚪ N/A | — | 同上 |
| `/user/[id]/comment` | ⚪ N/A | — | 同上 |
| `/user/[id]/contribute` | ⚪ N/A | — | 同上 |
| `/user/[id]/favorite` | ⚪ N/A | — | 同上 |
| `/about` | 🟢 SEO | ✅ 实质内容 | `关于我们 - ...` 静态描述 |
| `/about/[...slug]` | 🟢 SEO | ✅ 实质内容 | 用 mdx frontmatter 的 title + description；缺 description 时拼"${title} - 鲲 Galgame 补丁站"；frontmatter.banner 当 og:image。**如果 mdx 不存在，前置 `createError(404)` throw fatal 让 Nuxt error page 接管**，根本不会执行 useKunSeoMeta。 |

### 3.3 搜索

| 路径 | 状态 | JSON-LD | 原因 |
|---|---|---|---|
| `/search` | 🔴 NO SEO | 空壳 | 后端 `/api/search` 故意不应用 NSFW 过滤（搜索是用户主动行为，应返回完整结果集）。爬虫扫到 `?q=<nsfw 词>` 时仍能从 SSR HTML 拿到 NSFW 结果。disable SEO 双重保险：(1) `/search?q=...` URL 不被索引；(2) 即便有人指到该 URL，爬虫读到 noindex 就不会拾取 payload。 |

### 3.4 私密 / 写入 / 中转页（一律禁 SEO）

| 路径 | 状态 | JSON-LD | 原因 |
|---|---|---|---|
| `/account-banned` | 🔴 NO SEO | 空壳 | 错误页，不该被索引（避免"账号被封禁"snippet 出现在搜索结果） |
| `/auth/callback` | 🔴 NO SEO | 空壳 | OAuth 中转页，仅交换 code 然后 redirect，无内容可索引 |
| `/check-hash` | 🟢 SEO | ✅ | （已在 §3.1 — 工具页应索引以引流） |
| `/settings/user` | 🔴 NO SEO | 空壳 | 账户设置 — 仅 owner 可见，索引无意义且隐私风险 |
| `/me/submissions` | 🔴 NO SEO | 空壳 | 私人 wiki 提交记录（含 pending / declined 草稿） |
| `/edit` | ⚪ N/A | — | redirect → `/edit/create` |
| `/edit/create` | 🔴 NO SEO | 空壳 | 发布 Galgame 写入页 — 表单不该被索引 |
| `/edit/draft` | 🔴 NO SEO | 空壳 | 草稿编辑 |
| `/edit/rewrite` | 🔴 NO SEO | 空壳 | Galgame 元数据编辑 |
| `/galgame/taxonomy` | 🔴 NO SEO | 空壳 | Tag / Official / Engine / Series CRUD — wiki §15.1 admin/editor-only |

### 3.5 消息中心（10 个 page，全部禁 SEO）

| 路径 | 状态 | JSON-LD | 原因 |
|---|---|---|---|
| `/message` | 🔴 NO SEO | 空壳 | 消息中心父 layout |
| `/message/index` | ⚪ N/A | — | redirect → `/message/notice` |
| `/message/notice` | 🔴 NO SEO | 空壳 | 通知收件箱 |
| `/message/follow` | 🔴 NO SEO | 空壳 | 关注消息 |
| `/message/mention` | 🔴 NO SEO | 空壳 | @ 消息 |
| `/message/system` | 🔴 NO SEO | 空壳 | 系统消息 |
| `/message/patch-resource-create` | 🔴 NO SEO | 空壳 | 订阅的新补丁通知 |
| `/message/patch-resource-update` | 🔴 NO SEO | 空壳 | 订阅的补丁更新通知 |
| `/message/chat` | 🔴 NO SEO | 空壳 | 私聊列表 |
| `/message/chat/[link]` | 🔴 NO SEO | 空壳 | 私聊会话 — 包含双方私人对话内容 |

→ 所有消息相关页面共同特点：**per-user inbox，无公共内容**。索引这些页面对 SEO 无价值（爬虫拿不到登录态，看到的就是空 inbox 或登录 prompt），且消息内容可能引用 NSFW patch 链接造成间接泄漏。

### 3.6 管理后台（8 个 page，全部禁 SEO）

| 路径 | 状态 | JSON-LD | 原因 |
|---|---|---|---|
| `/admin` | 🔴 NO SEO | 空壳 | 管理面板父 layout |
| `/admin/index` | 🔴 NO SEO | 空壳 | 统计面板（user_count / galgame_count 等内部数字，禁泄） |
| `/admin/galgame` | 🔴 NO SEO | 空壳 | 全 patch 列表（含 NSFW，admin 视图按设计应看全集） |
| `/admin/comment` | 🔴 NO SEO | 空壳 | 评论管理（含 NSFW patch 的评论） |
| `/admin/resource` | 🔴 NO SEO | 空壳 | 资源管理（含 NSFW） |
| `/admin/log` | 🔴 NO SEO | 空壳 | 审计日志（内部操作记录） |
| `/admin/orphans` | 🔴 NO SEO | 空壳 | 孤儿补丁列表（数据修复工具） |
| `/admin/setting` | 🔴 NO SEO | 空壳 | 网站设置切换 |

→ 后端路由全 `moderatorAuth` 守护，爬虫拿不到数据，但加 `noindex` 是兜底（防止某次 middleware 出 bug 时数据暴露）+ 避免 admin URL 出现在搜索引擎"sitelinks" 里污染观感。

### 3.7 完整汇总

```
56 个 pages 总数
├─ 🟢 SEO 启用            15 (公开浏览页 + 详情页的"数据 OK + SFW"分支)
├─ 🔴 SEO 禁用            31 (私密/admin/message/edit/settings/search/错误页)
└─ ⚪ N/A                 10 (redirect 中间件 3 + 嵌套子 tab 7)
                          + 6 个详情页根据状态在 🟢 / 🔴 切换
```

详情页（patch/resource/tag/official/user/about-slug）的 SEO 状态在运行时由数据决定，不固定一种 — 上表条件式行展示了切换逻辑。

---

## 4. SEO 启用页的 og:image 策略

| Page | og:image 来源 |
|---|---|
| `/patch/[id]` (SFW loaded) | `resolveBannerUrl(patch.value)` — 优先 `effective_banner_hash` 拼 image_service CDN URL，fallback 到 `patch.banner` 老 URL |
| `/resource/[id]` (SFW loaded) | `resolveBannerUrl(detail.patch, 'mini')` — 同上但用 `mini` variant（460×259） |
| `/user/[id]` (loaded) | `user.value.avatar` — OAuth 头像 URL |
| `/about/[...slug]` (loaded) | `frontmatter.banner` — mdx 自带 |
| 其他 | fallback 到 `kunMoyuMoe.images[0].url || '/kungalgame.webp'` |

注意：useKunSeoMeta 不接受 `ogImage` 为空字符串 — 必须 `undefined` 才能 fallback 到默认。所以 page 内传 `bannerSrc.value || undefined` 而不是 `bannerSrc.value || ''`。

---

## 5. JSON-LD 当前覆盖与缺口

### 5.1 当前已注入（所有 useKunSeoMeta 页）

由 `nuxt-schema-org` 模块自动生成：

```json
{
  "@context": "https://schema.org",
  "@graph": [
    { "@type": "Organization", "url": "...", "name": "鲲 Galgame 补丁", ... },
    { "@type": "WebSite", "url": "...", "name": "...", "potentialAction": { ... } },
    { "@type": "WebPage", "url": "...", "name": "<page title>", "isPartOf": { ... } }
  ]
}
```

WebPage 的 `name` / `url` / `description` 由当前页的 useHead meta 推导。

### 5.2 当前未注入（缺口 — 后续优化项）

| 富类型 | 应当用在 | 当前状态 | 优先级 |
|---|---|---|---|
| `Article` / `CreativeWork` | `/patch/[id]` / `/about/[...slug]` | 未注入 | 中 — 详情页若加 Article schema 能让 Google 显示 publication date / author |
| `BreadcrumbList` | 详情页 / 子路由 | 未注入 | 中 — 提升搜索结果展示 |
| `Person` | `/user/[id]` | 未注入 | 低 |
| `SoftwareApplication` | `/resource/[id]` | 未注入 | 低 — Galgame 补丁本质是软件 |
| `SearchAction` (站内搜索) | WebSite schema 的 potentialAction | nuxt-schema-org 默认生成但未配 search URL template | 中 |
| `FAQPage` | `/about/notice/feedback` | 未注入 | 低 |

→ 富类型 schema 不影响"是否被索引"，但影响**搜索结果展示富片段**（rich snippets）的能力。当前实现保证安全，富化是后续 SEO 提升项。

### 5.3 useKunDisableSeo 的 JSON-LD 空壳

虽然 disable 调用了 `useSchemaOrg([defineOrganization({}), defineWebSite({}), defineWebPage()])`，输出的 JSON-LD 仍然是结构正确的：

```json
{
  "@context": "https://schema.org",
  "@graph": [
    { "@type": "Organization" },
    { "@type": "WebSite" },
    { "@type": "WebPage" }
  ]
}
```

但因为 `templateParams.schemaOrg = { host: '', path: '', inLanguage: 'zh-Hans' }`，nuxt-schema-org 在拼 URL / 站名时拿到空字符串 → 不会泄漏当前站点的可识别信息。配合 `<meta name="robots" content="noindex, nofollow">` 双重保证。

---

## 6. 未来改进项

按优先级降序：

1. **生成 sitemap.xml** — 当前只能靠链接发现页面。生成 `sitemap.xml` 主动列出所有 SFW patch / resource / tag / official 的 URL，提交到 Google Search Console。注意 sitemap 必须**显式过滤 NSFW**（用 wiki batch 拿 content_limit 后 filter）。
2. **/patch/[id] 加 Article schema** — `useSchemaOrg([defineArticle({ headline, datePublished, author, ... })])` 可显著提升搜索结果展示。
3. **/about/[...slug] 加 Article schema** — 同上。
4. **BreadcrumbList** — patch detail 子 tab、user detail 子 tab 都有清晰的层级，适合面包屑。
5. **SearchAction** 配 site search URL — 让 Google 给站点结果显示一个搜索框。
6. **robots.txt 加 Sitemap: 行** — 配合 #1 生成的 sitemap.xml。
7. **/admin 改成路由级 disallow** — robots.txt 加 `Disallow: /admin/`，比 noindex meta 早一步阻止爬虫请求（虽然现在 noindex 已经管用了）。

---

## 7. 改 SEO 的检查清单

每次新增 / 修改 page 时，按以下顺序核对：

1. **判断页面分类**：
   - 公开浏览（首页 / 列表 / 排行 / 工具页）→ **必须** useKunSeoMeta + 有意义的 title + 长尾 description
   - 详情页（带 :id 参数）→ **必须** 条件式：数据 OK 且 SFW → useKunSeoMeta；其他分支 → useKunDisableSeo
   - 私密 / 写入 / 中转 / 错误 → **必须** useKunDisableSeo
   - redirect 中间件 → 不需要声明
   - 嵌套子 tab → 不需要（父 layout 处理）

2. **字段引用检查**（D12 后）：
   - 不要再引用 `patch.introduction` / `patch.alias` / `patch.engine` / `patch.original_language`
   - `patch.name` 是 `KunLanguage` 对象，要用 `getPreferredLanguageText(patch.name)` 转 string
   - `patch.banner` 是 wiki 字段，要用 `resolveBannerUrl(patch)` 拼最优 URL
   - `patch.content_limit` 是 wiki 字段，由 enricher restamp 到 GalgameCard

3. **NSFW 检查**：
   - 该页是否可能展示 NSFW 数据？是 → 数据加载后必须按 `content_limit` 切 disable
   - 该页是否走 useApi？useApi 已综合判断（**登录绕过仅对 /patch/:id 和 /resource/:id 详情生效**；列表 / 工具 / user / taxonomy 子页一律 cookie 默认）决定 content_limit，**不要**在 page 内再做客户端 NSFW 过滤（违反 wiki §16.6 协议）

4. **NULL 检查**：
   - useAsyncData 拿到 null 时是否还在调 useKunSeoMeta 访问字段？→ 改成条件分支或前置 throw 404

5. **og:image**：
   - 详情页应当传 banner / cover / avatar 作为 ogImage 提升分享卡片质量
   - 传 `||  undefined`，不要传空字符串

6. **typecheck**：
   - 运行 `cd apps/web && npx nuxt typecheck`，关注新增的 useKunSeoMeta / useKunDisableSeo 调用是否引入类型错误

---

## 8. 与其他文档的关系

- **NSFW 实施细节**：参见 [`docs/galgame_wiki/00-handbook-for-downstream.md` §16](../galgame_wiki/00-handbook-for-downstream.md) 上游协议 + 本仓库 commit 历史 NSFW 系列改动。
- **D12 字段迁移**：参见 [`docs/proj/schema-ownership.md`](./schema-ownership.md) 和 patch model 注释。
- **Wiki enricher 行为**：源码 `apps/api/internal/galgame/enricher/enricher.go` 顶部注释。
- **kun-galgame-wiki-service 站点本身的 SEO**：上游 wiki 项目独立维护，moyu 不负责。
