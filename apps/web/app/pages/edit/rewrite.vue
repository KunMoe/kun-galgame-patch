<script setup lang="ts">
// Galgame metadata edit form.
//
// Per docs/galgame_wiki/01-galgame.md PUT /galgame/:gid, we send any subset
// of:
//   - name_{en_us,ja_jp,zh_cn,zh_tw}, intro_*
//   - content_limit, age_limit, original_language
//   - aliases (comma-separated string)
//   - banner via multipart `file`
//   - is_minor
//
// All editing is proxied through the backend (PUT /api/v1/galgame/:gid) which
// forwards the user's OAuth access_token; Wiki itself enforces creator/admin
// authorization. Tag / official / engine / series selection still requires a
// search-and-select UI which lives on the Wiki frontend.

useKunSeoMeta({
  title: '编辑 Galgame',
  description: '编辑 Galgame 元数据'
})

const route = useRoute()
const userStore = useUserStore()
const api = useApi()

if (!userStore.user.uid) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
}

const galgameId = computed(() => Number(route.query.id))
const validId = computed(
  () => Number.isFinite(galgameId.value) && galgameId.value > 0
)

const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://galgame.kungal.com'

const { data: detail, pending } = await useAsyncData<PatchDetail | null>(
  () => `edit-rewrite-${galgameId.value}`,
  async () => {
    if (!validId.value) return null
    const res = await api.get<PatchDetail>(`/patch/${galgameId.value}/detail`)
    return res.code === 0 ? res.data : null
  }
)

// Wiki accepts these literal values (see 01-galgame.md). For original_language
// we mirror the four-language UI that lives elsewhere on the site.
const CONTENT_LIMIT_OPTIONS = ['sfw', 'nsfw'] as const
const AGE_LIMIT_OPTIONS = ['all', 'r18'] as const
const ORIGINAL_LANG_OPTIONS = [
  { value: '', label: '保持不变' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'zh-cn', label: '简体中文' },
  { value: 'zh-tw', label: '繁體中文' },
  { value: 'en-us', label: 'English' }
] as const

interface FormState {
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  intro_en_us: string
  intro_ja_jp: string
  intro_zh_cn: string
  intro_zh_tw: string
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  original_language: string
  aliases: string
  is_minor: boolean
}

const form = reactive<FormState>({
  name_en_us: '',
  name_ja_jp: '',
  name_zh_cn: '',
  name_zh_tw: '',
  intro_en_us: '',
  intro_ja_jp: '',
  intro_zh_cn: '',
  intro_zh_tw: '',
  content_limit: 'sfw',
  age_limit: 'all',
  original_language: '',
  aliases: '',
  is_minor: false
})

// The optional banner file is held separately from `form` (FormData doesn't
// round-trip File objects cleanly through reactive state).
const bannerFile = ref<File | null>(null)
const bannerPreview = ref<string | null>(null)
const onBannerChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0] ?? null
  bannerFile.value = f
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
  bannerPreview.value = f ? URL.createObjectURL(f) : null
}
onBeforeUnmount(() => {
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
})

watch(
  detail,
  (d) => {
    if (!d) return
    form.name_en_us = d.name['en-us'] ?? ''
    form.name_ja_jp = d.name['ja-jp'] ?? ''
    form.name_zh_cn = d.name['zh-cn'] ?? ''
    form.name_zh_tw = d.name['zh-tw'] ?? ''
    form.intro_en_us = d.introduction_markdown['en-us'] ?? ''
    form.intro_ja_jp = d.introduction_markdown['ja-jp'] ?? ''
    form.intro_zh_cn = d.introduction_markdown['zh-cn'] ?? ''
    form.intro_zh_tw = d.introduction_markdown['zh-tw'] ?? ''
    form.content_limit = (d.content_limit as 'sfw' | 'nsfw') ?? 'sfw'
    // age_limit / original_language come from the embedded Wiki object on
    // PatchDetail (apps/web/app/shared/types/patch.d.ts); fall back to
    // sensible defaults when Wiki returned nothing.
    form.age_limit = (d.galgame?.age_limit as 'all' | 'r18') ?? 'all'
    form.original_language = d.galgame?.original_language ?? ''
    // aliases isn't returned by /patch/:id/detail (the enricher doesn't
    // surface it); leaving it blank means "don't touch". The user can still
    // edit it as a fresh value.
    form.aliases = ''
    form.is_minor = false
  },
  { immediate: true }
)

// Hand Wiki only the fields that actually changed.
const buildPayload = () => {
  if (!detail.value) return {}
  const d = detail.value
  const payload: Record<string, unknown> = {}

  if (form.name_en_us !== (d.name['en-us'] ?? '')) payload.name_en_us = form.name_en_us
  if (form.name_ja_jp !== (d.name['ja-jp'] ?? '')) payload.name_ja_jp = form.name_ja_jp
  if (form.name_zh_cn !== (d.name['zh-cn'] ?? '')) payload.name_zh_cn = form.name_zh_cn
  if (form.name_zh_tw !== (d.name['zh-tw'] ?? '')) payload.name_zh_tw = form.name_zh_tw

  if (form.intro_en_us !== (d.introduction_markdown['en-us'] ?? ''))
    payload.intro_en_us = form.intro_en_us
  if (form.intro_ja_jp !== (d.introduction_markdown['ja-jp'] ?? ''))
    payload.intro_ja_jp = form.intro_ja_jp
  if (form.intro_zh_cn !== (d.introduction_markdown['zh-cn'] ?? ''))
    payload.intro_zh_cn = form.intro_zh_cn
  if (form.intro_zh_tw !== (d.introduction_markdown['zh-tw'] ?? ''))
    payload.intro_zh_tw = form.intro_zh_tw

  if (form.content_limit !== d.content_limit) payload.content_limit = form.content_limit
  if (form.age_limit !== (d.galgame?.age_limit ?? 'all'))
    payload.age_limit = form.age_limit
  if (form.original_language !== (d.galgame?.original_language ?? '') && form.original_language)
    payload.original_language = form.original_language

  // aliases: only include when non-empty (Wiki replaces the alias set wholesale,
  // so sending "" would wipe out existing aliases).
  if (form.aliases.trim()) payload.aliases = form.aliases.trim()

  payload.is_minor = form.is_minor
  return payload
}

const submitting = ref(false)

const handleSubmit = async () => {
  if (!validId.value) {
    useKunMessage('缺少 Galgame ID', 'error')
    return
  }
  const payload = buildPayload()
  const realKeys = Object.keys(payload).filter((k) => k !== 'is_minor')
  if (realKeys.length === 0 && !bannerFile.value) {
    useKunMessage('没有任何字段被修改', 'warn')
    return
  }
  submitting.value = true
  try {
    let res: { code: number; message: string; data: unknown }
    if (bannerFile.value) {
      // multipart mode: send `data` (JSON string) + `file` (the banner image)
      // so backend can forward to Wiki as a single atomic edit.
      const fd = new FormData()
      fd.append('data', JSON.stringify(payload))
      fd.append('file', bannerFile.value, bannerFile.value.name)
      const config = useRuntimeConfig()
      const base = config.public.apiBase || ''
      const r = await $fetch.raw<typeof res>(`${base}/galgame/${galgameId.value}`, {
        method: 'PUT',
        body: fd,
        credentials: 'include'
      }).catch((e) => e?.response)
      res = (r?._data ?? { code: -1, message: '上传失败', data: null }) as typeof res
    } else {
      res = await api.put(`/galgame/${galgameId.value}`, payload)
    }

    if (res.code === 0) {
      useKunMessage('修改成功', 'success')
      // Invalidate any stale cache of this patch's data on the detail/header
      // pages before navigating back, so users see their edit immediately
      // instead of the previous Wiki snapshot.
      await refreshNuxtData([
        `patch-${galgameId.value}`,
        `patch-detail-${galgameId.value}`
      ])
      await navigateTo(`/patch/${galgameId.value}/introduction`)
    } else {
      useKunMessage(res.message || '修改失败', 'error')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="container mx-auto my-6 max-w-3xl">
    <KunHeader name="编辑 Galgame" description="修改 Galgame 元数据 (经 Galgame Wiki 转发)" />

    <KunLoading v-if="pending" description="加载游戏信息..." class-name="mt-6" />

    <KunNull
      v-else-if="!validId || !detail"
      class="mt-6"
      description="无法找到对应的 Galgame，请确认 URL 上的 id 参数"
    />

    <KunCard v-else class-name="mt-6">
      <div class="space-y-6 p-4">
        <section class="space-y-3">
          <h3 class="text-lg font-semibold">名称 (四语言)</h3>
          <label class="block">
            <span class="text-default-700 text-sm">简体中文</span>
            <KunInput v-model="form.name_zh_cn" placeholder="例如：你和她和她的恋爱" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">繁體中文</span>
            <KunInput v-model="form.name_zh_tw" placeholder="繁體中文名稱" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">日本語</span>
            <KunInput v-model="form.name_ja_jp" placeholder="日本語タイトル" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">English</span>
            <KunInput v-model="form.name_en_us" placeholder="English title" />
          </label>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">简介 (Markdown)</h3>
          <label class="block">
            <span class="text-default-700 text-sm">简体中文</span>
            <textarea
              v-model="form.intro_zh_cn"
              rows="6"
              class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
              placeholder="支持 Markdown 语法"
            />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">繁體中文</span>
            <textarea
              v-model="form.intro_zh_tw"
              rows="6"
              class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
            />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">日本語</span>
            <textarea
              v-model="form.intro_ja_jp"
              rows="6"
              class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
            />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">English</span>
            <textarea
              v-model="form.intro_en_us"
              rows="6"
              class="border-default/20 bg-background w-full rounded-lg border p-3 text-sm"
            />
          </label>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">Banner</h3>
          <p class="text-default-500 text-xs">
            可选，支持 JPEG / PNG / WebP，最大 10MB。上传后会在 Wiki 创建一个新版本快照。
          </p>
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            class="border-default/20 bg-background w-full rounded-lg border p-2 text-sm"
            @change="onBannerChange"
          />
          <div v-if="bannerPreview" class="mt-2">
            <img
              :src="bannerPreview"
              alt="新 banner 预览"
              class="bg-default-100 max-h-48 w-full rounded object-contain"
            />
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">内容分级</h3>
          <div class="flex gap-4">
            <label class="flex items-center gap-2">
              <input
                v-model="form.content_limit"
                type="radio"
                value="sfw"
                class="accent-primary"
              />
              <span>SFW (普通)</span>
            </label>
            <label class="flex items-center gap-2">
              <input
                v-model="form.content_limit"
                type="radio"
                value="nsfw"
                class="accent-primary"
              />
              <span>NSFW (含成人向元素)</span>
            </label>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">年龄分级</h3>
          <div class="flex gap-4">
            <label class="flex items-center gap-2">
              <input
                v-model="form.age_limit"
                type="radio"
                value="all"
                class="accent-primary"
              />
              <span>全年龄</span>
            </label>
            <label class="flex items-center gap-2">
              <input
                v-model="form.age_limit"
                type="radio"
                value="r18"
                class="accent-primary"
              />
              <span>R18</span>
            </label>
          </div>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">原始语言</h3>
          <select
            v-model="form.original_language"
            class="border-default/20 bg-background w-full rounded-lg border p-2 text-sm"
          >
            <option
              v-for="opt in ORIGINAL_LANG_OPTIONS"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </option>
          </select>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">别名</h3>
          <p class="text-default-500 text-xs">
            多个别名用英文逗号分隔；留空则不修改现有别名。提交时会
            <strong>替换</strong>整个别名集合，不要漏填已有的。
          </p>
          <KunInput v-model="form.aliases" placeholder="别名1, 别名2, 别名3" />
        </section>

        <section class="space-y-2">
          <label class="flex items-center gap-2">
            <input v-model="form.is_minor" type="checkbox" class="accent-primary" />
            <span class="text-sm">标记为小修改 (typo / 排版等，可在版本历史中过滤)</span>
          </label>
        </section>

        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-sm">
          <p class="text-default-700">
            如需修改 标签 / 会社 / 引擎 / 系列 等，请前往
            <a
              :href="`${wikiOrigin}/galgame/${galgameId}/edit`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              Galgame Wiki 编辑页
            </a>
            操作（这些字段需要搜索/选择 UI，本站不重复实现）。
          </p>
        </div>

        <div class="flex justify-end gap-2">
          <KunButton variant="bordered" :disabled="submitting" @click="$router.back()">
            取消
          </KunButton>
          <KunButton
            color="primary"
            :loading="submitting"
            :disabled="submitting"
            @click="handleSubmit"
          >
            提交修改
          </KunButton>
        </div>
      </div>
    </KunCard>
  </div>
</template>
