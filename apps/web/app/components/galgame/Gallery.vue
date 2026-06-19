<script setup lang="ts">
// Galgame screenshot 画廊 with a per-rating SFW gate (ported from kungal's
// galgame/Gallery.vue). Two independent axes, both filtered CLIENT-SIDE:
//   色情 (sexual)   — the global NSFW mode shows every level; SFW shows only the
//                     levels the viewer opted into (+ unrated).
//   暴力 (violence) — independent per-level opt-in (default off, behind a
//                     warning); the NSFW mode does NOT unlock it (sex and gore
//                     are separate sensitivities).
// An image shows iff BOTH its sexual and violence levels are permitted. The
// filter control stays reachable even when everything is hidden, so a fully
// rated gallery can still be revealed.
//
// Screenshots only carry an image_hash (no cdn_url) and the screenshot preset
// generates NO image_service variants, so thumb + lightbox both use the full
// image (imageServiceUrl with no variant).
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  screenshots: GalgameScreenshotRow[]
}>()

const settingStore = useSettingStore()

const showNsfw = computed(() => settingStore.data.kunNsfwEnable !== 'sfw')
const sexualLevels = computed(() => settingStore.data.gallerySexualLevels ?? [])
const violenceLevels = computed(
  () => settingStore.data.galleryViolenceLevels ?? []
)

// 色情: NSFW reveals every level; otherwise unrated (0) + opted-in levels.
// 暴力: unrated (0) + opted-in levels only, independent of the NSFW mode.
const sexualOk = (s: GalgameScreenshotRow) =>
  showNsfw.value || s.sexual === 0 || sexualLevels.value.includes(s.sexual)
const violenceOk = (s: GalgameScreenshotRow) =>
  s.violence === 0 || violenceLevels.value.includes(s.violence)

const allShots = computed(() =>
  [...(props.screenshots ?? [])].filter((s) => !!s.image_hash)
)

const sorted = computed(() =>
  allShots.value
    .filter((s) => sexualOk(s) && violenceOk(s))
    .sort((a, b) => {
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
      return a.image_hash.localeCompare(b.image_hash)
    })
)

const hiddenCount = computed(() => allShots.value.length - sorted.value.length)

// Only surface the filter when something is actually rated — an all-unrated
// gallery needs no control.
const hasRated = computed(() =>
  allShots.value.some((s) => s.sexual >= 1 || s.violence >= 1)
)

// Per-level image counts (level 1/2/3 → n) so the filter can show how many
// images each toggle reveals/hides.
const countLevels = (axis: 'sexual' | 'violence'): Record<number, number> => {
  const counts: Record<number, number> = { 1: 0, 2: 0, 3: 0 }
  for (const s of allShots.value) {
    const level = s[axis]
    if (level >= 1 && level <= 3) counts[level] = (counts[level] ?? 0) + 1
  }
  return counts
}
const sexualCounts = computed(() => countLevels('sexual'))
const violenceCounts = computed(() => countLevels('violence'))

// Per-tile rating rings: outer band = 色情 (warning), inner band = 暴力
// (danger); colour depth = level (轻/中/高). Nested inset box-shadows on a
// pointer-events-none overlay above the image, so they can't be clipped or
// block clicks. An axis with no rating draws nothing.
const RING_W = 2.5 // px per band
const RING_DEPTH: Record<number, number> = { 1: 60, 2: 80, 3: 100 }
const ringColor = (token: 'warning' | 'danger', level: number) =>
  `color-mix(in oklab, var(--color-${token}) ${RING_DEPTH[level] ?? 100}%, transparent)`

const ratingRing = (s: GalgameScreenshotRow) => {
  const shadows: string[] = []
  if (s.sexual >= 1) {
    shadows.push(`inset 0 0 0 ${RING_W}px ${ringColor('warning', s.sexual)}`)
  }
  if (s.violence >= 1) {
    const inset = s.sexual >= 1 ? RING_W * 2 : RING_W
    shadows.push(`inset 0 0 0 ${inset}px ${ringColor('danger', s.violence)}`)
  }
  return { boxShadow: shadows.join(', ') }
}

const imgSrc = (s: GalgameScreenshotRow) => imageServiceUrl(s.image_hash)
</script>

<template>
  <div v-if="allShots.length" class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-2xl font-bold">截图 / 画廊</h2>
      </div>
      <GalgameGalleryFilter
        v-if="hasRated"
        :show-nsfw="showNsfw"
        :hidden-count="hiddenCount"
        :sexual-counts="sexualCounts"
        :violence-counts="violenceCounts"
      />
    </div>

    <KunLightboxGallery v-if="sorted.length">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <KunLightboxGalleryItem
          v-for="s in sorted"
          :key="s.image_hash"
          :src="imgSrc(s)"
          :alt="s.caption || s.image_hash.slice(0, 8)"
          as="figure"
          class="border-default/20 block overflow-hidden rounded-lg border"
        >
          <div class="relative">
            <KunImage
              :src="imgSrc(s)"
              :alt="s.caption || s.image_hash.slice(0, 8)"
              loading="lazy"
              aspect-ratio="16 / 9"
              class-name="bg-default-100"
            />
            <!-- rating rings: outer=色情 inner=暴力, depth=level -->
            <div
              v-if="s.sexual >= 1 || s.violence >= 1"
              class="pointer-events-none absolute inset-0"
              :style="ratingRing(s)"
            />
          </div>
          <figcaption
            v-if="s.caption"
            class="text-default-500 px-2 py-1 text-xs"
          >
            {{ s.caption }}
          </figcaption>
        </KunLightboxGalleryItem>
      </div>
    </KunLightboxGallery>

    <KunNull
      v-else
      :description="`${hiddenCount} 张图片已按分级隐藏，点击「分级筛选」调整`"
    />
  </div>
</template>
