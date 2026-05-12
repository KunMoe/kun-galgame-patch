<script setup lang="ts">
// Galgame metadata edit form.
//
// Per docs/galgame_wiki/01-galgame.md, PUT /galgame/:gid accepts any subset
// of name / intro / content_limit / age_limit / aliases / etc. Editing is
// proxied through our backend (PUT /api/v1/galgame/:gid) which forwards the
// user's OAuth access_token so the Wiki Service can apply creator/admin
// authorization itself.
//
// Tag / official / engine selection is intentionally NOT in this form —
// those need a search-and-select UI which is more naturally done on the
// Wiki frontend. A link to the Wiki edit page is shown for those cases.

useKunSeoMeta({
  title: '编辑 Galgame',
  description: '编辑 Galgame 元数据'
})

const route = useRoute()
const userStore = useUserStore()
const api = useApi()

// Page-level auth gate. Cookie-backed Pinia gives us userStore.user.uid
// during SSR, so anonymous visits get a 302 from the server -- no flash of
// the form before the client-side redirect.
if (!userStore.user.uid) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
}

const galgameId = computed(() => Number(route.query.id))
const validId = computed(() => Number.isFinite(galgameId.value) && galgameId.value > 0)

const config = useRuntimeConfig()
const wikiOrigin =
  ((config.public as { wikiOrigin?: string }).wikiOrigin as string) ??
  'https://galgame.kungal.com'

// Initial values come from /patch/:id/detail (already proxies Wiki under the
// hood through the enricher).
const { data: detail, pending } = await useAsyncData<PatchDetail | null>(
  () => `edit-rewrite-${galgameId.value}`,
  async () => {
    if (!validId.value) return null
    const res = await api.get<PatchDetail>(`/patch/${galgameId.value}/detail`)
    return res.code === 0 ? res.data : null
  }
)

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
  is_minor: false
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
  },
  { immediate: true }
)

// We hand Wiki only the fields that actually changed (omit anything equal to
// the original). This minimizes diff noise in the revision history and
// avoids accidentally clobbering a parallel edit.
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
  // is_minor alone counts as no change; require at least one real diff.
  const keys = Object.keys(payload).filter((k) => k !== 'is_minor')
  if (keys.length === 0) {
    useKunMessage('没有任何字段被修改', 'warn')
    return
  }
  submitting.value = true
  try {
    const res = await api.put(`/galgame/${galgameId.value}`, payload)
    if (res.code === 0) {
      useKunMessage('修改成功', 'success')
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
              <span>NSFW (R18 / 成人向)</span>
            </label>
          </div>
        </section>

        <section class="space-y-2">
          <label class="flex items-center gap-2">
            <input v-model="form.is_minor" type="checkbox" class="accent-primary" />
            <span class="text-sm">标记为小修改 (typo / 排版等，可在版本历史中过滤)</span>
          </label>
        </section>

        <div class="border-default/20 bg-default-50 rounded-lg border p-3 text-sm">
          <p class="text-default-700">
            如需修改 banner / 标签 / 会社 / 引擎 / 别名 / 系列等，请前往
            <a
              :href="`${wikiOrigin}/galgame/${galgameId}/edit`"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary hover:underline"
            >
              Galgame Wiki 编辑页
            </a>
            操作。
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
