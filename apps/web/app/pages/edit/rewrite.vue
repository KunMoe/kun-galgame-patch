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

if (!userStore.user.id) {
  await navigateTo({ path: '/login', query: { from: route.fullPath } })
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

// content_limit / age_limit use literal radio inputs in the template; only
// original_language needs an options list (mirrors the four-language UI used
// elsewhere on the site). See docs/galgame_wiki/01-galgame.md.
const ORIGINAL_LANG_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: '保持不变' },
  { value: 'ja-jp', label: '日本語' },
  { value: 'zh-cn', label: '简体中文' },
  { value: 'zh-tw', label: '繁體中文' },
  { value: 'en-us', label: 'English' }
]

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

// ─── Taxonomy (handbook §15 / 01-galgame.md PUT presence semantics) ──────────
// tag_ids/official_ids/engine_ids on PUT /galgame/:gid are PRESENCE-based
// full-replace: omit = unchanged; send array = authoritative full set
// (rebuilt server-side; [] = clear all). So the form is PREFILLED with the
// galgame's CURRENT full set (from /patch/:id/detail, which the enricher
// already surfaces as tags[]/officials[]/wiki_engine_ids[]); the user
// add/removes on top, and buildPayload sends a field ONLY when its set
// actually changed — never a partial list (would silently wipe the rest).
const tagIds = ref<number[]>([])
const officialIds = ref<number[]>([])
const engineIds = ref<number[]>([])
const seriesId = ref<number | null>(null)
// Original sets captured from detail, for order-independent change detection.
const origTagIds = ref<number[]>([])
const origOfficialIds = ref<number[]>([])
const origEngineIds = ref<number[]>([])
// Resolved {id,name} so the picker chips show names, not "#id".
const tagInitial = ref<{ id: number; name: string }[]>([])
const officialInitial = ref<{ id: number; name: string }[]>([])

// Order-independent set equality for the id arrays.
const sameIdSet = (a: number[], b: number[]): boolean => {
  if (a.length !== b.length) return false
  const s = new Set(a)
  return b.every((x) => s.has(x))
}

// ─── W2 / PR3b: covers / screenshots (image_service hash) ─────────────────
// presence semantics same as tag_ids — prefill the FULL current set, diff
// before submit. CoversEditor handles pin/remove on existing rows; new banner
// is still uploaded via the bannerFile multipart `file` field below (Wiki
// auto-promotes the upload to covers[sort_order=0]). Screenshots go through
// image_service directly via ScreenshotsEditor.
const covers = ref<GalgameCoverRow[]>([])
const screenshots = ref<GalgameScreenshotRow[]>([])
const origCovers = ref<GalgameCoverRow[]>([])
const origScreenshots = ref<GalgameScreenshotRow[]>([])

// Order-independent equality for the cover/screenshot arrays. We compare by
// the 5-field shape since any field change (sort_order/caption/sexual/...)
// must roundtrip. Reuses the keyed Map for O(n).
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

    // Prefill the CURRENT full association sets (presence semantics: we must
    // round-trip the whole set, never just the deltas).
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

    // W2 / PR3b — prefill covers/screenshots from Wiki object. Deep-clone so
    // editor mutations don't bleed back into the cached detail object.
    const detailCovers = d.galgame?.covers ?? []
    const detailScreens = d.galgame?.screenshots ?? []
    covers.value = detailCovers.map((c) => ({ ...c }))
    screenshots.value = detailScreens.map((s) => ({ ...s }))
    origCovers.value = detailCovers.map((c) => ({ ...c }))
    origScreenshots.value = detailScreens.map((s) => ({ ...s }))
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

  // Associations: send the WHOLE current set, but only when it actually
  // changed vs the prefilled original (avoids spurious revisions; an
  // unchanged field is omitted = "leave unchanged" per presence semantics).
  // When changed we send the full array (incl. [] to clear) — never a delta.
  if (!sameIdSet(tagIds.value, origTagIds.value))
    payload.tag_ids = tagIds.value
  if (!sameIdSet(officialIds.value, origOfficialIds.value))
    payload.official_ids = officialIds.value
  if (!sameIdSet(engineIds.value, origEngineIds.value))
    payload.engine_ids = engineIds.value
  // Covers / screenshots presence semantics (W2): omit = keep, full array =
  // authoritative replace. Diff against the prefilled original; only send when
  // the row set changed (any field change on any row counts).
  if (!sameRowSet(covers.value, origCovers.value))
    payload.covers = covers.value
  if (!sameRowSet(screenshots.value, origScreenshots.value))
    payload.screenshots = screenshots.value
  // series_id is a plain optional scalar (not part of the §presence note);
  // only send when the user explicitly set a positive id.
  if (seriesId.value != null && seriesId.value > 0)
    payload.series_id = seriesId.value

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
            <KunTextarea
              v-model="form.intro_zh_cn"
              :rows="6"
              placeholder="支持 Markdown 语法"
            />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">繁體中文</span>
            <KunTextarea v-model="form.intro_zh_tw" :rows="6" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">日本語</span>
            <KunTextarea v-model="form.intro_ja_jp" :rows="6" />
          </label>
          <label class="block">
            <span class="text-default-700 text-sm">English</span>
            <KunTextarea v-model="form.intro_en_us" :rows="6" />
          </label>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">封面 / 截图</h3>
          <p class="text-default-500 text-xs">
            上传新封面：从下方文件选择器选一张图，提交后该图自动成为当前 Banner
            （Wiki 把它推到 covers 的 sort_order=0，原来的 banner 降级保留）。
            已有封面/截图集合在下面可单独管理（设为 banner / 移除 / 调序）。
            covers/screenshots 按 presence 全量替换，下方编辑器已预填该作当前全集。
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
          <GalgameEditCoversEditor v-model="covers" />
          <GalgameEditScreenshotsEditor v-model="screenshots" />
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">内容分级</h3>
          <KunSelect
            v-model="form.content_limit"
            :options="[
              { value: 'sfw', label: 'SFW (普通)' },
              { value: 'nsfw', label: 'NSFW (含成人向元素)' }
            ]"
            class-name="w-64"
          />
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">年龄分级</h3>
          <KunSelect
            v-model="form.age_limit"
            :options="[
              { value: 'all', label: '全年龄' },
              { value: 'r18', label: 'R18' }
            ]"
            class-name="w-64"
          />
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">原始语言</h3>
          <KunSelect v-model="form.original_language" :options="ORIGINAL_LANG_OPTIONS" />
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
          <KunCheckBox v-model="form.is_minor">
            标记为小修改 (typo / 排版等，可在版本历史中过滤)
          </KunCheckBox>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">标签 / 会社 / 引擎 / 系列</h3>
          <p class="text-default-500 text-xs">
            下方已<strong>预填该作品当前的全部</strong>标签 / 会社 / 引擎，在此基础上
            增删即可——提交时按整体集合替换（presence 语义），只有改动过的项才会提交。
            没有的条目可直接「没有？新建」。需要重命名 / 删除已有分类？前往
            <NuxtLink to="/galgame/taxonomy" class="text-primary hover:underline">
              分类管理
            </NuxtLink>
            。
          </p>
          <GalgameEditTaxonomyPicker
            v-model="tagIds"
            kind="tag"
            :initial="tagInitial"
          />
          <GalgameEditTaxonomyPicker
            v-model="officialIds"
            kind="official"
            :initial="officialInitial"
          />
          <GalgameEditTaxonomyPicker v-model="engineIds" kind="engine" />
          <label class="block">
            <span class="text-default-700 text-sm">系列 ID（可选）</span>
            <KunInput
              :model-value="seriesId ?? ''"
              type="number"
              placeholder="留空表示不修改系列归属"
              @update:model-value="
                seriesId = $event === '' ? null : Number($event)
              "
            />
          </label>
        </section>

        <section class="space-y-3">
          <h3 class="text-lg font-semibold">链接 / 别名 / 贡献者</h3>
          <p class="text-default-500 text-xs">
            这些子资源是即时生效的（每次操作在 Wiki 创建一个新版本快照），
            不随上方「提交修改」按钮一起提交。
          </p>
          <GalgameEditRelations v-if="validId" :gid="galgameId" />
        </section>

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
