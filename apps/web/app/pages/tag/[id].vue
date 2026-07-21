<script setup lang="ts">
// Standalone moyu tag detail page (replaces the previous external-link to a
// standalone wiki tag page from the patch introduction). Tag metadata now
// comes from the NextMoe catalog service, proxied by the moyu backend.
//
// Reads `GET /tag/_?tag_id=N&page&limit` which returns the tag entity + the
// associated paginated galgame list. The `:name` segment is cosmetic upstream;
// we always pass "_" via the composable to keep our internal URL tidy
// (`/tag/:id`).

import type { KunUIColor } from '@kungal/ui-core'

// keepalive: returning from a galgame restores this tag's page + scroll. The
// page is a computed off `?page=`, so on reactivation it re-reads the URL and
// refetches the right page (a brief, silent re-fetch — list fetches return null
// on miss, no toast). Mirrors kungal's feed keepalive.
// Kept alive via the central include list in app.vue, keyed by this name.
defineOptions({ name: 'tag-detail' })

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
const pageHref = usePageHref() // crawlable pagination (<a href>)
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

const { data, pending } = await useAsyncData(
  () => `tag-detail-${tagID.value}-${page.value}`,
  async () => {
    const res = await ge.tagDetail(tagID.value, { page: page.value, limit })
    if (res.code !== 0) return null
    return res.data
  },
  // Refetch on page change AND tag change (navigating between /tag/:id without
  // a full remount). Do NOT watch the derived `tag` entity and call refresh():
  // each fetch yields a new `data` object → new `tag` ref → the watch re-fires →
  // refresh() loops forever, pinning `pending` true, which disables every
  // KunPagination button (is-loading) so paging gets stuck after one click.
  { watch: [page, tagID] }
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

// Wiki end already applies sfw default at /tag/:name (see
// docs/galgame_wiki/00-handbook §16.2), so by construction the tag
// detail's galgame list never leaks NSFW entries to the SSR HTML. SEO is
// safe to enable on a loaded SFW tag. Disable when:
//   - the tag is a sexual (NSFW) category — the tag name/description itself is
//     a NSFW signal, so don't let search engines index it (mirrors the NSFW
//     SEO gate on patch/[id].vue), or
//   - the tag is missing / the wiki call failed (avoid indexing a 404 stub).
if (tag.value && tag.value.category !== 'sexual') {
  useKunSeoMeta({
    title: `标签 · ${tag.value.name}`,
    description: `${tag.value.name}（${tag.value.galgame_count ?? '0'} 个 Galgame）汉化补丁、中文补丁资源下载合集`
  })
} else {
  useKunDisableSeo(tag.value ? `标签 · ${tag.value.name}` : '标签详情')
}
</script>

<template>
  <div class="container mx-auto my-6">
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
          :page-href="pageHref"
          class="mt-6"
        />
      </section>
    </template>
  </div>
</template>
