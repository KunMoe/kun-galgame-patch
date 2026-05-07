<script setup lang="ts">
useKunSeoMeta({
  title: '关于我们',
  description: '鲲 Galgame 补丁的关于页面、公告与帮助文档'
})

// /about/posts returns both the flat list (rendered as cards on this page) and
// the directory tree (consumed by the sidebar) — see
// apps/api/internal/about/handler.ListPosts.
//
// We use `useFetch` rather than wrapping `useApi` in `useAsyncData` because
// `useFetch` has first-class SSR payload transfer: the data hydrates in place
// instead of going through a refetch-on-mount cycle that can momentarily
// flicker the sidebar to empty.
interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

const config = useRuntimeConfig()
const baseUrl = config.public.apiBase as string

const { data: response } = await useFetch<ApiEnvelope<KunPostsResponse>>(
  `${baseUrl}/about/posts`,
  {
    key: 'about-posts',
    credentials: 'include',
    // No reactive sources to watch — this list is keyed by URL only.
    watch: false
  }
)

const emptyTree: KunTreeNode = {
  name: 'about',
  label: '关于我们',
  path: '',
  type: 'directory',
  children: []
}

const posts = computed(() =>
  response.value?.code === 0 ? response.value.data.items : []
)
const tree = computed(() =>
  response.value?.code === 0 ? response.value.data.tree : emptyTree
)
</script>

<template>
  <div class="grid w-full gap-6 px-4 py-6 lg:grid-cols-[16rem_minmax(0,1fr)]">
    <aside
      class="hidden lg:sticky lg:top-20 lg:block lg:self-start lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto"
    >
      <AboutSidebar :tree="tree" />
    </aside>

    <section class="space-y-6">
      <AboutHeader />

      <div
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4"
      >
        <AboutCard v-for="post in posts" :key="post.slug" :post="post" />
      </div>

      <KunNull v-if="!posts.length" description="暂无文章" />
    </section>
  </div>
</template>
