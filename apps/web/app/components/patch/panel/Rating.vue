<script setup lang="ts">
const route = useRoute()
const api = useApi()
const { requireLogin } = useAuthModal()

const emit = defineEmits<{ 'update:loading': [boolean] }>()

const patch = inject<Ref<PatchHeader | null>>('patch')
const galgameId = computed(() => Number(route.params.id))
const workName = computed(() =>
  patch?.value ? getPreferredLanguageText(patch.value.name) : ''
)

const { version } = useRatingRefresh(galgameId)

const limit = 10
const page = ref(1)

interface ListResponse {
  items: GalgameRating[]
  total: number
}

const { data, pending } = await useAsyncData<ListResponse>(
  () => `patch-rating-${galgameId.value}-${page.value}-${version.value}`,
  async () => {
    const res = await api.get<ListResponse>(
      `/patch/${galgameId.value}/rating?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

watch(pending, (value) => emit('update:loading', value), { immediate: true })

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const recordOpen = ref(false)
const openRecord = () => {
  if (!requireLogin()) return
  recordOpen.value = true
}

const onDeleted = (id: number) => {
  if (!data.value) return
  data.value = {
    items: data.value.items.filter((r) => r.id !== id),
    total: Math.max(0, data.value.total - 1)
  }
}
</script>

<template>
  <div class="space-y-4">
    <KunHeader
      name="玩家评分"
      description="来自所有 NextMoe 站点的玩家评分，每人对每部作品只有一条"
      scale="h2"
    >
      <template #headerEndContent>
        <KunButton color="primary" size="sm" @click="openRecord">
          <KunIcon name="lucide:star" class="size-4" />
          评分
        </KunButton>
      </template>
    </KunHeader>

    <div v-if="data.items.length">
      <RatingItem
        v-for="rating in data.items"
        :key="rating.id"
        :rating="rating"
        @deleted="onDeleted"
      />
    </div>
    <KunNull v-else-if="!pending" description="还没有人评分，来写第一条吧" />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="page = $event"
      />
    </div>

    <RatingRecordModal
      v-model="recordOpen"
      :work-id="galgameId"
      :work-name="workName"
    />
  </div>
</template>
