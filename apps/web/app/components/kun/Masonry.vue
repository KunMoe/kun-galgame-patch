<script setup lang="ts" generic="T">
// KunMasonry — column-balanced grid that places each new item into the
// currently-shortest column. Wraps useMasonryColumns to provide a declarative
// API mirroring the legacy KunMasonryGrid (Next.js) the project shipped with
// pre-Nuxt-rewrite.
//
// Usage:
//   <KunMasonry :items="posts" :col-min-width="256" :gap="24">
//     <template #default="{ item }">
//       <AboutCard :post="item" />
//     </template>
//   </KunMasonry>
//
// Why a wrapper around the composable: the composable hands back T[][] (one
// array per column) and the consumer would otherwise write the v-for /
// grid-template-columns / per-column flex shell on every call site. This
// component centralizes that shell, keeping use sites a single declarative
// element with a scoped slot.
//
// SSR: container width is 0 server-side → colCount=1 → all items in one
// column. After hydration useResizeObserver fires and the layout re-flows;
// the `opacity-0 → opacity-100` transition keyed on `isReady` masks the
// otherwise-visible reflow. Trade-off accepted on purpose — alternatives
// (ClientOnly, sentinel placeholder) hurt either SEO or LCP.
interface Props {
  items: readonly T[]
  /** Min column width in px. Default 256 (matches legacy KunMasonryGrid). */
  colMinWidth?: number
  /** Gap between columns AND between items in a column, in px. Default 24. */
  gap?: number
  /** Override the wrapper class — e.g. add max-width or padding. */
  className?: string
}
const props = withDefaults(defineProps<Props>(), {
  colMinWidth: 256,
  gap: 24,
  className: ''
})

defineSlots<{
  default(props: { item: T; columnIndex: number; itemIndex: number }): unknown
}>()

const containerRef = ref<HTMLElement | null>(null)
const { columns, colCount, isReady } = useMasonryColumns<T>(
  () => props.items,
  {
    containerRef,
    colMinWidth: () => props.colMinWidth,
    gap: () => props.gap
  }
)
</script>

<template>
  <div
    ref="containerRef"
    :class="
      cn(
        'grid w-full transition-opacity duration-300',
        isReady ? 'opacity-100' : 'opacity-0',
        className
      )
    "
    :style="{
      gridTemplateColumns: `repeat(${colCount}, minmax(0, 1fr))`,
      gap: `${props.gap}px`
    }"
  >
    <!-- Each column is a vertical flex stack. Items inside the column inherit
         the same gap so column-axis and row-axis spacing stay symmetric. -->
    <div
      v-for="(col, ci) in columns"
      :key="ci"
      class="flex min-w-0 flex-col"
      :style="{ gap: `${props.gap}px` }"
    >
      <slot
        v-for="(item, ii) in col"
        :item="item"
        :column-index="ci"
        :item-index="ii"
      />
    </div>
  </div>
</template>
