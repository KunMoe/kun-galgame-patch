<script setup lang="ts">
import { useDebounceFn } from '@vueuse/core'

useKunDisableSeo('Galgame 列表 - 管理面板')

const api = useApi()
const page = ref(1)
const searchQuery = ref('')
const limit = 30

// /api/v1/admin/galgame returns the standard paginated envelope of GalgameCard
// rows — see apps/api/internal/admin/handler.GetGalgame.
interface ListResponse {
  items: GalgameCard[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'admin-galgame',
  async () => {
    const res = await api.get<ListResponse>(
      `/admin/galgame?page=${page.value}&limit=${limit}&search=${encodeURIComponent(searchQuery.value)}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const debouncedRefresh = useDebounceFn(() => {
  page.value = 1
  refresh()
}, 400)

watch(searchQuery, () => debouncedRefresh())
watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">Galgame 列表</h1>

    <KunInput v-model="searchQuery" placeholder="按 vndb_id 搜索（游戏名由 Wiki 维护）">
      <template #prefix>
        <KunIcon name="lucide:search" class="text-default-400 size-4" />
      </template>
    </KunInput>

    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-3">
      <KunCard v-for="g in data?.items" :key="g.id" :bordered="true">
        <div class="flex flex-wrap items-center gap-3">
          <KunImage
            v-if="resolveBannerUrl(g, 'mini')"
            :src="resolveBannerUrl(g, 'mini')"
            :alt="getPreferredLanguageText(g.name)"
            class-name="bg-default-100 h-16 w-28 rounded"
          />
          <div class="flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <NuxtLink
                :to="`/patch/${g.id}/introduction`"
                class="text-primary text-lg font-semibold hover:underline"
              >
                {{ getPreferredLanguageText(g.name) || `补丁 #${g.id}` }}
              </NuxtLink>
              <code class="bg-default-100 rounded px-2 py-0.5 text-xs">
                {{ g.vndb_id }}
              </code>
            </div>
            <div class="text-default-500 flex flex-wrap gap-3 text-xs">
              <span>浏览 {{ formatNumber(g.view) }}</span>
              <span>下载 {{ formatNumber(g.download) }}</span>
              <span>资源 {{ g.count.resource }}</span>
              <span>评论 {{ g.count.comment }}</span>
              <span>{{ formatDate(g.created, { isShowYear: true }) }}</span>
            </div>
          </div>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无补丁"
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="(v) => (page = v)"
      />
    </div>
  </div>
</template>
