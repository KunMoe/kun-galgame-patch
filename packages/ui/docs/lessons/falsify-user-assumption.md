# 证伪用户假设：症状真，假设通常错

> 首次识别于 [v0.4.6](../changelog/v0.4.6.md) — KunImage 5 个 prop 透传 + KunUI layer 注册 `none` provider。moyu 调查 `/about` 卡顿时，反驳了用户"sharp 在前端运行"的错误假设，再用 build artifact 找到真根因。本节是该版本反思的方法论沉淀。

## 用户报告与 moyu 反驳

**错误假设**：用户报告"sharp 在前端运行所以页面卡"。

**moyu 反驳**：

> Sharp 是 Node.js 原生绑定（libvips C 库），物理上不可能在浏览器里运行。浏览器没有 N-API 也没有 native binding 加载能力。我也跑了 build 验证：
>
> ```
> .output/server/node_modules/sharp      ← server bundle 里有
> .output/public/_nuxt/*.js              ← client bundle 里 0 sharp
> ```

直接证伪用户假设，没顺着错的方向修。这是 senior 调试的标志。

## 真正根因

| 症状 | 成因 |
|---|---|
| 首次加载慢 | 4 张 card banner 并行触发 IPX 冷启动（每张几百 ms server-side sharp transcode） |
| 滚动卡顿 | 没 width/height → 每张图加载时 layout shift → 浏览器反复 reflow |
| 流量浪费 | banner 已经是 author 时压好的 AVIF (30-100KB)，IPX 再过一遍 sharp 0 收益 |
| 缓存抖动 | IPX 默认 FS 缓存 5 分钟过期，冷启动反复 |

跟"sharp 在前端"完全无关 —— 但用户能感知到"卡"是真的，只是把根因猜错了。

## 反思 — sharp 假设给我们的元教训

用户报告"sharp 在前端运行"是错的，但他们能感知到"卡"是真的。**这种"症状真，假设错"的报告是最常见也最难处理的**：

- 顺着错假设修 → 修不到根因，用户继续抱怨
- 直接反驳 → 用户觉得你在踢皮球
- moyu 的做法：**用证据反驳假设 + 重新定义问题 + 找真根因** —— 这才是建设性回应

对应 KunUI 维护策略：用户报告卡 / 崩 / 闪烁时，**先问"我看到的物理证据是什么"**（build artifact 检查、network panel、devtools performance），再问"那真正的瓶颈是什么"，最后才修。不要被"用户给出的假设"牵着走。

## 通用调试模板

| 用户说 | 我必须先确认（用证据，不靠想象） |
|---|---|
| "X 库在前端跑导致慢" | grep `.output/public/_nuxt/*.js` 看 X 库是否真的进了 client bundle |
| "本地" | 哪个端口？是 dev server 还是 build 产物？同一 commit 吗？ |
| "moyu / kungal 上有 bug" | 部署版还是本地 dev？同一 commit 吗？ |
| "看 /xxx 页" | 拉一下页面 DOM 看是不是 KunUI 渲染的（react-aria 标记？data-slot？） |
| "modal / popover" | DOM 里搜 `KunModal` / `kun-` id prefix 确认是 KunUI |
| "图片加载慢" | network panel 看 actual download time + IPX hit/miss + cache headers |
| "页面卡" | devtools performance 录一段，看 main thread 时间分布 |

## v0.4.9 反向应用

[v0.4.9](../changelog/v0.4.9.md) 调 KunSelect 溢出 bug 时第一轮跑偏 —— 跑去测**生产部署**（moyu.moe 是老 Next.js + HeroUI），结果发现是 HeroUI 的 Select，写了一大段"用户看的不是 KunSelect"的报告。**全是错的**。

用户提醒"测试项目跑在本地 127.0.0.1:6969"后，我立刻测对了环境 + 用 DOM probe 抓到真 bug。

教训：**用户报告任何视觉 bug，第一件事是确认我看的环境跟用户看的环境一致**。5 行 Playwright `evaluate` 就能定性，比凭直觉硬猜快得多。

## 核心原则

> 用户能精准描述**症状**（卡 / 崩 / 闪烁 / 溢出 / 静默），但**根因猜测的命中率不高**。
>
> 工程师的工作是：
> 1. 承认症状真实存在
> 2. **不接受**用户的根因假设作为输入
> 3. 用 build artifact / DevTools / DOM probe 拿物理证据
> 4. 重新定义问题（"卡"→"哪段时间在哪个线程花了多久"）
> 5. 找真根因，修真根因
>
> 这不是"踢皮球"，恰恰相反 —— 是对用户体验负责的标准流程。顺着错假设修反而是不负责任。
