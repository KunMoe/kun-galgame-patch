<script setup lang="ts">
import { KUN_RANKING_LIMIT, KUN_RANKING_TABS } from '~/constants/ranking'

const route = useRoute()
const router = useRouter()
const api = useApi()

const sortBy = ref(String(route.query.sort_by ?? route.query.sortBy ?? 'view'))

useKunSeoMeta({
  title: 'Galgame 补丁排行榜',
  description:
    '鲲 Galgame 补丁站按浏览量 / 下载量 / 收藏数 / 评论数 / 资源数排序的 Galgame 补丁排行榜，发现当前最热门的中文汉化 Galgame 与最受欢迎的补丁资源。'
})

const { data, pending, refresh } = await useAsyncData<GalgameCard[]>(
  () => `ranking-patch-${sortBy.value}`,
  async () => {
    const res = await api.get<GalgameCard[]>(
      `/ranking/patch?sort_by=${sortBy.value}`
    )
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)

const sortOptions = [
  { value: 'view', label: '浏览量' },
  { value: 'download', label: '下载量' },
  { value: 'favorite', label: '收藏数' },
  { value: 'comment', label: '评论数' },
  { value: 'resource', label: '资源数' }
]

const onChangeSort = async (v: string | string[] | null) => {
  if (typeof v !== 'string') return
  sortBy.value = v
  await router.replace({ query: { ...route.query, sort_by: sortBy.value } })
  await refresh()
}
</script>

<template>
  <div class="container mx-auto my-6 space-y-6">
    <KunHeader
      name="排行榜单"
      :description="`按浏览量、下载量、收藏数、评论数与资源数排出的全站前 ${KUN_RANKING_LIMIT} 部 Galgame，统计自建站以来的累计数据，并跟随你在顶栏设置的内容显示范围`"
    />

    <div class="flex flex-wrap items-center justify-between gap-3">
      <KunTab
        model-value="patch"
        :items="KUN_RANKING_TABS"
        variant="pills"
        color="primary"
        size="sm"
      />
      <KunSelect
        :model-value="sortBy"
        :options="sortOptions"
        class-name="w-full sm:w-56"
        @update:model-value="onChangeSort"
      />
    </div>

    <KunLoading v-if="pending" description="正在获取排行榜..." />
    <div v-else class="space-y-2">
      <NuxtLink
        v-for="(patch, index) in data"
        :key="patch.id"
        :to="`/galgame/${patch.id}`"
        class="border-default/20 hover:bg-default-100 flex items-center gap-3 rounded-lg border p-3 transition-colors"
      >
        <span class="text-default-500 w-8 text-right font-mono font-semibold">
          {{ index + 1 }}
        </span>
        <KunImage
          v-if="resolveBannerUrl(patch, 'mini')"
          :src="resolveBannerUrl(patch, 'mini')"
          :alt="getPreferredLanguageText(patch.name)"
          class-name="bg-default-100 h-14 w-24 rounded"
        />
        <div class="flex-1">
          <div class="line-clamp-1 font-semibold">
            {{ getPreferredLanguageText(patch.name) }}
          </div>
          <div class="text-default-500 flex flex-wrap gap-3 text-xs">
            <span>浏览 {{ formatNumber(patch.view) }}</span>
            <span>下载 {{ formatNumber(patch.download) }}</span>
            <span>收藏 {{ formatNumber(patch.count.favorite_by) }}</span>
            <span>评论 {{ formatNumber(patch.count.comment) }}</span>
            <span>资源 {{ formatNumber(patch.count.resource) }}</span>
          </div>
        </div>
      </NuxtLink>
    </div>

    <KunNull v-if="!pending && !data?.length" description="暂无排行数据" />
  </div>
</template>
