<script setup lang="ts">
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

defineOptions({ name: 'label-detail' })

const route = useRoute()
const router = useRouter()
const browse = useTaxonomyBrowse()

const officialID = computed(() => Number(route.params.id))
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v } })
  }
})
const pageHref = usePageHref()
const limit = 24

const CATEGORY_LABEL: Record<string, string> = {
  game_brand: '游戏品牌',
  bunko: '文库',
  publisher: '发行商',
  anime_studio: '动画工作室',
  doujin_circle: '同人社团',
  group: '团体',
  other: '其他'
}

const VERDICT_NOT_FOUND = 40400

const { data, pending } = await useAsyncData(
  () => `official-detail-${officialID.value}-${page.value}`,
  async () => {
    const res = await browse.officialDetail(officialID.value, {
      page: page.value,
      limit
    })
    return { code: res.code, detail: res.code === 0 ? res.data : null }
  },
  { watch: [page, officialID] }
)

const notFound = () => kunPageError(404, '会社不存在')

if (data.value?.code === VERDICT_NOT_FOUND) throw notFound()

const movedTarget = (v: typeof data.value) =>
  v?.code === 0 ? (v.detail?.moved_to ?? 0) : 0
const hopTo = (to: number) =>
  navigateTo(`/galgame/official/${to}`, { redirectCode: 301, replace: true })

const moved = movedTarget(data.value)
if (moved > 0) await hopTo(moved)

watch(data, (v) => {
  if (v?.code === VERDICT_NOT_FOUND) showError(notFound())
  const to = movedTarget(v)
  if (to > 0) hopTo(to)
})

const official = computed(() => data.value?.detail?.official ?? null)
const galgames = computed<GalgameCard[]>(
  () => data.value?.detail?.galgames ?? []
)
const total = computed(() => data.value?.detail?.total ?? 0)

const logoSrc = computed(() =>
  imageServiceUrl((official.value?.logo_hash ?? '').trim())
)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

if (official.value) {
  useKunSeoMeta({
    title: `会社 · ${official.value.name}`,
    description: `${official.value.name}（${official.value.galgame_count ?? '0'} 个 Galgame）的汉化补丁、中文补丁资源下载合集`
  })
} else {
  useKunDisableSeo('会社详情')
}
</script>

<template>
  <div class="container mx-auto my-6">
    <KunLoading v-if="pending && !official" description="加载中..." />

    <KunNull v-else-if="!official" description="会社加载失败，请稍后重试" />

    <template v-else>
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <KunImage
            v-if="logoSrc"
            :src="logoSrc"
            :alt="official.name"
            object-fit="contain"
            loading="eager"
            class-name="border-default/20 bg-content1 size-14 shrink-0 rounded-lg border p-1"
          />
          <h1 class="text-2xl font-bold sm:text-3xl">{{ official.name }}</h1>
          <KunChip color="success" variant="flat" size="sm">
            {{ CATEGORY_LABEL[official.category] ?? official.category }}
          </KunChip>
          <KunChip color="default" size="sm">
            {{ official.galgame_count ?? 0 }} 个 Galgame
          </KunChip>
          <KunChip
            v-if="official.imprint_galgame_count"
            color="secondary"
            variant="flat"
            size="sm"
          >
            自有 {{ official.own_galgame_count }} · 经旗下
            {{ official.imprint_galgame_count }}
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

      <section class="mt-6">
        <div class="mb-4 flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-xl font-bold">由此会社发布的 Galgame</h2>
        </div>

        <KunNull v-if="!galgames.length" description="暂无关联作品" />

        <GalgameList v-else :items="galgames" />

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
