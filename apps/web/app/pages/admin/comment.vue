<script setup lang="ts">
useKunDisableSeo('评论管理')

const api = useApi()
const page = ref(1)
const limit = 30
// Review-queue filter: '' = all, 'pending' = awaiting approval (comment-verify).
const statusFilter = ref<'' | 'pending'>('')

interface ListResponse {
  items: PatchComment[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'admin-comments',
  async () => {
    const params = new URLSearchParams({
      page: String(page.value),
      limit: String(limit)
    })
    if (statusFilter.value) params.set('status', statusFilter.value)
    const res = await api.get<ListResponse>(`/admin/comment?${params.toString()}`)
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch([page, statusFilter], () => refresh())

// Reset to page 1 when switching filters so we don't land on an out-of-range page.
const setFilter = (v: '' | 'pending') => {
  if (statusFilter.value === v) return
  statusFilter.value = v
  page.value = 1
}

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const handleApprove = async (id: number) => {
  const res = await api.put(`/admin/comment/${id}/approve`)
  if (res.code === 0) {
    useKunMessage('已通过审核', 'success')
    await refresh()
  } else {
    useKunMessage(res.message || '操作失败', 'error')
  }
}

const handleDelete = async (id: number) => {
  const ok = await useKunAlert({
    title: '删除评论',
    message: '确定要删除这条评论吗？此操作不可恢复。'
  })
  if (!ok) return
  const res = await api.delete(`/admin/comment/${id}`)
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    await refresh()
  } else {
    useKunMessage(res.message || '删除失败', 'error')
  }
}

const chipClass = (active: boolean) => [
  'cursor-pointer rounded-md px-2.5 py-1 text-sm transition-colors',
  active
    ? 'bg-primary/15 text-primary font-medium'
    : 'text-default-600 hover:bg-default-100'
]
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-bold">评论管理</h1>
      <!-- Review-queue filter. '待审核' surfaces comments held by the
           评论需要审核 toggle so moderators can approve/reject them. -->
      <div class="flex gap-1">
        <button type="button" :class="chipClass(statusFilter === '')" @click="setFilter('')">
          全部
        </button>
        <button
          type="button"
          :class="chipClass(statusFilter === 'pending')"
          @click="setFilter('pending')"
        >
          待审核
        </button>
      </div>
    </div>

    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-3">
      <KunCard v-for="c in data?.items" :key="c.id" :bordered="true">
        <div class="flex items-start gap-3">
          <KunAvatar :user="c.user" size="sm" />
          <div class="flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="font-semibold">{{ c.user.name }}</span>
              <KunChip v-if="c.status === 1" size="sm" variant="flat" color="warning">
                待审核
              </KunChip>
              <span class="text-default-500">
                在
                <NuxtLink
                  :to="`/patch/${c.galgame_id}/comment`"
                  class="text-primary hover:underline"
                >
                  补丁 #{{ c.galgame_id }}
                </NuxtLink>
              </span>
              <span class="text-default-400 text-xs">
                {{
                  formatDate(c.created, { isShowYear: true, isPrecise: true })
                }}
              </span>
            </div>
            <p class="whitespace-pre-wrap">{{ c.content }}</p>
          </div>
          <div class="flex shrink-0 gap-2">
            <KunButton
              v-if="c.status === 1"
              size="sm"
              variant="light"
              color="success"
              @click="handleApprove(c.id)"
            >
              通过
            </KunButton>
            <KunButton
              size="sm"
              variant="light"
              color="danger"
              @click="handleDelete(c.id)"
            >
              {{ c.status === 1 ? '拒绝' : '删除' }}
            </KunButton>
          </div>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      :description="statusFilter === 'pending' ? '没有待审核的评论 🎉' : '暂无评论'"
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
