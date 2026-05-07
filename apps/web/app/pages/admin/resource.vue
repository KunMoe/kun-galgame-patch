<script setup lang="ts">
useKunSeoMeta({ title: '补丁资源管理' })

const api = useApi()
const page = ref(1)
const limit = 30

interface ListResponse {
  items: AdminResourceItem[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'admin-resources',
  async () => {
    const res = await api.get<ListResponse>(
      `/admin/resource?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const handleDelete = async (id: number) => {
  if (!confirm('确定要删除这个补丁资源吗?')) return
  const res = await api.delete(`/admin/resource/${id}`)
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
    <h1 class="text-2xl font-bold">补丁资源管理</h1>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="border-default/20 border-b">
          <tr>
            <th class="px-3 py-2 text-left">Galgame</th>
            <th class="px-3 py-2 text-left">资源名</th>
            <th class="px-3 py-2 text-left">发布者</th>
            <th class="px-3 py-2 text-left">大小</th>
            <th class="px-3 py-2 text-left">下载</th>
            <th class="px-3 py-2 text-left">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in data?.items"
            :key="r.id"
            class="border-default/20 hover:bg-default-50 border-b"
          >
            <td class="px-3 py-2">
              <NuxtLink
                :to="`/patch/${r.galgame_id}/resource`"
                class="text-primary hover:underline"
              >
                补丁 #{{ r.galgame_id }}
              </NuxtLink>
            </td>
            <td class="px-3 py-2">{{ r.name || '—' }}</td>
            <td class="px-3 py-2">{{ r.user?.name ?? '—' }}</td>
            <td class="text-default-500 px-3 py-2">{{ r.size }}</td>
            <td class="text-default-500 px-3 py-2">{{ r.download }}</td>
            <td class="px-3 py-2">
              <div class="flex gap-2">
                <NuxtLink :to="`/resource/${r.id}`">
                  <KunButton size="sm" variant="light">查看</KunButton>
                </NuxtLink>
                <KunButton
                  size="sm"
                  variant="light"
                  color="danger"
                  @click="handleDelete(r.id)"
                >
                  删除
                </KunButton>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无资源"
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
