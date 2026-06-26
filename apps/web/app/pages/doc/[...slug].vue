<script setup lang="ts">
import { useContentBlurUp } from '@kungal/ui-vue'

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

// ThumbHash blur-up for the doc body images (KunUI decodes the data-thumbhash
// the API emits on each <img>; width/height reserve the aspect ratio).
const contentEl = ref<HTMLElement | null>(null)
useContentBlurUp(contentEl)

// Use useFetch for both endpoints — Nuxt's payload mechanism transfers the
// SSR-rendered data to the client without a refetch-on-mount window where the
// reactive ref briefly flips to null/empty (which is what makes the TOC vanish
// after hydration when wrapping useApi in useAsyncData).
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
  throw createError({
    statusCode: 404,
    statusMessage: '文章未找到',
    fatal: true
  })
}

// detail nullness is already handled by the createError(404) above — if we
// reach here we have frontmatter. About posts are project-authored mdx
// content (intro / changelog / FAQ / feedback) — fully SEO-safe.
useKunSeoMeta({
  title: detail.value.frontmatter.title,
  description:
    detail.value.frontmatter.description ||
    `${detail.value.frontmatter.title} - 鲲 Galgame 补丁站`,
  ogType: 'article',
  ogImage: (detail.value.frontmatter as { banner?: string }).banner || undefined
})

// Sidebar uses the tree fetched by /doc/posts. Cached under the same key so
// it shares the response with /about index.
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

// detail.html is server-rendered via markdown.RenderWithTOC (goldmark, no
// html.WithUnsafe → already sanitized at the source: raw HTML escaped,
// dangerous URLs dropped). Bound directly; no client-side sanitizer.
const html = computed(() => detail.value?.html ?? '')

const toc = computed<KunTOCItem[]>(() => detail.value?.toc ?? [])
</script>

<template>
  <div
    v-if="detail"
    class="grid w-full gap-6 py-6 lg:grid-cols-[16rem_minmax(0,1fr)_16rem]"
  >
    <aside
      class="hidden lg:sticky lg:top-20 lg:block lg:self-start lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto"
    >
      <AboutSidebar :tree="tree" :active-slug="slugParam" />
    </aside>

    <article class="min-w-0">
      <AboutBlogHeader :frontmatter="detail.frontmatter" />
      <div ref="contentEl" class="kun-prose mt-6" v-html="html" />
      <AboutNavigation :prev="detail.prev" :next="detail.next" />
    </article>

    <aside
      class="hidden lg:sticky lg:top-20 lg:block lg:self-start lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto"
    >
      <AboutTableOfContents :items="toc" />
    </aside>
  </div>
</template>

<style>
.kun-prose {
  line-height: 1.75;
}
.kun-prose h1,
.kun-prose h2,
.kun-prose h3,
.kun-prose h4 {
  margin-top: 1.5em;
  margin-bottom: 0.6em;
  font-weight: 700;
  scroll-margin-top: 6rem;
}
.kun-prose h1 {
  font-size: 1.875rem;
}
.kun-prose h2 {
  font-size: 1.5rem;
}
.kun-prose h3 {
  font-size: 1.25rem;
}
.kun-prose p {
  margin: 1em 0;
}
.kun-prose a {
  color: var(--color-primary);
  text-decoration: underline;
}
.kun-prose .kun-mention {
  color: var(--color-primary);
  font-weight: 500;
  text-decoration: none;
}
.kun-prose ul,
.kun-prose ol {
  margin: 1em 0;
  padding-left: 1.5rem;
}
.kun-prose ul {
  list-style: disc;
}
.kun-prose ol {
  list-style: decimal;
}
.kun-prose code {
  padding: 0.15em 0.4em;
  border-radius: 0.25rem;
  background: color-mix(in oklab, var(--color-default) 20%, transparent);
  font-size: 0.9em;
}
.kun-prose pre {
  margin: 1em 0;
  padding: 1em;
  border-radius: 0.5rem;
  background: color-mix(in oklab, var(--color-default) 15%, transparent);
  overflow: auto;
}
.kun-prose pre code {
  background: none;
  padding: 0;
}
.kun-prose blockquote {
  margin: 1em 0;
  padding: 0.5em 1em;
  border-left: 3px solid var(--color-primary);
  color: var(--color-foreground);
  opacity: 0.85;
}
.kun-prose img {
  margin: 1em auto;
  max-width: 100%;
  border-radius: 0.5rem;
}
.kun-prose table {
  border-collapse: collapse;
  margin: 1em 0;
  width: 100%;
}
.kun-prose th,
.kun-prose td {
  padding: 0.5rem 0.75rem;
  border: 1px solid color-mix(in oklab, var(--color-default) 30%, transparent);
}
</style>
