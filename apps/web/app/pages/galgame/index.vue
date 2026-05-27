<script setup lang="ts">
import {
  ALL_SUPPORTED_TYPE,
  SUPPORTED_TYPE_MAP
} from '~/constants/resource'
import {
  GALGAME_SORT_FIELD_LABEL_MAP,
  GALGAME_SORT_YEARS,
  GALGAME_SORT_YEARS_MAP
} from '~/constants/galgame'

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
// `year` is reserved for a future Wiki-side filter; the patch backend's
// galgameListRequest currently only filters by translation type and sorts.
const selectedYear = ref(String(route.query.year ?? 'all'))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'galgame-list',
  async () => {
    // Query params are snake_case to match apps/api/internal/common/handler.go
    // galgameListRequest.
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

const yearOptions = computed(() =>
  GALGAME_SORT_YEARS.map((y) => ({
    value: y,
    label: GALGAME_SORT_YEARS_MAP[y] ?? y
  }))
)

const updateQuery = async () => {
  await router.replace({
    query: {
      page: page.value,
      type: selectedType.value,
      sort_field: sortField.value,
      sort_order: sortOrder.value,
      year: selectedYear.value
    }
  })
  await refresh()
}

const onChangeType = (v: string | number) => {
  selectedType.value = String(v)
  page.value = 1
  updateQuery()
}
const onChangeSortField = (v: string | number) => {
  sortField.value = String(v)
  page.value = 1
  updateQuery()
}
const onChangeYear = (v: string | number) => {
  selectedYear.value = String(v)
  page.value = 1
  updateQuery()
}
const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
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

    <KunCard :bordered="true">
      <div
        class="flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-end"
      >
        <KunSelect
          label="类型筛选"
          placeholder="选择类型"
          :model-value="selectedType"
          :options="typeOptions"
          class-name="flex-1 min-w-48"
          @update:model-value="onChangeType"
        />
        <KunSelect
          label="发售年份"
          placeholder="选择年份"
          :model-value="selectedYear"
          :options="yearOptions"
          class-name="flex-1 min-w-48"
          @update:model-value="onChangeYear"
        />
        <KunSelect
          label="排序字段"
          :model-value="sortField"
          :options="sortFieldOptions"
          class-name="flex-1 min-w-48"
          @update:model-value="onChangeSortField"
        />
        <KunButton variant="flat" color="default" @click="toggleSortOrder">
          <KunIcon
            :name="
              sortOrder === 'asc' ? 'lucide:arrow-up-az' : 'lucide:arrow-down-az'
            "
            class="size-4"
          />
          {{ sortOrder === 'asc' ? '升序' : '降序' }}
        </KunButton>
      </div>
    </KunCard>

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
