<script setup lang="ts">
// keepalive: returning from a comment's target restores this list's page +
// scroll instead of resetting to page 1. Safe — static route, page seeded from
// the URL into a plain ref (no route-param computeds that misfire when cached).
// Kept alive via the central include list in app.vue, keyed by this name.
defineOptions({ name: 'comment-feed' })

const route = useRoute()
const router = useRouter()
const api = useApi()

// SFW-only listing for anonymous crawlers (comments whose owning patch is
// NSFW are filtered out by enricher.FilterByGalgameContentLimit on the
// /api/comment endpoint). Safe to give a descriptive SEO blurb.
useKunSeoMeta({
  title: '最新评论',
  description:
    '鲲 Galgame 补丁站的全站最新评论流，看其他玩家对各款 Galgame 中文汉化补丁的安装体验、剧情讨论和评分反馈。'
})

const page = ref(Number(route.query.page ?? 1))
const pageHref = usePageHref() // crawlable pagination (<a href>)
const limit = 20

// commentListRequest requires sort_field / sort_order.
interface ListResponse {
  items: PatchComment[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'comment-list',
  async () => {
    const params = new URLSearchParams({
      sort_field: 'created',
      sort_order: 'desc',
      page: String(page.value),
      limit: String(limit)
    })
    const res = await api.get<ListResponse>(`/comment?${params.toString()}`)
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
const onChangePage = async (v: number) => {
  page.value = v
  await router.replace({ query: { page: v } })
  await refresh()
  if (import.meta.client) window.scrollTo({ top: 0 })
}
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader name="最新评论" description="浏览全站的最新补丁评论" />
    <KunLoading v-if="pending" description="加载评论中..." />
    <div v-else class="space-y-4">
      <CommentCard
        v-for="c in data?.items"
        :key="c.id"
        :comment="c"
      />
    </div>
    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无评论"
    />
    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        :page-href="pageHref"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
