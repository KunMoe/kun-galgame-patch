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

const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-galgames`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/patch?page=1&limit=20`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)
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
  </div>
</template>
