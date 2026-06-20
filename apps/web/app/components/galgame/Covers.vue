<script setup lang="ts">
// "查看所有封面" modal for the galgame detail banner.
//
// The patch header (/patch/:id) is enriched from the wiki's /galgame/batch,
// which is a whitelist DTO that does NOT carry the `covers` array (only
// `effective_banner_hash`). So we lazily fetch /patch/:id/detail (backed by the
// single /galgame/:gid, which returns the full covers) the first time the modal
// opens — no cost unless the user actually wants to see the covers.
//
// Covers are galgame_banner-preset image_service images, so they have a `mini`
// variant for the grid thumbnail; the lightbox opens the full image.
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{ galgameId: number }>()
const open = defineModel<boolean>({ required: true })

const api = useApi()

const covers = ref<GalgameCoverRow[] | null>(null)
const loading = ref(false)
const failed = ref(false)

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
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <KunLightboxGalleryItem
            v-for="c in covers"
            :key="c.image_hash"
            :src="imageServiceUrl(c.image_hash)"
            :alt="`封面 ${c.sort_order + 1}`"
            as="figure"
            class="border-default/20 bg-default-100 block overflow-hidden rounded-lg border"
          >
            <KunImage
              :src="imageServiceUrl(c.image_hash, 'mini')"
              :alt="`封面 ${c.sort_order + 1}`"
              loading="lazy"
              aspect-ratio="16 / 9"
              class-name="bg-default-100"
            />
          </KunLightboxGalleryItem>
        </div>
      </KunLightboxGallery>
    </div>
  </KunModal>
</template>
