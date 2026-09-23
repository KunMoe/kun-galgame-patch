<script setup lang="ts">
import { KUN_RANKING_LIMIT, KUN_RANKING_TABS } from '~/constants/ranking'

const route = useRoute()
const router = useRouter()
const api = useApi()

const sortBy = ref(
  String(route.query.sort_by ?? route.query.sortBy ?? 'moemoepoint')
)

useKunSeoMeta({
  title: '用户排行榜',
  description:
    '鲲 Galgame 补丁站用户排行榜，按萌萌点、发布的 Galgame / 补丁资源数量、评论数排名，了解社区最活跃的汉化补丁贡献者。'
})

const { data, pending, refresh } = await useAsyncData<RankingUser[]>(
  () => `ranking-user-${sortBy.value}`,
  async () => {
    const res = await api.get<RankingUser[]>(
      `/ranking/user?sort_by=${sortBy.value}`
    )
    return res.code === 0 ? res.data : []
  },
  { default: () => [] }
)

const sortOptions = [
  { value: 'moemoepoint', label: '萌萌点' },
  { value: 'patch_count', label: '发布 Galgame' },
  { value: 'resource_count', label: '补丁资源数' },
  { value: 'comment_count', label: '评论数' }
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
      :description="`按萌萌点、发布的 Galgame 数、补丁资源数与评论数排出的全站前 ${KUN_RANKING_LIMIT} 名，统计自建站以来的累计数据，每次打开都是实时计算的结果`"
    />

    <div class="flex flex-wrap items-center justify-between gap-3">
      <KunTab
        model-value="user"
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
        v-for="(user, index) in data"
        :key="user.id"
        :to="`/user/${user.id}/resource`"
        class="border-default/20 hover:bg-default-100 flex items-center gap-3 rounded-lg border p-3 transition-colors"
      >
        <span class="text-default-500 w-8 text-right font-mono font-semibold">
          {{ index + 1 }}
        </span>
        <KunAvatar :user="toKunUser(user)" :is-navigation="false" size="md" />
        <div class="flex-1">
          <div class="font-semibold">{{ user.name }}</div>
          <div class="text-default-500 flex flex-wrap gap-3 text-xs">
            <span>萌萌点 {{ user.moemoepoint }}</span>
            <span>Galgame {{ user.patch_count }}</span>
            <span>补丁资源 {{ user.resource_count }}</span>
            <span>评论 {{ user.comment_count }}</span>
          </div>
        </div>
      </NuxtLink>
    </div>

    <KunNull v-if="!pending && !data?.length" description="暂无排行数据" />
  </div>
</template>
