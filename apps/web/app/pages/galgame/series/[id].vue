<script setup lang="ts">
const route = useRoute()
const api = useApi()

const seriesId = computed(() => Number((route.params as { id: string }).id))

const PAGE_SIZE = 24

const { data } = await useAsyncData<SeriesDetail | null>(
  () => `galgame-series-${seriesId.value}`,
  async () => {
    const res = await api.get<SeriesDetail>(
      `/galgame/series/${seriesId.value}?page=1&limit=${PAGE_SIZE}`
    )
    return res.code === 0 ? res.data : null
  }
)

const series = computed(() => data.value)

const galgames = ref<GalgameCard[]>([])
const page = ref(1)
const loadingMore = ref(false)

watch(
  () => data.value,
  (d) => {
    if (d) {
      galgames.value = [...d.galgames]
      page.value = 1
    }
  },
  { immediate: true }
)

const hasMore = computed(
  () => !!series.value && galgames.value.length < series.value.total
)

const loadMore = async () => {
  if (!hasMore.value || loadingMore.value) {
    return
  }
  loadingMore.value = true
  const next = page.value + 1
  const res = await api.get<SeriesDetail>(
    `/galgame/series/${seriesId.value}?page=${next}&limit=${PAGE_SIZE}`
  )
  loadingMore.value = false
  if (res.code !== 0 || !res.data) {
    return
  }
  galgames.value.push(...res.data.galgames)
  page.value = next
}

useHead(() => ({
  title: series.value ? `${series.value.name} 系列的 Galgame` : 'Galgame 系列'
}))
</script>

<template>
  <div
    v-if="series"
    class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4"
  >
    <div class="bg-content1 shadow-kun-sm rounded-3xl p-6 sm:p-8">
      <div class="space-y-2">
        <h1 class="text-2xl font-bold break-words sm:text-3xl">
          {{ series.name }} 系列的 Galgame
        </h1>
        <p v-if="series.description" class="text-default-600">
          {{ series.description }}
        </p>
        <p class="text-default-500 text-sm">
          本页展示该系列的全部 Galgame，共 {{ series.total }} 部。默认仅显示
          SFW 的 Galgame，查看 NSFW Galgame 请在顶部设置面板打开 NSFW 开关。
        </p>
      </div>
    </div>

    <div
      v-if="galgames.length"
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
    >
      <GalgameCard v-for="g in galgames" :key="g.id" :patch="g" />
    </div>
    <KunNull v-else description="该系列下暂无 Galgame" />

    <div v-if="hasMore" class="flex justify-center">
      <KunButton
        variant="flat"
        color="primary"
        :is-loading="loadingMore"
        @click="loadMore"
      >
        加载更多作品
      </KunButton>
    </div>
  </div>

  <KunNull v-else description="未找到该系列" />
</template>
