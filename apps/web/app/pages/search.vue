<script setup lang="ts">
import { useDebounceFn } from '@vueuse/core'

// The /search endpoint is intentionally exempt from the global content_limit
// gate (returns sfw + nsfw together — it's a user-initiated action), which
// means a crawler hitting /search?q=<nsfw-term> WOULD see NSFW result names
// in the SSR HTML. Disable SEO on this surface so:
//   1. /search?q=... URLs don't get indexed at all (avoids polluting search
//      results with our internal search pages)
//   2. crawlers ignore the dynamic NSFW-bearing payload regardless of query
useKunDisableSeo('搜索')

const route = useRoute()
const router = useRouter()
const api = useApi()

// Two search modes:
//   - 'galgame' : full-text Galgame search (Meilisearch via POST /search)
//   - 'model'   : find patch resources by AI-translation model name
//                 (GET /resource?model=…); results link to /resource/:id
type SearchMode = 'galgame' | 'model'
const mode = ref<SearchMode>(
  route.query.mode === 'model' ? 'model' : 'galgame'
)

const query = ref(String(route.query.q ?? ''))
const page = ref(Number(route.query.page ?? 1))
const limit = 24

// `include_intro` is the only search-scope toggle the wiki-delegated /search
// endpoint actually supports (D11). Alias/tag are always searchable in
// Meilisearch's index, so the old per-scope checkboxes are gone.
const searchInIntroduction = ref(false)

const results = ref<GalgameCard[]>([])
const resourceResults = ref<PatchResource[]>([])
const total = ref(0)
const loading = ref(false)
const hasSearched = ref(false)

// Backend /search delegates to Wiki and returns SearchHit items: the flat
// GalgameHit fields (name_zh_cn, ...) + has_patch + optional local patch row.
// GalgameCard.vue expects the enriched shape (name: KunLanguage, count, ...),
// so map every hit into that shape with safe zero defaults.
interface SearchHit {
  id: number
  vndb_id: string
  name_en_us: string
  name_ja_jp: string
  name_zh_cn: string
  name_zh_tw: string
  banner: string
  content_limit: string
  has_patch: boolean
  patch?: {
    id: number
    view?: number
    download?: number
    created?: string
  } | null
}

const mapHit = (h: SearchHit): GalgameCard =>
  ({
    id: h.id,
    vndb_id: h.vndb_id,
    bid: null,
    name: {
      'en-us': h.name_en_us ?? '',
      'ja-jp': h.name_ja_jp ?? '',
      'zh-cn': h.name_zh_cn ?? '',
      'zh-tw': h.name_zh_tw ?? ''
    },
    banner: h.banner ?? '',
    view: h.patch?.view ?? 0,
    download: h.patch?.download ?? 0,
    type: [],
    language: [],
    platform: [],
    content_limit: (h.content_limit as KunContentLimit) || 'sfw',
    status: 0,
    created: h.patch?.created ?? new Date().toISOString(),
    resource_update_time: h.patch?.created ?? new Date().toISOString(),
    count: { favorite_by: 0, contribute_by: 0, resource: 0, comment: 0 }
  }) as GalgameCard

const resetResults = () => {
  results.value = []
  resourceResults.value = []
  total.value = 0
  hasSearched.value = false
}

const searchModel = async (q: string) => {
  const params = new URLSearchParams({
    model: q,
    sort_field: 'created',
    sort_order: 'desc',
    page: String(page.value),
    limit: String(limit)
  })
  const res = await api.get<{ items: PatchResource[]; total: number }>(
    `/resource?${params.toString()}`
  )
  if (res.code === 0) {
    resourceResults.value = res.data?.items ?? []
    total.value = res.data?.total ?? 0
  } else {
    resourceResults.value = []
    total.value = 0
    useKunMessage(res.message || '搜索失败', 'error')
  }
  router.replace({ query: { q: query.value, page: page.value, mode: 'model' } })
}

const searchGalgame = async (q: string) => {
  // Wire shape matches backend SearchRequest: `q` string, flat filters,
  // required page/limit. Response is response.Paginated → data.{items,total}.
  const res = await api.post<{ items: SearchHit[]; total: number }>('/search', {
    q,
    page: page.value,
    limit,
    include_intro: searchInIntroduction.value
  })
  if (res.code === 0) {
    results.value = (res.data?.items ?? []).map(mapHit)
    total.value = res.data?.total ?? 0
  } else {
    results.value = []
    total.value = 0
    useKunMessage(res.message || '搜索失败', 'error')
  }
  router.replace({ query: { q: query.value, page: page.value } })
}

const doSearch = async () => {
  const q = query.value.trim()
  if (!q) {
    resetResults()
    return
  }
  loading.value = true
  try {
    if (mode.value === 'model') {
      await searchModel(q)
    } else {
      await searchGalgame(q)
    }
    hasSearched.value = true
  } finally {
    loading.value = false
  }
}

const debouncedSearch = useDebounceFn(() => {
  page.value = 1
  doSearch()
}, 500)

watch([query, searchInIntroduction], () => {
  debouncedSearch()
})
// Mode switches re-search immediately (a deliberate action, not typing).
watch(mode, () => {
  page.value = 1
  resetResults()
  doSearch()
})

onMounted(() => {
  if (query.value) doSearch()
})

const totalPages = computed(() => Math.ceil(total.value / limit))
const onChangePage = (v: number) => {
  page.value = v
  doSearch()
  if (import.meta.client) window.scrollTo({ top: 0 })
}
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader
      name="搜索"
      description="搜索本站的 Galgame 补丁，或按模型搜索补丁资源"
    />

    <!-- mode toggle -->
    <div class="flex flex-wrap gap-2">
      <KunButton
        :variant="mode === 'galgame' ? 'flat' : 'light'"
        :color="mode === 'galgame' ? 'primary' : 'default'"
        rounded="full"
        @click="mode = 'galgame'"
      >
        <KunIcon name="lucide:gamepad-2" class="size-4" />
        搜索 Galgame
      </KunButton>
      <KunButton
        :variant="mode === 'model' ? 'flat' : 'light'"
        :color="mode === 'model' ? 'primary' : 'default'"
        rounded="full"
        @click="mode = 'model'"
      >
        <KunIcon name="lucide:bot" class="size-4" />
        按模型搜索资源
      </KunButton>
    </div>

    <KunInput
      v-model="query"
      :placeholder="
        mode === 'model'
          ? '输入模型名搜索补丁资源，例如 claude-opus-4.7'
          : '输入关键词搜索...'
      "
      size="lg"
      autofocus
    >
      <template #prefix>
        <KunIcon name="lucide:search" class="text-default-400 size-5" />
      </template>
    </KunInput>

    <div v-if="mode === 'galgame'" class="flex flex-wrap gap-4">
      <KunCheckBox v-model="searchInIntroduction" label="搜索简介内容" />
    </div>

    <KunLoading v-if="loading" description="正在搜索..." />

    <!-- Galgame results -->
    <div
      v-else-if="mode === 'galgame' && results.length"
      class="grid grid-cols-2 gap-2 sm:gap-6 lg:grid-cols-3 xl:grid-cols-4"
    >
      <GalgameCard v-for="p in results" :key="p.id" :patch="p" />
    </div>

    <!-- Model (resource) results — link to /resource/:id via ResourceCard -->
    <div
      v-else-if="mode === 'model' && resourceResults.length"
      class="grid grid-cols-1 gap-3 sm:gap-6 md:grid-cols-2"
    >
      <ResourceCard v-for="r in resourceResults" :key="r.id" :resource="r" />
    </div>

    <KunNull
      v-else-if="hasSearched"
      :description="
        mode === 'model'
          ? '没有找到使用该模型的补丁资源'
          : '没有找到匹配的 Galgame'
      "
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="loading"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
