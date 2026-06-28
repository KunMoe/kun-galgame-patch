<script setup lang="ts">
// keepalive: returning from a resource detail restores this list's page +
// filters + scroll instead of remounting and resetting to page 1 (borrowed from
// kungal's feed). The page stays URL-synced for fresh visits / shared links.
definePageMeta({ keepalive: true })

const route = useRoute()
const router = useRouter()
const api = useApi()

// SFW-only listing for anonymous crawlers (resources whose owning patch is
// NSFW are filtered out by enricher.FilterByGalgameContentLimit on the
// /api/resource endpoint).
useKunSeoMeta({
  title: '最新补丁资源',
  description:
    '鲲 Galgame 补丁站的全站最新补丁资源列表，覆盖 Windows / 安卓 / KRKR / Tyranor 平台的中文汉化、官方中文、AI 翻译等 Galgame 补丁资源，免费下载。'
})

const page = ref(Number(route.query.page ?? 1))
const pageHref = usePageHref() // crawlable pagination (<a href>)
const limit = 20

// /api/v1/resource is a paginated list — see apps/api/internal/common/handler.go
// resourceListRequest. sort_field / sort_order are required.
interface ListResponse {
  items: PatchResource[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'resource-list',
  async () => {
    const params = new URLSearchParams({
      sort_field: 'created',
      sort_order: 'desc',
      page: String(page.value),
      limit: String(limit)
    })
    const res = await api.get<ListResponse>(`/resource?${params.toString()}`)
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
    <KunHeader
      name="补丁资源"
      description="浏览本站收录的最新补丁资源下载"
    />
    <KunLoading v-if="pending" description="加载资源中..." />
    <div v-else class="grid grid-cols-1 gap-3 sm:gap-6 md:grid-cols-2">
      <ResourceCard
        v-for="r in data?.items"
        :key="r.id"
        :resource="r"
      />
    </div>
    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无资源"
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
