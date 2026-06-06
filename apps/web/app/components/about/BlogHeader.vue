<script setup lang="ts">
// Ported from refs/legacy/next-web/components/about/BlogHeader.tsx, layout
// preserved 1:1: banner on top → big tracking-tight title → author row with
// bordered round avatar + name + calendar/date → primary-colored info banner
// for the description.
//
// Differences vs the legacy React version:
//   - KunAvatar takes a KunUser-shaped object; frontmatter only carries
//     author_name/author_avatar/author_uid (snake_case API fields), so we
//     render the avatar as a plain KunImage rounded-full + border instead
//     of constructing a synthetic KunUser. Visually identical.
//   - HeroUI <Alert> → KunInfo color="primary". Same primary tint + flat
//     banner shape.
interface Props {
  frontmatter: KunPostFrontmatter
}

const props = defineProps<Props>()
</script>

<template>
  <header class="w-full space-y-4">
    <!-- Banner — pre-optimized AVIF authored at build time; KunImage
         provider="none" skips the IPX → sharp roundtrip. eager + high
         fetchpriority because this is the LCP element on a post detail
         page. aspect-ratio reserves layout before bytes arrive — 16/9 to
         match the source banners (1920×1080) and the list cards, so the full
         image shows instead of being cropped to a strip. -->
    <div
      v-if="props.frontmatter.banner"
      class="relative w-full overflow-hidden rounded-xl"
    >
      <KunImage
        :src="props.frontmatter.banner"
        :alt="props.frontmatter.title"
        provider="none"
        loading="eager"
        fetchpriority="high"
        aspect-ratio="16 / 9"
        class-name="w-full"
      />
    </div>

    <h1 class="mt-8 text-2xl font-bold tracking-tight sm:text-4xl">
      {{ props.frontmatter.title }}
    </h1>

    <div class="flex items-center gap-3">
      <KunImage
        v-if="props.frontmatter.author_avatar"
        :src="props.frontmatter.author_avatar"
        :alt="props.frontmatter.author_name"
        :skeleton="false"
        class-name="border-default-200 size-10 shrink-0 rounded-full border-2"
      />
      <div class="flex flex-col gap-1">
        <h2 class="text-sm leading-none font-semibold">
          {{ props.frontmatter.author_name }}
        </h2>
        <div class="text-default-400 flex items-center gap-2 text-sm">
          <KunIcon name="lucide:calendar-days" class="size-4" />
          <p>
            {{
              props.frontmatter.date
                ? formatDate(props.frontmatter.date, {
                    isPrecise: true,
                    isShowYear: true
                  })
                : ''
            }}
          </p>
        </div>
      </div>
    </div>

    <KunInfo
      v-if="props.frontmatter.description"
      color="primary"
      variant="flat"
      :description="props.frontmatter.description"
    />
  </header>
</template>
