<script setup lang="ts">
// Backend wraps lists in response.Paginated -> { items, total }. Items are
// already enriched GalgameCards via enricher.EnrichPatches (Wiki batch).
//
// API path is /user/:id/patch (backend route name), even though the tab is
// labeled "Galgame" on the frontend -- the local row is `patch`, the
// galgame metadata comes from Wiki via the enricher.
const route = useRoute()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: GalgameCard[]
  total: number
}

const page = ref(1)
const limit = 20
const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-galgames`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/patch?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)
// Reset to page 1 when the profile owner changes — this tab component is reused
// across /user/:id navigations, so page state would otherwise carry over.
watch(userId, () => {
  page.value = 1
})
const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
const onChangePage = (v: number) => {
  page.value = v
  if (import.meta.client) window.scrollTo({ top: 0 })
}
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <div
      v-else-if="data?.items?.length"
      class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
    >
      <GalgameCard
        v-for="patch in data.items"
        :key="patch.id"
        :patch="patch"
      />
    </div>
    <KunNull v-else description="该用户暂未发布任何 Galgame" />

    <div v-if="totalPages > 1" class="mt-6 flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
