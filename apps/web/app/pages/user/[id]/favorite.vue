<script setup lang="ts">
// Backend wraps lists in response.Paginated -> { items, total }. Items are
// enriched GalgameCards via enricher.EnrichPatches.
// keepalive: returning from a detail restores this tab's page + scroll. `page`
// is a computed off ?page=, so reactivation re-reads the URL for the right page.
// Kept alive via the central include list in app.vue, keyed by this name.
defineOptions({ name: 'user-favorite' })

const route = useRoute()
const router = useRouter()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: GalgameCard[]
  total: number
}

// Page in the URL (?page=) so back-nav / shared links restore it; switching to
// another user lands on a clean URL → page 1.
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v) => router.replace({ query: { ...route.query, page: String(v) } })
})
const limit = 20
const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-favorites`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/favorite?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)
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
    <KunNull v-else description="该用户暂无收藏" />

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
