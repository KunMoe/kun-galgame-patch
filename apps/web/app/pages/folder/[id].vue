<script setup lang="ts">
defineOptions({ name: 'folder-detail' })

const route = useRoute()
const router = useRouter()
const api = useApi()
const folderId = computed(() => Number(route.params.id))

interface FolderDetailResponse {
  folder: Folder | null
  patches: GalgameCard[]
  total: number
}

const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v) => router.replace({ query: { ...route.query, page: String(v) } })
})
const limit = 24
const pageHref = usePageHref()

const { data, pending } = await useAsyncData<FolderDetailResponse>(
  () => `folder-${folderId.value}`,
  async () => {
    const res = await api.get<FolderDetailResponse>(
      `/folder/${folderId.value}?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { folder: null, patches: [], total: 0 }
  },
  {
    default: () => ({ folder: null, patches: [], total: 0 }),
    watch: [page]
  }
)

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
const onChangePage = (v: number) => {
  page.value = v
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const labelOf = (folder: Folder) =>
  folder.name.trim() || (folder.is_default ? '默认收藏夹' : '未命名收藏夹')

useHead(() => ({
  title: data.value?.folder ? `${labelOf(data.value.folder)} - 收藏夹` : '收藏夹'
}))
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunLoading v-if="pending" description="加载中..." />

    <template v-else-if="data?.folder">
      <div class="space-y-2">
        <div class="flex items-center gap-2">
          <KunIcon
            :name="
              data.folder.visibility === 'private' ? 'lucide:lock' : 'lucide:folder'
            "
            class="text-default-400 size-5 shrink-0"
          />
          <h1 class="text-foreground text-xl font-semibold">
            {{ labelOf(data.folder) }}
          </h1>
        </div>
        <p v-if="data.folder.description" class="text-default-500 text-sm">
          {{ data.folder.description }}
        </p>
        <p class="text-default-400 text-xs">
          {{ data.folder.item_count }} 个游戏
          <!-- Two reasons the numbers differ. The folder is shared with the
               forum, so it can hold games this site has no patch page for; and
               the reader's NSFW gate hides rows here the same way it does on
               every other list. Both are left out rather than rendered as
               holes. -->
          <template v-if="data.folder.item_count !== data.total">
            · 本站收录 {{ data.total }} 个
          </template>
        </p>
      </div>

      <GalgameList v-if="data.patches.length" :items="data.patches" />
      <KunNull v-else description="这个收藏夹还是空的" />

      <div v-if="totalPages > 1" class="flex justify-center">
        <KunPagination
          :current-page="page"
          :total-page="totalPages"
          :is-loading="pending"
          :page-href="pageHref"
          @update:current-page="onChangePage"
        />
      </div>
    </template>

    <KunNull v-else description="收藏夹不存在或未公开" />
  </div>
</template>
