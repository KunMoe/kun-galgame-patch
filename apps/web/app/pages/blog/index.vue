<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const api = useApi()

useKunSeoMeta({
  title: '博客',
  description:
    '鲲 Galgame 补丁站的博客：站点公告、开发随笔、Galgame 相关文章与更新动态。'
})

const page = ref(Number(route.query.page ?? 1))
const limit = 12

interface ListResponse {
  items: BlogCard[]
  total: number
}

const { data, pending, refresh } = await useAsyncData<ListResponse>(
  'blog-list',
  async () => {
    const res = await api.get<ListResponse>(
      `/blog?page=${page.value}&limit=${limit}`
    )
    return res.code === 0 ? res.data : { items: [], total: 0 }
  },
  { default: () => ({ items: [], total: 0 }) }
)

const totalPages = computed(() => Math.ceil((data.value?.total ?? 0) / limit))

const onChangePage = async (v: number) => {
  page.value = v
  await router.replace({ query: { page: v } })
  await refresh()
  if (import.meta.client) window.scrollTo({ top: 0 })
}

const fmtDate = (d: string) => new Date(d).toLocaleDateString('zh-CN')
</script>

<template>
  <div class="container mx-auto my-4 space-y-6">
    <KunHeader name="博客" description="站点公告 · 开发随笔 · Galgame 文章" />

    <KunLoading v-if="pending" description="加载博客中..." />

    <div
      v-else
      class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"
    >
      <NuxtLink
        v-for="b in data?.items"
        :key="b.id"
        :to="`/blog/${b.id}`"
        class="block"
      >
        <KunCard
          class-name="h-full transition-shadow hover:shadow-lg"
        >
          <img
            v-if="b.banner"
            :src="b.banner"
            :alt="b.title"
            loading="lazy"
            class="mb-3 aspect-video w-full rounded-lg object-cover"
          />
          <h2 class="line-clamp-2 text-lg font-semibold">{{ b.title }}</h2>
          <p
            v-if="b.summary"
            class="text-default-500 mt-1 line-clamp-2 text-sm"
          >
            {{ b.summary }}
          </p>
          <div
            class="text-default-400 mt-3 flex items-center gap-3 text-xs"
          >
            <span>{{ fmtDate(b.created) }}</span>
            <span class="flex items-center gap-1">
              <KunIcon name="lucide:eye" class="size-3.5" />
              {{ b.view }}
            </span>
          </div>
        </KunCard>
      </NuxtLink>
    </div>

    <KunNull
      v-if="!pending && !data?.items?.length"
      description="暂无博客"
    />

    <div v-if="totalPages > 1" class="flex justify-center">
      <KunPagination
        :current-page="page"
        :total-page="totalPages"
        :is-loading="pending"
        @update:current-page="onChangePage"
      />
    </div>
  </div>
</template>
