<script setup lang="ts">
// "查看所有封面" modal for the galgame detail banner.
//
// The patch header (/patch/:id) is enriched from the wiki's /galgame/batch,
// which is a whitelist DTO that does NOT carry the `covers` array (only
// `effective_banner_hash`). So we lazily fetch /patch/:id/detail (backed by the
// single /galgame/:gid, which returns the full covers) the first time the modal
// opens — no cost unless the user actually wants to see the covers.
//
// The wiki syncs a VN's whole VNDB /cv gallery (main + every release cover), each
// tagged with `kind`; we group by kind so 主封面 / 盒装正面 / 数字版 / 封底 … are
// separated. Covers are galgame_banner-preset image_service images, so they have a
// `mini` variant for the grid thumbnail; the lightbox opens the full image.
import {
  imageServiceUrl,
  imageAspectRatio
} from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{ galgameId: number }>()
const open = defineModel<boolean>({ required: true })

const api = useApi()

const covers = ref<GalgameCoverRow[] | null>(null)
const loading = ref(false)
const failed = ref(false)

// Kind → display label, in the order sections should appear. Anything unknown /
// empty falls into 其它 at the end.
const KIND_LABEL: Record<string, string> = {
  main: '主封面',
  pkgfront: '盒装正面',
  dig: '数字版',
  pkgback: '封底',
  pkgcontent: '内页',
  pkgside: '书脊',
  pkgmed: '碟面',
  '': '其它'
}
const KIND_ORDER = Object.keys(KIND_LABEL)

// Covers grouped into ordered, labeled sections (only non-empty kinds shown).
const groups = computed(() => {
  const byKind = new Map<string, GalgameCoverRow[]>()
  for (const c of covers.value ?? []) {
    const k = KIND_LABEL[c.kind ?? ''] !== undefined ? (c.kind ?? '') : ''
    if (!byKind.has(k)) byKind.set(k, [])
    byKind.get(k)!.push(c)
  }
  return KIND_ORDER.filter((k) => byKind.has(k)).map((k) => ({
    kind: k,
    label: KIND_LABEL[k],
    covers: byKind.get(k)!
  }))
})

// Fetch once and cache: covers don't change while the page is open.
const load = async () => {
  if (covers.value || loading.value) return
  loading.value = true
  failed.value = false
  const res = await api.get<PatchDetail>(`/patch/${props.galgameId}/detail`)
  if (res.code === 0 && res.data) {
    covers.value = [...(res.data.galgame?.covers ?? [])]
      .filter((c) => !!c.image_hash)
      .sort((a, b) => a.sort_order - b.sort_order)
  } else {
    failed.value = true
  }
  loading.value = false
}

watch(open, (v) => {
  if (v) load()
})
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-3xl w-full">
    <div class="space-y-4">
      <div class="flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-xl font-bold">所有封面</h2>
      </div>

      <KunLoading v-if="loading" description="加载封面中..." />
      <KunNull v-else-if="failed" description="加载失败, 请稍后再试" />
      <KunNull v-else-if="covers && !covers.length" description="该游戏暂无封面" />

      <KunLightboxGallery v-else-if="covers">
        <div class="space-y-5">
          <section v-for="g in groups" :key="g.kind" class="space-y-2">
            <h3 class="text-default-600 text-sm font-medium">
              {{ g.label }}
              <span class="text-default-400">({{ g.covers.length }})</span>
            </h3>
            <!-- items-start: covers now have varied real aspect ratios, so the
                 grid must NOT stretch a row's cells to equal height — otherwise
                 a short (landscape) cell shows its figure background as bars
                 next to a tall (portrait) neighbour. Top-align so each figure
                 hugs its own cover. -->
            <div class="grid grid-cols-1 items-start gap-3 sm:grid-cols-2">
              <KunLightboxGalleryItem
                v-for="c in g.covers"
                :key="c.image_hash"
                :src="imageServiceUrl(c.image_hash)"
                :alt="g.label"
                as="figure"
                class="border-default/20 bg-default-100 block overflow-hidden rounded-lg border"
              >
                <!-- Covers are often portrait box art. Size the box to the
                     cover's REAL aspect ratio and load the FULL image with the
                     default object-cover, so box ratio == image ratio: no crop
                     AND no letterbox bars. (The `mini` variant is a 16:9 CROP —
                     pairing it with a real-ratio box left white bars and a
                     pre-cropped cover, so we don't use it here.) Pre-backfill
                     the ratio falls back to 16/9. This is an opt-in modal with
                     few covers and the lightbox loads the full image on click
                     anyway, so serving full here costs little. -->
                <KunImage
                  :src="imageServiceUrl(c.image_hash)"
                  :alt="g.label"
                  loading="lazy"
                  :aspect-ratio="imageAspectRatio(c.width, c.height)"
                  :thumbhash="c.thumbhash"
                  class-name="bg-default-100"
                />
              </KunLightboxGalleryItem>
            </div>
          </section>
        </div>
      </KunLightboxGallery>
    </div>
  </KunModal>
</template>
