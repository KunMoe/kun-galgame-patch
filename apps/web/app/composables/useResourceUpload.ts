// Drives the patch-resource upload flow (apps/api/internal/common/upload).
//
// One server-driven flow — the artifact service decides single-PUT vs multipart
// and the part size; the client obeys the Init response (never hardcodes a chunk
// size). The opaque artifact_uuid is the only identifier carried to the resource
// form.
//
//   POST /upload/init     → { artifact_uuid, multipart, upload_url | parts[] + part_size }
//   single : PUT file → upload_url
//   multi  : PUT each file.slice(part_size) → parts[i].url (parallel), collect ETags
//   POST /upload/complete → server verifies size + deducts quota → { artifact_uuid, size }
//   POST /upload/resume   → { uploaded_parts[] (skip), parts[] (missing) | upload_url }
//   POST /upload/abort    → soft-delete on user cancel / fatal mid-upload error
//
// Resumable multipart uploads: an interruption (network drop / refresh / closed
// tab) leaves the uploaded parts in B2. We persist {uuid + file identity +
// progress} per galgame (useResourceResumeUploads) and resumeUpload() asks the
// artifact service which parts already landed, re-PUTting only the missing ones.
// Explicit cancel() aborts the artifact + drops the record; an interruption keeps
// it so the publish modal can offer to continue.
//
// The per-role single-file cap (1/5/20 GB for user/moderator/admin) mirrors Go's
// internal/constants/upload.go UploadTier for an instant local error; the server
// is authoritative (it also enforces the per-user daily quota).

const GiB = 1024 * 1024 * 1024

// Upload concurrency scales with file size to trade throughput against resume
// granularity. The window a resume must re-upload after an interrupt is
// (parallel parts × 16 MB part size), so we keep that window a small fraction of
// the file: small uploads run fewer parts in parallel (finer resume — and they
// finish fast anyway), large uploads keep full parallelism (raw throughput
// matters, and 64 MB is a small slice of a 0.5–1 GB upload). 16 MB is the
// server's part-size floor, so ≤16 MB re-upload is the best achievable. The
// boundaries hold the worst-case re-upload at ≤12.5% of the file.
//   < 256 MB → 1 stream  (re-upload ≤16 MB on interrupt)
//   < 512 MB → 2 streams (≤32 MB)
//   ≥ 512 MB → 4 streams (≤64 MB)
const partsConcurrency = (fileSize: number): number => {
  if (fileSize < 256 * 1024 * 1024) return 1
  if (fileSize < 512 * 1024 * 1024) return 2
  return 4
}

interface InitPart {
  part_number: number
  url: string
}

interface InitResponse {
  artifact_uuid: string
  multipart: boolean
  upload_url?: string
  part_size?: number
  parts?: InitPart[]
  expires_at: string
}

interface ResumePart {
  part_number: number
  etag: string
  size: number
}

interface ResumeResponse {
  artifact_uuid: string
  multipart: boolean
  upload_url?: string
  part_size?: number
  parts?: InitPart[]
  uploaded_parts?: ResumePart[]
  expires_at: string
}

interface CompleteResponse {
  artifact_uuid: string
  size: number
}

export type UploadStatus =
  | 'idle'
  | 'preparing'
  | 'uploading'
  | 'completing'
  | 'done'
  | 'error'
  | 'aborted'

export interface UploadResult {
  artifactUuid: string
  size: number // server-verified bytes
}

type ResumeStore = ReturnType<typeof useResourceResumeUploads>

export const useResourceUpload = () => {
  const api = useApi()
  const userStore = useUserStore()
  // Per-role single-file cap (mirrors backend constants.UploadTier). admin is
  // checked first since isModerator may also be true for admins.
  const maxFileSize = computed(() =>
    userStore.isAdmin ? 20 * GiB : userStore.isModerator ? 5 * GiB : GiB
  )
  const status = ref<UploadStatus>('idle')
  // Per-byte progress so the bar moves smoothly even during a single large PUT.
  // Multipart aggregates uploaded-bytes-so-far across parts.
  const uploadedBytes = ref(0)
  const totalBytes = ref(0)
  const errorMessage = ref('')

  // Abort context — kept so the user can cancel mid-upload and the backend
  // soft-deletes the in-progress artifact rather than waiting for the GC sweep.
  let abortUuid: string | null = null
  let cancelled = false
  // Resume bookkeeping for the active multipart flow: the per-galgame store the
  // in-progress upload is persisted in, so progress updates land + an
  // interruption stays resumable until completed or explicitly cancelled.
  let activeStore: ResumeStore | null = null

  const progressPercent = computed(() => {
    if (totalBytes.value === 0) return 0
    return Math.min(
      100,
      Math.round((uploadedBytes.value / totalBytes.value) * 100)
    )
  })

  const reset = () => {
    status.value = 'idle'
    uploadedBytes.value = 0
    totalBytes.value = 0
    errorMessage.value = ''
    abortUuid = null
    cancelled = false
    activeStore = null
  }

  // Track active XHRs so cancel() can yank them all. Cleared after each flow.
  const activeXhrs: XMLHttpRequest[] = []

  // PUT one blob to a presigned URL with progress reporting. Native fetch has no
  // upload progress event, so we drop to XHR. Resolves with the ETag (multipart
  // needs it for complete; '' for single-PUT).
  const putToS3 = (
    url: string,
    body: Blob,
    onProgress: (delta: number) => void
  ): Promise<string> =>
    new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      let lastLoaded = 0
      xhr.open('PUT', url)
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          onProgress(e.loaded - lastLoaded)
          lastLoaded = e.loaded
        }
      })
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const etag = (xhr.getResponseHeader('ETag') || '').replace(/"/g, '')
          resolve(etag)
        } else {
          reject(new Error(`上传分片失败 (HTTP ${xhr.status})`))
        }
      })
      xhr.addEventListener('error', () =>
        reject(new Error('网络中断，请检查连接'))
      )
      xhr.addEventListener('abort', () => reject(new Error('已取消上传')))
      activeXhrs.push(xhr)
      xhr.send(body)
    })

  // PUT a set of multipart parts (slices of `file` by `partSize`) with
  // size-scaled concurrency (partsConcurrency), collecting per-part ETags. The
  // live progress bar (uploadedBytes)
  // accrues per byte for smoothness, but the PERSISTED resume progress tracks
  // only fully-COMMITTED parts (committedBytes): B2 stores — and resume can skip
  // — a part only once its whole PUT has landed, so the persisted % must reflect
  // committed parts, NOT the optimistic live count that also includes in-flight
  // parts. Otherwise the resume list promises progress (say 62%) a resume can't
  // honor — it continues from the last committed boundary (say 46%).
  // baseCommittedBytes is what's already stored before this call: 0 for a fresh
  // upload, the sum of already-uploaded parts for a resume. Shared by both.
  const putParts = async (
    file: File,
    partSize: number,
    parts: InitPart[],
    baseCommittedBytes: number
  ): Promise<{ part_number: number; etag: string }[]> => {
    const n = parts.length
    const etags = new Array<string>(n).fill('')
    let next = 0
    let committedBytes = baseCommittedBytes
    let firstError: Error | null = null

    const worker = async () => {
      while (next < n && !firstError) {
        const i = next++
        const p = parts[i]!
        const start = (p.part_number - 1) * partSize
        const end = Math.min(start + partSize, file.size)
        try {
          etags[i] = await putToS3(p.url, file.slice(start, end), (delta) => {
            uploadedBytes.value += delta
          })
          // Part fully stored in B2 — advance the committed (resumable) frontier
          // and persist it so the resume list shows the real continue-from point.
          committedBytes += end - start
          if (activeStore && abortUuid) {
            activeStore.setProgress(
              abortUuid,
              Math.round((committedBytes / totalBytes.value) * 100)
            )
          }
        } catch (e) {
          if (!firstError) firstError = e as Error
        }
      }
    }

    const concurrency = partsConcurrency(file.size)
    await Promise.all(Array.from({ length: Math.min(concurrency, n) }, worker))
    if (firstError) throw firstError
    return parts.map((p, i) => ({ part_number: p.part_number, etag: etags[i]! }))
  }

  // Finalize via the BFF (server verifies size + deducts quota once). Returns the
  // verified result; throws on a non-zero code so the caller keeps the upload
  // resumable.
  const complete = async (
    artifactUuid: string,
    parts?: { part_number: number; etag: string }[]
  ): Promise<UploadResult> => {
    status.value = 'completing'
    const res = await api.post<CompleteResponse>('/upload/complete', {
      artifact_uuid: artifactUuid,
      declared_size: totalBytes.value,
      ...(parts ? { parts } : {})
    })
    if (res.code !== 0 || !res.data) {
      throw new Error(res.message || '完成上传失败')
    }
    status.value = 'done'
    uploadedBytes.value = totalBytes.value // pin to 100%
    return { artifactUuid: res.data.artifact_uuid, size: res.data.size }
  }

  const upload = async (
    file: File,
    galgameId: number
  ): Promise<UploadResult> => {
    reset()
    if (file.size > maxFileSize.value) {
      const msg = `文件大小超过 ${maxFileSize.value / GiB} GB 上限`
      status.value = 'error'
      errorMessage.value = msg
      throw new Error(msg)
    }
    totalBytes.value = file.size

    try {
      status.value = 'preparing'
      const initRes = await api.post<InitResponse>('/upload/init', {
        galgame_id: galgameId,
        file_name: file.name,
        file_size: file.size,
        mime_type: file.type
      })
      if (initRes.code !== 0 || !initRes.data) {
        throw new Error(initRes.message || '初始化上传失败')
      }
      const init = initRes.data
      abortUuid = init.artifact_uuid

      status.value = 'uploading'
      if (init.multipart) {
        // Persist BEFORE the first PUT so even an immediate failure is resumable.
        activeStore = useResourceResumeUploads(galgameId)
        activeStore.upsert({
          artifactUuid: init.artifact_uuid,
          name: file.name,
          size: file.size,
          lastModified: file.lastModified,
          progress: 0,
          updatedAt: Date.now()
        })
        const parts = await putParts(file, init.part_size ?? 0, init.parts ?? [], 0)
        if (cancelled) throw new Error('已取消上传')
        const result = await complete(init.artifact_uuid, parts)
        // Done — drop the resume record; this artifact is now a real resource.
        activeStore.remove(init.artifact_uuid)
        return result
      }

      // Single-PUT (< 50MB): cheap to redo, so no resume bookkeeping — a failure
      // just aborts and the user re-inits.
      try {
        await putToS3(init.upload_url!, file, (delta) => {
          uploadedBytes.value += delta
        })
      } catch (e) {
        api.post('/upload/abort', { artifact_uuid: init.artifact_uuid }).catch(() => {})
        abortUuid = null
        throw e
      }
      if (cancelled) throw new Error('已取消上传')
      return await complete(init.artifact_uuid)
    } catch (e) {
      const msg = e instanceof Error ? e.message : '上传失败'
      status.value = cancelled ? 'aborted' : 'error'
      errorMessage.value = msg
      throw e
    }
  }

  // Continue an interrupted multipart upload from its breakpoint: ask the server
  // which parts already landed in B2 (skip + reuse their ETags) and PUT only the
  // missing ones, then complete. Falls back to a fresh upload if the session is
  // gone (GC'd / completed / expired → resume errors).
  const resumeUpload = async (
    file: File,
    galgameId: number,
    artifactUuid: string
  ): Promise<UploadResult> => {
    cancelled = false
    errorMessage.value = ''
    uploadedBytes.value = 0
    totalBytes.value = file.size
    abortUuid = artifactUuid
    activeStore = useResourceResumeUploads(galgameId)

    let resume: ResumeResponse
    try {
      status.value = 'preparing'
      const res = await api.post<ResumeResponse>('/upload/resume', {
        artifact_uuid: artifactUuid
      })
      if (res.code !== 0 || !res.data) {
        throw new Error(res.message || '续传初始化失败')
      }
      resume = res.data
    } catch {
      // No longer resumable — drop the stale record and start over.
      activeStore.remove(artifactUuid)
      return upload(file, galgameId)
    }

    try {
      status.value = 'uploading'
      if (!resume.multipart) {
        // Single-part resume = re-PUT the whole file to the fresh URL.
        await putToS3(resume.upload_url!, file, (delta) => {
          uploadedBytes.value += delta
        })
        if (cancelled) throw new Error('已取消上传')
        const result = await complete(artifactUuid)
        activeStore.remove(artifactUuid)
        return result
      }

      const uploaded = resume.uploaded_parts ?? []
      const missing = resume.parts ?? []
      // Start the live bar AND the committed-bytes base where the already-stored
      // parts leave off.
      const committedBase = uploaded.reduce((sum, p) => sum + p.size, 0)
      uploadedBytes.value = committedBase
      const fresh = await putParts(file, resume.part_size ?? 0, missing, committedBase)
      if (cancelled) throw new Error('已取消上传')
      const parts = [
        ...uploaded.map((p) => ({ part_number: p.part_number, etag: p.etag })),
        ...fresh
      ].sort((a, b) => a.part_number - b.part_number)
      const result = await complete(artifactUuid, parts)
      activeStore.remove(artifactUuid)
      return result
    } catch (e) {
      // Keep the resume record — the uploaded parts stay in B2 for another try.
      const msg = e instanceof Error ? e.message : '续传失败'
      status.value = cancelled ? 'aborted' : 'error'
      errorMessage.value = msg
      throw e
    }
  }

  // User-initiated cancel: abort in-flight XHRs + tell the server to soft-delete
  // the in-progress artifact, and drop its resume record (explicit give-up).
  const cancel = () => {
    cancelled = true
    activeXhrs.forEach((x) => x.abort())
    activeXhrs.length = 0
    if (abortUuid) {
      api.post('/upload/abort', { artifact_uuid: abortUuid }).catch(() => {})
      activeStore?.remove(abortUuid)
      abortUuid = null
    }
    status.value = 'aborted'
  }

  return {
    status: readonly(status),
    uploadedBytes: readonly(uploadedBytes),
    totalBytes: readonly(totalBytes),
    progressPercent,
    errorMessage: readonly(errorMessage),
    upload,
    resumeUpload,
    cancel,
    reset
  }
}
