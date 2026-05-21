<script setup lang="ts">
// /user/:uid/resource returns paginated PatchResources. user/service
// attaches each row's owning patch summary (id / vndb_id / name / banner)
// from the Wiki Service via the same path the global resource list uses --
// see attachPatchSummaries in apps/api/internal/user/service/service.go.
const route = useRoute()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: UserResourceItem[]
  total: number
}

const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-resources`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/resource?page=1&limit=20`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const patchName = (r: UserResourceItem) =>
  r.patch?.name ? getPreferredLanguageText(r.patch.name) : `补丁 #${r.galgame_id}`

const patchBanner = (r: UserResourceItem) =>
  resolveBannerUrl(r.patch, 'mini') || '/kungalgame-trans.webp'
</script>

<template>
  <div>
    <KunLoading v-if="pending" description="加载中..." />
    <div v-else-if="data?.items?.length" class="space-y-3">
      <NuxtLink
        v-for="r in data.items"
        :key="r.id"
        :to="`/patch/${r.galgame_id}/resource`"
        class="border-default/20 bg-background hover:bg-default-100 flex gap-4 rounded-lg border p-4 transition-colors"
      >
        <img
          :src="patchBanner(r)"
          :alt="patchName(r)"
          class="bg-default-100 h-24 w-40 shrink-0 rounded object-cover"
        />
        <div class="flex-1 space-y-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="hover:text-primary-500 text-lg font-semibold line-clamp-2">
              {{ patchName(r) }}
            </h3>
            <KunChip variant="flat">
              {{ formatDistanceToNow(r.created) }}
            </KunChip>
          </div>
          <KunPatchAttribute
            :types="r.type"
            :languages="r.language"
            :platforms="r.platform"
            size="sm"
          />
        </div>
      </NuxtLink>
    </div>
    <KunNull v-else description="该用户暂未发布任何资源" />
  </div>
</template>
