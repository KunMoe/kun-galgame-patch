<script setup lang="ts">
// Fixed-width sidebar listing the entire posts tree. Used on both the /doc
// index page and the doc-detail page; the tree itself is fetched once via
// /doc/posts.
interface Props {
  tree: KunTreeNode
  activeSlug?: string
}

const props = withDefaults(defineProps<Props>(), { activeSlug: '' })

// Skip the synthetic "about" root and render its children directly.
const children = computed(() =>
  props.tree.type === 'directory' ? props.tree.children ?? [] : []
)
</script>

<template>
  <!-- The parent <aside> handles sticky positioning + scroll bounds; this
       component is just the styled chrome. Matches the next-web original:
       a right divider (border-r) only, not a boxed card — the sidebar is
       part of the page layout, not a floating panel. -->
  <nav class="border-default-200 pr-3 lg:border-r">
    <NuxtLink
      to="/doc"
      class="hover:bg-default-100 mb-2 flex items-center gap-2 rounded-md px-2 py-2 text-base font-semibold"
    >
      <KunIcon name="lucide:book-open-text" class="text-primary size-5" />
      目录
    </NuxtLink>

    <KunNull
      v-if="!children.length"
      description="暂无文章"
      class="px-2 py-4 text-xs"
    />

    <AboutSideTreeItem
      v-for="(child, i) in children"
      :key="i"
      :node="child"
      :level="0"
      :active-slug="props.activeSlug"
    />
  </nav>
</template>
