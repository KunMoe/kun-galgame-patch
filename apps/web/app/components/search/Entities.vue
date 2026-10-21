<script setup lang="ts">
import {
  SEARCH_ENTITY_FAMILIES,
  SEARCH_ENTITY_LIMIT,
  isSearchEntityFamily,
  searchQueryWithout
} from './items'

const props = defineProps<{
  keywords: string
}>()

const route = useRoute()
const router = useRouter()
const api = useApi()

const family = computed<SearchEntityFamily | 'all'>(() => {
  const value = route.query.family
  const raw = Array.isArray(value) ? value[0] : value
  return isSearchEntityFamily(raw) ? raw : 'all'
})

const setFamily = (value: SearchEntityFamily | 'all') => {
  if (value === family.value) {
    return
  }
  const query = searchQueryWithout(route.query, 'family')
  if (value !== 'all') {
    query.family = value
  }
  router.replace({ query })
}

const familyItems = computed(() => [
  { value: 'all', textValue: '全部', icon: 'lucide:layout-grid' },
  ...SEARCH_ENTITY_FAMILIES
])

const groups = ref<SearchEntityGroup[]>([])
const pending = ref(!!props.keywords)
const failed = ref(false)
const top = useTemplateRef<HTMLElement>('top')

// The reader's page belongs in the URL beside the keyword and the family: while
// it lived in a local ref, opening an entry from page 5 and coming back landed
// on page 1, and a shared link never carried the page it was shared from.
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (value) =>
    router.replace({ query: { ...route.query, page: String(value) } })
})

const isAll = computed(() => family.value === 'all')
const limit = computed(() =>
  isAll.value ? SEARCH_ENTITY_LIMIT.all : SEARCH_ENTITY_LIMIT.one
)

// Watching the request's own query string rather than the values behind it:
// every one of them is read off route.query, which hands back a fresh object on
// any navigation, so watching those would refetch when nothing asked had changed.
const query = computed(() => {
  const params = new URLSearchParams({
    keywords: props.keywords,
    page: String(page.value),
    limit: String(limit.value)
  })
  if (!isAll.value) {
    params.set('family', family.value)
  }
  return params.toString()
})

let latest = 0

const load = async () => {
  const current = ++latest
  if (!props.keywords) {
    groups.value = []
    pending.value = false
    return
  }
  pending.value = true
  const res = await api.get<SearchEntityResult>(`/search/entity?${query.value}`)
  if (current !== latest) {
    return
  }
  failed.value = res.code !== 0
  groups.value = res.data?.groups ?? []
  pending.value = false
}

watch(
  [query, page],
  async ([, nextPage], previous) => {
    // Paging keeps the rows it is paging through, dimmed under the loading
    // overlay; a new keyword or family replaces them, and leaving the previous
    // family's entries up reads as results for the query just typed.
    const paged = previous !== undefined && nextPage !== previous[1]
    if (!paged) {
      groups.value = []
    }
    await load()
    if (paged) {
      top.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  },
  { immediate: true }
)

const isEmpty = computed(
  () =>
    !pending.value &&
    !failed.value &&
    groups.value.every((group) => !group.items.length)
)

// One family per request, so the single-family tab is the only one with a page
// count to divide: 全部 is a preview of every family at once.
const totalPage = computed(() =>
  isAll.value
    ? 0
    : Math.ceil((groups.value[0]?.total ?? 0) / SEARCH_ENTITY_LIMIT.one)
)
</script>

<template>
  <div ref="top" class="scroll-mt-40 space-y-5">
    <KunTab
      :items="familyItems"
      :model-value="family"
      variant="light"
      size="sm"
      name="search-entity-family"
      @update:model-value="
        (value) => setFamily(value as SearchEntityFamily | 'all')
      "
    />

    <SearchSkeleton v-if="pending && !groups.length" shape="entity" />

    <template v-else>
      <!--
        KunLoading's wrapper is display:contents, which generates no box — so
        the space-y margin lands on nothing and the last group sits flush
        against the paginator.
      -->
      <div>
        <KunLoading :loading="pending">
          <div class="space-y-5">
            <SearchEntityGroup
              v-for="group in groups"
              :key="group.family"
              :group="group"
              :keywords="keywords"
              :show-header="isAll"
              :show-cap="isAll"
              @open="setFamily"
            />
          </div>
        </KunLoading>
      </div>

      <KunNull
        v-if="failed"
        description="鲲 Galgame 论坛数据搜索没能完成, 请稍后重试"
      />
      <KunNull
        v-else-if="isEmpty"
        description="鲲 Galgame 论坛数据里没有找到匹配的条目"
      />

      <div v-if="totalPage > 1" class="flex justify-center">
        <KunPagination
          v-model:current-page="page"
          :total-page="totalPage"
          :is-loading="pending"
        />
      </div>
    </template>
  </div>
</template>
