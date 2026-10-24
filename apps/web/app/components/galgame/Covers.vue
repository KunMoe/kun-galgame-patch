<script setup lang="ts">
import {
  imageServiceUrl,
  imageAspectRatio
} from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{ galgameId: number }>()
const open = defineModel<boolean>({ required: true })

const api = useApi()
const { stance } = useKunNsfwStance()

const covers = ref<GalgameCoverRow[] | null>(null)
const loading = ref(false)
const failed = ref(false)

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

const load = async () => {
  if (covers.value || loading.value) return
  loading.value = true
  failed.value = false
  const res = await api.get<PatchDetail>(`/patch/${props.galgameId}/detail`)
  if (res.code === 0 && res.data) {
    covers.value = [...(res.data.galgame?.covers ?? [])]
      .filter(
        (c) => !!c.image_hash && (stance.value !== 'hide' || c.sexual === 0)
      )
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
  <KunModal
    v-model="open"
    inner-class-name="max-w-3xl w-full"
    aria-label="所有封面"
  >
    <div class="space-y-4">
      <div class="flex items-center gap-3">
        <div class="bg-primary h-6 w-1 rounded" />
        <h2 class="text-xl font-bold">所有封面</h2>
      </div>

      <KunLoading v-if="loading" description="加载封面中..." />
      <KunNull v-else-if="failed" description="加载失败, 请稍后再试" />
      <KunNull
        v-else-if="covers && !covers.length"
        description="该游戏暂无封面"
      />

      <KunLightboxGallery v-else-if="covers">
        <div class="space-y-5">
          <section v-for="g in groups" :key="g.kind" class="space-y-2">
            <h3 class="text-default-600 text-sm font-medium">
              {{ g.label }}
              <span class="text-default-400">({{ g.covers.length }})</span>
            </h3>
            <div class="grid grid-cols-1 items-start gap-3 sm:grid-cols-2">
              <KunLightboxGalleryItem
                v-for="c in g.covers"
                :key="c.image_hash"
                :src="imageServiceUrl(c.image_hash)"
                :alt="g.label"
                as="figure"
                class="border-default/20 bg-default-100 block overflow-hidden rounded-lg border"
              >
                <KunNsfwMask
                  :nsfw="c.sexual !== 0"
                  rounded="rounded-none"
                  :label="
                    c.sexual === null ? '该封面尚未评级' : '该封面带有分级'
                  "
                >
                  <KunImage
                    :src="imageServiceUrl(c.image_hash)"
                    :alt="g.label"
                    loading="lazy"
                    :aspect-ratio="imageAspectRatio(c.width, c.height)"
                    :thumbhash="c.thumbhash"
                    class-name="bg-default-100"
                  />
                </KunNsfwMask>
              </KunLightboxGalleryItem>
            </div>
          </section>
        </div>
      </KunLightboxGallery>
    </div>
  </KunModal>
</template>
