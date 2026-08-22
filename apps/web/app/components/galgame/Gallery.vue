<script setup lang="ts">
import {
  imageServiceUrl,
  imageAspectRatio
} from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  screenshots: GalgameScreenshotRow[]
}>()

const settingStore = useSettingStore()

const showNsfw = computed(() => settingStore.data.kunNsfwEnable !== 'sfw')
const sexualLevels = computed(() => settingStore.data.gallerySexualLevels ?? [])
const violenceLevels = computed(
  () => settingStore.data.galleryViolenceLevels ?? []
)

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

const PREVIEW = 12
const isExpanded = ref(false)
const visible = computed(() =>
  isExpanded.value ? sorted.value : sorted.value.slice(0, PREVIEW)
)
const folded = computed(() => sorted.value.length - visible.value.length)
const foldIndex = computed(() => (folded.value > 0 ? visible.value.length - 1 : -1))
const expand = () => {
  isExpanded.value = true
}

const hasRated = computed(() =>
  allShots.value.some((s) => s.sexual >= 1 || s.violence >= 1)
)

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

const RING_W = 2.5
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
    <div class="flex flex-wrap items-end justify-between gap-3">
      <KunHeader name="截图 / 画廊" scale="h2" />
      <GalgameGalleryFilter
        v-if="hasRated"
        :show-nsfw="showNsfw"
        :hidden-count="hiddenCount"
        :sexual-counts="sexualCounts"
        :violence-counts="violenceCounts"
      />
    </div>

    <KunLightboxGallery v-if="sorted.length">
      <div
        class="grid grid-cols-1 items-start gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        <KunLightboxGalleryItem
          v-for="(s, i) in visible"
          :key="s.image_hash"
          :src="imgSrc(s)"
          :alt="s.caption || s.image_hash.slice(0, 8)"
          :wrap="false"
          v-slot="{ open }"
        >
          <button
            type="button"
            class="group relative block w-full overflow-hidden rounded-lg text-left"
            :aria-label="
              i === foldIndex ? '显示全部截图' : (s.caption || '查看截图')
            "
            @click="i === foldIndex ? expand() : open()"
          >
            <div class="relative">
              <KunImage
                :src="imgSrc(s)"
                :alt="s.caption || s.image_hash.slice(0, 8)"
                loading="lazy"
                :aspect-ratio="imageAspectRatio(s.width, s.height)"
                :thumbhash="s.thumbhash"
                class-name="bg-default-100"
              />
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
            <div
              v-if="i === foldIndex"
              class="absolute inset-0 flex flex-col items-center justify-center gap-1 rounded-lg bg-black/60 text-white"
            >
              <span class="text-lg font-medium">+{{ folded }}</span>
              <span class="text-xs">显示全部</span>
            </div>
          </button>
        </KunLightboxGalleryItem>
      </div>
    </KunLightboxGallery>

    <KunNull
      v-else
      :description="`${hiddenCount} 张图片已按分级隐藏，点击「分级筛选」调整`"
    />
  </div>
</template>
