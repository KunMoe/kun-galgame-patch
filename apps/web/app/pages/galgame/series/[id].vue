<script setup lang="ts">
defineOptions({ name: 'series-detail' })

const route = useRoute()
const router = useRouter()
const browse = useTaxonomyBrowse()

const seriesID = computed(() => Number(route.params.id))
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v } })
  }
})
const pageHref = usePageHref()
const limit = 24

const VERDICT_NOT_FOUND = 40400

const { data, pending } = await useAsyncData(
  () => `series-detail-${seriesID.value}-${page.value}`,
  async () => {
    const res = await browse.seriesDetail(seriesID.value, {
      page: page.value,
      limit
    })
    return { code: res.code, detail: res.code === 0 ? res.data : null }
  },
  { watch: [page, seriesID] }
)

const notFound = () => kunPageError(404, '系列不存在')

if (data.value?.code === VERDICT_NOT_FOUND) throw notFound()
watch(data, (v) => {
  if (v?.code === VERDICT_NOT_FOUND) showError(notFound())
})

const series = computed(() => data.value?.detail?.series ?? null)
const galgames = computed<GalgameCard[]>(
  () => data.value?.detail?.galgames ?? []
)
const total = computed(() => data.value?.detail?.total ?? 0)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

// has_nsfw is counted before the reader's gate, so a series whose members are
// all r18 answers zero works to an SFW reader. Without this the page reads
// "暂无关联作品" and looks broken.
const isGatedEmpty = computed(
  () => !galgames.value.length && !!series.value?.has_nsfw
)

if (series.value) {
  useKunSeoMeta({
    title: `系列 · ${series.value.name}`,
    description: `${series.value.name} 系列（${series.value.galgame_count ?? '0'} 个 Galgame）的汉化补丁、中文补丁资源下载合集`
  })
} else {
  useKunDisableSeo('系列详情')
}
</script>

<template>
  <div class="container mx-auto my-6">
    <KunLoading v-if="pending && !series" description="加载中..." />

    <KunNull v-else-if="!series" description="系列加载失败，请稍后重试" />

    <template v-else>
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold sm:text-3xl">{{ series.name }}</h1>
          <KunChip color="default" size="sm">
            {{ series.galgame_count ?? 0 }} 个 Galgame
          </KunChip>
        </div>
        <p
          v-if="series.description"
          class="text-default-700 mt-3 text-sm whitespace-pre-wrap"
        >
          {{ series.description }}
        </p>
      </section>

      <section class="mt-6">
        <div class="mb-4 flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-xl font-bold">该系列的 Galgame</h2>
        </div>

        <KunNull
          v-if="isGatedEmpty"
          description="该系列的作品均为 NSFW 内容，请在顶栏切换内容显示后查看"
        />
        <KunNull v-else-if="!galgames.length" description="暂无关联作品" />

        <GalgameList v-else :items="galgames" />

        <KunPagination
          v-if="totalPage > 1"
          v-model:current-page="page"
          :total-page="totalPage"
          :is-loading="pending"
          :page-href="pageHref"
          class="mt-6"
        />
      </section>
    </template>
  </div>
</template>
