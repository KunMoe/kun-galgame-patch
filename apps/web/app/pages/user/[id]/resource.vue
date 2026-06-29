<script setup lang="ts">
// /user/:id/resource returns paginated PatchResources. user/service
// attaches each row's owning patch summary (id / vndb_id / name / banner)
// from the Wiki Service via the same path the global resource list uses --
// see attachPatchSummaries in apps/api/internal/user/service/service.go.
// keepalive: returning from a detail restores this tab's page + scroll. `page`
// is a computed off ?page=, so reactivation re-reads the URL for the right page.
// Kept alive via the central include list in app.vue, keyed by this name.
defineOptions({ name: 'user-resource' })

const route = useRoute()
const router = useRouter()
const api = useApi()
const userId = computed(() => Number(route.params.id))

interface ListResponse {
  items: UserResourceItem[]
  total: number
}

// Page in the URL (?page=) so back-nav / shared links restore it; switching to
// another user lands on a clean URL → page 1.
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v) => router.replace({ query: { ...route.query, page: String(v) } })
})
const limit = 20
const { data, pending } = await useAsyncData<ListResponse>(
  () => `user-${userId.value}-resources`,
  async () => {
    const res = await api.get<ListResponse>(
      `/user/${userId.value}/resource?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }), watch: [page] }
)
const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))
const onChangePage = (v: number) => {
  page.value = v
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const patchName = (r: UserResourceItem) =>
  r.patch?.name ? getPreferredLanguageText(r.patch.name) : `补丁 #${r.galgame_id}`

// Title with the resource's OWN name — a user often publishes several resources
// for the same game, so titling each by the galgame name made the rows
// indistinguishable. Fall back to the galgame name when the resource is unnamed.
const resourceTitle = (r: UserResourceItem) => r.name || patchName(r)

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
        :to="`/resource/${r.id}`"
        class="border-default/20 bg-content1 shadow-kun-sm hover:bg-default-100 flex gap-4 rounded-lg border p-4 transition-colors"
      >
        <KunImage
          :src="patchBanner(r)"
          :alt="patchName(r)"
          class-name="bg-default-100 h-24 w-40 shrink-0 rounded"
        />
        <div class="flex-1 space-y-2">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <h3
                class="hover:text-primary-500 text-lg font-semibold line-clamp-2"
              >
                {{ resourceTitle(r) }}
              </h3>
              <!-- galgame name as subtitle so the game context isn't lost (the
                   banner is the game's), shown only when the title is the
                   resource's own name to avoid duplicating it. -->
              <p
                v-if="r.name && r.patch?.name"
                class="text-default-500 line-clamp-1 text-xs"
              >
                {{ patchName(r) }}
              </p>
            </div>
            <KunChip variant="flat" class="shrink-0">
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

    <div v-if="totalPages > 1" class="mt-6 flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
