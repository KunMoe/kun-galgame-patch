<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

const route = useRoute()
const api = useApi()
const id = computed(() => Number(route.params.id))

const { data: blog } = await useAsyncData<BlogDetail | null>(
  `blog-${id.value}`,
  async () => {
    const res = await api.get<BlogDetail>(`/blog/${id.value}`)
    return res.code === 0 ? res.data : null
  },
  { default: () => null }
)

if (!blog.value) {
  throw createError({ statusCode: 404, statusMessage: '博客不存在', fatal: true })
}

useKunSeoMeta({
  title: blog.value.title,
  description: blog.value.summary || blog.value.title
})

const html = computed(() =>
  DOMPurify.sanitize(blog.value?.content_html ?? '', { ADD_ATTR: ['data-id'] })
)
const fmtDate = (d?: string) => (d ? new Date(d).toLocaleDateString('zh-CN') : '')

// Fire-and-forget view bump (anonymous, client only).
onMounted(() => {
  api.put(`/blog/${id.value}/view`).catch(() => {})
})
</script>

<template>
  <article v-if="blog" class="container mx-auto my-4 max-w-3xl space-y-4">
    <img
      v-if="blog.banner"
      :src="blog.banner"
      :alt="blog.title"
      class="aspect-video w-full rounded-xl object-cover"
    />
    <h1 class="text-2xl font-bold sm:text-3xl">{{ blog.title }}</h1>
    <div class="text-default-400 flex flex-wrap items-center gap-3 text-sm">
      <span v-if="blog.user">{{ blog.user.name }}</span>
      <span>{{ fmtDate(blog.created) }}</span>
      <span class="flex items-center gap-1">
        <KunIcon name="lucide:eye" class="size-4" />
        {{ blog.view }}
      </span>
    </div>
    <div class="kun-prose mt-4" v-html="html" />
  </article>
</template>

<style scoped>
.kun-prose {
  line-height: 1.85;
  word-break: break-word;
}
.kun-prose :deep(h1),
.kun-prose :deep(h2),
.kun-prose :deep(h3),
.kun-prose :deep(h4) {
  font-weight: 700;
  margin: 1.4em 0 0.6em;
  line-height: 1.3;
}
.kun-prose :deep(h1) {
  font-size: 1.6rem;
}
.kun-prose :deep(h2) {
  font-size: 1.35rem;
}
.kun-prose :deep(h3) {
  font-size: 1.15rem;
}
.kun-prose :deep(p) {
  margin: 0.85em 0;
}
.kun-prose :deep(ul),
.kun-prose :deep(ol) {
  margin: 0.85em 0;
  padding-left: 1.5em;
  list-style: revert;
}
.kun-prose :deep(a) {
  color: var(--color-primary, #006fee);
  text-decoration: underline;
}
.kun-prose :deep(img) {
  max-width: 100%;
  border-radius: 0.5rem;
  margin: 0.5em 0;
}
.kun-prose :deep(pre) {
  background: var(--color-default-100, #f4f4f5);
  padding: 1em;
  border-radius: 0.5rem;
  overflow-x: auto;
}
.kun-prose :deep(code) {
  font-family: ui-monospace, monospace;
}
.kun-prose :deep(blockquote) {
  border-left: 3px solid var(--color-default-300, #d4d4d8);
  padding-left: 1em;
  color: var(--color-default-500, #71717a);
  margin: 0.85em 0;
}
</style>
