<script setup lang="ts">
// Standalone moyu official (developer/publisher) detail page. Mirrors the
// tag/[id].vue layout — galgame's `GET /official/_?official_id=N` returns the
// official + paginated associated galgames.

// keepalive: returning from a galgame restores this official's page + scroll.
// The page is a computed off `?page=`, so reactivation re-reads the URL and
// refetches the right page (brief silent re-fetch). Mirrors kungal's feed.
// Kept alive via the central include list in app.vue, keyed by this name.
defineOptions({ name: 'official-detail' })

const route = useRoute()
const router = useRouter()
const ge = useGalgameEdit()

const officialID = computed(() => Number(route.params.id))
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v } })
  }
})
const pageHref = usePageHref() // crawlable pagination (<a href>)
const limit = 24

const CATEGORY_LABEL: Record<string, string> = {
  company: '公司',
  individual: '个人',
  amateur: '同人'
}

const { data, pending, refresh } = await useAsyncData(
  () => `official-detail-${officialID.value}-${page.value}`,
  async () => {
    const res = await ge.officialDetail(officialID.value, {
      page: page.value,
      limit
    })
    if (res.code !== 0) return null
    return res.data
  },
  { watch: [page] }
)

const official = computed(() => data.value?.official ?? null)
const galgames = computed<GalgameCard[]>(
  () => data.value?.galgames ?? []
)
const total = computed(() => data.value?.total ?? 0)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

// Same gate shape as tag/[id].vue — galgame /official/:name end has sfw
// default by §16.2 protocol, so the loaded path is SEO-safe; null path
// disables to avoid indexing a missing-entity stub.
if (official.value) {
  useKunSeoMeta({
    title: `会社 · ${official.value.name}`,
    description: `${official.value.name}（${official.value.galgame_count ?? '0'} 个 Galgame）的汉化补丁、中文补丁资源下载合集`
  })
} else {
  useKunDisableSeo('会社详情')
}

watch(official, () => refresh(), { flush: 'post' })
</script>

<template>
  <div class="container mx-auto my-6">
    <KunLoading v-if="pending && !official" description="加载中..." />

    <KunNull v-else-if="!official" description="会社不存在或加载失败" />

    <template v-else>
      <!-- Header -->
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold sm:text-3xl">{{ official.name }}</h1>
          <KunChip color="success" variant="flat" size="sm">
            {{ CATEGORY_LABEL[official.category] ?? official.category }}
          </KunChip>
          <KunChip color="default" size="sm">
            {{ official.galgame_count ?? 0 }} 个 Galgame
          </KunChip>
          <a
            v-if="official.link"
            :href="official.link"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary text-sm hover:underline"
          >
            <KunIcon name="lucide:external-link" class="inline size-3.5" />
            官网
          </a>
        </div>
        <p
          v-if="official.description"
          class="text-default-700 mt-3 text-sm whitespace-pre-wrap"
        >
          {{ official.description }}
        </p>
        <div v-if="official.aliases?.length" class="mt-3 flex flex-wrap gap-2">
          <span class="text-default-500 text-sm">别名：</span>
          <span
            v-for="a in official.aliases"
            :key="a"
            class="bg-default-100 rounded-full px-2 py-0.5 text-xs"
          >
            {{ a }}
          </span>
        </div>
        <p v-if="official.lang" class="text-default-500 mt-2 text-xs">
          主语言: {{ official.lang }}
        </p>
      </section>

      <!-- Associated Galgames -->
      <section class="mt-6">
        <div class="mb-4 flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-xl font-bold">由此会社发布的 Galgame</h2>
        </div>

        <KunNull
          v-if="!galgames.length"
          description="暂无关联作品"
        />

        <div
          v-else
          class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
        >
          <!-- Backend now serves moyu-enriched GalgameCard shape for
               official detail (GalgameTaxonomyDetailProxy) — same shape as
               home / galgame index, render the same component. -->
          <GalgameCard v-for="g in galgames" :key="g.id" :patch="g" />
        </div>

        <KunPagination
          v-if="totalPage > 1"
          v-model:current-page="page"
          :total-page="totalPage"
          :is-loading="pending"
          :page-href="pageHref"
          class="mt-6"
        />
      </section>
    </template>
  </div>
</template>
