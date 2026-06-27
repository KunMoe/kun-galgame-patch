<script setup lang="ts">
// Patch-resource publish / edit modal.
//
// Same component covers both create and edit modes so layout + validation +
// upload code don't fork. Mode is decided by whether `resource` prop is
// supplied:
//   - create: POST /patch/:id/resource         — fresh row
//   - edit:   PUT  /patch/resource/:resourceId — mutate existing
//
// Two storage modes (mutually exclusive form shapes):
//   storage='s3'   : a file is uploaded; server stamps content = s3_key and
//                    materializes the download URL at /resource/:id/link time
//                    via S3Client.PublicURL.
//   storage='user' : `content` is the user's own comma-separated link list.
//
// Edit-mode niceties:
//   - existing s3 file is shown as-is; user must click "替换文件" to enter
//     the upload flow (otherwise PUT carries the unchanged s3_key/content and
//     server skips the file-history audit row — pure metadata edit).
//   - a "修改原因 / reason" field appears, gets serialized into the
//     PatchResourceFileHistory.reason when the file actually changes.
//
// Drag-and-drop: the drop zone wraps the upload section. When the user is
// dragging a file over the modal, the zone highlights; on drop we feed the
// File through the same handleFilePicked path as the click-to-pick button.
// KunFileInput itself has no DnD support, so we implement it locally.
import {
  resourceTypes,
  storageTypes,
  SUPPORTED_LANGUAGE,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM,
  SUPPORTED_PLATFORM_MAP,
  ALLOWED_EXTENSIONS
} from '~/constants/resource'

interface Props {
  patchId: number
  // When supplied → edit mode; the form is pre-populated from this row and
  // submit goes via PUT instead of POST. Pass the same PatchResource shape
  // the list endpoint returns.
  resource?: PatchResource | null
}
const props = withDefaults(defineProps<Props>(), { resource: null })

const emit = defineEmits<{
  close: []
  // create mode: full new row (composeResource shape).
  // edit mode:   the locally-mutated row (server returns OKMessage, no body,
  //              so we hand back the form state merged onto the original).
  success: [resource: PatchResource]
}>()

const api = useApi()
const userStore = useUserStore()
const uploader = useResourceUpload()

const isEdit = computed(() => props.resource !== null)

// ─── Form state ───────────────────────────────────────────
const form = reactive({
  storage: (props.resource?.storage as 's3' | 'user') ?? 's3',
  name: props.resource?.name ?? '',
  model_name: props.resource?.model_name ?? '',
  // edit-mode pre-fills with the existing s3_key (legacy rows) — kept as-is
  // unless the user replaces the file via the "替换文件" affordance below.
  s3_key: props.resource?.s3_key ?? '',
  // artifact-backed blob id (current upload path). Set on a fresh upload;
  // pre-filled in edit mode so a metadata-only save preserves the file pointer.
  artifact_uuid: props.resource?.artifact_uuid ?? '',
  // For storage='user' content is the link list; for storage='s3' the server
  // overwrites Content = S3Key on submit, so this field is informational only.
  // Pre-fill with the raw stored value (it's the s3_key, not the public URL).
  content: props.resource?.content ?? '',
  size: props.resource?.size ?? '',
  code: props.resource?.code ?? '',
  password: props.resource?.password ?? '',
  note: props.resource?.note ?? '',
  type: [...(props.resource?.type ?? [])],
  language: [...(props.resource?.language ?? [])],
  platform: [...(props.resource?.platform ?? [])]
})
// Reason memo — only meaningful in edit mode + only persisted into the
// patch_resource_file_history row when the file substantively changed
// (storage / s3_key / content differs from current). Pure metadata edits
// don't write history regardless of whether reason was filled.
const reason = ref('')

const toggle = (list: string[], v: string) => {
  const i = list.indexOf(v)
  if (i >= 0) list.splice(i, 1)
  else list.push(v)
}

const storageOptions = storageTypes.map((t) => ({
  value: t.value,
  label: t.label
}))

watch(
  () => form.storage,
  () => {
    // Switching storage type invalidates any in-progress upload state and
    // the previous file pointer — fresh start in both create and edit.
    form.s3_key = ''
    form.artifact_uuid = ''
    form.content = ''
    form.size = ''
    uploader.reset()
    pickedFile.value = null
    replaceMode.value = false
    resumeUuid.value = null
  }
)

// ─── File picker + upload ──────────────────────────────────
const pickedFile = ref<File | null>(null)
const uploadError = ref('')
// In edit mode the original file is shown until the user clicks "替换文件".
// replaceMode flips the UI from "file summary card" to "drop zone + picker".
const replaceMode = ref(false)
// ─── Resumable uploads ────────────────────────────────────
// Interrupted multipart uploads for THIS galgame are persisted (uuid + file
// identity + progress) in localStorage so the modal can offer to continue them
// across a page reload. resumeUuid is set when the staged file matches a pending
// record (size+lastModified) — submit then resumes from the breakpoint instead
// of restarting; the already-uploaded parts live in B2 on the artifact side.
const resumeStore = useResourceResumeUploads(props.patchId)
const pending = ref<PatchPendingUpload[]>([])
const resumeUuid = ref<string | null>(null)
const refreshPending = () => {
  pending.value = resumeStore.list()
}
// Match the staged file against the pending records (size+lastModified, not name)
// so a re-picked / moved / renamed-but-identical file resumes from its breakpoint.
const syncResumeForPicked = () => {
  const f = pickedFile.value
  const match = f
    ? resumeStore
        .list()
        .find((p) => p.size === f.size && p.lastModified === f.lastModified)
    : undefined
  resumeUuid.value = match ? match.artifactUuid : null
}
onMounted(refreshPending)

// True iff we're showing the existing file with no replacement chosen.
// Used to lock the file-card affordances to a read-only summary.
const showingExistingFile = computed(
  () =>
    isEdit.value &&
    form.storage === 's3' &&
    !replaceMode.value &&
    !pickedFile.value
)

// ─── Drag and drop ────────────────────────────────────────
// Counter (not boolean) so nested children's dragenter/dragleave don't flicker
// the highlight off — every enter is matched by exactly one leave per element.
const dragDepth = ref(0)
const isDragging = computed(() => dragDepth.value > 0)
const onDragEnter = (e: DragEvent) => {
  if (!e.dataTransfer?.types?.includes('Files')) return
  e.preventDefault()
  dragDepth.value++
}
const onDragOver = (e: DragEvent) => {
  if (!e.dataTransfer?.types?.includes('Files')) return
  e.preventDefault()
  // dropEffect=copy gives the OS-level "+" cursor; UX cue that drop is allowed.
  e.dataTransfer.dropEffect = 'copy'
}
const onDragLeave = (e: DragEvent) => {
  e.preventDefault()
  if (dragDepth.value > 0) dragDepth.value--
}
const onDrop = (e: DragEvent) => {
  e.preventDefault()
  dragDepth.value = 0
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  // In edit mode dropping a file always enters replaceMode — matches the
  // click-to-pick path's semantics.
  if (showingExistingFile.value) replaceMode.value = true
  handleFilePicked([file])
}

const isValidExt = (name: string) => {
  const lower = name.toLowerCase()
  return ALLOWED_EXTENSIONS.some((ext) => lower.endsWith(ext))
}

// Stage-then-confirm flow:
//   handleFilePicked  → validate ext + setPickedFile (no upload, instant)
//   confirmUpload     → user-initiated; only now do we hit the upload API
//
// Reading File.name / size / type is O(1) metadata — even a 5 GB drop is
// instant. Bytes aren't touched until xhr.send(blob) inside the composable,
// and that's stream-read by the browser one chunk at a time (multipart caps
// at 10 MiB × 4 parallel = ~40 MiB resident).
const handleFilePicked = (files: File[]) => {
  const f = files[0]
  if (!f) return
  if (!isValidExt(f.name)) {
    useKunMessage(`仅支持 ${ALLOWED_EXTENSIONS.join(' ')} 格式`, 'warn')
    return
  }
  uploadError.value = ''
  // Drop / re-pick after an aborted or errored upload: reset uploader state
  // so the new file's confirm button starts clean (no stale progress %).
  uploader.reset()
  pickedFile.value = f
  // Re-picking the file of an interrupted upload offers to resume from the
  // breakpoint instead of restarting.
  syncResumeForPicked()
}

// New uploads are artifact-backed; clear any stale legacy s3_key.
const applyUploadResult = (result: { artifactUuid: string; size: number }) => {
  form.artifact_uuid = result.artifactUuid
  form.s3_key = ''
  form.size = `${(result.size / (1024 * 1024)).toFixed(3)} MB`
}

const confirmUpload = async () => {
  const file = pickedFile.value
  if (!file) return
  uploadError.value = ''
  try {
    const result = resumeUuid.value
      ? await uploader.resumeUpload(file, props.patchId, resumeUuid.value)
      : await uploader.upload(file, props.patchId)
    applyUploadResult(result)
  } catch (e) {
    uploadError.value = e instanceof Error ? e.message : '上传失败'
  } finally {
    // A completed upload drops its resume record (→ resumeUuid clears); an
    // interrupted one keeps it so 重试 resumes from the breakpoint.
    refreshPending()
    syncResumeForPicked()
  }
}

// From the on-open resume list: the user re-picked the matching file (validated
// in ResumeList), so stage it and continue from the breakpoint.
const handleContinuePending = async (
  record: PatchPendingUpload,
  file: File
) => {
  uploader.reset()
  pickedFile.value = file
  resumeUuid.value = record.artifactUuid
  uploadError.value = ''
  try {
    const result = await uploader.resumeUpload(
      file,
      props.patchId,
      record.artifactUuid
    )
    applyUploadResult(result)
  } catch (e) {
    uploadError.value = e instanceof Error ? e.message : '续传失败'
  } finally {
    refreshPending()
    syncResumeForPicked()
  }
}

// Discard an unfinished upload: soft-delete the artifact (its B2 parts are
// reclaimed by GC) and drop the local record.
const handleDeletePending = async (artifactUuid: string) => {
  await api.post('/upload/abort', { artifact_uuid: artifactUuid }).catch(() => {})
  resumeStore.remove(artifactUuid)
  if (resumeUuid.value === artifactUuid) resumeUuid.value = null
  refreshPending()
}

const removeFile = () => {
  // Cancel covers in-flight uploads (xhr.abort + multipart abort); for staged
  // files it's a no-op since uploader.status is still 'idle'.
  uploader.cancel()
  uploader.reset()
  pickedFile.value = null
  if (isEdit.value && props.resource) {
    // Restore the existing-file summary so the user can submit without
    // changing the file (treat "移除" in edit mode as "cancel my replacement").
    form.s3_key = props.resource.s3_key ?? ''
    form.artifact_uuid = props.resource.artifact_uuid ?? ''
    form.size = props.resource.size ?? ''
    replaceMode.value = false
  } else {
    form.s3_key = ''
    form.artifact_uuid = ''
    form.size = ''
  }
  // cancel() aborts+drops the record only if a flow started this session; an
  // old pending record (matched but never resumed) stays for the ResumeList.
  resumeUuid.value = null
  refreshPending()
}

const uploadingNow = computed(
  () =>
    uploader.status.value === 'preparing' ||
    uploader.status.value === 'uploading' ||
    uploader.status.value === 'completing'
)

// Staged = file picked but upload not yet started (or failed/aborted, ready
// to retry). Distinguished from `uploadingNow` and `uploadedOk` so the UI
// can offer the appropriate primary action.
const uploadedOk = computed(() => uploader.status.value === 'done')
const stagedNotUploaded = computed(
  () => pickedFile.value !== null && !uploadingNow.value && !uploadedOk.value
)

// Format File.size (bytes) — used pre-upload while we only have the local
// File. After upload completes the server-verified size lands in form.size.
const formatBytesMB = (n: number) => `${(n / (1024 * 1024)).toFixed(3)} MB`

// ─── User-link mode (storage='user') ────────────────────────
const userLinks = computed<string[]>({
  get: () =>
    form.storage === 'user'
      ? form.content
          .split(',')
          .map((s) => s.trim())
          .filter((s) => s.length > 0 || form.content.endsWith(','))
      : [],
  set: (v) => {
    form.content = v.join(',')
  }
})
const addUserLink = () => {
  const next = [...userLinks.value, '']
  form.content = next.join(',')
}
const updateUserLink = (i: number, v: string) => {
  const next = [...userLinks.value]
  next[i] = v
  form.content = next.join(',')
}
const removeUserLink = (i: number) => {
  if (userLinks.value.length <= 1) return
  const next = userLinks.value.filter((_, idx) => idx !== i)
  form.content = next.join(',')
}

// ─── Validation + submit ───────────────────────────────────
const submitting = ref(false)
const validate = (): string | null => {
  if (form.type.length === 0) return '请选择资源类型'
  if (form.language.length === 0) return '请选择语言'
  if (form.platform.length === 0) return '请选择平台'
  if (!form.size.trim()) return '请填写资源大小'
  if (form.storage === 's3') {
    // staged-but-not-uploaded: distinct message vs no-file-at-all so the
    // user knows the next step is "点击确认上传" rather than "重选文件".
    if (stagedNotUploaded.value) return '请点击 "确认上传" 完成文件上传'
    if (uploadingNow.value) return '文件正在上传中，请稍候'
    // artifact-backed (new/replaced) OR legacy s3_key (metadata-only edit).
    if (!form.artifact_uuid && !form.s3_key) return '请上传补丁文件'
  } else {
    if (userLinks.value.filter((l) => l.trim()).length === 0)
      return '请至少添加一条资源链接'
  }
  return null
}

const handleSubmit = async () => {
  const err = validate()
  if (err) {
    useKunMessage(err, 'warn')
    return
  }
  submitting.value = true
  try {
    const basePayload = {
      galgame_id: props.patchId,
      storage: form.storage,
      name: form.name,
      model_name: form.model_name,
      artifact_uuid: form.artifact_uuid,
      s3_key: form.s3_key,
      content: form.content,
      size: form.size,
      code: form.code,
      password: form.password,
      note: form.note,
      type: form.type,
      language: form.language,
      platform: form.platform
    }

    if (isEdit.value && props.resource) {
      // Server returns the fully-rendered row (note_html, update_time, user
      // brief all re-resolved server-side) — use it directly. The previous
      // hand-rolled merge kept the old note_html so the resource description
      // appeared "stuck" until a full page refetch.
      const res = await api.put<PatchResource>(
        `/patch/resource/${props.resource.id}`,
        { ...basePayload, reason: reason.value }
      )
      if (res.code === 0 && res.data) {
        useKunMessage('资源已更新', 'success')
        emit('success', res.data)
        emit('close')
      } else {
        useKunMessage(res.message || '更新失败', 'error')
      }
    } else {
      const res = await api.post<PatchResource>(
        `/patch/${props.patchId}/resource`,
        basePayload
      )
      if (res.code === 0 && res.data) {
        useKunMessage('资源发布成功', 'success')
        emit('success', res.data)
        emit('close')
      } else {
        useKunMessage(res.message || '发布失败', 'error')
      }
    }
  } finally {
    submitting.value = false
  }
}

// ─── Display helpers ───────────────────────────────────────

const STATUS_LABEL: Record<string, string> = {
  preparing: '正在准备上传...',
  uploading: '正在上传中...',
  completing: '正在校验文件...',
  done: '上传文件成功',
  error: '上传文件失败',
  aborted: '已取消上传'
}
const uploadStatusLabel = computed(
  () => STATUS_LABEL[uploader.status.value] ?? ''
)

const quotaLimitBytes = computed(() =>
  userStore.isModerator ? 5 * 1024 * 1024 * 1024 : 100 * 1024 * 1024
)
const quotaUsedMB = computed(() =>
  (userStore.user.daily_upload_size / (1024 * 1024)).toFixed(3)
)
const quotaLimitMB = computed(() =>
  (quotaLimitBytes.value / (1024 * 1024)).toFixed(0)
)
const quotaPercent = computed(() =>
  Math.min(
    100,
    Math.round(
      (userStore.user.daily_upload_size / quotaLimitBytes.value) * 100
    )
  )
)

// File name to display in the "existing file" summary card. We don't have the
// original filename in the row (s3_key is sanitized path); show the trailing
// segment so the user at least recognizes it.
const existingFileName = computed(() => {
  if (!props.resource?.s3_key) return ''
  const parts = props.resource.s3_key.split('/')
  return parts[parts.length - 1] ?? props.resource.s3_key
})
</script>

<template>
  <!-- Outer wrapper owns drag-and-drop listeners so a file dropped anywhere
       inside the modal (not just on the picker button) is captured. The
       highlight ring on `isDragging` is a UX cue. -->
  <div
    class="flex max-h-[80vh] w-full max-w-2xl flex-col gap-4"
    :class="
      cn(
        'ring-2 ring-transparent transition',
        isDragging && 'ring-primary/60 ring-offset-2'
      )
    "
    @dragenter="onDragEnter"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <!-- header -->
    <div class="space-y-2">
      <h2 class="text-xl font-bold">
        {{ isEdit ? '编辑补丁资源' : '创建补丁资源' }}
      </h2>
      <div class="text-default-500 space-y-1 text-sm">
        <div class="flex flex-wrap gap-x-4">
          <NuxtLink
            to="/doc/notice/patch-tutorial"
            class="text-primary hover:underline"
          >
            鲲 Galgame 补丁资源系统介绍
          </NuxtLink>
          <NuxtLink
            to="/doc/notice/paradigm"
            class="text-primary hover:underline"
          >
            鲲 Galgame 补丁资源发布规范
          </NuxtLink>
        </div>
        <p>
          今日已使用存储 <strong>{{ quotaUsedMB }} MB</strong> /
          {{ quotaLimitMB }} MB（{{
            userStore.isModerator ? '创作者' : '普通用户'
          }}每日额度，每天早上 8 点重置）
        </p>
        <KunProgress :value="quotaPercent" size="sm" />
      </div>
    </div>

    <!-- body (scrollable) -->
    <div class="-mx-1 flex-1 overflow-y-auto px-1">
      <form class="space-y-6" @submit.prevent="handleSubmit">
        <!-- storage type ----------------------------------- -->
        <section class="space-y-2">
          <h3 class="text-lg font-medium">选择存储类型</h3>
          <p class="text-default-500 text-sm">
            确定您的补丁体积大小以便选择合适的存储方式
          </p>
          <KunSelect
            v-model="form.storage"
            label="请选择您的资源存储类型"
            :options="storageOptions"
          />
        </section>

        <!-- file uploader (s3 mode) ----------------------- -->
        <section v-if="form.storage === 's3'" class="space-y-2">
          <h3 class="text-lg font-medium">
            {{ isEdit ? '补丁文件' : '上传资源' }}
          </h3>
          <p class="text-default-500 text-sm">
            支持 {{ ALLOWED_EXTENSIONS.join(' ') }} 压缩格式，单文件最大 1 GB。
            <span v-if="isDragging" class="text-primary font-medium">
              松开鼠标以上传文件
            </span>
            <span v-else>
              支持点击选择或直接拖拽到此处。
            </span>
          </p>

          <!-- Interrupted uploads for this galgame (persisted across reloads).
               Resuming needs the user to re-pick the file (the browser can't
               read it by path), so the hint spells that out. -->
          <div v-if="pending.length && !pickedFile" class="space-y-2">
            <div
              class="text-warning border-warning/30 bg-warning/10 flex items-start gap-2 rounded-lg border p-3 text-sm"
            >
              <KunIcon name="lucide:history" class="mt-0.5 shrink-0" />
              <span>
                您有 {{ pending.length }}
                个未完成的上传，点击「继续上传」后重新选择同一文件即可从断点续传
              </span>
            </div>
            <ResourceResumeList
              :pending="pending"
              @continue="handleContinuePending"
              @delete="handleDeletePending"
            />
          </div>

          <!-- Existing file summary (edit mode, no replacement chosen) -->
          <div
            v-if="showingExistingFile"
            class="border-default/20 bg-default-50 flex items-center justify-between gap-3 rounded-lg border p-4"
          >
            <div class="flex min-w-0 items-center gap-2">
              <KunIcon
                name="lucide:file-check"
                class="text-success size-6 shrink-0"
              />
              <div class="min-w-0">
                <p class="truncate font-medium">{{ existingFileName }}</p>
                <p class="text-default-500 text-sm">
                  当前文件 · {{ form.size }}
                </p>
              </div>
            </div>
            <KunButton
              color="primary"
              variant="flat"
              size="sm"
              @click="replaceMode = true"
            >
              <KunIcon name="lucide:refresh-cw" class="size-4" />
              替换文件
            </KunButton>
          </div>

          <!-- Picker (create mode, or edit-mode replacement chosen) -->
          <div v-else-if="!pickedFile" class="space-y-2">
            <div
              :class="
                cn(
                  'border-default-200 rounded-lg border-2 border-dashed p-6 text-center transition-colors',
                  isDragging && 'border-primary bg-primary/10'
                )
              "
            >
              <KunIcon
                name="lucide:upload-cloud"
                class="text-default-400 mx-auto mb-2 size-10"
              />
              <p class="text-default-600 mb-3 text-sm">
                {{ isDragging ? '松开鼠标放下文件' : '拖拽文件到此处，或点击下方按钮选择' }}
              </p>
              <KunFileInput
                :accept="ALLOWED_EXTENSIONS.join(',')"
                :max-size="1024 * 1024 * 1024"
                :trigger-text="isEdit ? '选择替换文件' : '选择补丁文件'"
                trigger-icon="lucide:file-up"
                :error="uploadError"
                @change="handleFilePicked"
                @error-pick="(msg) => (uploadError = msg)"
              />
              <KunButton
                v-if="isEdit && replaceMode"
                variant="light"
                color="default"
                size="sm"
                class-name="mt-2"
                @click="
                  () => {
                    replaceMode = false
                    if (props.resource) {
                      form.s3_key = props.resource.s3_key ?? ''
                      form.size = props.resource.size ?? ''
                    }
                  }
                "
              >
                取消替换，保留原文件
              </KunButton>
            </div>
          </div>

          <!-- File card — three states stacked vertically depending on the
               uploader's status:
                 staged       : show "确认上传" (primary) + "移除"
                 uploading    : show progress bar + "取消上传"
                 uploaded     : show success badge + "重新选择"
                 error/aborted: show error + "重试上传" / "移除" -->
          <div
            v-else
            class="border-default/20 bg-default-50 space-y-3 rounded-lg border p-4"
          >
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2">
                <KunIcon
                  :name="uploadedOk ? 'lucide:file-check' : 'lucide:file'"
                  :class="
                    cn('size-6 shrink-0', uploadedOk ? 'text-success' : 'text-primary')
                  "
                />
                <div class="min-w-0">
                  <p class="truncate font-medium">{{ pickedFile.name }}</p>
                  <p class="text-default-500 text-sm">
                    {{ formatBytesMB(pickedFile.size) }}
                    <span v-if="uploadedOk" class="text-success ml-1">· 已上传</span>
                    <span v-else-if="stagedNotUploaded" class="text-default-400 ml-1">· 等待上传</span>
                  </p>
                </div>
              </div>

              <!-- Primary action button — semantics vary by state -->
              <div class="flex gap-2">
                <!-- staged: 确认上传 -->
                <KunButton
                  v-if="stagedNotUploaded && uploader.status.value !== 'error'"
                  color="primary"
                  size="sm"
                  @click="confirmUpload"
                >
                  <KunIcon name="lucide:cloud-upload" class="size-4" />
                  {{ resumeUuid ? '继续上传' : '确认上传' }}
                </KunButton>
                <!-- error: 重试 -->
                <KunButton
                  v-else-if="uploader.status.value === 'error'"
                  color="warning"
                  size="sm"
                  @click="confirmUpload"
                >
                  <KunIcon name="lucide:rotate-cw" class="size-4" />
                  重试上传
                </KunButton>
                <!-- Remove / cancel — label changes by state -->
                <KunButton
                  color="danger"
                  :variant="uploadingNow ? 'light' : 'flat'"
                  size="sm"
                  @click="removeFile"
                >
                  {{ uploadingNow ? '取消上传' : uploadedOk ? '重新选择' : '移除' }}
                </KunButton>
              </div>
            </div>

            <!-- Breakpoint hint: the staged file matches an interrupted upload,
                 so submitting continues from where it stopped. -->
            <p
              v-if="resumeUuid && stagedNotUploaded"
              class="text-warning flex items-center gap-1.5 text-xs"
            >
              <KunIcon name="lucide:history" class="size-3.5" />
              检测到未完成的上传，将从断点继续
            </p>

            <div v-if="uploadingNow || uploadedOk">
              <p class="text-default-500 mb-1 text-sm">
                {{ uploadStatusLabel }}
                <span v-if="uploadingNow">{{ uploader.progressPercent.value }}%</span>
              </p>
              <KunProgress
                :value="uploader.progressPercent.value"
                size="sm"
                :color="uploadedOk ? 'success' : 'primary'"
              />
            </div>

            <p
              v-if="uploader.status.value === 'error'"
              class="text-danger text-sm"
            >
              {{ uploader.errorMessage.value }}
            </p>
          </div>
        </section>

        <!-- resource links (user mode) ---------------------- -->
        <section v-if="form.storage === 'user'" class="space-y-2">
          <h3 class="text-lg font-medium">资源链接</h3>
          <p class="text-default-500 text-sm">
            请添加您的资源链接，建议每条单独添加，以便后续维护
          </p>

          <div
            v-for="(link, i) in (userLinks.length === 0 ? [''] : userLinks)"
            :key="i"
            class="flex items-center gap-2"
          >
            <KunChip color="primary" variant="flat" size="sm">
              自定义链接
            </KunChip>
            <div class="flex-1">
              <KunInput
                :model-value="link"
                placeholder="请输入资源链接（http(s):// 开头）"
                @update:model-value="updateUserLink(i, String($event))"
              />
            </div>
            <KunButton
              v-if="i === (userLinks.length === 0 ? 0 : userLinks.length - 1)"
              is-icon-only
              variant="flat"
              color="primary"
              @click="addUserLink"
              aria-label="新增链接"
            >
              <KunIcon name="lucide:plus" class="size-4" />
            </KunButton>
            <KunButton
              v-else
              is-icon-only
              variant="flat"
              color="danger"
              @click="removeUserLink(i)"
              aria-label="删除该链接"
            >
              <KunIcon name="lucide:x" class="size-4" />
            </KunButton>
          </div>
        </section>

        <!-- type / language / platform / size --------------- -->
        <section class="space-y-3">
          <h3 class="text-lg font-medium">资源详情</h3>

          <div>
            <p class="mb-2 text-sm font-medium">
              类型 <span class="text-danger">*</span>
            </p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="t in resourceTypes"
                :key="t.value"
                type="button"
                @click="toggle(form.type, t.value)"
              >
                <KunChip
                  :color="form.type.includes(t.value) ? 'primary' : 'default'"
                  :variant="form.type.includes(t.value) ? 'solid' : 'flat'"
                  size="md"
                >
                  {{ t.label }}
                </KunChip>
              </button>
            </div>
          </div>

          <div>
            <p class="mb-2 text-sm font-medium">
              语言 <span class="text-danger">*</span>
            </p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="l in SUPPORTED_LANGUAGE"
                :key="l"
                type="button"
                @click="toggle(form.language, l)"
              >
                <KunChip
                  :color="form.language.includes(l) ? 'primary' : 'default'"
                  :variant="form.language.includes(l) ? 'solid' : 'flat'"
                  size="md"
                >
                  {{ SUPPORTED_LANGUAGE_MAP[l] }}
                </KunChip>
              </button>
            </div>
          </div>

          <div>
            <p class="mb-2 text-sm font-medium">
              平台 <span class="text-danger">*</span>
            </p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="p in SUPPORTED_PLATFORM"
                :key="p"
                type="button"
                @click="toggle(form.platform, p)"
              >
                <KunChip
                  :color="form.platform.includes(p) ? 'primary' : 'default'"
                  :variant="form.platform.includes(p) ? 'solid' : 'flat'"
                  size="md"
                >
                  {{ SUPPORTED_PLATFORM_MAP[p] }}
                </KunChip>
              </button>
            </div>
          </div>

          <KunInput
            v-model="form.size"
            label="大小 (MB 或 GB)"
            placeholder="请输入资源的大小，例如 1.007MB"
            :disabled="form.storage === 's3'"
          />
        </section>

        <!-- optional metadata ------------------------------- -->
        <section class="space-y-3">
          <KunInput
            v-model="form.name"
            label="资源名称（可选）"
            placeholder="例如：枯れない世界と終わる花 翻译补丁"
          />
          <KunInput
            v-model="form.model_name"
            label="AI 模型名称（可选）"
            placeholder="若为 AI 翻译，建议填写模型名，例如 claude-3-7-sonnet-20250219"
          />
          <KunInput
            v-model="form.code"
            label="提取码"
            placeholder="如资源链接需要密码，请填写"
          />
          <KunInput
            v-model="form.password"
            label="解压码"
            placeholder="如解压需要密码，请填写"
          />
        </section>

        <!-- note (markdown) -------------------------------- -->
        <section class="space-y-2">
          <h3 class="text-lg font-medium">资源备注</h3>
          <div class="text-default-500 text-sm">
            建议详细说明补丁的使用方法、注意事项、原创/授权声明、更新日志等。
          </div>
          <!-- Rich markdown editor (same as comments / galgame intro): supports
               formatting, image upload, and @mention. Uncontrolled — key by the
               resource id so it remounts with the right initial note when the
               edit modal is reused for a different resource. -->
          <KunMilkdownDualEditorProvider
            :key="`note-${props.resource?.id ?? 'new'}`"
            :value-markdown="form.note"
            @set-markdown="(val) => (form.note = val)"
          />
        </section>

        <!-- edit-mode: reason memo for audit trail -->
        <section v-if="isEdit" class="space-y-2">
          <h3 class="text-lg font-medium">修改原因（可选）</h3>
          <p class="text-default-500 text-sm">
            仅当替换文件时会写入审计日志（patch_resource_file_history）；纯改备注/分类则不记录。
          </p>
          <KunInput
            v-model="reason"
            placeholder="例如：修复某段翻译 / 升级到 v1.1"
          />
        </section>
      </form>
    </div>

    <!-- footer -->
    <div class="flex items-center justify-end gap-2 border-t border-default/15 pt-3">
      <KunButton color="danger" variant="light" @click="emit('close')">
        取消
      </KunButton>
      <KunButton
        color="primary"
        :loading="submitting || uploadingNow"
        :disabled="submitting || uploadingNow"
        @click="handleSubmit"
      >
        <KunIcon
          v-if="!submitting && !uploadingNow"
          :name="isEdit ? 'lucide:save' : 'lucide:upload'"
          class="size-4"
        />
        {{
          submitting
            ? isEdit
              ? '保存中...'
              : '发布中...'
            : uploadingNow
              ? '正在上传补丁资源...'
              : isEdit
                ? '保存修改'
                : '发布资源'
        }}
      </KunButton>
    </div>
  </div>
</template>
