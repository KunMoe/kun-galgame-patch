<script setup lang="ts">
// /user/:id/comment returns paginated PatchComments with the owning patch
// summary attached (see user/service.attachPatchSummaries). The row carries
// the local `like_count` and `galgame_id`; backend does not currently fill
// content_html for this list since the user-profile view shows plain content.
// keepalive: returning from a detail restores this tab's page + scroll. `page`
// is a computed off ?page=, so reactivation re-reads the URL for the right page.
definePageMeta({ keepalive: true })

const route = useRoute()
const router = useRouter()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: UserComment[]
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
  () => `user-${userId.value}-comments`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/comment?page=${page.value}&limit=${limit}`
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

const patchName = (c: UserComment) =>
  c.patch?.name ? getPreferredLanguageText(c.patch.name) : `补丁 #${c.galgame_id}`
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else-if="data?.items?.length" class="space-y-3">
      <NuxtLink
        v-for="c in data.items"
        :key="c.id"
        :to="`/patch/${c.galgame_id}/comment`"
        class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 block rounded-lg border p-4 transition-colors"
      >
        <div class="text-default-500 mb-1 text-sm">
          评论在
          <span class="text-primary">{{ patchName(c) }}</span>
        </div>
        <p class="whitespace-pre-wrap line-clamp-3">{{ c.content }}</p>
        <div class="text-default-500 mt-2 flex items-center gap-4 text-xs">
          <div class="flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" class="size-3.5" />
            {{ c.like_count }}
          </div>
          <span>{{ formatDistanceToNow(c.created) }}</span>
        </div>
      </NuxtLink>
    </div>
    <KunNull v-else description="该用户暂无评论" />

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
