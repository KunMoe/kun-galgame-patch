<script setup lang="ts">
useKunSeoMeta({
  title: '文档',
  description:
    '鲲 Galgame 补丁站的项目介绍、团队成员、公告、更新日志、用户协议、使用帮助、隐私政策、反馈与联系方式。'
})

// /doc/posts returns both the flat list (rendered as cards on this page) and
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
const baseUrl = (import.meta.server && config.apiBaseSsr
  ? config.apiBaseSsr
  : config.public.apiBase) as string

const { data: response } = await useFetch<ApiEnvelope<KunPostsResponse>>(
  `${baseUrl}/doc/posts`,
  {
    key: 'doc-posts',
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
  <div class="grid w-full gap-6 py-6 lg:grid-cols-[16rem_minmax(0,1fr)]">
    <aside
      class="hidden lg:sticky lg:top-20 lg:block lg:self-start lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto"
    >
      <AboutSidebar :tree="tree" />
    </aside>

    <section class="space-y-6">
      <AboutHeader />

      <!-- Masonry — col-min-width 256 + gap 24 mirrors the legacy
           Next.js KunMasonryGrid so card layout is visually identical to
           the original (1 col mobile → 4 col wide desktop, balanced fill).
           Each card goes into the shortest column at distribution time. -->
      <KunMasonry :items="posts" :col-min-width="256" :gap="24">
        <template #default="{ item }">
          <AboutCard :post="item" />
        </template>
      </KunMasonry>

      <KunNull v-if="!posts.length" description="暂无文章" />
    </section>
  </div>
</template>
