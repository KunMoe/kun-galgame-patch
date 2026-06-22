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
//   POST /upload/abort    → soft-delete on user cancel / fatal mid-upload error
//
// The 1 GB cap mirrors Go's internal/constants/upload.go for an instant local
// error; the server enforces it (and the per-user quota) too.

const MAX_LARGE_FILE_SIZE = 1024 * 1024 * 1024
// 4 parallel PUTs is the sweet spot for residential uplinks — much more and the
// browser's per-host connection cap queues anyway, less leaves bandwidth idle.
const PARALLEL_PARTS = 4

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

export const useResourceUpload = () => {
  const api = useApi()
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

  // Upload all multipart parts (server-driven part_size + presigned part URLs),
  // PARALLEL_PARTS at a time. Returns the per-part ETags for complete.
  const uploadParts = async (
    file: File,
    init: InitResponse
  ): Promise<{ part_number: number; etag: string }[]> => {
    const partSize = init.part_size ?? 0
    const urls = init.parts ?? []
    const n = urls.length
    const etags = new Array<string>(n).fill('')
    let next = 0
    let firstError: Error | null = null

    const worker = async () => {
      while (next < n && !firstError) {
        const i = next++
        const p = urls[i]!
        const start = (p.part_number - 1) * partSize
        const end = Math.min(start + partSize, file.size)
        const blob = file.slice(start, end)
        try {
          etags[i] = await putToS3(p.url, blob, (delta) => {
            uploadedBytes.value += delta
          })
        } catch (e) {
          if (!firstError) firstError = e as Error
        }
      }
    }

    await Promise.all(
      Array.from({ length: Math.min(PARALLEL_PARTS, n) }, worker)
    )
    if (firstError) throw firstError
    return urls.map((p, i) => ({ part_number: p.part_number, etag: etags[i]! }))
  }

  const upload = async (
    file: File,
    galgameId: number
  ): Promise<UploadResult> => {
    reset()
    if (file.size > MAX_LARGE_FILE_SIZE) {
      const msg = '文件大小超过 1GB 上限'
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
      let parts: { part_number: number; etag: string }[] | undefined
      if (init.multipart) {
        try {
          parts = await uploadParts(file, init)
        } catch (e) {
          // Best-effort cleanup so the in-progress artifact doesn't linger.
          api.post('/upload/abort', { artifact_uuid: init.artifact_uuid }).catch(() => {})
          abortUuid = null
          throw e
        }
      } else {
        await putToS3(init.upload_url!, file, (delta) => {
          uploadedBytes.value += delta
        })
      }
      if (cancelled) throw new Error('已取消上传')

      status.value = 'completing'
      const completeRes = await api.post<CompleteResponse>('/upload/complete', {
        artifact_uuid: init.artifact_uuid,
        declared_size: file.size,
        ...(parts ? { parts } : {})
      })
      abortUuid = null
      if (completeRes.code !== 0 || !completeRes.data) {
        throw new Error(completeRes.message || '完成上传失败')
      }

      status.value = 'done'
      uploadedBytes.value = totalBytes.value // pin to 100%
      return {
        artifactUuid: completeRes.data.artifact_uuid,
        size: completeRes.data.size
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : '上传失败'
      status.value = cancelled ? 'aborted' : 'error'
      errorMessage.value = msg
      throw e
    }
  }

  // User-initiated cancel: abort in-flight XHRs + tell the server to soft-delete
  // the in-progress artifact so it doesn't sit as an orphan.
  const cancel = () => {
    cancelled = true
    activeXhrs.forEach((x) => x.abort())
    activeXhrs.length = 0
    if (abortUuid) {
      api.post('/upload/abort', { artifact_uuid: abortUuid }).catch(() => {})
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
    cancel,
    reset
  }
}
