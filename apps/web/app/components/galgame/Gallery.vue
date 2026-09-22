<script setup lang="ts">
import {
  imageServiceUrl,
  imageAspectRatio
} from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  screenshots: GalgameScreenshotRow[]
}>()

const settingStore = useSettingStore()
const { stance } = useKunNsfwStance()

// Under 'blur' the rated shots still pass this gate — they are masked below
// rather than filtered out, which is the whole difference between the two
// stances. The per-level opt-in keeps its own job for the 'hide' reader.
const showNsfw = computed(() => stance.value !== 'hide')
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

// Same fold the character grid and the staff list on this page already use, so
// a long gallery does not push the rest of the introduction off the screen.
// Every visible tile stays a whole screenshot: covering the last one with the
// count would cost a click target the lightbox needs.
const COLLAPSED = 12
const isExpanded = ref(false)
const isCollapsible = computed(() => sorted.value.length > COLLAPSED)
const visible = computed(() =>
  isCollapsible.value && !isExpanded.value
    ? sorted.value.slice(0, COLLAPSED)
    : sorted.value
)

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
          v-for="s in visible"
          :key="s.image_hash"
          :src="imgSrc(s)"
          :alt="s.caption || s.image_hash.slice(0, 8)"
          as="figure"
          class="border-default/20 block overflow-hidden rounded-lg border"
        >
          <KunNsfwMask
            :nsfw="s.sexual >= 1 || s.violence >= 1"
            rounded="rounded-none"
            label="该截图带有分级"
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
          </KunNsfwMask>
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

    <KunButton
      v-if="isCollapsible"
      variant="flat"
      color="primary"
      size="sm"
      @click="isExpanded = !isExpanded"
    >
      <KunIcon
        :name="isExpanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
      />
      {{
        isExpanded ? '收起截图' : `展开其余 ${sorted.length - COLLAPSED} 张截图`
      }}
    </KunButton>
  </div>
</template>
