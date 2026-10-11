<script setup lang="ts">
import {
  SEARCH_CATEGORY_MAP,
  SEARCH_PAGE_SIZE,
  isSearchResourceScope,
  readSearchGalgameFilter,
  searchGalgameFilterQuery
} from './items'

const props = defineProps<{
  keywords: string
  type: SearchPagedType
}>()

const route = useRoute()
const router = useRouter()
const api = useApi()

const results = ref<SearchResultItem[]>([])
const total = ref(0)
const pending = ref(!!props.keywords)
const failed = ref(false)
const top = useTemplateRef<HTMLElement>('top')

// The reader's page belongs in the URL beside the keyword and the filters: while
// it lived in a local ref, opening a result from page 5 and coming back landed
// on page 1, and a shared link never carried the page it was shared from.
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (value) =>
    router.replace({ query: { ...route.query, page: String(value) } })
})

const meta = computed(() => SEARCH_CATEGORY_MAP[props.type])
const limit = computed(() => SEARCH_PAGE_SIZE[props.type])
const totalPage = computed(() => Math.ceil(total.value / limit.value))

// Each lane reads its own filters off the URL, the same place the keyword and
// the category come from, so a filtered search is one shareable link.
const laneParams = computed<Record<string, string>>(() => {
  if (props.type === 'galgame') {
    return searchGalgameFilterQuery(readSearchGalgameFilter(route.query))
  }
  if (props.type === 'resource' && isSearchResourceScope(route.query.scope)) {
    return { scope: 'model' }
  }
  return {}
})

// Watching the request's own query string rather than the values behind it:
// every one of them is read off route.query, which hands back a fresh object on
// any navigation, so watching those would refetch when nothing asked had changed.
const query = computed(() =>
  new URLSearchParams({
    keywords: props.keywords,
    type: props.type,
    page: String(page.value),
    limit: String(limit.value),
    ...laneParams.value
  }).toString()
)

let latest = 0

const load = async () => {
  const current = ++latest
  if (!props.keywords) {
    results.value = []
    total.value = 0
    pending.value = false
    return
  }
  pending.value = true
  const res = await api.get<{ items: SearchResultItem[]; total: number }>(
    `/search?${query.value}`
  )
  if (current !== latest) {
    return
  }
  // Telling the reader "nothing found" when the request never came back is the
  // one thing this must not do.
  failed.value = res.code !== 0
  results.value = res.data?.items ?? []
  total.value = res.data?.total ?? 0
  pending.value = false
}

const { blockKey } = useTagBlocks()

watch(
  [query, page, blockKey],
  async ([, nextPage], previous) => {
    // Paging keeps the rows it is paging through, dimmed under the loading
    // overlay; a new keyword or filter replaces them, and leaving the previous
    // lane's rows up reads as results for the query just typed.
    const paged = previous !== undefined && nextPage !== previous[1]
    if (!paged) {
      results.value = []
    }
    await load()
    // The paginator sits below a full page of results, so a page opened from it
    // would otherwise start scrolled past its own first row.
    if (paged) {
      top.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  },
  { immediate: true }
)
</script>

<template>
  <div ref="top" class="scroll-mt-40 space-y-6">
    <SearchGalgameFilter
      v-if="type === 'galgame'"
      :total="total"
      :pending="pending"
    />
    <SearchResourceScope
      v-else-if="type === 'resource'"
      :keywords="keywords"
      :total="total"
      :pending="pending"
    />

    <p v-else class="text-default-500 text-sm">
      <template v-if="pending && !results.length">正在搜索…</template>
      <template v-else-if="total">
        共 <span class="text-default-700 tabular-nums">{{ total }}</span>
        {{ meta.countUnit }}
      </template>
    </p>

    <SearchSkeleton
      v-if="pending && !results.length"
      :shape="type === 'galgame' ? 'card' : 'row'"
    />

    <!--
      KunLoading's wrapper is display:contents, which generates no box — so the
      space-y margin lands on nothing and the results sit flush against the
      paginator. The div gives that margin something to be applied to.
    -->
    <div v-else-if="results.length">
      <KunLoading :loading="pending">
        <SearchResult :results="results" :type="type" :keywords="keywords" />
      </KunLoading>
    </div>

    <KunNull v-else-if="failed" description="搜索没能完成, 请稍后重试" />

    <KunNull v-else-if="keywords" description="杂鱼杂鱼杂鱼~什么也没有搜索到" />

    <div v-if="totalPage > 1" class="flex justify-center">
      <KunPagination
        v-model:current-page="page"
        :total-page="totalPage"
        :is-loading="pending"
      />
    </div>
  </div>
</template>
