<script setup lang="ts">
// Edit one's own status=3 / status=4 draft.
//
// Backed by PATCH /galgame/:gid (which wiki auto-flips status=4 → 3, so a
// declined draft re-enters the review queue on save). See
// docs/galgame_wiki/07-submission.md §PATCH /galgame/:gid.

useKunDisableSeo('编辑草稿')

const route = useRoute()
const api = useApi()

// Unauthed users see the login modal in place (via <AuthRequired> in the
// template), not a redirect to home — see edit/create.vue for the no-auto-OAuth
// reasoning.

const gid = computed(() => Number(route.query.id))
const validId = computed(() => Number.isFinite(gid.value) && gid.value > 0)

// Wiki GET /galgame/:gid returns the full galgame for any status if the
// caller is the submitter (per the api docs). On a non-owner / not-found
// the response either omits the row or returns a 4xx — we surface the
// error to the user.
interface WikiGalgame {
  id: number
  status: number
  vndb_id: string
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  banner: string
  effective_banner_hash: string
  intro_en_us: string
  intro_ja_jp: string
  intro_zh_cn: string
  intro_zh_tw: string
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  original_language: string
  alias?: { id: number; name: string }[]
}
interface DetailResp {
  galgame: WikiGalgame
}

// We hit the wiki directly via its public detail endpoint forwarded by the
// moyu backend's existing pass-through? Actually this site has no detail
// proxy — but the patch detail (/patch/:id/detail) won't surface intro_*
// fields for unpublished entries. Easiest path: hit the wiki origin
// directly (it's CORS-enabled per integration-guide.md §6 for read endpoints).
const config = useRuntimeConfig()
const wikiOrigin = computed(
  () =>
    ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
    'https://wiki.kungal.com'
)

const { data: detail, pending } = await useAsyncData<WikiGalgame | null>(
  () => `draft-${gid.value}`,
  async () => {
    if (!validId.value) return null
    const res = await $fetch<{ code: number; data: DetailResp }>(
      `${wikiOrigin.value}/api/galgame/${gid.value}`,
      { credentials: 'include' }
    ).catch(() => null)
    return res?.code === 0 ? res.data.galgame : null
  }
)

// No vndb_id field: a draft is, by construction, a game VNDB doesn't have
// (VNDB titles are auto-synced by Wiki and reached via the claim path, not
// submit). Same rationale as the submit form in create.vue.
interface FormState {
  name_zh_cn: string
  name_zh_tw: string
  name_ja_jp: string
  name_en_us: string
  intro_zh_cn: string
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  original_language: string
  aliases: string
}
const form = reactive<FormState>({
  name_zh_cn: '',
  name_zh_tw: '',
  name_ja_jp: '',
  name_en_us: '',
  intro_zh_cn: '',
  content_limit: 'sfw',
  age_limit: 'r18',
  original_language: 'ja-jp',
  aliases: ''
})

watch(
  detail,
  (d) => {
    if (!d) return
    form.name_zh_cn = d.name_zh_cn ?? ''
    form.name_zh_tw = d.name_zh_tw ?? ''
    form.name_ja_jp = d.name_ja_jp ?? ''
    form.name_en_us = d.name_en_us ?? ''
    form.intro_zh_cn = d.intro_zh_cn ?? ''
    form.content_limit = (d.content_limit as 'sfw' | 'nsfw') ?? 'sfw'
    form.age_limit = (d.age_limit as 'all' | 'r18') ?? 'r18'
    form.original_language = d.original_language || 'ja-jp'
    // aliases isn't on the draft detail surface (it's an array of {id,name}
    // on GalgameFull) — pre-fill from the comma-joined names so the user
    // can see the current set before editing.
    form.aliases = (d.alias ?? []).map((a) => a.name).join(', ')
  },
  { immediate: true }
)

const bannerFile = ref<File | null>(null)
// The cover cropper (ImageCropper) renders its own preview and hands back a
// cropped + optionally mosaicked webp blob; wrap it as the File the submit
// path uploads.
const onBannerComplete = (blob: Blob) => {
  bannerFile.value = new File([blob], 'cover.webp', { type: 'image/webp' })
}

const submitting = ref(false)
const submitError = ref<string | null>(null)

const handleSubmit = async () => {
  submitError.value = null

  // Wiki PATCH accepts any subset; vndb_id intentionally omitted (drafts are
  // non-VNDB submissions). For aliases we always send so the user can
  // explicitly clear them by emptying the input.
  const payload: Record<string, unknown> = {
    name_zh_cn: form.name_zh_cn.trim(),
    name_zh_tw: form.name_zh_tw.trim(),
    name_ja_jp: form.name_ja_jp.trim(),
    name_en_us: form.name_en_us.trim(),
    intro_zh_cn: form.intro_zh_cn.trim(),
    content_limit: form.content_limit,
    age_limit: form.age_limit,
    original_language: form.original_language,
    aliases: form.aliases.trim()
  }

  submitting.value = true
  try {
    let res: { code: number; message: string; data: unknown }
    if (bannerFile.value) {
      const fd = new FormData()
      fd.append('data', JSON.stringify(payload))
      fd.append('file', bannerFile.value, bannerFile.value.name)
      const cfg = useRuntimeConfig()
      const base = cfg.public.apiBase || ''
      const raw = await $fetch
        .raw<typeof res>(`${base}/galgame/${gid.value}`, {
          method: 'PATCH',
          body: fd,
          credentials: 'include'
        })
        .catch((e) => e?.response)
      res = (raw?._data ?? { code: -1, message: '上传失败', data: null }) as typeof res
    } else {
      res = await api.patch(`/galgame/${gid.value}`, payload)
    }

    if (res.code === 0) {
      useKunMessage('已重新提交审核', 'success')
      await navigateTo('/me/submissions')
      return
    }
    submitError.value = res.message || '保存失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <AuthRequired>
    <!-- Outer wrapper aligns with header (max-w-7xl via default layout); form
       body uses inner narrow column for readability. -->
    <div class="container mx-auto my-4">
    <KunHeader name="编辑草稿" description="修改后将自动重新进入审核队列" />
    <div class="mx-auto max-w-3xl">

    <KunLoading v-if="pending" class-name="mt-6" description="加载草稿..." />

    <KunNull
      v-else-if="!validId || !detail"
      class-name="mt-6"
      description="找不到对应草稿，可能已被审核通过或撤回。"
    />

    <KunCard v-else :bordered="true" class-name="mt-6">
      <div class="space-y-4 p-4">
        <section class="space-y-3">
          <h3 class="font-semibold">名称（四语言）</h3>
          <label class="block">
            <span class="text-default-700 text-sm">简体中文</span>
            <KunInput v-model="form.name_zh_cn" placeholder="简体中文名称" />
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

        <section class="space-y-2">
          <h3 class="font-semibold">简介（简体中文）</h3>
          <KunMilkdownDualEditorProvider
            :value-markdown="form.intro_zh_cn"
            @set-markdown="(val) => (form.intro_zh_cn = val)"
          />
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">Banner（可选，更换图片）</h3>
          <KunCropperImageCropper
            :aspect-ratio="16 / 9"
            hint="点击或拖放图片，更换 Banner"
            description="将按 16:9 裁剪，可选对敏感区域打码后提交"
            @complete="onBannerComplete"
            @remove="bannerFile = null"
          />
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">分级</h3>
          <div class="flex flex-wrap items-center gap-6 text-sm">
            <KunRadioGroup
              v-model="form.content_limit"
              orientation="horizontal"
              :options="[
                { value: 'sfw', label: 'SFW' },
                { value: 'nsfw', label: 'NSFW' }
              ]"
            />
            <span class="text-default-300">|</span>
            <KunRadioGroup
              v-model="form.age_limit"
              orientation="horizontal"
              :options="[
                { value: 'all', label: '全年龄' },
                { value: 'r18', label: 'R18' }
              ]"
            />
          </div>
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">原始语言</h3>
          <KunSelect
            v-model="form.original_language"
            :options="[
              { value: 'ja-jp', label: '日本語' },
              { value: 'zh-cn', label: '简体中文' },
              { value: 'zh-tw', label: '繁體中文' },
              { value: 'en-us', label: 'English' }
            ]"
          />
        </section>

        <section class="space-y-2">
          <h3 class="font-semibold">别名</h3>
          <KunInput v-model="form.aliases" placeholder="别名1, 别名2, 别名3" />
          <p class="text-default-500 text-xs">
            英文逗号分隔。提交时会<strong>替换</strong>整个别名集合。
          </p>
        </section>

        <div
          v-if="submitError"
          class="border-danger/30 bg-danger/10 text-danger rounded-lg border p-3 text-sm"
        >
          {{ submitError }}
        </div>

        <div class="flex justify-end gap-2">
          <KunButton
            variant="bordered"
            :disabled="submitting"
            @click="$router.back()"
          >
            取消
          </KunButton>
          <KunButton
            color="primary"
            :loading="submitting"
            :disabled="submitting"
            @click="handleSubmit"
          >
            保存并重新提交
          </KunButton>
        </div>
      </div>
    </KunCard>
    </div>
    </div>
  </AuthRequired>
</template>
