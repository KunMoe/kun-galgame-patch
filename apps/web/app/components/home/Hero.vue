<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'

// The carousel rotates the PINNED published docs (toggled via /admin/doc),
// newest first by date — served by the Go API GET /doc/pinned. (It used to read
// stale static posts/*.mdx frontmatter, which ignored the DB `pin` flag, so
// pinning a doc had no effect on the home page.)
interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}
interface PinnedDoc {
  title: string
  banner: string
  description: string
  date: string
  slug: string
  category: string
  author_name: string
  author_avatar: string
}

const config = useRuntimeConfig()
// SSR talks to the Go API by its in-cluster name; the browser uses the public
// base (same dual-base pattern as the /doc pages).
const baseUrl = (
  import.meta.server && config.apiBaseSsr
    ? config.apiBaseSsr
    : config.public.apiBase
) as string

const { data } = await useFetch<ApiEnvelope<PinnedDoc[]>>(
  `${baseUrl}/doc/pinned`,
  { key: 'home-carousel-pinned', default: () => ({ code: 0, message: '', data: [] }) }
)

const posts = computed<HomeCarouselMetadata[]>(() =>
  (data.value?.data ?? []).map((d) => ({
    title: d.title,
    banner: d.banner,
    description: d.description,
    date: d.date,
    authorName: d.author_name,
    authorAvatar: d.author_avatar,
    pin: true,
    directory: d.category,
    link: `/doc/${d.slug}`
  }))
)
</script>

<template>
  <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
    <div class="pointer-events-none hidden select-none md:block">
      <!-- aspect-ratio reserves the box pre-load so the sibling carousel
           column (grid stretch + md:h-full) doesn't collapse on slow loads.
           Asset is 1920×1080. -->
      <KunImage
        src="/kungalgame-trans.webp"
        :alt="kunMoyuMoe.titleShort"
        aspect-ratio="16 / 9"
        class-name="rounded-2xl"
      />
    </div>

    <HomeCarousel :posts="posts ?? []" />
  </div>
</template>
