<script setup lang="ts">
useKunSeoMeta({ title: '评论管理' })

const api = useApi()
const page = ref(1)
const limit = 30

interface ListResponse {
  items: PatchComment[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'admin-comments',
  async () => {
    const res = await api.get<ListResponse>(
      `/admin/comment?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

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
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold">评论管理</h1>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="space-y-3">
      <KunCard v-for="c in data?.items" :key="c.id" :bordered="true">
        <div class="flex items-start gap-3">
          <KunAvatar :user="c.user" size="sm" />
          <div class="flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="font-semibold">{{ c.user.name }}</span>
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
          <KunButton
            size="sm"
            variant="light"
            color="danger"
            @click="handleDelete(c.id)"
          >
            删除
          </KunButton>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无评论"
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
