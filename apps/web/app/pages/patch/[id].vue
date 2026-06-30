<script setup lang="ts">
import {
  GALGAME_AGE_LIMIT_DETAIL,
  GALGAME_AGE_LIMIT_MAP
} from '~/constants/galgame'

const route = useRoute()
const api = useApi()
const userStore = useUserStore()
const settingStore = useSettingStore()

const galgameId = computed(() => Number(route.params.id))

// "查看所有封面" modal (lazy-fetches the covers on open — see GalgameCovers).
const coversOpen = ref(false)

const { data: patch } = await useAsyncData<PatchHeader | null>(
  () => `patch-${galgameId.value}`,
  async () => {
    const res = await api.get<PatchHeader>(`/patch/${galgameId.value}`)
    return res.code === 0 ? res.data : null
  }
)

// Creator-chip subtitle: how many 补丁资源 the displayed user has uploaded on
// moyu (resource_count). The chip shows the wiki entry creator (fallback: the
// patch publisher), so fetch that user's moyu profile once for the count.
const chipUserId = computed(
  () => patch.value?.creator?.id ?? patch.value?.user?.id ?? 0
)
const { data: chipUserInfo } = await useAsyncData<UserInfo | null>(
  () => `patch-chip-user-${chipUserId.value}`,
  async () => {
    if (!chipUserId.value) return null
    const res = await api.get<UserInfo>(`/user/${chipUserId.value}`)
    return res.code === 0 ? res.data : null
  },
  { watch: [chipUserId] }
)
const creatorDescription = computed(
  () => `已发布 ${chipUserInfo.value?.resource_count ?? 0} 个游戏补丁`
)

// Distinguish "real 404" from "NSFW gate" when patch is null.
//
// The backend returns 404 for both cases (intentional: a distinguishing
// error code would itself be NSFW signal for crawlers). On the frontend we
// can still distinguish reliably because *useApi* already applied the
// caller's effective content_limit:
//   - logged-in user        → useApi sent 'all', a 404 means truly missing
//   - global NSFW mode on   → useApi sent 'all', same
//   - this id already acked → useApi sent 'all', same
// So the only branch where a NSFW patch can produce a null is:
//   anonymous + sfw mode + not yet acked.
// That branch is exactly the "show NSFW confirm" UI path. Anything else
// falls through to KunNull's generic "patch not found".
const shouldShowNsfwConfirm = computed(() => {
  if (patch.value) return false
  if (userStore.user.id > 0) return false
  if (settingStore.data.kunNsfwEnable !== 'sfw') return false
  if (settingStore.isNsfwAcked(galgameId.value)) return false
  return true
})

const confirmNsfw = () => {
  settingStore.ackNsfw(galgameId.value)
  // Hard reload so the SSR pass re-runs useApi with the new ack state
  // baked into the URL — purely client-side refetch would also work, but
  // a reload guarantees every child route (resource / comment tab data
  // pre-fetched via useAsyncData) gets the new gate value too.
  if (import.meta.client) location.reload()
}

const displayName = computed(() =>
  patch.value ? getPreferredLanguageText(patch.value.name) : ''
)

// "本站尚未收录" state. moyu no longer materializes a stub row on view, so the
// backend returns `is_on_forum: false` on a wiki-only card when there is no real
// local patch row. We render a read-only Galgame info page (wiki metadata) + a
// 发布补丁 CTA, and hide the patch-only surfaces (stats / 资源 / 评论 / 收藏).
//
// Keyed on is_on_forum (row exists), NOT resource_count: a freshly-registered
// game (just published via the wizard, 0 resources yet) HAS a row → is_on_forum
// true → normal page, so the owner can reach the 资源 tab to upload the first
// resource. `=== false` so a missing flag (shouldn't happen here) stays normal.
const isNoPatch = computed(() => patch.value?.is_on_forum === false)

// 发布补丁 CTA → the publish wizard, pre-seeded with this galgame's name so it
// auto-searches and the user is one click from selecting it (works for VNDB and
// original works alike — see edit/create.vue's ?q= handling).
const publishHref = computed(
  () => `/edit/create?q=${encodeURIComponent(displayName.value)}`
)

// SEO contract for this route:
//   - patch loaded + sfw     → full SEO (title / desc / og image with banner)
//   - patch loaded + nsfw    → disable SEO. The patch *is* visible to the
//     viewer (they got here because they're logged-in / acked / opted-in),
//     but we must not let search engines index a NSFW page.
//   - patch null + NSFW gate → disable SEO (the confirm placeholder is
//     intentionally generic; an indexable title would itself be a NSFW
//     signal: "X 不存在" vs "X 含 NSFW 内容确认页" are distinguishable).
//   - patch null + truly missing → disable SEO (404 stub shouldn't index).
//
// patch.banner survived the D12 metadata move because the enricher writes
// the wiki galgame.banner verbatim onto GalgameCard — see
// apps/api/internal/galgame/enricher/enricher.go applyGalgame.
if (patch.value && patch.value.content_limit === 'sfw' && !isNoPatch.value) {
  const cover = resolveBannerUrl(patch.value) || undefined
  useKunSeoMeta({
    title: displayName.value || `补丁 ${galgameId.value}`,
    description: displayName.value
      ? `${displayName.value} 的中文补丁、汉化补丁、AI 翻译补丁等资源下载`
      : '',
    ogType: 'article',
    ogImage: cover
  })
} else {
  useKunDisableSeo(displayName.value || `补丁 ${galgameId.value}`)
}

onMounted(async () => {
  // No view count for a not-yet-收录 galgame — it has no row, so the increment is
  // a backend no-op anyway; skip the wasted round-trip.
  if (isNoPatch.value) return
  await api.put(`/patch/${galgameId.value}/view`).catch(() => {})
})

provide('patch', patch)

// Writable computed so it can drive KunTab's v-model. The "set" path is a
// no-op because KunTab.href already calls navigateTo(); the route change
// re-runs the getter and the tab indicator follows.
const currentTab = computed({
  get: () => route.path.split('/').filter(Boolean).pop() ?? 'introduction',
  set: () => {}
})

// "编辑历史" / "编辑请求" tabs proxy the Wiki revision/PR surface that
// handbook §15 makes mandatory for moyu (pages/patch/[id]/revisions.vue,
// prs.vue).
const tabs = computed(() => {
  const all = [
    { key: 'introduction', title: 'Galgame 信息', href: `/patch/${galgameId.value}/introduction` },
    { key: 'resource', title: '补丁资源下载', href: `/patch/${galgameId.value}/resource` },
    { key: 'comment', title: '游戏评论', href: `/patch/${galgameId.value}/comment` },
    { key: 'revisions', title: '编辑历史', href: `/patch/${galgameId.value}/revisions` },
    { key: 'prs', title: '编辑请求', href: `/patch/${galgameId.value}/prs` }
  ]
  // Not yet 收录 → drop only 补丁资源下载 (there's no patch to download; 发布补丁
  // is the CTA). 游戏评论 STAYS — commenting lazily records the game (kungal's
  // interaction-driven ingest), same as 收藏.
  return isNoPatch.value
    ? all.filter((t) => ['introduction', 'comment', 'revisions', 'prs'].includes(t.key))
    : all
})
</script>

<template>
  <div v-if="patch" class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4">
    <!-- ── Hero ───────────────────────────────────────── -->
    <div
      class="bg-content1 shadow-kun-sm overflow-hidden rounded-3xl"
    >
      <div class="flex flex-col gap-5 p-6 sm:flex-row sm:p-8">
        <!-- Single-item Gallery: lets the user click the hero cover to
             open it full-screen with zoom/pan/download. Single item
             still benefits from the toolbar (zoom into details), which
             is the whole point on mobile where the inline cover is small.
             `as="div"` keeps the existing aspect-video container; the
             cursor-zoom-in cue tells users the cover is interactive.
             We deliberately do NOT wrap the resource cover in
             resource/[id].vue with this — that thumbnail is a
             <NuxtLink> to the patch detail page (different intent). -->
        <div class="relative w-full shrink-0 sm:w-72 lg:w-80">
          <KunLightboxGallery>
            <!-- Hero cover at its REAL aspect ratio (fallback 16/9 until the
                 pinned cover's metadata backfills) so a portrait cover is no
                 longer cropped to 16:9; KunImage owns the ratio box now, so the
                 wrapper drops `aspect-video`. ThumbHash blur-up on the LCP. -->
            <KunLightboxGalleryItem
              :src="resolveBannerUrl(patch) || '/kungalgame-trans.webp'"
              :alt="displayName"
              as="div"
              class="border-default/20 bg-default-100 w-full overflow-hidden rounded-2xl border shadow-lg"
            >
              <KunImage
                :src="resolveBannerUrl(patch) || '/kungalgame-trans.webp'"
                :alt="displayName"
                loading="eager"
                fetchpriority="high"
                :aspect-ratio="resolveBannerAspectRatio(patch)"
                :thumbhash="resolveBannerThumbhash(patch)"
                class-name="block w-full"
                image-class-name="transition-transform duration-300 hover:scale-[1.03]"
              />
            </KunLightboxGalleryItem>
          </KunLightboxGallery>
          <!-- 查看所有封面 — sibling of the lightbox item (not nested) so it
               doesn't trigger the banner lightbox; opens the covers modal. -->
          <button
            type="button"
            class="bg-background/80 hover:bg-background shadow-kun-sm absolute right-2 bottom-2 z-10 inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium backdrop-blur transition-colors"
            @click="coversOpen = true"
          >
            <KunIcon name="lucide:images" class="size-4" />
            查看所有封面
          </button>
          <GalgameCovers v-model="coversOpen" :galgame-id="galgameId" />
        </div>

        <div class="flex min-w-0 flex-1 flex-col justify-between gap-4">
          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <h1
                class="text-2xl leading-tight font-bold break-words sm:text-3xl"
              >
                {{ displayName }}
              </h1>
              <KunTooltip
                :text="GALGAME_AGE_LIMIT_DETAIL[patch.content_limit]"
                position="right"
              >
                <KunChip
                  :color="patch.content_limit === 'sfw' ? 'success' : 'danger'"
                  variant="flat"
                >
                  {{ GALGAME_AGE_LIMIT_MAP[patch.content_limit] }}
                </KunChip>
              </KunTooltip>
            </div>

            <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <template v-for="(value, key) in patch.name" :key="key">
                <span
                  v-if="value && value !== displayName"
                  class="text-default-500 text-xs"
                >
                  {{ value }}
                </span>
              </template>
            </div>

            <KunPatchAttribute
              :types="patch.type"
              :languages="patch.language"
              :platforms="patch.platform"
              size="sm"
            />
          </div>

          <div class="space-y-4">
            <div
              class="border-default/20 flex flex-col items-start justify-between gap-4 border-t pt-4 sm:flex-row sm:items-center"
            >
              <!-- 词条创建者 = wiki galgame.user_id（单一可信源，与 kungal 对齐）。
                   只展示词条创建者；缺数据时回退展示补丁发布者(patch.user，moyu
                   本地数据)，避免空白。 -->
              <div class="flex flex-col gap-1.5">
                <KunUserChip
                  :user="patch.creator ?? patch.user"
                  :description="creatorDescription"
                />
              </div>
              <KunCardStats
                v-if="!isNoPatch"
                :patch="{ ...patch, created: patch.created }"
                :disable-tooltip="false"
                :is-mobile="false"
              />
              <KunChip v-else color="warning" variant="flat" size="sm">
                本站尚未收录
              </KunChip>
            </div>

            <PatchHeaderActions :patch="patch" />
          </div>
        </div>
      </div>
    </div>

    <!-- ── 本站暂无补丁 CTA (no real patch on moyu yet) ──── -->
    <div
      v-if="isNoPatch"
      class="border-warning/30 bg-warning/5 flex flex-col items-start gap-3 rounded-2xl border p-5 sm:flex-row sm:items-center sm:justify-between"
    >
      <div class="flex items-start gap-3">
        <div
          class="bg-warning/15 text-warning flex size-10 shrink-0 items-center justify-center rounded-full"
        >
          <KunIcon name="lucide:circle-alert" class="size-5" />
        </div>
        <div>
          <p class="font-semibold">本站尚未收录此游戏</p>
          <p class="text-default-500 text-sm">
            当前页面的资料均来自 Galgame Wiki，本站还没有它的补丁或本地数据。收藏 / 评论
            都会让它被本站收录（但您不会成为该游戏的创建者，也不会获得萌萌点奖励）；发布补丁同样会让它被收录，并照常获得发布补丁的萌萌点奖励。
          </p>
        </div>
      </div>
      <KunButton color="primary" :href="publishHref" class-name="shrink-0">
        <KunIcon name="lucide:plus-circle" class="size-4" />
        发布补丁
      </KunButton>
    </div>

    <!-- ── Tabs ───────────────────────────────────────── -->
    <!-- scrollable: 5 个 Tab 在手机端会超出视口宽度;KunTab 的 scrollable
         让 tablist 横向滚动(overflow-x-auto scrollbar-hide)而非撑破布局。 -->
    <KunTab
      v-model="currentTab"
      :items="tabs.map((t) => ({ value: t.key, textValue: t.title, href: t.href }))"
      variant="underlined"
      color="primary"
      size="md"
      scrollable
    />

    <div>
      <NuxtPage />
    </div>
  </div>

  <!-- NSFW confirm placeholder. Rendered by SSR with no actual NSFW data
       embedded (intentionally generic text + warning icon) so it's safe to
       index — search engines see "this content needs confirmation", not
       the game's name/banner/intro. Visible only on the
       (anonymous + sfw mode + not acked) branch; everyone else either has
       the data or sees the real "not found" state. -->
  <div
    v-else-if="shouldShowNsfwConfirm"
    class="mx-auto flex w-full max-w-xl flex-col items-center gap-6 px-4 py-16 text-center"
  >
    <div
      class="bg-danger/10 text-danger flex size-16 items-center justify-center rounded-full"
    >
      <KunIcon name="lucide:shield-alert" class="size-8" />
    </div>
    <div class="space-y-2">
      <h1 class="text-2xl font-bold">该 Galgame 含有 NSFW 内容</h1>
      <p class="text-default-500 text-sm leading-relaxed">
        您需要点击下方按钮以确认查看。<br />
        登录后无需每次确认。
      </p>
    </div>
    <div class="flex flex-col gap-2 sm:flex-row">
      <KunButton color="danger" size="md" @click="confirmNsfw">
        我已知晓，仍要查看
      </KunButton>
      <NuxtLink to="/">
        <KunButton variant="light" color="default" size="md">
          返回首页
        </KunButton>
      </NuxtLink>
    </div>
  </div>

  <KunNull v-else description="Galgame 不存在" />
</template>
