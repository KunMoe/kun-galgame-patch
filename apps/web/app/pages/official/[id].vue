<script setup lang="ts">
// Standalone moyu official (developer/publisher) detail page. Mirrors the
// tag/[id].vue layout — Wiki's `GET /official/_?official_id=N` returns the
// official + paginated associated galgames.

import { resolveBannerUrl } from '~/shared/utils/resolveBannerUrl'

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
  title: official.value ? `会社 · ${official.value.name}` : '会社详情',
  description: official.value
    ? `${official.value.name}（${official.value.galgame_count ?? '0'} 个 Galgame）`
    : ''
})

watch(official, () => refresh(), { flush: 'post' })
</script>

<template>
  <div class="container mx-auto my-6 max-w-5xl px-4">
    <KunLoading v-if="pending && !official" description="加载中..." />

    <KunNull v-else-if="!official" description="会社不存在或加载失败" />

    <template v-else>
      <!-- Header -->
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold sm:text-3xl">{{ official.name }}</h1>
          <KunBadge color="success" variant="flat" size="sm">
            {{ CATEGORY_LABEL[official.category] ?? official.category }}
          </KunBadge>
          <KunBadge color="default" size="sm">
            {{ official.galgame_count ?? 0 }} 个 Galgame
          </KunBadge>
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
