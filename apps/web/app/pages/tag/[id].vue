<script setup lang="ts">
// Standalone moyu tag detail page (replaces the previous external-link to
// galgame.kungal.com/tag/:id from the patch introduction).
//
// Reads Wiki's `GET /tag/_?tag_id=N&page&limit` which returns the tag entity
// + the associated paginated galgame list. `:name` segment is cosmetic on
// Wiki side; we always pass "_" via the composable to keep our internal URL
// tidy (`/tag/:id`).

import { resolveBannerUrl } from '~/shared/utils/resolveBannerUrl'

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
const CATEGORY_COLOR: Record<string, string> = {
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
const galgames = computed<Record<string, unknown>[]>(() => {
  const items = data.value?.items ?? data.value?.galgames ?? []
  return items as Record<string, unknown>[]
})
const total = computed(() => data.value?.total ?? 0)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

const displayName = (g: Record<string, unknown>): string =>
  (g.name_zh_cn as string) ||
  (g.name_zh_tw as string) ||
  (g.name_ja_jp as string) ||
  (g.name_en_us as string) ||
  `#${g.id}`

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
          <NuxtLink
            v-for="g in galgames"
            :key="g.id as number"
            :to="`/patch/${g.id}`"
            class="border-default/20 hover:border-primary/40 group block overflow-hidden rounded-lg border transition-colors"
          >
            <div class="aspect-video w-full overflow-hidden">
              <img
                :src="
                  resolveBannerUrl(g as never, 'mini') ||
                  '/kungalgame-trans.webp'
                "
                :alt="displayName(g)"
                loading="lazy"
                class="bg-default-100 size-full object-cover transition-transform duration-300 group-hover:scale-105"
              />
            </div>
            <div class="p-2">
              <p class="line-clamp-2 text-sm font-medium">
                {{ displayName(g) }}
              </p>
              <p
                v-if="g.vndb_id"
                class="text-default-400 mt-0.5 text-xs"
              >
                {{ g.vndb_id }}
              </p>
            </div>
          </NuxtLink>
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
