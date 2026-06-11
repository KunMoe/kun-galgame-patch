# 上传限制（补丁资源文件 / 图片）

> 本文档汇总 moyu 全站**所有文件上传的大小、类型、配额、命名等限制**，以及它们在前后端各自的执行点。改动任何上传相关阈值时**请先读本文档**——前后端常量是**两份手抄副本**（不是共享代码），漏改一边会造成「前端放行、后端拒绝」或「前端拦截、后端其实允许」的不一致。
>
> 注意区分两条物理上完全不同的上传通道：**补丁资源文件**直传 S3（presigned URL，绕过 API），**图片**经 API 转发到 OAuth image_service。两者的大小上限差了两个数量级，原因见 §4。

## TL;DR

| 上传对象 | 大小上限 | 类型限制 | 通道 |
|---|---|---|---|
| 补丁资源文件（小文件） | **200 MB** 单发 PutObject | `.zip` / `.rar` / `.7z` | 直传 S3 |
| 补丁资源文件（大文件） | **1 GB** multipart（10 MiB/片） | 同上 | 直传 S3 |
| 补丁资源 > 1 GB | 无（不上传） | —— | 改用「自定义链接」(`user` storage) |
| 图片（封面 / 头像 / 正文 / 编辑器截图） | **10 MB** | image_service 按 preset 各自限制 | API 转发 image_service |
| API 全局请求体 | **10 MB**（Fiber `BodyLimit`） | —— | 仅约束「经过 API」的请求 |
| 每用户每日上传配额（普通） | **100 MB / 天** | —— | 资源文件计入 |
| 每用户每日上传配额（管理/版主/创作者） | **5 GB / 天** | —— | 同上 |

- 关键直觉：**1 GB 的补丁能上传，但 API 的请求体只有 10 MB**——因为资源文件用 presigned URL **直传对象存储，根本不经过 API**。10 MB 的 `BodyLimit` 只卡「经过 API 的 multipart」，也就是图片上传。
- 大小限制是**声明—校验—复核**三段式：前端预检 → 后端发 presigned URL 前按声明大小预检配额 → 上传完成后 `HeadObject` 复核**真实大小**，不符就删文件。客户端无法靠谎报大小绕过配额。

---

## 1. 补丁资源文件上传

### 1.1 大小分级

权威常量在 `apps/api/internal/constants/upload.go`：

| 常量 | 值 | 含义 |
|---|---|---|
| `MaxSmallFileSize` | `200 * 1024 * 1024`（200 MB） | ≤ 此值走单发 `PutObject` |
| `MaxLargeFileSize` | `1 * 1024 * 1024 * 1024`（1 GB） | 单文件整条流程的硬上限 |
| `MultipartPartSize` | `10 * 1024 * 1024`（10 MiB） | 大文件 multipart 分片大小（S3 推荐值） |

- **≤ 200 MB**：`InitSmall` → 单个 presigned PUT URL（`service.go:240`，超 200 MB 报「小文件上限 200MB，请走 multipart」）。
- **200 MB < x ≤ 1 GB**：`InitMultipart`，分片数 = `ceil(file_size / 10 MiB)`，后端**强制校验** `part_count` 必须等于按声明大小算出的片数（`service.go:280`），否则拒绝——防止客户端把片数与真实大小解耦、逼服务器签出上千个无用 part URL。前端并发上传 4 片（`PARALLEL_PARTS = 4`）。
- **> 1 GB**：S3 通道不接受，前端 `useResourceUpload.ts` 在选文件时即报「文件大小超过 1GB 上限」；用户必须改用「自定义链接」存储（见 §1.4）。

### 1.2 文件类型

后端权威白名单（**按扩展名**，不看 MIME）：

```go
// constants/upload.go:31
var AllowedResourceExtensions = []string{".zip", ".rar", ".7z"}
```

- 校验点：`service.go:150`（`validatePreUpload`，发 presigned URL 前），不在白名单报「不支持的文件类型」。
- 前端镜像：`apps/web/app/constants/resource.ts` 的 `ALLOWED_EXTENSIONS = ['.zip', '.rar', '.7z']`，`resource/Publish.vue` 选文件时按扩展名预检。
- ⚠️ 该文件里还有一个 `ALLOWED_MIME_TYPES`（`application/zip` / `application/x-lz4` / `application/x-rar-compressed`），但**后端并不按 MIME 校验**，扩展名才是唯一真源。改类型只需动两处扩展名常量。

### 1.3 命名与 S3 key

- DTO 字段长度上限（`apps/api/internal/patch/dto/dto.go`，`PatchResourceCreateRequest`）：

  | 字段 | 上限 | 说明 |
  |---|---|---|
  | `Name` | 300 | 资源展示名 |
  | `S3Key` | 2048 | 完整对象键 |
  | `Content` | 1007 | `s3` 存储时被服务端覆盖为 s3_key；`user` 存储时是下载链接 |
  | `Code` / `Password` / `ModelName` | 1007 | 提取码 / 解压密码 / 模型名 |
  | `Note` | 10007 | 备注 |
  | `Type` / `Language` / `Platform` | 数组 `min=1, max=10` | 见 §1.5 枚举 |

- **文件名清洗**（`service.go:105` `sanitizeFileName`，对齐旧站 `sanitizeFileName.ts`）：仅保留 `\p{L}\p{N}_-`（字母 / 数字 / 下划线 / 连字符），其余字符剔除，保留扩展名，**基名截断到 100 字符**。
- **S3 key 结构**：`patch/{galgameId}/{random64}/{sanitizedFileName}`，中段是 64 位 `[A-Za-z0-9]` 随机串（`S3KeyRandomLength = 64`，替代旧站的 BLAKE3 hash）。提交资源时后端校验 s3_key 必须以 `patch/{galgameId}/` 开头，做路径隔离。

### 1.4 存储类型（`storage`）

`resource.ts` 的 `SUPPORTED_RESOURCE_LINK = ['s3', 'user']`：

| 值 | 含义 | 适用 |
|---|---|---|
| `s3` | 对象存储直传 | < 1 GB 补丁；稳定、永不过期 |
| `user` | 用户自定义链接（逗号分隔，如百度网盘 / 磁力） | > 1 GB 补丁；由用户自行保证可用 |

- `s3`：`Content` 可留空，服务端覆盖为 s3_key；下载链接在请求时由 `S3Client.PublicURL + s3_key` **实时拼接**（`service.go` `GetResourceDownloadInfo`），所以换 bucket 公共域名不需要回填数据库。
- `user`：`Content` 即用户提供的下载链接，服务层强制 `min=1`。

### 1.5 资源元数据枚举（`resource.ts`）

- **类型** `SUPPORTED_TYPE`：`manual / ai / machine_polishing / machine / save / crack / fix / mod / other`
- **语言** `SUPPORTED_LANGUAGE`：`zh-Hans / zh-Hant / ja / en / other`
- **平台** `SUPPORTED_PLATFORM`：`windows / android / macos / ios / linux / other`

每项数组长度 `min=1, max=10`（DTO 校验）。

### 1.6 每日配额

`constants/upload.go`：

| 常量 | 值 |
|---|---|
| `UserDailyUploadLimit` | 100 MB / 天 |
| `CreatorDailyUploadLimit` | 5 GB / 天 |

- 「特权」(`privileged`) 由 handler 从 OAuth roles claim 解析（管理 / 版主 / 创作者）——本地 user 表 OAuth 迁移后已不存 role。
- 计入字段：`auth user` 表的 `daily_upload_size`（原子 `UPDATE ... + actual`）。
- 校验两次：发 presigned URL 前按**声明大小**预检（`service.go:161`，快速失败）；完成时按 `HeadObject` 的**真实大小**复检（`service.go:223`），超额则删除已上传对象。

### 1.7 完整执行流程（防绕过）

1. **前端预检**（`useResourceUpload.ts`）：> 1 GB 直接拒绝；扩展名预检。
2. **Init**（`InitSmall` / `InitMultipart`）：扩展名 + 大小 + 配额（按声明）预检 → 签发 presigned URL(s)。
3. **客户端直传 S3**（绕过 API，不受 `BodyLimit` 约束）。
4. **Complete**（`verifyAndFinalize`，`service.go:180`）：
   - `HeadObject` 确认对象存在；
   - 真实大小 ≠ 声明大小 → 删除 + 报错；真实大小 > 1 GB → 删除 + 报错；
   - **幂等**：Redis `SET NX`（key `upload:complete:{s3Key}`，TTL 24h）保证同一 s3_key 只扣一次配额，失败路径释放标记以便重试（MOYU-PR7 / M5）；
   - 配额复检 → 原子累加 `daily_upload_size`。
5. **Presigned URL 时效**：`PresignPutObjectTTL = 2h`，`PresignUploadPartTTL = 4h`。
6. **孤儿清理**：未完成的 multipart 超过 `MultipartUploadOrphanTTL = 24h` 由 cron 清理，cron 间隔 `AbortedMultipartCleanupInterval = 6h`。

---

## 2. 图片上传

所有图片（封面 / 头像 / 正文图 / 编辑器截图）经 API 转发到 OAuth image_service（C4 契约），**moyu 不自建 S3**。

### 2.1 通用 image_service 端点

`POST /api/upload/image-service`（`internal/common/upload/handler.go:109`）：

- **大小上限 10 MB**：`fh.Size > 10*1024*1024` 报「文件超过 10MB 上限」（`handler.go:125`）。这同时也被 Fiber 全局 `BodyLimit`（§4）兜底。
- **preset**（必填）转发给 image_service，决定处理与配额：
  - `topic`：正文图 / 编辑器截图（默认）。⚠️ topic preset **拒绝 AVIF**（无解码器），上传前需先把 AVIF 转 PNG（见 memory `image-service-upload-contract`）。
  - `galgame_banner`：galgame 封面（需 OAuth client 开启该 preset）。
  - `avatar`：用户头像。
- 每 preset 的大小/格式细则 + 每 client 每日配额由 **image_service 侧**强制，moyu 只兜 10 MB 上限。

### 2.2 头像

`internal/user/handler/handler.go:27` `readImageFormFile`：`f.Size > 10*1024*1024` 报「图片超过 10MB」。

### 2.3 galgame 封面 / banner

走 `PUT /galgame/:gid` multipart（Wiki 自动把 hash 升为 `covers[sort_order=0]`），`internal/patch/handler/handler.go:836` 与 `:1182`：`fh.Size > 10*1024*1024` 报「banner 超过 10MB 上限」。

### 2.4 编辑器正文图

Milkdown upload 插件（`apps/web/app/components/kun/milkdown/plugins/upload/uploader.ts`）打到 `/api/upload/image-service`，preset=`topic`，继承 10 MB 上限。

### 2.5 前端裁剪器输出约束

封面发布 / 编辑用 `ImageCropper.vue`（CropperJS v2），输出前会：

| 约束 | 值 | 位置 |
|---|---|---|
| 最大宽度 | 1920 px（仅缩小，不放大） | `ImageCropper.vue:125` |
| 宽高比 | 16:9（封面默认） | `ImageCropper.vue` props `aspectRatio` |
| 初始覆盖 | 0.9 | selection.initialCoverage |
| 输出格式 / 质量 | `image/webp` @ 0.9 | `ImageCropper.vue:132` |

裁剪后可选「裁剪并打码」走 `ImageMosaic.vue`（canvas 马赛克）。这些是**前端产物约束**，不是服务端校验——真正的 10 MB 上限在服务端。

---

## 3. 字段长度限制（非文件，但同属上传/提交约束）

`apps/api/internal/patch/dto/dto.go` 其它请求体上限（节选）：

| 字段 | 上限 |
|---|---|
| `VndbID`（建补丁） | 20 |
| 评论 `Content` | 10007 |
| 资源各字段 | 见 §1.3 |

---

## 4. 为什么图片 10 MB、补丁却能 1 GB？

Fiber 全局 `BodyLimit = 10 MB`（`internal/app/app.go:185`）约束的是**请求体真正流过 API 进程**的请求：

- **图片** = multipart 表单 POST 到 API，API 再转发 image_service → 受 10 MB 约束（也正好对齐 image_service 的预期）。
- **补丁资源文件** = 客户端拿 presigned URL **直接 PUT 到对象存储**，字节流**根本不经过 API**，所以与 `BodyLimit` 无关；API 只在前后用小 JSON 请求处理 init/complete（声明大小、s3_key、ETag 列表等，都是 KB 级）。

> 仓库内**没有**自定义 nginx / 反向代理体积限制；生产体积约束继承自 kun-galgame-infra 栈。若未来在 infra 前置代理加 `client_max_body_size`，需保证 ≥ 10 MB 否则图片上传会在代理层被截。

---

## 5. 真源对照（改限制时要同步的所有点）

| 限制 | 后端真源 | 前端镜像 |
|---|---|---|
| 资源大小分级（200MB/1GB/10MiB） | `constants/upload.go:9,12,15` | `useResourceUpload.ts`（`MAX_SMALL_FILE_SIZE` 等） |
| 资源扩展名白名单 | `constants/upload.go:31` | `constants/resource.ts:138`（`ALLOWED_EXTENSIONS`） |
| 每日配额 | `constants/upload.go:20,21` | （仅后端） |
| presign / 清理 TTL | `constants/upload.go:26,27,37,40` | （仅后端） |
| 文件名清洗（100 字符） | `common/upload/service.go:105` | 旧站 `sanitizeFileName.ts`（已迁移） |
| 图片 10 MB | `common/upload/handler.go:125`、`user/handler/handler.go:27`、`patch/handler/handler.go:836,1182` | `ImageCropper.vue` 产物约束（间接） |
| Fiber BodyLimit 10 MB | `app/app.go:185` | （仅后端） |
| 资源 DTO 字段长度 | `patch/dto/dto.go:60-74` | 表单组件 maxlength（按需） |
| 存储类型 / 元数据枚举 | （服务层校验） | `constants/resource.ts` |

> 改阈值的金科玉律：**先改后端常量（真源），再同步前端镜像**。只改前端 = 安全形同虚设；只改后端 = 用户被「上传到一半才报错」的体验劝退。
