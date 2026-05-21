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
  const ok = await useKunAlert({
    title: '删除补丁资源',
    message: '确定要删除这个补丁资源吗？此操作不可恢复。'
  })
  if (!ok) return
  const res = await api.delete(`/admin/resource/${id}`)
  if (res.code === 0) {
    useKunMessage('已删除', 'success')
    await refresh()
  } else {
    useKunMessage(res.message || '删除失败', 'error')
  }
}

// ─── MOYU-PR5 / M3 — Resource file replacement history modal ────────────
// Surfaces patch_resource_file_history rows for one resource so admins can
// trace "this download is broken" complaints to who/when/why the file was
// swapped. Reads /api/v1/admin/resource/:id/history (paginated).
interface FileHistoryItem {
  id: number
  resource_id: number
  old_storage: string
  old_s3_key: string
  old_blake3: string
  old_size: string
  old_content: string
  reason: string
  actor_id: number
  actor_role: number
  created_at: string
}
const ACTOR_ROLE_LABEL: Record<number, string> = {
  0: '未知',
  1: '用户',
  2: '协管',
  3: '管理员'
}

const histOpen = ref(false)
const histResourceId = ref<number | null>(null)
const histLoading = ref(false)
const histItems = ref<FileHistoryItem[]>([])
const histTotal = ref(0)
const histPage = ref(1)
const histLimit = 30

const loadHistory = async () => {
  if (!histResourceId.value) return
  histLoading.value = true
  try {
    const res = await api.get<{ items: FileHistoryItem[]; total: number }>(
      `/admin/resource/${histResourceId.value}/history?page=${histPage.value}&limit=${histLimit}`
    )
    if (res.code === 0) {
      histItems.value = res.data?.items ?? []
      histTotal.value = res.data?.total ?? 0
    } else {
      useKunMessage(res.message || '加载历史失败', 'error')
    }
  } finally {
    histLoading.value = false
  }
}

const openHistory = async (id: number) => {
  histResourceId.value = id
  histPage.value = 1
  histItems.value = []
  histTotal.value = 0
  histOpen.value = true
  await loadHistory()
}

watch(histPage, loadHistory)

const histTotalPages = computed(() =>
  Math.max(1, Math.ceil(histTotal.value / histLimit))
)
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
                  @click="openHistory(r.id)"
                >
                  历史
                </KunButton>
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

    <!-- MOYU-PR5 / M3 — File history modal -->
    <KunModal v-model="histOpen" :is-show-close-button="true">
      <div class="max-h-[85vh] w-[92vw] max-w-2xl space-y-3 overflow-y-auto p-5">
        <h3 class="text-lg font-semibold">
          资源 #{{ histResourceId }} · 文件替换历史
        </h3>
        <p class="text-default-500 text-xs">
          仅记录文件本身的替换（storage / s3_key / 外链 内容变化）。修改备注、
          下载码等元数据不会写入历史。删除资源时历史会随之级联删除。
        </p>

        <KunLoading v-if="histLoading" description="加载中..." />
        <KunNull v-else-if="!histItems.length" description="该资源没有文件替换历史" />

        <div v-else class="space-y-2">
          <KunCard v-for="h in histItems" :key="h.id" :bordered="true">
            <div class="space-y-1 p-3 text-xs">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-mono font-semibold">#{{ h.id }}</span>
                <span class="text-default-500">
                  {{ formatDate(h.created_at, { isPrecise: true, isShowYear: true }) }}
                </span>
                <span class="text-default-500">
                  · 操作者 #{{ h.actor_id }}
                  ({{ ACTOR_ROLE_LABEL[h.actor_role] ?? '—' }})
                </span>
              </div>
              <div class="text-default-700">
                <span class="text-default-500">旧 storage:</span>
                {{ h.old_storage }}
              </div>
              <div v-if="h.old_s3_key" class="text-default-700 break-all">
                <span class="text-default-500">旧 s3_key:</span>
                {{ h.old_s3_key }}
              </div>
              <div v-if="h.old_blake3" class="text-default-700 break-all">
                <span class="text-default-500">旧 blake3:</span>
                {{ h.old_blake3 }}
              </div>
              <div v-if="h.old_size" class="text-default-700">
                <span class="text-default-500">旧 size:</span>
                {{ h.old_size }}
              </div>
              <div v-if="h.old_content" class="text-default-700 break-all">
                <span class="text-default-500">旧 content:</span>
                {{ h.old_content }}
              </div>
              <div
                v-if="h.reason"
                class="border-default/20 mt-2 rounded border-l-2 pl-2 italic"
              >
                {{ h.reason }}
              </div>
            </div>
          </KunCard>

          <div v-if="histTotalPages > 1" class="flex justify-center pt-2">
            <KunPagination
              v-model:current-page="histPage"
              :total-page="histTotalPages"
              :is-loading="histLoading"
            />
          </div>
        </div>
      </div>
    </KunModal>
  </div>
</template>
