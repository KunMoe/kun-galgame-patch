<script setup lang="ts">
import {
  SEARCH_FILTER_QUERY_KEYS,
  isSearchType,
  searchQueryWithout
} from './items'

const route = useRoute()
const router = useRouter()
const api = useApi()
const searchStore = useSearchStore()

// The URL is the only source of truth for what is being searched: the command
// palette, a shared link and the back button all hand the keyword over that way.
const keywords = computed(() => {
  const value = route.query.q
  return ((Array.isArray(value) ? value[0] : value) ?? '').trim()
})

const currentType = computed<SearchType>(() => {
  const value = route.query.type
  const raw = Array.isArray(value) ? value[0] : value
  return isSearchType(raw) ? raw : 'all'
})

const setKeywords = (value: string) => {
  if (value === keywords.value) {
    return
  }
  // A 资料库 family chosen for the previous keyword is not a filter the reader
  // asked to keep — it would silently narrow the new search to 标签 only.
  const query = searchQueryWithout(route.query, 'family')
  if (value) {
    query.q = value
  } else {
    delete query.q
  }
  router.replace({ query })
}

// A lane's filters are that lane's own: 资料库's family, 补丁资源's match scope
// and Galgame's 高级筛选 all mean nothing to the tab being opened.
const setType = (value: SearchType) => {
  if (value === currentType.value) {
    return
  }
  const query = searchQueryWithout(route.query, ...SEARCH_FILTER_QUERY_KEYS)
  query.type = value
  router.replace({ query })
}

const overview = ref<SearchLanes | null>(null)
// Seeded from the URL rather than false: the immediate watcher below flips it
// synchronously during hydration, so a false here renders a server tree without
// the skeletons the client's first paint has, and every count mismatches.
const overviewPending = ref(!!keywords.value)
const overviewFailed = ref(false)

let latest = 0

// The overview is fetched for every tab, not just 全部: its totals are what the
// category rail counts with, so a deep link straight into 用户 still gets them.
const loadOverview = async (value: string) => {
  const current = ++latest
  if (!value) {
    overview.value = null
    overviewFailed.value = false
    overviewPending.value = false
    return
  }
  overviewPending.value = true
  const res = await api.get<SearchLanes>(
    `/search/overview?keywords=${encodeURIComponent(value)}`
  )
  if (current !== latest) {
    return
  }
  overview.value = res.code === 0 ? res.data : null
  // Without this the content column would just be blank, which reads as "no
  // results" rather than "the request failed".
  overviewFailed.value = res.code !== 0
  overviewPending.value = false
}

// Nothing awaits this on the server, so an SSR fetch is a request whose answer
// is thrown away and then made again on hydration.
watch(
  keywords,
  (value) => {
    if (!import.meta.server) {
      loadOverview(value)
    }
  },
  { immediate: true }
)
</script>

<template>
  <div class="container mx-auto my-4 min-h-[calc(100dvh-16rem)] space-y-6">
    <KunHeader
      name="搜索"
      description="一次搜索整站: Galgame, 资料库中的角色 / 会社 / Staff / 标签 / 系列, 补丁资源, 以及发布它们的用户。"
    >
      <template #endContent>
        <div class="text-default-500 text-sm">
          搜索结果一并包含 NSFW 的 Galgame; 未开启 NSFW 时, 资料库中的成人标签,
          以及 R18 游戏的补丁资源会被隐藏。Galgame 可按会社 / 标签 / 发售年份进一步筛选,
          补丁资源可以只按 AI 模型名匹配。不带关键词浏览请前往
          <KunLink to="/gallib">Galgame 资料库</KunLink>。
        </div>
      </template>
    </KunHeader>

    <!--
      The box follows the reader down the page instead of scrolling away: on a
      results page the next thing they want is almost always a different query.
      Its own background is opaque because the rail slides underneath it.
    -->
    <div
      class="bg-background/90 lg:sticky lg:top-16 lg:z-20 lg:-mx-2 lg:px-2 lg:py-2 lg:backdrop-blur"
    >
      <SearchBox
        :keywords="keywords"
        @submit="setKeywords"
        @remember="searchStore.remember($event)"
      />
    </div>

    <ClientOnly v-if="!keywords">
      <SearchHistory @select="setKeywords" />
    </ClientOnly>

    <div v-else class="gap-8 lg:grid lg:grid-cols-[13rem_minmax(0,1fr)]">
      <div class="mb-6 lg:mb-0">
        <div class="lg:sticky lg:top-[8.75rem]">
          <SearchNav
            :model-value="currentType"
            :totals="overview?.totals ?? null"
            :pending="overviewPending"
            @update:model-value="setType"
          />
        </div>
      </div>

      <div class="min-w-0">
        <SearchOverview
          v-if="currentType === 'all'"
          :keywords="keywords"
          :overview="overview"
          :pending="overviewPending"
          :failed="overviewFailed"
          @open="setType"
        />

        <SearchEntities
          v-else-if="currentType === 'entity'"
          :keywords="keywords"
        />

        <SearchComments
          v-else-if="currentType === 'comment'"
          :keywords="keywords"
        />

        <SearchList
          v-else
          :key="currentType"
          :keywords="keywords"
          :type="currentType as SearchPagedType"
        />
      </div>
    </div>
  </div>
</template>
