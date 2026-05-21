<script setup lang="ts">
import { ADMIN_STATS_SUM_MAP, ADMIN_STATS_MAP } from '~/constants/admin'

const api = useApi()

const emptySum: SumData = {
  user_count: 0,
  galgame_count: 0,
  resource_count: 0,
  comment_count: 0
}

const { data: sum } = await useAsyncData<SumData>(
  'admin-stats-sum',
  async () => {
    const res = await api.get<SumData>('/admin/stats/sum')
    return res.code === 0 ? res.data : { ...emptySum }
  },
  { default: () => ({ ...emptySum }) }
)

const days = ref(7)
const emptyOverview: OverviewData = {
  new_user: 0,
  new_active_user: 0,
  new_galgame: 0,
  new_resource: 0,
  new_comment: 0
}

const { data: overview, pending, refresh } = await useAsyncData<OverviewData>(
  'admin-stats-overview',
  async () => {
    const res = await api.get<OverviewData>(`/admin/stats?days=${days.value}`)
    return res.code === 0 ? res.data : { ...emptyOverview }
  },
  { default: () => ({ ...emptyOverview }) }
)

watch(days, () => refresh())
</script>

<template>
  <div class="space-y-8">
    <div class="space-y-4">
      <h2 class="text-2xl font-bold">数据统计</h2>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <KunCard
          v-for="(title, key) in ADMIN_STATS_SUM_MAP"
          :key="key"
          :bordered="true"
        >
          <div>
            <div class="text-2xl font-bold">
              {{ formatNumber((sum as any)?.[key] ?? 0) }}
            </div>
            <div class="text-default-500 text-sm">{{ title }}</div>
          </div>
        </KunCard>
      </div>
    </div>

    <KunDivider color="default" />

    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-3">
        <h3 class="text-lg font-semibold whitespace-nowrap">
          最近 {{ days }} 天内数据
        </h3>
        <div class="max-w-xs flex-1">
          <KunSlider v-model="days" :min="1" :max="60" :step="1" />
        </div>
      </div>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <KunCard
          v-for="(title, key) in ADMIN_STATS_MAP"
          :key="key"
          :bordered="true"
        >
          <div>
            <div class="text-2xl font-bold">
              {{ formatNumber((overview as any)?.[key] ?? 0) }}
            </div>
            <div class="text-default-500 text-sm">{{ title }}</div>
          </div>
        </KunCard>
      </div>
    </div>
  </div>
</template>
