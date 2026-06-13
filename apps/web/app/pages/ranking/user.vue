<script setup lang="ts">
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
  { value: 'patch_count', label: '补丁发布数' },
  { value: 'resource_count', label: '资源发布数' },
  { value: 'comment_count', label: '评论数' }
]

// KunSelect's v-model widened to `string | string[] | null` in KunUI 0.14.0
// (multiple/clearable support). This is a single select → only a string ever
// arrives; guard the other shapes to satisfy the type.
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
      description="查看全部时间的数据累计"
    />

    <div class="flex flex-wrap items-center gap-3">
      <div class="flex gap-2">
        <NuxtLink
          to="/ranking/user"
          class="bg-primary text-primary-foreground flex items-center gap-1 rounded-md px-3 py-1.5 text-sm"
        >
          <KunIcon name="lucide:user" class="size-4" />
          用户排名
        </NuxtLink>
        <NuxtLink
          to="/ranking/patch"
          class="hover:bg-default-100 text-foreground flex items-center gap-1 rounded-md px-3 py-1.5 text-sm"
        >
          <KunIcon name="lucide:puzzle" class="size-4" />
          补丁排名
        </NuxtLink>
      </div>
      <KunSelect
        :model-value="sortBy"
        :options="sortOptions"
        class-name="min-w-40"
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
        <span
          class="text-default-500 w-8 text-right font-mono font-semibold"
        >
          {{ index + 1 }}
        </span>
        <KunAvatar :user="user" :is-navigation="false" size="md" />
        <div class="flex-1">
          <div class="font-semibold">{{ user.name }}</div>
          <div class="text-default-500 flex flex-wrap gap-3 text-xs">
            <span>萌萌点 {{ user.moemoepoint }}</span>
            <span>补丁 {{ user.patch_count }}</span>
            <span>资源 {{ user.resource_count }}</span>
            <span>评论 {{ user.comment_count }}</span>
          </div>
        </div>
      </NuxtLink>
    </div>

    <KunNull v-if="!pending && !data?.length" description="暂无排行数据" />
  </div>
</template>
