<script setup lang="ts">
useKunDisableSeo('孤儿补丁 - 管理面板')

const api = useApi()
const page = ref(1)
const limit = 20

// A single patch (only the local fields needed before enrichment)
interface OrphanPatch {
  id: number
  vndb_id: string
  resource_count: number
  comment_count: number
  favorite_count: number
  download: number
  view: number
  user_id: number
  created: string
  user?: { id: number; name: string; avatar: string }
}

interface OrphanListResponse {
  items: OrphanPatch[]
  total: number
  pending_count: number      // rows with vndb_id = 'pending-N' (never filled)
  bad_vndb_count: number     // rows with a malformed vndb_id (not vN, not pending-)
}

const { data, pending, refresh } = await useAsyncData<OrphanListResponse>(
  'admin-orphan-patches',
  async () => {
    const res = await api.get<OrphanListResponse>(
      `/admin/patch/orphans?page=${page.value}&limit=${limit}`
    )
    return res.code === 0
      ? res.data
      : { items: [], total: 0, pending_count: 0, bad_vndb_count: 0 }
  },
  { default: () => ({ items: [], total: 0, pending_count: 0, bad_vndb_count: 0 }) }
)

watch(page, () => refresh())

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

// Per-row "vndb_id currently being edited"
const editVndbID = reactive<Record<number, string>>({})
const submitting = reactive<Record<number, boolean>>({})

const isPlaceholder = (v: string) => v.startsWith('pending-')

const handleRebind = async (galgameId: number) => {
  const newID = (editVndbID[galgameId] || '').trim()
  if (!/^v\d+$/.test(newID)) {
    useKunMessage('vndb_id 格式不合法，应为 vXXX', 'error')
    return
  }
  submitting[galgameId] = true
  try {
    const res = await api.put(`/patch/${galgameId}`, { vndb_id: newID })
    if (res.code === 0) {
      useKunMessage('已重新绑定，Wiki 校验通过', 'success')
      // Clear this row's edit input after a successful rebind; `delete` on the
      // reactive record is intentional (the row's input field is keyed by id).
      // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
      delete editVndbID[galgameId]
      await refresh()
    } else {
      useKunMessage(res.message || '重绑失败（Wiki 里可能还没这个游戏）', 'error')
    }
  } finally {
    submitting[galgameId] = false
  }
}

const handleDelete = async (galgameId: number) => {
  const ok = await useKunAlert({
    title: '删除补丁',
    message: `确定删除补丁 #${galgameId}？会级联删除其资源、评论、收藏关系，不可恢复！`
  })
  if (!ok) return
  submitting[galgameId] = true
  try {
    const res = await api.delete(`/patch/${galgameId}`)
    if (res.code === 0) {
      useKunMessage('已删除', 'success')
      await refresh()
    } else {
      useKunMessage(res.message || '删除失败', 'error')
    }
  } finally {
    submitting[galgameId] = false
  }
}

const vndbLink = (v: string) =>
  /^v\d+$/.test(v) ? `https://vndb.org/${v}` : ''
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">孤儿补丁</h1>
      <p class="text-default-500 mt-1 text-sm">
        这些补丁的 vndb_id 在 Galgame Wiki 里查不到对应游戏 —— 要么是 Moyu
        作者发布时没填，要么是填错了。请逐条人工处理：输入正确的 vndb_id
        重绑，或直接删除。
      </p>
    </div>

    <!-- Stats cards -->
    <div class="grid gap-3 sm:grid-cols-3">
      <KunCard :bordered="true">
        <p class="text-default-500 text-xs">总计</p>
        <p class="mt-1 text-2xl font-bold">{{ data?.total ?? 0 }}</p>
      </KunCard>
      <KunCard :bordered="true">
        <p class="text-default-500 text-xs">未填 vndb_id</p>
        <p class="mt-1 text-2xl font-bold text-warning">
          {{ data?.pending_count ?? 0 }}
        </p>
        <p class="text-default-400 mt-1 text-xs">
          vndb_id 形如 <code>pending-N</code>
        </p>
      </KunCard>
      <KunCard :bordered="true">
        <p class="text-default-500 text-xs">vndb_id 格式无效</p>
        <p class="mt-1 text-2xl font-bold text-danger">
          {{ data?.bad_vndb_count ?? 0 }}
        </p>
        <p class="text-default-400 mt-1 text-xs">
          不是 <code>vN</code> 形式 (如填成 release id <code>rN</code>)
        </p>
      </KunCard>
    </div>

    <KunLoading v-if="pending" description="加载中..." />

    <div v-else class="space-y-3">
      <KunCard v-for="p in data?.items" :key="p.id" :bordered="true">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-start">
          <!-- Left: basic info -->
          <div class="flex-1 space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-mono text-default-500 text-sm">
                #{{ p.id }}
              </span>
              <KunChip
                size="sm"
                variant="flat"
                :color="isPlaceholder(p.vndb_id) ? 'warning' : 'danger'"
              >
                {{ isPlaceholder(p.vndb_id) ? '未填' : '格式无效' }}
              </KunChip>
              <code
                class="bg-default-100 rounded px-2 py-0.5 text-xs"
              >
                {{ p.vndb_id }}
              </code>
              <a
                v-if="vndbLink(p.vndb_id)"
                :href="vndbLink(p.vndb_id)"
                target="_blank"
                rel="noopener"
                class="text-primary text-xs hover:underline"
              >
                查 VNDB ↗
              </a>
            </div>

            <div class="flex flex-wrap items-center gap-3 text-xs text-default-500">
              <span v-if="p.user">
                发布者:
                <NuxtLink
                  :to="`/user/${p.user.id}`"
                  class="text-primary hover:underline"
                >
                  {{ p.user.name }}
                </NuxtLink>
              </span>
              <span>资源数: {{ p.resource_count }}</span>
              <span>评论: {{ p.comment_count }}</span>
              <span>收藏: {{ p.favorite_count }}</span>
              <span>下载: {{ p.download }}</span>
              <span>浏览: {{ p.view }}</span>
              <span>{{ formatDate(p.created, { isShowYear: true }) }}</span>
            </div>
          </div>

          <!-- Right: actions -->
          <div class="flex flex-col gap-2 lg:w-96">
            <div class="flex items-center gap-2">
              <KunInput
                v-model="editVndbID[p.id]"
                placeholder="填入正确的 vndb_id（如 v12345）"
                class="flex-1"
              />
              <KunButton
                color="primary"
                size="sm"
                :loading="submitting[p.id]"
                :disabled="submitting[p.id]"
                @click="handleRebind(p.id)"
              >
                重绑
              </KunButton>
            </div>
            <div class="flex justify-end">
              <KunButton
                color="danger"
                variant="light"
                size="sm"
                :disabled="submitting[p.id]"
                @click="handleDelete(p.id)"
              >
                删除此补丁
              </KunButton>
            </div>
          </div>
        </div>
      </KunCard>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="没有孤儿补丁了 🎉"
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
