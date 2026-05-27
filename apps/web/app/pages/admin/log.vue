<script setup lang="ts">
import { ADMIN_LOG_TYPE_MAP } from '~/constants/admin'

useKunDisableSeo('管理日志')

const api = useApi()
const page = ref(1)
const limit = 30

interface ListResponse {
  items: AdminLog[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'admin-logs',
  async () => {
    const res = await api.get<ListResponse>(
      `/admin/log?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">管理日志</h1>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-2">
      <div
        v-for="l in data?.items"
        :key="l.id"
        class="border-default/20 bg-background flex items-start gap-3 rounded-lg border p-3"
      >
        <KunAvatar v-if="l.user" :user="l.user" size="sm" />
        <div class="flex-1 space-y-1">
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="font-semibold">{{ l.user?.name ?? '系统' }}</span>
            <KunChip size="sm" variant="flat" color="primary">
              {{ ADMIN_LOG_TYPE_MAP[l.type] ?? l.type }}
            </KunChip>
            <span class="text-default-500 text-xs">
              {{
                formatDate(l.created, { isShowYear: true, isPrecise: true })
              }}
            </span>
          </div>
          <p class="text-sm whitespace-pre-wrap break-all">{{ l.content }}</p>
        </div>
      </div>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无日志"
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
