<script setup lang="ts">
// Patch-resource publish modal.
//
// Ported from apps/next-web/components/patch/resource/publish/PublishResource.tsx
// to Nuxt + KunUI. The form-state shape mirrors the backend DTO
// (apps/api/internal/patch/dto/dto.go PatchResourceCreateRequest) exactly so
// CreateResource consumes our POST body without translation. Upload itself is
// the D10 presigned-URL flow encapsulated in useResourceUpload.
//
// Two storage modes (mutually exclusive form shapes):
//   storage='s3'   : file required; on upload completion we stamp s3_key and
//                    derive `content` = "s3://" + s3_key (presentation marker;
//                    GET /resource/:id/link replaces it with a presigned GET
//                    when the user clicks 获取资源链接).
//   storage='user' : no file; `content` is the user's own list of links,
//                    comma-separated. ResourceLinksInput handles the UX.
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
}
const props = defineProps<Props>()

const emit = defineEmits<{
  // Parent owns the modal v-model; emit close so it can flip the ref.
  close: []
  // Resource row from the server response (composeResource shape). Parent
  // typically pushes this into the list it already has so the new row appears
  // without a full refetch.
  success: [resource: PatchResource]
}>()

const api = useApi()
const userStore = useUserStore()
const uploader = useResourceUpload()

// ─── Form state ───────────────────────────────────────────
//
// Shape is the wire-format we'll POST — no separate "view model". Keeps the
// component free of mapping code and means the validation rules below double
// as documentation for what the server requires.
const form = reactive({
  storage: 's3' as 's3' | 'user',
  name: '',
  model_name: '',
  s3_key: '',
  content: '',
  size: '',
  code: '',
  password: '',
  note: '',
  type: [] as string[],
  language: [] as string[],
  platform: [] as string[]
})

// Multi-select via toggle-chip pattern (KunUI has no MultiSelect yet; chips
// give the same UX as next-web's HeroUI Select selectionMode="multiple"
// without the dropdown overhead).
const toggle = (list: string[], v: string) => {
  const i = list.indexOf(v)
  if (i >= 0) list.splice(i, 1)
  else list.push(v)
}

const storageOptions = storageTypes.map((t) => ({
  value: t.value,
  label: t.label
}))

// Reset the upload-related fields when switching storage mode so e.g.
// flipping s3 → user → s3 doesn't carry over a stale s3_key from the
// first upload attempt.
watch(
  () => form.storage,
  () => {
    form.s3_key = ''
    form.content = form.storage === 'user' ? '' : ''
    form.size = ''
    uploader.reset()
    pickedFile.value = null
  }
)

// ─── File picker + upload ──────────────────────────────────
const pickedFile = ref<File | null>(null)
const uploadError = ref('')

const handleFilePicked = async (files: File[]) => {
  const f = files[0]
  if (!f) return
  uploadError.value = ''
  pickedFile.value = f
  try {
    const result = await uploader.upload(f, props.patchId)
    form.s3_key = result.s3Key
    // Marker so submit serializes a non-empty content; backend ignores the
    // value for storage='s3' (it serves a presigned GET via /link instead),
    // but PatchResourceCreateRequest validation still requires content.min=1.
    form.content = `s3://${result.s3Key}`
    form.size = `${(result.size / (1024 * 1024)).toFixed(3)} MB`
  } catch (e) {
    uploadError.value = e instanceof Error ? e.message : '上传失败'
  }
}

const removeFile = () => {
  uploader.cancel()
  pickedFile.value = null
  form.s3_key = ''
  form.content = ''
  form.size = ''
  uploader.reset()
}

const uploadingNow = computed(
  () =>
    uploader.status.value === 'preparing' ||
    uploader.status.value === 'uploading' ||
    uploader.status.value === 'completing'
)

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
onMounted(() => {
  // Initialize at least one empty row so the user sees the input on first
  // switch to 'user' mode; cheaper than rendering an Add-link CTA empty state.
  if (form.storage === 'user' && form.content === '') form.content = ''
})

// ─── Validation + submit ───────────────────────────────────
const submitting = ref(false)
// Mirror the DTO's required predicates (apps/api/.../dto.go) so submit guards
// surface errors next to the field instead of relying on a 400 round-trip.
const validate = (): string | null => {
  if (form.type.length === 0) return '请选择资源类型'
  if (form.language.length === 0) return '请选择语言'
  if (form.platform.length === 0) return '请选择平台'
  if (!form.size.trim()) return '请填写资源大小'
  if (form.storage === 's3') {
    if (!form.s3_key) return '请上传补丁文件'
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
    const res = await api.post<PatchResource>(
      `/patch/${props.patchId}/resource`,
      {
        galgame_id: props.patchId,
        storage: form.storage,
        name: form.name,
        model_name: form.model_name,
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
    )
    if (res.code === 0 && res.data) {
      useKunMessage('资源发布成功', 'success')
      emit('success', res.data)
      emit('close')
    } else {
      useKunMessage(res.message || '发布失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}

// ─── Display helpers ───────────────────────────────────────
const fileSizeLabel = computed(() => {
  if (!pickedFile.value) return ''
  return `${(pickedFile.value.size / (1024 * 1024)).toFixed(3)} MB`
})

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

// Daily upload quota — read straight from the user store (set by /auth/me).
// 5GB for moderator/admin, 100MB otherwise (mirrors backend dailyLimit()).
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
</script>

<template>
  <div class="flex max-h-[80vh] w-full max-w-2xl flex-col gap-4">
    <!-- header -->
    <div class="space-y-2">
      <h2 class="text-xl font-bold">创建补丁资源</h2>
      <div class="text-default-500 space-y-1 text-sm">
        <div class="flex flex-wrap gap-x-4">
          <NuxtLink
            to="/about/notice/patch-tutorial"
            class="text-primary hover:underline"
          >
            鲲 Galgame 补丁资源系统介绍
          </NuxtLink>
          <NuxtLink
            to="/about/notice/paradigm"
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
          <h3 class="text-lg font-medium">上传资源</h3>
          <p class="text-default-500 text-sm">
            我们支持 {{ ALLOWED_EXTENSIONS.join(' ') }} 压缩格式，单文件最大 1
            GB。文件名中的特殊字符会被自动去除，仅保留下划线（_）连字符（-）和后缀。
          </p>

          <div v-if="!pickedFile" class="space-y-2">
            <KunFileInput
              :accept="ALLOWED_EXTENSIONS.join(',')"
              :max-size="1024 * 1024 * 1024"
              trigger-text="选择补丁文件"
              trigger-icon="lucide:file-up"
              :error="uploadError"
              @change="handleFilePicked"
              @error-pick="(msg) => (uploadError = msg)"
            />
          </div>

          <div
            v-else
            class="border-default/20 bg-default-50 space-y-3 rounded-lg border p-4"
          >
            <div class="flex items-center justify-between gap-3">
              <div class="flex min-w-0 items-center gap-2">
                <KunIcon
                  name="lucide:file"
                  class="text-primary size-6 shrink-0"
                />
                <div class="min-w-0">
                  <p class="truncate font-medium">{{ pickedFile.name }}</p>
                  <p class="text-default-500 text-sm">{{ fileSizeLabel }}</p>
                </div>
              </div>
              <KunButton
                v-if="uploadingNow"
                color="danger"
                variant="light"
                size="sm"
                @click="removeFile"
              >
                取消
              </KunButton>
              <KunButton
                v-else
                color="danger"
                variant="flat"
                size="sm"
                @click="removeFile"
              >
                移除
              </KunButton>
            </div>

            <div v-if="uploadingNow || uploader.status.value === 'done'">
              <p class="text-default-500 mb-1 text-sm">
                {{ uploadStatusLabel }}
                <span v-if="uploadingNow">{{ uploader.progressPercent.value }}%</span>
              </p>
              <KunProgress
                :value="uploader.progressPercent.value"
                size="sm"
                :color="uploader.status.value === 'done' ? 'success' : 'primary'"
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

        <!-- resource links (user mode + s3 stub view) --------- -->
        <section
          v-if="form.storage === 'user' || form.content"
          class="space-y-2"
        >
          <h3 class="text-lg font-medium">资源链接</h3>
          <p class="text-default-500 text-sm">
            {{
              form.storage === 'user'
                ? '请添加您的资源链接，建议每条单独添加，以便后续维护'
                : '已为您自动创建对象存储下载链接 ✓'
            }}
          </p>

          <template v-if="form.storage === 'user'">
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
                  @update:model-value="
                    updateUserLink(i, String($event))
                  "
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
          </template>

          <template v-else>
            <div class="flex items-center gap-2">
              <KunChip color="primary" variant="flat" size="sm">
                对象存储下载
              </KunChip>
              <KunInput
                :model-value="form.content"
                disabled
                placeholder="（上传成功后自动生成）"
              />
            </div>
          </template>
        </section>

        <!-- type / language / platform / size --------------- -->
        <section class="space-y-3">
          <h3 class="text-lg font-medium">资源详情</h3>

          <div>
            <p class="mb-2 text-sm font-medium">类型 <span class="text-danger">*</span></p>
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
            <p class="mb-2 text-sm font-medium">语言 <span class="text-danger">*</span></p>
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
            <p class="mb-2 text-sm font-medium">平台 <span class="text-danger">*</span></p>
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
          <KunTextarea
            v-model="form.note"
            placeholder="例如：注意事项 / 使用说明 / 原创或授权说明 / 更新日志"
            :rows="6"
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
        <KunIcon v-if="!submitting && !uploadingNow" name="lucide:upload" class="size-4" />
        {{
          submitting
            ? '发布中...'
            : uploadingNow
              ? '正在上传补丁资源...'
              : '发布资源'
        }}
      </KunButton>
    </div>
  </div>
</template>
