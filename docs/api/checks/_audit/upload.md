# Domain: upload

## Summary
6 endpoints in scope (all under `/upload`, group `auth`). The two-stage presigned direct-upload flow (D10) is sound and FE↔BE shapes match the composables exactly (`useResourceUpload.ts` for small/multipart; `useGalgameEdit.ts` + milkdown `uploader.ts` for image-service). I verified `small/init`, `multipart/init`, and `image-service`/`abort` validation live. Findings: one low-severity idempotency-ordering defect in `verifyAndFinalize` (SETNX marker set before the DB quota deduction → a transient DB write failure can leave a completed upload with quota un-deducted), and one low-severity amplification gap (`part_count` not validated against `file_size`). No high/critical issues. Dropped two suspected issues as false positives (see Cross-cutting).

## Endpoints

### POST /upload/small/init — UploadHandler.InitSmall
- verdict: ok
- tested: live. Valid `{galgame_id:6690,file_name:"test patch v1.0.zip",file_size:1048576}` → `{"code":0,...,"data":{"s3_key":"patch/6690/<64rand>/testpatchv10.zip","upload_url":"https://s3.us-east-005.backblazeb2.com/...X-Amz-Expires=7200..."}}`. Bad ext `.exe` → `40000 "不支持的文件类型: .exe"`. >200MB → `40000 "小文件上限 200MB，请走 multipart"`. No cookie → `40100`. `{}` → `40000 "GalgameID is required; FileName is required; FileSize is required"`.
- FE contract: `useResourceUpload.ts:30-33 SmallInitResponse {s3_key, upload_url}` matches BE `dto.go:13-16` json tags. Request body `{galgame_id,file_name,file_size}` (`useResourceUpload.ts:134-138`) matches `SmallInitRequest` json tags. Extension allow-list `.zip/.rar/.7z` (`constants/upload.go AllowedResourceExtensions`) + 200MB cap enforced. `sanitizeFileName` strips basename dots ("v1.0"→"v10") but preserves the real extension via `filepath.Ext` — matches the documented TS `sanitizeFileName.ts` behavior; not a bug.

### POST /upload/small/complete — UploadHandler.CompleteSmall
- verdict: ok
- tested: ready curl for human (not run — it deducts `daily_upload_size` and requires a real PUT'd object first):
  `curl -s -X POST http://127.0.0.1:5214/api/v1/upload/small/complete -H 'Cookie: moyu_session=...' -H 'Content-Type: application/json' -d '{"s3_key":"patch/6690/<key from init>/testpatchv10.zip","declared_size":1048576}'` (only after PUTting the file to the init upload_url). Expected `{code:0,data:{s3_key,size}}`.
- BE path: `service.go:234 CompleteSmall` → `verifyAndFinalize` (StatObject → size==declared check → SETNX idempotency → conditional quota UPDATE). Response `CompleteResponse {s3_key,size}` (`dto.go:25-28`) matches FE `CompleteResponse {s3_key, size}` (`useResourceUpload.ts:42-45`). Size-mismatch deletes the object and errors. SETNX dedupe (`upload:complete:<s3Key>`, 24h NX) prevents double quota-charge on replay — verified in code at `service.go:61-79,186-192`.
- issues:
  - [low][bug] Idempotency marker is set BEFORE the quota deduction; a transient DB error on the `UpdateColumn` (service.go:203-206) returns an error WITHOUT deleting the S3 object, yet the SETNX key stays set. A client retry then hits `!first` (service.go:190) and returns `{size}` success with NO quota deducted → `daily_upload_size` under-counted for a real upload. EVIDENCE: internal/common/upload/service.go:186 — `first, err := s.markCompleteOnce(ctx, s3Key)` runs before the DB update at :203-206 which has no rollback of the marker. FIX (BE): deduct quota first via an atomic conditional UPDATE (or guard the marker), and only set the SETNX marker AFTER a successful deduction; alternatively `DEL` the marker on any error returned after :186.

### POST /upload/multipart/init — UploadHandler.InitMultipart
- verdict: fix (low)
- tested: live. ≤200MB `{file_size:1048576}` → `40000 "≤ 200MB 请走 /upload/small"`. Valid 300MB,part_count:30 → `{"code":0,...,"data":{"s3_key":"patch/6690/<64rand>/big.zip","upload_id":"4_z4a7a...","part_urls":["https://s3...?X-Amz..."]}}` (real B2 multipart created; left for the 6h/24h orphan-cleanup cron). Missing part_count → `40000 "PartCount is required"`.
- FE contract: `MultipartInitResponse {s3_key, upload_id, part_urls}` (`useResourceUpload.ts:35-39`) matches BE `dto.go:41-45`. Request `{galgame_id,file_name,file_size,part_count}` matches `MultipartInitRequest`. `part_urls[i]` ↔ part_number i+1 documented and used correctly (`useResourceUpload.ts:200-203` PUTs `part_urls[i]`, completes with `part_number: idx+1`).
- issues:
  - [low][bug] `part_count` is client-supplied and only bounded by `validate:"...max=10000"`; the server never checks it equals `ceil(file_size / MultipartPartSize)`. A client can request, e.g., `part_count=10000` for a 201MB file, forcing the server to presign 10000 URLs (CPU/alloc amplification). Capped at 10000 so impact is limited, but it is unvalidated input. EVIDENCE: internal/common/upload/service.go:261-270 — `for i := 1; i <= req.PartCount; i++ { ...PresignUploadPart... }` with no reconciliation against `req.FileSize`. FIX (BE): in `InitMultipart`, compute `want := (req.FileSize + MultipartPartSize - 1) / MultipartPartSize` and reject if `req.PartCount != want` (or `> want`).

### POST /upload/multipart/complete — UploadHandler.CompleteMultipart
- verdict: ok
- tested: ready curl for human (not run — mutating, deducts quota, needs a real in-progress multipart with uploaded parts):
  `curl -s -X POST http://127.0.0.1:5214/api/v1/upload/multipart/complete -H 'Cookie: moyu_session=...' -H 'Content-Type: application/json' -d '{"s3_key":"<from init>","upload_id":"<from init>","declared_size":314572800,"parts":[{"part_number":1,"etag":"<etag>"},...]}'`. Expected `{code:0,data:{s3_key,size}}`.
- BE path: `service.go:276` → `s3.CompleteMultipart` (parts need not be sorted) → shared `verifyAndFinalize`. Request `parts:[{part_number,etag}]` (`UploadedPart` dto.go:48-51) matches FE `parts: etags.map((etag,idx)=>({part_number:idx+1,etag}))` (`useResourceUpload.ts:229-231`). ETag quote-stripping done client-side; minio-go accepts either. Response shape matches.
- issues:
  - [low][bug] Same SETNX-before-deduction ordering as `small/complete` (shared `verifyAndFinalize`). Same fix applies.

### POST /upload/multipart/abort — UploadHandler.AbortMultipart
- verdict: ok
- tested: live (auth gate only). No cookie → `40100`. Body `{s3_key,upload_id}` (`MultipartAbortRequest` dto.go:62-65) matches FE abort calls (`useResourceUpload.ts:216,225,274` send `{s3_key,upload_id}`). Returns `OKMessage "已放弃上传"` (envelope `{code:0,message:"已放弃上传",data:null}`); FE fires-and-forgets with `.catch(()=>{})`, so the message/no-data response is fine.
- BE path: `handler.go:152-163` → `s3.AbortMultipart` (`AbortMultipartUpload`). No quota touched. Note: abort takes no ownership check — any authed user who knows another user's `s3_key`+`upload_id` could abort their in-progress upload. EVIDENCE: internal/common/upload/service.go:294-296 — abort uses only req fields, no userID. This is effectively unexploitable (the random 64-char key + B2 upload_id are unguessable secrets returned only to the owner), so not reported as a defect.

### POST /upload/image-service — UploadHandler.UploadImageService
- verdict: ok
- tested: live validation. No `preset` → `40000 "缺少 preset 字段"`. No `file` → `40000 "缺少 file 字段"`. (A real image upload to image_service :9278 not executed — needs upstream + real binary; ready curl: `curl -s -X POST http://127.0.0.1:5214/api/v1/upload/image-service -H 'Cookie: moyu_session=...' -F 'preset=topic' -F 'file=@/path/to/image.webp'`.)
- Proxy correctness: handler enforces `auth` (MustGetUser) + presence of `preset`+`file`, 10MB per-file cap (`handler.go:125`), forwards via `imageclient.Upload` with multipart `preset`+`file` and the file's Content-Type (`handler.go:134-135`). Returns upstream `UploadResult {hash,url,variant_urls,width,height,size_bytes,deduplicated}` (`imageclient` struct) which matches FE `ImageServiceUploadResult` (`useGalgameEdit.ts:272-280`) and the milkdown consumer reading `data.url` (`uploader.ts:23-31`). `variant_urls` is guaranteed non-nil (imageclient sets `{}` if missing) — matches FE `Record<string,string>`. Upstream auth is HTTP-Basic with the project OAuth client_id/secret (defaulted in `app.go:144-154`); errors mapped to 80008/422-60002/internal — FE only checks `code!==0`, so mapping is cosmetic and fine. Body-cap source: Fiber global `BodyLimit:10MB` (`app.go:181`); handler's explicit `fh.Size>10MB` check is the precise per-file guard.

## Cross-cutting
- [info] Daily-quota field `User.DailyUploadSize` is Go `int` (model.go:23), cast to `int64` at use (service.go:147,198). On amd64 `int` is 64-bit so the 5GB creator limit is safe; only a 32-bit build would overflow. Not reported as a defect (deployment is 64-bit). 
- [dropped false-positive] Suspected "image-service exposes admin-only side effects without role gate" — NO: it is intentionally any-authed-user (screenshot/editor image upload); upstream image_service enforces per-client quota + moderation. EVIDENCE: handler.go:110 `_ = middleware.MustGetUser(c)` (auth only, by design per handler doc-comment).
- [dropped false-positive] Suspected "quota checked against declared_size at init can be bypassed" — NO: the authoritative deduction at complete re-checks actual `HeadObject` size against the live `daily_upload_size` and deletes the object if over limit (service.go:198-201), so init's declared-size pre-check is only an early UX reject, not the enforcement point.
- [note] My live `multipart/init` test created one real orphan B2 multipart upload (s3_key `patch/6690/YGqRDRsPJIW.../big.zip`); it will be reaped by the existing orphan-cleanup cron (`MultipartUploadOrphanTTL` 24h, runs every 6h). No completion / no quota was deducted.
