<script setup lang="ts">
import { SEARCH_CATEGORIES } from './items'

const props = defineProps<{
  modelValue: SearchType
  totals: SearchTotals | null
  pending: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: SearchType]
}>()

// Partial, because 评论 has no count to show: its upstream face answers a
// keyset cursor and no total. An absent number renders as no number at all
// rather than as a zero.
const counts = computed<Partial<Record<SearchType, number>>>(() => {
  const totals = props.totals
  if (!totals) {
    return {}
  }
  const sum = Object.values(totals).reduce((acc, value) => acc + value, 0)
  return { ...totals, all: sum }
})

const countOf = (value: SearchType) => counts.value[value]

const handleSelect = (value: string) =>
  emit('update:modelValue', value as SearchType)
</script>

<template>
  <nav aria-label="搜索分类">
    <!-- The rail is rendered twice, one copy per breakpoint. KunTab derives its
         tab/panel ids from `name`, so both copies need their own or the
         document carries a duplicate id for every category. -->
    <div class="hidden lg:block">
      <KunTab
        :items="SEARCH_CATEGORIES"
        :model-value="modelValue"
        orientation="vertical"
        variant="underlined"
        align="start"
        :full-width="true"
        name="search-category"
        @update:model-value="handleSelect"
      >
        <template #tab="{ item }">
          <span class="flex w-full items-center gap-2">
            <KunIcon :name="item.icon" class="size-4 shrink-0" />
            <span class="truncate">{{ item.textValue }}</span>
            <SearchNavCount
              :value="countOf(item.value as SearchType)"
              :pending="pending"
            />
          </span>
        </template>
      </KunTab>
    </div>

    <div class="lg:hidden">
      <KunTab
        :items="SEARCH_CATEGORIES"
        :model-value="modelValue"
        variant="light"
        size="sm"
        name="search-category-narrow"
        @update:model-value="handleSelect"
      >
        <template #tab="{ item }">
          <span class="flex items-center gap-1.5">
            <KunIcon :name="item.icon" class="size-4 shrink-0" />
            <span>{{ item.textValue }}</span>
            <SearchNavCount
              :value="countOf(item.value as SearchType)"
              :pending="pending"
              :inline="true"
            />
          </span>
        </template>
      </KunTab>
    </div>
  </nav>
</template>
