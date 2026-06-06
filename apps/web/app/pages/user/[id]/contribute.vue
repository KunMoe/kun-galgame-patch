<script setup lang="ts">
// /user/:id/contribute returns a paginated list of GalgameCard-shaped patches
// the user has contributed to (see apps/api/internal/user/handler GetUserContributions
// which passes the rows through enricher.EnrichPatches).
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
  () => `user-${userId.value}-contribute`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/contribute?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)
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
    <div v-else-if="data?.items?.length" class="space-y-2">
      <NuxtLink
        v-for="c in data.items"
        :key="c.id"
        :to="`/patch/${c.id}/introduction`"
        class="border-default/20 bg-background hover:bg-default-100 flex items-center justify-between rounded-lg border p-3 transition-colors"
      >
        <span class="font-medium line-clamp-1">
          {{ getPreferredLanguageText(c.name) }}
        </span>
        <span class="text-default-500 text-xs">
          {{ formatDistanceToNow(c.created) }}
        </span>
      </NuxtLink>
    </div>
    <KunNull v-else description="该用户暂无贡献记录" />

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
