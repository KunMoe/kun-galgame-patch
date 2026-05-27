<script setup lang="ts">
// Galgame metadata edit form. The page is split into 5 KunTab sections so
// users see one focused chunk at a time instead of an 8-section monolith.
// A sticky bottom action bar surfaces 取消 / 提交修改 from any section
// without scrolling, and section tabs carry a small dot indicator when
// that section has unsaved changes so the user always knows what their
// submit will actually send.
//
// Per docs/galgame_wiki/01-galgame.md PUT /galgame/:gid, we send any
// subset of the metadata fields with presence semantics (omit = keep,
// send = authoritative). Tag / official / engine / series id arrays also
// follow presence semantics — prefilled with the current full set, only
// resent when changed (see buildPayload).

useKunDisableSeo('编辑 Galgame')

const route = useRoute()
const userStore = useUserStore()
const api = useApi()

// Unauthed → bounce to home (see edit/create.vue for the loop reasoning).
if (!userStore.user.id) {
  await navigateTo('/')
}

const galgameId = computed(() => Number(route.query.id))
const validId = computed(
  () => Number.isFinite(galgameId.value) && galgameId.value > 0
)

const { data: detail, pending } = await useAsyncData<PatchDetail | null>(
  () => `edit-rewrite-${galgameId.value}`,
  async () => {
    if (!validId.value) return null
    const res = await api.get<PatchDetail>(`/patch/${galgameId.value}/detail`)
    return res.code === 0 ? res.data : null
  }
)

const ORIGINAL_LANG_OPTIONS = [
  { value: '', label: '保持不变' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'zh-cn', label: '简体中文' },
  { value: 'zh-tw', label: '繁體中文' },
  { value: 'en-us', label: 'English' }
] as const

// Per-language tab definitions for the 4-language name/intro pickers. Using
// inner tabs reclaims the ~600px of vertical space the previous version
// burned on stacked editors that most users only filled 1-2 of.
type LangKey = 'zh-cn' | 'zh-tw' | 'ja-jp' | 'en-us'
const LANG_TABS: { value: LangKey; textValue: string }[] = [
  { value: 'zh-cn', textValue: '简体中文' },
  { value: 'zh-tw', textValue: '繁體中文' },
  { value: 'ja-jp', textValue: '日本語' },
  { value: 'en-us', textValue: 'English' }
]
const activeLang = ref<LangKey>('zh-cn')

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

// Banner file held separately — FormData doesn't round-trip File through
// reactive state cleanly.
const bannerFile = ref<File | null>(null)
const bannerPreview = ref<string | null>(null)
watch(bannerFile, (f) => {
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
  bannerPreview.value = f ? URL.createObjectURL(f) : null
})
onBeforeUnmount(() => {
  if (bannerPreview.value) URL.revokeObjectURL(bannerPreview.value)
})

// ─── Taxonomy ids (presence semantics, see buildPayload) ──────────────
const tagIds = ref<number[]>([])
const officialIds = ref<number[]>([])
const engineIds = ref<number[]>([])
const seriesId = ref<number | null>(null)
const origTagIds = ref<number[]>([])
const origOfficialIds = ref<number[]>([])
const origEngineIds = ref<number[]>([])
const tagInitial = ref<{ id: number; name: string }[]>([])
const officialInitial = ref<{ id: number; name: string }[]>([])

const sameIdSet = (a: number[], b: number[]): boolean => {
  if (a.length !== b.length) return false
  const s = new Set(a)
  return b.every((x) => s.has(x))
}

// ─── covers / screenshots (presence semantics, see buildPayload) ──────
const covers = ref<GalgameCoverRow[]>([])
const screenshots = ref<GalgameScreenshotRow[]>([])
const origCovers = ref<GalgameCoverRow[]>([])
const origScreenshots = ref<GalgameScreenshotRow[]>([])

const rowKey = (r: GalgameCoverRow | GalgameScreenshotRow): string => {
  const c = (r as GalgameScreenshotRow).caption ?? ''
  return `${r.image_hash}|${r.sort_order}|${r.sexual}|${r.violence}|${r.source}|${r.source_key}|${c}`
}
const sameRowSet = (
  a: (GalgameCoverRow | GalgameScreenshotRow)[],
  b: (GalgameCoverRow | GalgameScreenshotRow)[]
): boolean => {
  if (a.length !== b.length) return false
  const ka = new Set(a.map(rowKey))
  return b.every((r) => ka.has(rowKey(r)))
}

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
    form.age_limit = (d.galgame?.age_limit as 'all' | 'r18') ?? 'all'
    form.original_language = d.galgame?.original_language ?? ''
    form.aliases = ''
    form.is_minor = false

    const tIds = (d.tags ?? []).map((t) => t.id)
    const oIds = (d.officials ?? []).map((o) => o.id)
    const eIds = d.wiki_engine_ids ?? []
    origTagIds.value = tIds
    origOfficialIds.value = oIds
    origEngineIds.value = eIds
    tagIds.value = [...tIds]
    officialIds.value = [...oIds]
    engineIds.value = [...eIds]
    tagInitial.value = (d.tags ?? []).map((t) => ({ id: t.id, name: t.name }))
    officialInitial.value = (d.officials ?? []).map((o) => ({
      id: o.id,
      name: o.name
    }))

    const detailCovers = d.galgame?.covers ?? []
    const detailScreens = d.galgame?.screenshots ?? []
    covers.value = detailCovers.map((c) => ({ ...c }))
    screenshots.value = detailScreens.map((s) => ({ ...s }))
    origCovers.value = detailCovers.map((c) => ({ ...c }))
    origScreenshots.value = detailScreens.map((s) => ({ ...s }))
  },
  { immediate: true }
)

// ─── Per-section "modified" indicators ────────────────────────────────
// Each computed reports whether the section has at least one unsaved
// change vs the prefilled originals. Used to render the dot on the
// section tab + to count visible changes in the sticky bar.
const basicChanged = computed(() => {
  if (!detail.value) return false
  const d = detail.value
  return (
    form.content_limit !== d.content_limit ||
    form.age_limit !== (d.galgame?.age_limit ?? 'all') ||
    (form.original_language !== (d.galgame?.original_language ?? '') &&
      !!form.original_language) ||
    !!form.aliases.trim()
  )
})
const textChanged = computed(() => {
  if (!detail.value) return false
  const d = detail.value
  return (
    form.name_en_us !== (d.name['en-us'] ?? '') ||
    form.name_ja_jp !== (d.name['ja-jp'] ?? '') ||
    form.name_zh_cn !== (d.name['zh-cn'] ?? '') ||
    form.name_zh_tw !== (d.name['zh-tw'] ?? '') ||
    form.intro_en_us !== (d.introduction_markdown['en-us'] ?? '') ||
    form.intro_ja_jp !== (d.introduction_markdown['ja-jp'] ?? '') ||
    form.intro_zh_cn !== (d.introduction_markdown['zh-cn'] ?? '') ||
    form.intro_zh_tw !== (d.introduction_markdown['zh-tw'] ?? '')
  )
})
const mediaChanged = computed(
  () =>
    !!bannerFile.value ||
    !sameRowSet(covers.value, origCovers.value) ||
    !sameRowSet(screenshots.value, origScreenshots.value)
)
const taxonomyChanged = computed(
  () =>
    !sameIdSet(tagIds.value, origTagIds.value) ||
    !sameIdSet(officialIds.value, origOfficialIds.value) ||
    !sameIdSet(engineIds.value, origEngineIds.value) ||
    (seriesId.value != null && seriesId.value > 0)
)
const totalChangedSections = computed(
  () =>
    Number(basicChanged.value) +
    Number(textChanged.value) +
    Number(mediaChanged.value) +
    Number(taxonomyChanged.value)
)

// ─── Tabs ─────────────────────────────────────────────────────────────
// Sections are ordered roughly by edit frequency: text fixes are the most
// common edit, then basic flags, then occasional taxonomy / media work,
// and finally the instant-effect (separate from this form's "submit"
// flow) links/aliases at the end. Each tab carries a small ● when that
// section has unsaved changes.
type SectionKey = 'text' | 'basic' | 'taxonomy' | 'media' | 'instant'
const activeSection = ref<SectionKey>('text')
const sectionItems = computed(() => [
  {
    value: 'text',
    textValue: textChanged.value ? '多语言文本 ●' : '多语言文本'
  },
  { value: 'basic', textValue: basicChanged.value ? '基本信息 ●' : '基本信息' },
  {
    value: 'taxonomy',
    textValue: taxonomyChanged.value ? '分类 ●' : '分类'
  },
  { value: 'media', textValue: mediaChanged.value ? '媒体 ●' : '媒体' },
  { value: 'instant', textValue: '即时管理' }
])

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

  if (form.content_limit !== d.content_limit)
    payload.content_limit = form.content_limit
  if (form.age_limit !== (d.galgame?.age_limit ?? 'all'))
    payload.age_limit = form.age_limit
  if (
    form.original_language !== (d.galgame?.original_language ?? '') &&
    form.original_language
  )
    payload.original_language = form.original_language

  if (form.aliases.trim()) payload.aliases = form.aliases.trim()

  if (!sameIdSet(tagIds.value, origTagIds.value)) payload.tag_ids = tagIds.value
  if (!sameIdSet(officialIds.value, origOfficialIds.value))
    payload.official_ids = officialIds.value
  if (!sameIdSet(engineIds.value, origEngineIds.value))
    payload.engine_ids = engineIds.value
  if (!sameRowSet(covers.value, origCovers.value)) payload.covers = covers.value
  if (!sameRowSet(screenshots.value, origScreenshots.value))
    payload.screenshots = screenshots.value
  if (seriesId.value != null && seriesId.value > 0)
    payload.series_id = seriesId.value

  payload.is_minor = form.is_minor
  return payload
}

const submitting = ref(false)
const canSubmit = computed(
  () =>
    !submitting.value &&
    (totalChangedSections.value > 0 || !!bannerFile.value)
)

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
      const fd = new FormData()
      fd.append('data', JSON.stringify(payload))
      fd.append('file', bannerFile.value, bannerFile.value.name)
      const config = useRuntimeConfig()
      const base = config.public.apiBase || ''
      const r = await $fetch
        .raw<typeof res>(`${base}/galgame/${galgameId.value}`, {
          method: 'PUT',
          body: fd,
          credentials: 'include'
        })
        .catch((e) => e?.response)
      res = (r?._data ?? { code: -1, message: '上传失败', data: null }) as typeof res
    } else {
      res = await api.put(`/galgame/${galgameId.value}`, payload)
    }

    if (res.code === 0) {
      useKunMessage('修改成功', 'success')
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
  <div class="container mx-auto my-6">
    <KunHeader
      name="编辑 Galgame"
      description="修改 Galgame 元数据 (经 Galgame Wiki 转发)"
    />

    <KunLoading
      v-if="pending"
      description="加载游戏信息..."
      class-name="mt-6"
    />

    <KunNull
      v-else-if="!validId || !detail"
      class="mt-6"
      description="无法找到对应的 Galgame，请确认 URL 上的 id 参数"
    />

    <div v-else class="mx-auto mt-6 max-w-3xl pb-24">
      <!-- Section nav: KunTab in pill variant so each section reads as a
           top-level chunk; the ● suffix on textValue is the unsaved-change
           indicator (avoids needing custom slot for icons). -->
      <KunTab
        v-model="activeSection"
        :items="sectionItems"
        variant="pills"
        color="primary"
        size="md"
        scrollable
        class="mb-4"
      />

      <KunCard>
        <!-- ─── 多语言文本：name + intro grouped by language ─────────── -->
        <div v-show="activeSection === 'text'" class="space-y-4 p-4">
          <p class="text-default-500 text-xs">
            选择一种语言编辑该语言的标题和简介。大多数用户只需要填写 1-2 种
            语言；未填写的语言保持当前值不变。
          </p>
          <KunTab
            v-model="activeLang"
            :items="LANG_TABS"
            variant="underlined"
            color="primary"
            size="sm"
          />
          <div v-for="lang in LANG_TABS" :key="lang.value">
            <div v-show="activeLang === lang.value" class="space-y-3">
              <label class="block">
                <span class="text-default-700 text-sm">标题</span>
                <KunInput
                  v-if="lang.value === 'zh-cn'"
                  v-model="form.name_zh_cn"
                  placeholder="例如：你和她和她的恋爱"
                />
                <KunInput
                  v-else-if="lang.value === 'zh-tw'"
                  v-model="form.name_zh_tw"
                  placeholder="繁體中文名稱"
                />
                <KunInput
                  v-else-if="lang.value === 'ja-jp'"
                  v-model="form.name_ja_jp"
                  placeholder="日本語タイトル"
                />
                <KunInput
                  v-else
                  v-model="form.name_en_us"
                  placeholder="English title"
                />
              </label>
              <label class="block">
                <span class="text-default-700 text-sm">简介 (Markdown)</span>
                <KunTextarea
                  v-if="lang.value === 'zh-cn'"
                  v-model="form.intro_zh_cn"
                  :rows="10"
                  placeholder="支持 Markdown 语法"
                />
                <KunTextarea
                  v-else-if="lang.value === 'zh-tw'"
                  v-model="form.intro_zh_tw"
                  :rows="10"
                />
                <KunTextarea
                  v-else-if="lang.value === 'ja-jp'"
                  v-model="form.intro_ja_jp"
                  :rows="10"
                />
                <KunTextarea v-else v-model="form.intro_en_us" :rows="10" />
              </label>
            </div>
          </div>
        </div>

        <!-- ─── 基本信息：rating + language + aliases + is_minor ────── -->
        <div v-show="activeSection === 'basic'" class="space-y-5 p-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <h3 class="text-sm font-semibold">内容分级</h3>
              <KunRadioGroup
                v-model="form.content_limit"
                orientation="horizontal"
                :options="[
                  { value: 'sfw', label: 'SFW (普通)' },
                  { value: 'nsfw', label: 'NSFW (含成人向元素)' }
                ]"
              />
            </div>
            <div class="space-y-2">
              <h3 class="text-sm font-semibold">年龄分级</h3>
              <KunRadioGroup
                v-model="form.age_limit"
                orientation="horizontal"
                :options="[
                  { value: 'all', label: '全年龄' },
                  { value: 'r18', label: 'R18' }
                ]"
              />
            </div>
          </div>

          <div class="space-y-2">
            <h3 class="text-sm font-semibold">原始语言</h3>
            <KunSelect
              v-model="form.original_language"
              :options="ORIGINAL_LANG_OPTIONS"
            />
          </div>

          <div class="space-y-2">
            <h3 class="text-sm font-semibold">别名</h3>
            <p class="text-default-500 text-xs">
              多个别名用英文逗号分隔；留空则不修改现有别名。提交时会
              <strong>替换</strong>整个别名集合，不要漏填已有的。
            </p>
            <KunInput v-model="form.aliases" placeholder="别名1, 别名2, 别名3" />
          </div>

          <div>
            <KunCheckBox v-model="form.is_minor">
              标记为小修改 (typo / 排版等，可在版本历史中过滤)
            </KunCheckBox>
          </div>
        </div>

        <!-- ─── 分类：tag/official/engine/series ────────────────────── -->
        <div v-show="activeSection === 'taxonomy'" class="space-y-4 p-4">
          <p class="text-default-500 text-xs">
            下方已<strong>预填该作品当前的全部</strong>标签 / 会社 / 引擎，
            在此基础上增删即可——提交时按整体集合替换（presence 语义），
            只有改动过的项才会提交。没有的条目可直接「没有？新建」。
            需要重命名 / 删除已有分类？前往
            <NuxtLink to="/galgame/taxonomy" class="text-primary hover:underline">
              分类管理
            </NuxtLink>。
          </p>
          <div class="space-y-3">
            <div class="space-y-1">
              <h3 class="text-sm font-semibold">标签</h3>
              <GalgameEditTaxonomyPicker
                v-model="tagIds"
                kind="tag"
                :initial="tagInitial"
              />
            </div>
            <div class="space-y-1">
              <h3 class="text-sm font-semibold">开发商 / 发行商</h3>
              <GalgameEditTaxonomyPicker
                v-model="officialIds"
                kind="official"
                :initial="officialInitial"
              />
            </div>
            <div class="space-y-1">
              <h3 class="text-sm font-semibold">引擎</h3>
              <GalgameEditTaxonomyPicker v-model="engineIds" kind="engine" />
            </div>
            <label class="block space-y-1">
              <span class="text-sm font-semibold">系列 ID（可选）</span>
              <KunInput
                :model-value="seriesId ?? ''"
                type="number"
                placeholder="留空表示不修改系列归属"
                @update:model-value="
                  seriesId = $event === '' ? null : Number($event)
                "
              />
            </label>
          </div>
        </div>

        <!-- ─── 媒体：banner upload + covers / screenshots ──────────── -->
        <div v-show="activeSection === 'media'" class="space-y-5 p-4">
          <div class="space-y-3">
            <h3 class="text-sm font-semibold">新封面（可选）</h3>
            <p class="text-default-500 text-xs">
              上传一张图，提交后自动成为当前 Banner
              （Wiki 把它推到 covers 的 sort_order=0，原来的 banner 降级保留）。
            </p>
            <KunFileInput
              v-model="bannerFile"
              accept="image/jpeg,image/png,image/webp"
              :max-size="10 * 1024 * 1024"
              hint="JPEG / PNG / WebP，最大 10 MB"
              trigger-text="选择新封面"
              trigger-icon="lucide:image-plus"
              @error-pick="useKunMessage($event, 'error')"
            />
            <div v-if="bannerPreview">
              <KunImage
                :src="bannerPreview"
                alt="新 banner 预览"
                object-fit="contain"
                class-name="bg-default-100 block max-h-48 w-full rounded"
              />
            </div>
          </div>

          <div class="space-y-3">
            <h3 class="text-sm font-semibold">封面集合</h3>
            <p class="text-default-500 text-xs">
              `sort_order=0` 钉住为当前 banner；其余按顺序展示。按 presence
              语义全量替换，已预填该作当前全集。
            </p>
            <GalgameEditCoversEditor v-model="covers" />
          </div>

          <div class="space-y-3">
            <h3 class="text-sm font-semibold">截图集合</h3>
            <GalgameEditScreenshotsEditor v-model="screenshots" />
          </div>
        </div>

        <!-- ─── 即时管理：links + aliases (writes effect immediately) ── -->
        <div v-show="activeSection === 'instant'" class="space-y-4 p-4">
          <div
            class="border-warning/30 bg-warning/10 text-warning-700 rounded-lg border p-3 text-xs"
          >
            <p class="font-medium">⚡ 注意</p>
            <p class="mt-1">
              此处的链接 / 别名是<strong>即时生效</strong>的（每次操作
              在 Wiki 创建一个新版本快照），不随上方「提交修改」按钮一起提交。
            </p>
          </div>
          <GalgameEditRelations v-if="validId" :gid="galgameId" />
        </div>
      </KunCard>
    </div>

    <!-- ─── Sticky action bar ──────────────────────────────────────────
         Fixed to the bottom of the viewport so submit/cancel are always
         one tap away no matter how long the active section gets. Backdrop
         blur + bg/80 echo the top bar so the whole UI feels one piece. -->
    <div
      v-if="!pending && validId && detail"
      class="bg-background/80 border-default/20 fixed inset-x-0 bottom-0 z-20 border-t backdrop-blur-md"
      :style="{ paddingBottom: 'env(safe-area-inset-bottom)' }"
    >
      <div
        class="mx-auto flex max-w-3xl items-center justify-between gap-3 px-4 py-3"
      >
        <div class="text-default-500 min-w-0 truncate text-xs">
          <template v-if="totalChangedSections > 0 || bannerFile">
            <span class="text-primary font-medium">
              ● {{ totalChangedSections + (bannerFile ? 1 : 0) }} 项待提交
            </span>
            <span class="ml-1">
              ({{
                [
                  basicChanged && '基本',
                  textChanged && '文本',
                  taxonomyChanged && '分类',
                  (mediaChanged || bannerFile) && '媒体'
                ]
                  .filter(Boolean)
                  .join(' / ')
              }})
            </span>
          </template>
          <template v-else> 没有任何修改 </template>
        </div>
        <div class="flex shrink-0 gap-2">
          <KunButton
            variant="bordered"
            size="sm"
            :disabled="submitting"
            @click="$router.back()"
          >
            取消
          </KunButton>
          <KunButton
            color="primary"
            size="sm"
            :loading="submitting"
            :disabled="!canSubmit"
            @click="handleSubmit"
          >
            提交修改
          </KunButton>
        </div>
      </div>
    </div>
  </div>
</template>
