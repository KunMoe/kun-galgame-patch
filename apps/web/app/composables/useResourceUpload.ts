// Drives the D10 patch-resource upload flow (apps/api/internal/common/upload).
//
// Two paths share one composable so the Publish modal can stay file-size
// agnostic — caller just passes the File + galgameId and watches `progress`.
//
//   ≤ 200 MB : POST /upload/small/init     → presigned PUT URL
//              PUT file directly to S3
//              POST /upload/small/complete → server HeadObject + quota deduct
//
//   > 200 MB : POST /upload/multipart/init     → upload_id + N presigned URLs
//              PUT each 10 MiB part (parallel up to PARALLEL_PARTS)
//              POST /upload/multipart/complete → CompleteMultipartUpload + verify
//              (POST /upload/multipart/abort on user cancel or fatal mid-upload error)
//
// The 200 MB / 1 GB / 10 MiB thresholds mirror Go's internal/constants/upload.go.
// Keep them in sync — Go enforces them server-side too, but matching here gives
// the user an instant local error instead of a round-trip 4xx.

const MAX_SMALL_FILE_SIZE = 200 * 1024 * 1024
const MAX_LARGE_FILE_SIZE = 1024 * 1024 * 1024
const MULTIPART_PART_SIZE = 10 * 1024 * 1024
// 4 parallel PUTs is the sweet spot for residential uplinks — much more and
// the browser's per-host connection cap starts queueing anyway, less and we
// leave bandwidth idle.
const PARALLEL_PARTS = 4

interface SmallInitResponse {
  s3_key: string
  upload_url: string
}

interface MultipartInitResponse {
  s3_key: string
  upload_id: string
  part_urls: string[]
}

interface CompleteResponse {
  s3_key: string
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
  s3Key: string
  size: number // server-verified bytes
}

export const useResourceUpload = () => {
  const api = useApi()
  const status = ref<UploadStatus>('idle')
  // Per-byte progress so the bar moves smoothly even during a single large
  // small-file PUT. Multipart aggregates uploaded-bytes-so-far across parts.
  const uploadedBytes = ref(0)
  const totalBytes = ref(0)
  const errorMessage = ref('')

  // Multipart abort context — kept so the user can cancel mid-upload and the
  // backend cleans up the orphan upload_id rather than waiting for the
  // 24h cron sweep.
  let multipartAbort: { s3Key: string; uploadId: string } | null = null
  let cancelled = false

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
    multipartAbort = null
    cancelled = false
  }

  // PUT one blob to a presigned URL with progress reporting. Native fetch
  // has no upload progress event, so we drop to XHR for this — it's the only
  // browser API that does. Resolves with the ETag for multipart, '' for small.
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
          // Report deltas so the multipart caller can sum across parallel parts
          // without bookkeeping per-part offsets.
          onProgress(e.loaded - lastLoaded)
          lastLoaded = e.loaded
        }
      })
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          // Multipart needs the part ETag for CompleteMultipartUpload. Header
          // comes back quoted ("abc123") — minio-go accepts either form but we
          // strip for cleanliness.
          const etag = (xhr.getResponseHeader('ETag') || '').replace(/"/g, '')
          resolve(etag)
        } else {
          reject(new Error(`S3 PUT 失败 (HTTP ${xhr.status})`))
        }
      })
      xhr.addEventListener('error', () =>
        reject(new Error('网络中断，请检查连接'))
      )
      xhr.addEventListener('abort', () => reject(new Error('已取消上传')))
      // Save the abort handle so cancel() can stop the request immediately.
      activeXhrs.push(xhr)
      xhr.send(body)
    })

  // Track active XHRs so cancel() can yank them all. Cleared after each flow.
  const activeXhrs: XMLHttpRequest[] = []

  const uploadSmall = async (file: File, galgameId: number): Promise<UploadResult> => {
    status.value = 'preparing'
    const initRes = await api.post<SmallInitResponse>('/upload/small/init', {
      galgame_id: galgameId,
      file_name: file.name,
      file_size: file.size
    })
    if (initRes.code !== 0 || !initRes.data) {
      throw new Error(initRes.message || '初始化上传失败')
    }
    const { s3_key, upload_url } = initRes.data

    status.value = 'uploading'
    await putToS3(upload_url, file, (delta) => {
      uploadedBytes.value += delta
    })
    if (cancelled) throw new Error('已取消上传')

    status.value = 'completing'
    const completeRes = await api.post<CompleteResponse>(
      '/upload/small/complete',
      { s3_key, declared_size: file.size }
    )
    if (completeRes.code !== 0 || !completeRes.data) {
      throw new Error(completeRes.message || '完成上传失败')
    }
    return { s3Key: completeRes.data.s3_key, size: completeRes.data.size }
  }

  const uploadMultipart = async (
    file: File,
    galgameId: number
  ): Promise<UploadResult> => {
    const partCount = Math.ceil(file.size / MULTIPART_PART_SIZE)
    status.value = 'preparing'
    const initRes = await api.post<MultipartInitResponse>(
      '/upload/multipart/init',
      {
        galgame_id: galgameId,
        file_name: file.name,
        file_size: file.size,
        part_count: partCount
      }
    )
    if (initRes.code !== 0 || !initRes.data) {
      throw new Error(initRes.message || '初始化分片上传失败')
    }
    const { s3_key, upload_id, part_urls } = initRes.data
    multipartAbort = { s3Key: s3_key, uploadId: upload_id }

    status.value = 'uploading'

    // Worker pool: parts is a fixed queue, each worker grabs the next index
    // and PUTs that part. parallel by PARALLEL_PARTS; resolves with [etag, ...]
    // in part_number order.
    const etags: string[] = new Array(partCount).fill('')
    let nextPart = 0
    let firstError: Error | null = null

    const worker = async () => {
      while (nextPart < partCount && !firstError) {
        const i = nextPart++
        const start = i * MULTIPART_PART_SIZE
        const end = Math.min(start + MULTIPART_PART_SIZE, file.size)
        const blob = file.slice(start, end)
        try {
          const etag = await putToS3(part_urls[i]!, blob, (delta) => {
            uploadedBytes.value += delta
          })
          etags[i] = etag
        } catch (e) {
          if (!firstError) firstError = e as Error
        }
      }
    }

    await Promise.all(
      Array.from({ length: Math.min(PARALLEL_PARTS, partCount) }, worker)
    )

    if (firstError) {
      // Best-effort cleanup; ignore abort errors so the user sees the real
      // upload failure not the cleanup noise.
      api
        .post('/upload/multipart/abort', { s3_key, upload_id })
        .catch(() => {})
      multipartAbort = null
      throw firstError
    }
    if (cancelled) throw new Error('已取消上传')

    status.value = 'completing'
    const completeRes = await api.post<CompleteResponse>(
      '/upload/multipart/complete',
      {
        s3_key,
        upload_id,
        declared_size: file.size,
        parts: etags.map((etag, idx) => ({ part_number: idx + 1, etag }))
      }
    )
    multipartAbort = null
    if (completeRes.code !== 0 || !completeRes.data) {
      throw new Error(completeRes.message || '完成分片上传失败')
    }
    return { s3Key: completeRes.data.s3_key, size: completeRes.data.size }
  }

  const upload = async (file: File, galgameId: number): Promise<UploadResult> => {
    reset()
    if (file.size > MAX_LARGE_FILE_SIZE) {
      const msg = '文件大小超过 1GB 上限'
      status.value = 'error'
      errorMessage.value = msg
      throw new Error(msg)
    }
    totalBytes.value = file.size
    try {
      const result =
        file.size <= MAX_SMALL_FILE_SIZE
          ? await uploadSmall(file, galgameId)
          : await uploadMultipart(file, galgameId)
      status.value = 'done'
      // Pin the bar at 100% even if XHR's last progress event under-reported.
      uploadedBytes.value = totalBytes.value
      return result
    } catch (e) {
      const msg = e instanceof Error ? e.message : '上传失败'
      status.value = cancelled ? 'aborted' : 'error'
      errorMessage.value = msg
      throw e
    }
  }

  // User-initiated cancel: abort any in-flight XHRs and tell the server to
  // tear down the multipart upload so it doesn't sit as an orphan.
  const cancel = () => {
    cancelled = true
    activeXhrs.forEach((x) => x.abort())
    activeXhrs.length = 0
    if (multipartAbort) {
      api
        .post('/upload/multipart/abort', {
          s3_key: multipartAbort.s3Key,
          upload_id: multipartAbort.uploadId
        })
        .catch(() => {})
      multipartAbort = null
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
    cancel,
    reset
  }
}
