<script setup lang="ts">

interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

const route = useRoute()
const config = useRuntimeConfig()
const baseUrl = (import.meta.server && config.apiBaseSsr
  ? config.apiBaseSsr
  : config.public.apiBase) as string

const slugParam = computed(() => {
  const raw = route.params.slug
  return Array.isArray(raw) ? raw.join('/') : String(raw ?? '')
})


const { data: detailResponse } = await useFetch<ApiEnvelope<KunPostDetail>>(
  `${baseUrl}/doc/post`,
  {
    key: `doc-post-${slugParam.value}`,
    query: { slug: slugParam.value },
    credentials: 'include',
    watch: false
  }
)

const detail = computed<KunPostDetail | null>(() =>
  detailResponse.value?.code === 0 ? detailResponse.value.data : null
)

if (!detail.value) {
  throw kunPageError(404, '文章未找到')
}

useKunSeoMeta({
  title: detail.value.frontmatter.title,
  description:
    detail.value.frontmatter.description ||
    `${detail.value.frontmatter.title} - 鲲 Galgame 补丁站`,
  ogType: 'article',
  ogImage: (detail.value.frontmatter as { banner?: string }).banner || undefined
})

const emptyTree: KunTreeNode = {
  name: 'about',
  label: '关于我们',
  path: '',
  type: 'directory',
  children: []
}
const { data: postsResponse } = await useFetch<ApiEnvelope<KunPostsResponse>>(
  `${baseUrl}/doc/posts`,
  { key: 'doc-posts', credentials: 'include', watch: false }
)
const tree = computed<KunTreeNode>(() =>
  postsResponse.value?.code === 0 ? postsResponse.value.data.tree : emptyTree
)

const html = computed(() => detail.value?.html ?? '')

const toc = computed<KunTOCItem[]>(() => detail.value?.toc ?? [])
</script>

<template>
  <div
    v-if="detail"
    class="grid w-full gap-6 py-6 lg:grid-cols-[16rem_minmax(0,1fr)_16rem]"
  >
    <aside class="hidden lg:sticky lg:top-20 lg:block lg:self-start">
      <KunOverlayScroll class="lg:max-h-[calc(100vh-6rem)]">
        <AboutSidebar :tree="tree" :active-slug="slugParam" />
      </KunOverlayScroll>
    </aside>

    <article class="min-w-0">
      <AboutBlogHeader :frontmatter="detail.frontmatter" />
      <KunContent :content="html" class-name="kun-prose-normal mt-6" />
      <AboutNavigation :prev="detail.prev" :next="detail.next" />
    </article>

    <aside
      class="hidden lg:sticky lg:top-20 lg:block lg:self-start lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto"
    >
      <AboutTableOfContents :items="toc" />
    </aside>
  </div>
</template>
