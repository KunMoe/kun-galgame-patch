<script setup lang="ts">
import {
  SEARCH_GALGAME_FILTER_KEYS,
  SEARCH_GALGAME_SORTS,
  SEARCH_GALGAME_TAG_MAX,
  SEARCH_GALGAME_YEAR_MIN,
  readSearchGalgameFilter,
  searchGalgameFilterQuery,
  searchQueryWithout
} from './items'

defineProps<{ total: number; pending: boolean }>()

const route = useRoute()
const router = useRouter()
const entityNames = useEntityNames()

const filter = computed(() => readSearchGalgameFilter(route.query))

const apply = (next: Partial<SearchGalgameFilter>) => {
  const merged = { ...filter.value, ...next }
  const kept = searchQueryWithout(route.query, ...SEARCH_GALGAME_FILTER_KEYS)
  router.replace({ query: { ...kept, ...searchGalgameFilterQuery(merged) } })
}

const toggleTag = (item: SearchEntityItem) => {
  entityNames.remember(item)
  const ids = filter.value.tag_ids
  apply({
    tag_ids: ids.includes(item.id)
      ? ids.filter((id) => id !== item.id)
      : [...ids, item.id].slice(0, SEARCH_GALGAME_TAG_MAX)
  })
}

const toggleCompany = (item: SearchEntityItem) => {
  entityNames.remember(item)
  apply({ company_id: filter.value.company_id === item.id ? 0 : item.id })
}

const yearRangeLabel = computed(() => {
  const { released_from: from, released_to: to } = filter.value
  if (from && to) {
    return from === to ? `${from} 年` : `${from} - ${to}`
  }
  return from ? `${from} 年至今` : `${to} 年以前`
})

const chips = computed<FilterChip[]>(() => {
  const list: FilterChip[] = []
  if (filter.value.company_id) {
    list.push({
      key: 'company',
      prefix: '会社',
      label: entityNames.labelOf('company', filter.value.company_id)
    })
  }
  for (const id of filter.value.tag_ids) {
    list.push({
      key: `tag:${id}`,
      prefix: '标签',
      label: entityNames.labelOf('tag', id)
    })
  }
  if (filter.value.released_from || filter.value.released_to) {
    list.push({ key: 'years', label: yearRangeLabel.value })
  }
  return list
})

const removeChip = (key: string) => {
  const [dimension, value] = key.split(':')
  if (dimension === 'company') {
    apply({ company_id: 0 })
  } else if (dimension === 'tag') {
    apply({
      tag_ids: filter.value.tag_ids.filter((id) => id !== Number(value))
    })
  } else {
    apply({ released_from: '', released_to: '' })
  }
}

const clearFilters = () =>
  apply({
    tag_ids: [],
    company_id: 0,
    released_from: '',
    released_to: ''
  })

watch(
  filter,
  () =>
    entityNames.resolve({
      company: [filter.value.company_id],
      tag: filter.value.tag_ids
    }),
  { immediate: true }
)
</script>

<template>
  <FilterBar
    :chips="chips"
    :total="total"
    :pending="pending"
    unit="个 Galgame"
    @remove="removeChip"
    @clear="clearFilters"
  >
    <KunTooltip
      text="鲲 Galgame 论坛数据的索引里没有评分, 按评分排序需要 infra 先提供该字段。"
      position="bottom"
    >
      <FilterMenu
        icon="lucide:arrow-down-up"
        label="排序"
        :options="SEARCH_GALGAME_SORTS"
        :model-value="filter.sort"
        empty-value="relevance"
        @update:model-value="apply({ sort: $event as string })"
      />
    </KunTooltip>

    <FilterEntityMenu
      family="company"
      icon="lucide:building-2"
      label="会社"
      placeholder="搜索会社名, 例如 Key"
      :selected-ids="filter.company_id ? [filter.company_id] : []"
      :selected-items="entityNames.itemsOf('company', [filter.company_id])"
      @toggle="toggleCompany"
    />

    <FilterEntityMenu
      family="tag"
      icon="lucide:tag"
      label="标签"
      placeholder="搜索标签, 例如 校园"
      :selected-ids="filter.tag_ids"
      :selected-items="entityNames.itemsOf('tag', filter.tag_ids)"
      multiple
      :max="SEARCH_GALGAME_TAG_MAX"
      @toggle="toggleTag"
    />

    <FilterYears
      :from="filter.released_from"
      :to="filter.released_to"
      :min="SEARCH_GALGAME_YEAR_MIN"
      @update="apply({ released_from: $event.from, released_to: $event.to })"
    />
  </FilterBar>
</template>
