<script setup lang="ts">
import type { KunUIColor } from '@kungal/ui-core'

defineOptions({ name: 'tag-detail' })

const route = useRoute()
const router = useRouter()
const browse = useTaxonomyBrowse()

const tagID = computed(() => Number(route.params.id))
const page = computed({
  get: () => Number(route.query.page) || 1,
  set: (v: number) => {
    router.push({ query: { ...route.query, page: v } })
  }
})
const pageHref = usePageHref()
const limit = 24

const KIND_LABEL: Record<string, string> = {
  content: '内容',
  meta: '元信息'
}
const TIER_LABEL: Record<string, string> = {
  core: '核心',
  longtail: '长尾',
  hidden: '隐藏'
}
const TIER_COLOR: Record<string, KunUIColor> = {
  core: 'primary',
  longtail: 'default',
  hidden: 'warning'
}

const VERDICT_NOT_FOUND = 40400

const { data, pending } = await useAsyncData(
  () => `tag-detail-${tagID.value}-${page.value}`,
  async () => {
    const res = await browse.tagDetail(tagID.value, { page: page.value, limit })
    return { code: res.code, detail: res.code === 0 ? res.data : null }
  },
  { watch: [page, tagID] }
)

const notFound = () => kunPageError(404, '标签不存在')

if (data.value?.code === VERDICT_NOT_FOUND) throw notFound()
watch(data, (v) => {
  if (v?.code === VERDICT_NOT_FOUND) showError(notFound())
})

const tag = computed(() => data.value?.detail?.tag ?? null)
const galgames = computed<GalgameCard[]>(
  () => data.value?.detail?.galgames ?? []
)
const total = computed(() => data.value?.detail?.total ?? 0)
const totalPage = computed(() => Math.max(1, Math.ceil(total.value / limit)))

if (tag.value && !tag.value.sexual) {
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

    <KunNull v-else-if="!tag" description="标签加载失败，请稍后重试" />

    <template v-else>
      <section class="border-default/20 rounded-xl border p-5">
        <div class="flex flex-wrap items-center gap-3">
          <h1 class="text-2xl font-bold sm:text-3xl">{{ tag.name }}</h1>
          <KunChip
            :color="TIER_COLOR[tag.tier] ?? 'default'"
            variant="flat"
            size="sm"
          >
            {{ TIER_LABEL[tag.tier] ?? tag.tier }}
          </KunChip>
          <KunChip v-if="tag.kind" color="default" variant="flat" size="sm">
            {{ KIND_LABEL[tag.kind] ?? tag.kind }}
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

      <section class="mt-6">
        <div class="mb-4 flex items-center gap-3">
          <div class="bg-primary h-6 w-1 rounded" />
          <h2 class="text-xl font-bold">包含此标签的 Galgame</h2>
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
