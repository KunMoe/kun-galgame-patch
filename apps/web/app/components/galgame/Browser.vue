<script setup lang="ts">
import {
  ALL_SUPPORTED_TYPE,
  SUPPORTED_LANGUAGE,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM,
  SUPPORTED_PLATFORM_MAP,
  SUPPORTED_TYPE_MAP
} from '~/constants/resource'
import {
  GALGAME_LIBRARY_SORT_FIELD_LABEL_MAP,
  GALGAME_SORT_FIELD_LABEL_MAP
} from '~/constants/galgame'

const props = defineProps<{ mode: 'resource' | 'library' }>()

const isLibrary = computed(() => props.mode === 'library')
const defaultSortField = isLibrary.value ? 'popularity' : 'resource_update_time'

// Catalog answers 400 past ten tag ids.
const TAG_MAX = 10

const route = useRoute()
const router = useRouter()
const api = useApi()
const settingStore = useSettingStore()
const entityNames = useEntityNames()

const readList = (value: unknown): string[] =>
  String(value ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)

const readIds = (value: unknown): number[] =>
  readList(value)
    .map((id) => Number(id))
    .filter((id) => Number.isInteger(id) && id > 0)

const page = ref(Number(route.query.page ?? 1))
const pageHref = usePageHref()
const selectedType = ref(String(route.query.type ?? 'all'))
const sortField = ref(String(route.query.sort_field ?? defaultSortField))
const sortOrder = ref(String(route.query.sort_order ?? 'desc'))
const releasedFrom = ref(String(route.query.released_from ?? ''))
const releasedTo = ref(String(route.query.released_to ?? ''))
const selectedMonths = ref(
  readIds(route.query.released_months).filter((m) => m >= 1 && m <= 12)
)
const languages = ref(
  readList(route.query.language).filter((v) => SUPPORTED_LANGUAGE.includes(v))
)
const platforms = ref(
  readList(route.query.platform).filter((v) => SUPPORTED_PLATFORM.includes(v))
)
const companyId = ref(Number(route.query.company_id) || 0)
const tagIds = ref(readIds(route.query.tag_ids).slice(0, TAG_MAX))

const limit = 24

interface ListResponse {
  galgames: GalgameCard[]
  total: number
}

const buildQuery = (): Record<string, string> => {
  const query: Record<string, string> = {
    page: String(page.value),
    sort_field: sortField.value,
    sort_order: sortOrder.value
  }
  if (releasedFrom.value) {
    query.released_from = releasedFrom.value
  }
  if (releasedTo.value) {
    query.released_to = releasedTo.value
  }
  if (isLibrary.value) {
    if (companyId.value) {
      query.company_id = String(companyId.value)
    }
    if (tagIds.value.length) {
      query.tag_ids = tagIds.value.join(',')
    }
    return query
  }
  query.type = selectedType.value
  if (languages.value.length) {
    query.language = languages.value.join(',')
  }
  if (platforms.value.length) {
    query.platform = platforms.value.join(',')
  }
  if (selectedMonths.value.length) {
    query.released_months = [...selectedMonths.value]
      .sort((a, b) => a - b)
      .join(',')
  }
  return query
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  `galgame-list-${props.mode}`,
  async () => {
    const params = new URLSearchParams({
      ...buildQuery(),
      selected_type: isLibrary.value ? 'all' : selectedType.value,
      limit: String(limit)
    })
    params.delete('type')
    if (isLibrary.value) {
      params.set('library', 'true')
    }

    const res = await api.get<ListResponse>(`/galgame?${params.toString()}`)
    if (res.code !== 0) {
      return { galgames: [], total: 0 }
    }
    return res.data
  },
  { default: () => ({ galgames: [], total: 0 }) }
)

const typeOptions = computed<FilterOption[]>(() =>
  ALL_SUPPORTED_TYPE.map((value) => ({
    value,
    label: SUPPORTED_TYPE_MAP[value] ?? value
  }))
)

const languageOptions: FilterOption[] = SUPPORTED_LANGUAGE.map((value) => ({
  value,
  label: SUPPORTED_LANGUAGE_MAP[value] ?? value
}))

const platformOptions: FilterOption[] = SUPPORTED_PLATFORM.map((value) => ({
  value,
  label: SUPPORTED_PLATFORM_MAP[value] ?? value
}))

const monthOptions: FilterOption[] = Array.from({ length: 12 }, (_, i) => ({
  value: String(i + 1),
  label: `${i + 1} 月`
}))

const sortFieldOptions = computed<FilterOption[]>(() =>
  Object.entries(
    isLibrary.value
      ? GALGAME_LIBRARY_SORT_FIELD_LABEL_MAP
      : GALGAME_SORT_FIELD_LABEL_MAP
  ).map(([value, label]) => ({ value, label }))
)

const yearRangeLabel = computed(() => {
  if (releasedFrom.value && releasedTo.value) {
    return releasedFrom.value === releasedTo.value
      ? `${releasedFrom.value} 年`
      : `${releasedFrom.value} - ${releasedTo.value}`
  }
  return releasedFrom.value
    ? `${releasedFrom.value} 年至今`
    : `${releasedTo.value} 年以前`
})

const chips = computed<FilterChip[]>(() => {
  const list: FilterChip[] = []
  if (!isLibrary.value && selectedType.value !== 'all') {
    list.push({
      key: 'type',
      label: SUPPORTED_TYPE_MAP[selectedType.value] ?? selectedType.value
    })
  }
  for (const value of languages.value) {
    list.push({
      key: `language:${value}`,
      prefix: '语言',
      label: SUPPORTED_LANGUAGE_MAP[value] ?? value
    })
  }
  for (const value of platforms.value) {
    list.push({
      key: `platform:${value}`,
      prefix: '平台',
      label: SUPPORTED_PLATFORM_MAP[value] ?? value
    })
  }
  if (companyId.value) {
    list.push({
      key: 'company',
      prefix: '会社',
      label: entityNames.labelOf('company', companyId.value)
    })
  }
  for (const id of tagIds.value) {
    list.push({
      key: `tag:${id}`,
      prefix: '标签',
      label: entityNames.labelOf('tag', id)
    })
  }
  if (releasedFrom.value || releasedTo.value) {
    list.push({ key: 'years', label: yearRangeLabel.value })
  }
  for (const month of [...selectedMonths.value].sort((a, b) => a - b)) {
    list.push({ key: `month:${month}`, label: `${month} 月` })
  }
  return list
})

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const updateQuery = async () => {
  await router.replace({ query: buildQuery() })
  await refresh()
}

const apply = (mutate: () => void) => {
  mutate()
  page.value = 1
  updateQuery()
}

const setYears = (range: { from: string; to: string }) =>
  apply(() => {
    releasedFrom.value = range.from
    releasedTo.value = range.to
  })

const toggleCompany = (item: SearchEntityItem) => {
  entityNames.remember(item)
  apply(() => {
    companyId.value = companyId.value === item.id ? 0 : item.id
  })
}

const toggleTag = (item: SearchEntityItem) => {
  entityNames.remember(item)
  apply(() => {
    tagIds.value = tagIds.value.includes(item.id)
      ? tagIds.value.filter((id) => id !== item.id)
      : [...tagIds.value, item.id].slice(0, TAG_MAX)
  })
}

const removeChip = (key: string) => {
  const [dimension, value] = key.split(':')
  apply(() => {
    if (dimension === 'type') {
      selectedType.value = 'all'
    } else if (dimension === 'language') {
      languages.value = languages.value.filter((item) => item !== value)
    } else if (dimension === 'platform') {
      platforms.value = platforms.value.filter((item) => item !== value)
    } else if (dimension === 'company') {
      companyId.value = 0
    } else if (dimension === 'tag') {
      tagIds.value = tagIds.value.filter((id) => id !== Number(value))
    } else if (dimension === 'years') {
      releasedFrom.value = ''
      releasedTo.value = ''
    } else if (dimension === 'month') {
      selectedMonths.value = selectedMonths.value.filter(
        (month) => month !== Number(value)
      )
    }
  })
}

const clearFilters = () =>
  apply(() => {
    selectedType.value = 'all'
    languages.value = []
    platforms.value = []
    companyId.value = 0
    tagIds.value = []
    releasedFrom.value = ''
    releasedTo.value = ''
    selectedMonths.value = []
  })

const onChangePage = (value: number) => {
  page.value = value
  updateQuery()
  if (import.meta.client) {
    window.scrollTo({ top: 0 })
  }
}

watch(
  () => settingStore.data.showGalgamesWithoutResource,
  () => apply(() => {})
)

watch(
  [companyId, tagIds],
  () => entityNames.resolve({ company: [companyId.value], tag: tagIds.value }),
  { immediate: true }
)
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader :name="isLibrary ? 'Galgame 信息资料库' : 'Galgame 补丁资源库'">
      <template #description>
        <p class="text-default-500">
          <template v-if="isLibrary">
            资料库收录的全部 Galgame, 无论本站是否有补丁资源。想找下载请前往
            <KunLink to="/galgame">Galgame 补丁资源库</KunLink>。
          </template>
          <template v-else>
            本站已经有补丁资源的 Galgame。想浏览本站暂无资源的作品资料请前往
            <KunLink to="/gallib">Galgame 信息资料库</KunLink>。
          </template>
          本页面默认仅显示 SFW (内容安全) 的内容, 您可以在
          <KunLink to="/settings/system">系统设置</KunLink> 中切换显示全部 (包括
          NSFW)。
        </p>
      </template>
    </KunHeader>

    <FilterBar
      :chips="chips"
      :total="data?.total ?? 0"
      :pending="pending"
      unit="部"
      @remove="removeChip"
      @clear="clearFilters"
    >
      <FilterMenu
        icon="lucide:arrow-down-up"
        label="排序"
        :options="sortFieldOptions"
        :model-value="sortField"
        :empty-value="defaultSortField"
        @update:model-value="apply(() => (sortField = $event as string))"
      />

      <KunTooltip
        :text="sortOrder === 'desc' ? '当前降序' : '当前升序'"
        position="bottom"
      >
        <button
          type="button"
          aria-label="切换排序方向"
          :class="filterPillSquareClass(false)"
          @click="
            apply(() => (sortOrder = sortOrder === 'desc' ? 'asc' : 'desc'))
          "
        >
          <KunIcon
            :name="
              sortOrder === 'desc' ? 'lucide:arrow-down' : 'lucide:arrow-up'
            "
            class="size-4 text-inherit"
          />
        </button>
      </KunTooltip>

      <span class="bg-default-200 h-6 w-px" aria-hidden="true" />

      <template v-if="isLibrary">
        <FilterEntityMenu
          family="company"
          icon="lucide:building-2"
          label="会社"
          placeholder="搜索会社名, 例如 Key"
          :selected-ids="companyId ? [companyId] : []"
          :selected-items="entityNames.itemsOf('company', [companyId])"
          @toggle="toggleCompany"
        />
        <FilterEntityMenu
          family="tag"
          icon="lucide:tag"
          label="标签"
          placeholder="搜索标签, 例如 校园"
          :selected-ids="tagIds"
          :selected-items="entityNames.itemsOf('tag', tagIds)"
          multiple
          :max="TAG_MAX"
          @toggle="toggleTag"
        />
      </template>

      <template v-else>
        <FilterMenu
          icon="lucide:puzzle"
          label="补丁类型"
          :options="typeOptions"
          :model-value="selectedType"
          empty-value="all"
          @update:model-value="apply(() => (selectedType = $event as string))"
        />
        <FilterMenu
          icon="lucide:languages"
          label="语言"
          multiple
          :options="languageOptions"
          :model-value="languages"
          @update:model-value="apply(() => (languages = $event as string[]))"
        />
        <FilterMenu
          icon="lucide:monitor-smartphone"
          label="平台"
          multiple
          :options="platformOptions"
          :model-value="platforms"
          @update:model-value="apply(() => (platforms = $event as string[]))"
        />
      </template>

      <FilterYears
        :from="releasedFrom"
        :to="releasedTo"
        @update="setYears"
      />

      <FilterMenu
        v-if="!isLibrary"
        icon="lucide:calendar-days"
        label="发售月份"
        multiple
        :columns="3"
        :options="monthOptions"
        :model-value="selectedMonths.map(String)"
        @update:model-value="
          apply(() => (selectedMonths = ($event as string[]).map(Number)))
        "
      />

      <template #end>
        <GalgameDisplaySettings />
      </template>
    </FilterBar>

    <KunLoading v-if="pending" description="正在获取 Galgame 数据..." />
    <GalgameList v-else :items="data?.galgames ?? []" class="mb-8" />

    <KunNull
      v-if="!pending && !data?.galgames?.length"
      description="暂无数据"
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
