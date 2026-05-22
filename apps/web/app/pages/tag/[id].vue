<script setup lang="ts">
// Standalone moyu tag detail page (replaces the previous external-link to
// galgame.kungal.com/tag/:id from the patch introduction).
//
// Reads Wiki's `GET /tag/_?tag_id=N&page&limit` which returns the tag entity
// + the associated paginated galgame list. `:name` segment is cosmetic on
// Wiki side; we always pass "_" via the composable to keep our internal URL
// tidy (`/tag/:id`).

import type { KunUIColor } from '@kun/ui/app/components/kun/ui/type'

const route = useRoute()
const router = useRouter()
const ge = useGalgameEdit()

const tagID = computed(() => Number(route.params.id))
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v } })
  }
})
const limit = 24

const CATEGORY_LABEL: Record<string, string> = {
  content: '内容',
  sexual: '性相关',
  technical: '技术'
}
const CATEGORY_COLOR: Record<string, KunUIColor> = {
  content: 'primary',
  sexual: 'danger',
  technical: 'success'
}

const { data, pending, refresh } = await useAsyncData(
  () => `tag-detail-${tagID.value}-${page.value}`,
  async () => {
    const res = await ge.tagDetail(tagID.value, { page: page.value, limit })
    if (res.code !== 0) return null
    return res.data
  },
  { watch: [page] }
)

// Wiki shapes the response with `tag` for the entity + `items`/`galgames` +
// `total` for the paginated galgame list. Be defensive about which key it
// uses since the doc shows the request but not the exact response.
const tag = computed(() => data.value?.tag ?? null)
const galgames = computed<GalgameCard[]>(
  () => data.value?.galgames ?? []
)
const total = computed(() => data.value?.total ?? 0)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

useKunSeoMeta({
  title: tag.value ? `标签 · ${tag.value.name}` : '标签详情',
  description: tag.value
    ? `${tag.value.name}（${tag.value.galgame_count ?? '0'} 个 Galgame）`
    : ''
})

watch(tag, () => refresh(), { flush: 'post' })
</script>

<template>
  <div class="container mx-auto my-6 max-w-5xl px-4">
    <KunLoading v-if="pending && !tag" description="加载中..." />

    <KunNull v-else-if="!tag" description="标签不存在或加载失败" />

    <template v-else>
      <!-- Header -->
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold sm:text-3xl">{{ tag.name }}</h1>
          <KunChip
            :color="CATEGORY_COLOR[tag.category] ?? 'default'"
            variant="flat"
            size="sm"
          >
            {{ CATEGORY_LABEL[tag.category] ?? tag.category }}
          </KunChip>
          <KunChip color="default" size="sm">
            {{ tag.galgame_count ?? 0 }} 个 Galgame
          </KunChip>
        </div>
        <p
          v-if="tag.description"
          class="text-default-700 mt-3 text-sm whitespace-pre-wrap"
        >
          {{ tag.description }}
        </p>
        <div v-if="tag.aliases?.length" class="mt-3 flex flex-wrap gap-2">
          <span class="text-default-500 text-sm">别名：</span>
          <span
            v-for="a in tag.aliases"
            :key="a"
            class="bg-default-100 rounded-full px-2 py-0.5 text-xs"
          >
            {{ a }}
          </span>
        </div>
      </section>

      <!-- Associated Galgames -->
      <section class="mt-6">
        <div class="mb-4 flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-xl font-bold">包含此标签的 Galgame</h2>
        </div>

        <KunNull
          v-if="!galgames.length"
          description="暂无关联作品"
        />

        <div
          v-else
          class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4"
        >
          <!-- Backend now serves moyu-enriched GalgameCard shape for tag
               detail (WikiTaxonomyDetailProxy joins each Wiki galgame with
               its local patch row for stats), so no client-side adapter
               is needed — render the same component as home / galgame
               index does for full visual + data parity. -->
          <GalgameCard v-for="g in galgames" :key="g.id" :patch="g" />
        </div>

        <KunPagination
          v-if="totalPage > 1"
          v-model:current-page="page"
          :total-page="totalPage"
          :is-loading="pending"
          class="mt-6"
        />
      </section>
    </template>
  </div>
</template>
