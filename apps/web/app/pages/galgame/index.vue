<script setup lang="ts">
import {
  ALL_SUPPORTED_TYPE,
  SUPPORTED_TYPE_MAP
} from '~/constants/resource'
import { GALGAME_SORT_FIELD_LABEL_MAP } from '~/constants/galgame'

const route = useRoute()
const router = useRouter()
const api = useApi()

// SFW-by-default per useApi resolution — listing only contains sfw rows
// for anonymous crawlers, so a rich keyword-laden description is safe.
useKunSeoMeta({
  title: 'Galgame 列表',
  description:
    '鲲 Galgame 补丁站收录的全部 Galgame 列表，按发布时间、资源更新时间、浏览量、下载量排序，支持按平台 / 语言 / 翻译类型筛选，免费下载 Windows / 安卓 / KRKR / Tyranor 等平台的 Galgame 中文汉化补丁。'
})

const page = ref(Number(route.query.page ?? 1))
const selectedType = ref(String(route.query.type ?? 'all'))
const sortField = ref(String(route.query.sort_field ?? 'resource_update_time'))
const sortOrder = ref(String(route.query.sort_order ?? 'desc'))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'galgame-list',
  async () => {
    // Query params are snake_case to match apps/api/internal/common/handler.go
    // galgameListRequest. Year filtering would live here too, but the backend
    // (galgameListRequest) doesn't currently support it — wiki-side year
    // filter is on the search endpoint only. If/when /api/galgame grows year
    // support, add the same chip row pattern below.
    const params = new URLSearchParams({
      selected_type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      page: String(page.value),
      limit: String(limit)
    })
    const res = await api.get<ListResponse>(`/galgame?${params.toString()}`)
    if (res.code !== 0) return { galgames: [], total: 0 }
    return res.data
  },
  { default: () => ({ galgames: [], total: 0 }) }
)

const typeOptions = computed(() =>
  ALL_SUPPORTED_TYPE.map((t) => ({
    value: t,
    label: SUPPORTED_TYPE_MAP[t] ?? t
  }))
)

const sortFieldOptions = computed(() =>
  Object.entries(GALGAME_SORT_FIELD_LABEL_MAP).map(([value, label]) => ({
    value,
    label
  }))
)

const updateQuery = async () => {
  await router.replace({
    query: {
      page: page.value,
      type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value
    }
  })
  await refresh()
}

const setType = (v: string) => {
  if (selectedType.value === v) return
  selectedType.value = v
  page.value = 1
  updateQuery()
}
const setSortField = (v: string) => {
  if (sortField.value === v) return
  sortField.value = v
  page.value = 1
  updateQuery()
}
const setSortOrder = (v: 'asc' | 'desc') => {
  if (sortOrder.value === v) return
  sortOrder.value = v
  page.value = 1
  updateQuery()
}
const onChangePage = (v: number) => {
  page.value = v
  updateQuery()
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader
      name="Galgame"
      description="本页面默认仅显示了 SFW (内容安全) 的补丁, 您可以在网站右上角切换显示全部补丁 (包括 NSFW, 也就是显示可能带有涩涩的补丁)"
    />

    <!-- Filter chip rows — modelled on kungal's GalgameCardNav. Each
         dimension is one horizontally-scrollable row of buttons, active
         option lit primary, inactive muted. Much faster than dropdown
         selects (one click vs two), and visually matches kungal so a user
         hopping between the two sites doesn't have to relearn the
         filter language. overflow-x-auto + flex-nowrap handles cases
         where translation-type options exceed the row width on mobile;
         no extra KunScrollShadow component needed. -->
    <div class="space-y-1.5">
      <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
        <button
          v-for="opt in typeOptions"
          :key="opt.value"
          type="button"
          :class="[
            'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
            selectedType === opt.value
              ? 'bg-primary/15 text-primary font-medium'
              : 'text-default-600 hover:bg-default-100'
          ]"
          @click="setType(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <div class="-mx-1 flex gap-1 overflow-x-auto px-1 pb-0.5">
        <button
          v-for="opt in sortFieldOptions"
          :key="opt.value"
          type="button"
          :class="[
            'shrink-0 cursor-pointer rounded-md px-2.5 py-1 text-sm whitespace-nowrap transition-colors',
            sortField === opt.value
              ? 'bg-primary/15 text-primary font-medium'
              : 'text-default-600 hover:bg-default-100'
          ]"
          @click="setSortField(opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <!-- Sort direction = explicit pair (not toggle) so the user always
           sees what the alternative would be. Matches kungal's icon pair. -->
      <div class="flex items-center gap-1.5">
        <button
          type="button"
          aria-label="降序"
          :class="[
            'cursor-pointer rounded-md p-1 transition-colors',
            sortOrder === 'desc'
              ? 'bg-primary/15 text-primary'
              : 'text-default-500 hover:bg-default-100'
          ]"
          @click="setSortOrder('desc')"
        >
          <KunIcon name="lucide:arrow-down" class="size-4" />
        </button>
        <button
          type="button"
          aria-label="升序"
          :class="[
            'cursor-pointer rounded-md p-1 transition-colors',
            sortOrder === 'asc'
              ? 'bg-primary/15 text-primary'
              : 'text-default-500 hover:bg-default-100'
          ]"
          @click="setSortOrder('asc')"
        >
          <KunIcon name="lucide:arrow-up" class="size-4" />
        </button>
      </div>
    </div>

    <KunLoading v-if="pending" description="正在获取 Galgame 数据..." />
    <div
      v-else
      class="mx-auto mb-8 grid grid-cols-2 gap-2 sm:gap-6 lg:grid-cols-3 xl:grid-cols-4"
    >
      <GalgameCard
        v-for="patch in data?.galgames"
        :key="patch.id"
        :patch="patch"
      />
    </div>

    <KunNull
      v-if="!pending && !data?.galgames?.length"
      description="暂无数据"
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
