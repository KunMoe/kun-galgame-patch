# Domain: search-about

> 审计于 run1（schema 强制输出），此处为整理后的最终结论。

## Summary
3 端点（POST /search、GET /about/posts、GET /about/post）端到端核对 + live curl。1 个真实低危缺陷已修；run1 初稿的两个 cross-cutting 经核对后 drop（误报）。

## Endpoints

### POST /api/v1/search — SearchHandler.Search
- verdict: ok
- tested: 代理 Wiki Meilisearch（公开）；请求/响应 shape 与 FE 对齐。

### GET /api/v1/about/posts — AboutHandler.ListPosts
- verdict: ok
- tested: `{items, tree}`（KunPostsResponse）；items=listMetadata、tree=buildTree；与 /about 索引 + 文档详情侧栏一致。

### GET /api/v1/about/post — AboutHandler.GetPost
- verdict: fix（已修复，低危）
- issue [low][bug]：含 `..` 的 slug 被正确拦截（`..` 检查在任何文件读取之前 return，无穿越/泄露），但返回通用 `fmt.Errorf("invalid slug")`，被 handler 的 else 分支映射为 **50000**（应为 4xx）。实测 `slug=../../etc/passwd` → `code:50000`。
- evidence：service.go:211-213 `if strings.Contains(slug,"..") { return nil, fmt.Errorf("invalid slug") }`；handler.go:46-50 仅 `os.ErrNotExist` → 40400，其余 → 50000。
- fix（BE）：`..` 分支改 `return nil, os.ErrNotExist` → 既有 handler 分支映射为 **404**（穿越探测看起来就是普通 404，最安全）。实测修复后 → `code:40400`。

## Cross-cutting
- none（run1 初稿的两条 cross-cutting 核对后判为误报，已 drop）。
