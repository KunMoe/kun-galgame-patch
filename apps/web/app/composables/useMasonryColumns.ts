import { useResizeObserver } from '@vueuse/core'
import type { MaybeRefOrGetter, Ref, ComputedRef } from 'vue'

// useMasonryColumns is the core engine behind <KunMasonry>. It splits a flat
// items array into N column buckets sized so each column is at least
// `colMinWidth` wide given the container's current width.
//
// Distribution: each incoming item goes into the currently-shortest column.
// "Shortest" is approximated by **item count**, not real DOM heights — same
// trade-off the legacy KunMasonryGrid (Next.js) shipped with. Works well when
// items have similar heights (the about-page card grid is the target use
// case); when item heights vary wildly the result will be slightly uneven but
// never broken. Measuring real DOM heights would require two passes (render →
// measure → redistribute) which flickers visibly and complicates SSR.
//
// SSR: on the server `useResizeObserver` is a no-op, so `containerWidth`
// stays 0 → `colCount` is 1 → all items end up in a single column. After
// hydration the observer fires once with the real width and the layout
// re-flows. Consumers should fade the wrapper in via `isReady` to mask the
// brief reflow (KunMasonry does this).
//
// Future: when `grid-template-rows: masonry` reaches cross-browser stable
// (Safari 26 has it, Chromium 140+ behind a flag as of mid-2026), the
// component can short-circuit to a pure-CSS path inside an @supports block
// and reduce this composable to a fallback. Keeping the JS path here makes
// today's UX deterministic across browsers.

export interface UseMasonryColumnsOptions {
  /**
   * The wrapping element whose width drives column count. Watch via
   * useResizeObserver; updates whenever the element resizes.
   */
  containerRef: Ref<HTMLElement | null>
  /**
   * Minimum width per column in px. Column count is computed as
   * `floor((containerWidth + gap) / (colMinWidth + gap))`, clamped to ≥ 1.
   * Default 256 to match the legacy KunMasonryGrid.
   */
  colMinWidth?: MaybeRefOrGetter<number>
  /**
   * Gap between columns AND between items inside a column, in px.
   * Default 24.
   */
  gap?: MaybeRefOrGetter<number>
}

export interface UseMasonryColumnsReturn<T> {
  /** Current width of the container in px, 0 before the observer fires. */
  containerWidth: Ref<number>
  /** Resolved column count, ≥ 1. */
  colCount: ComputedRef<number>
  /** N buckets, each holding the items assigned to that column in order. */
  columns: ComputedRef<T[][]>
  /** True after the first resize-observer callback (i.e. width is real). */
  isReady: Ref<boolean>
}

export const useMasonryColumns = <T>(
  items: MaybeRefOrGetter<readonly T[]>,
  opts: UseMasonryColumnsOptions
): UseMasonryColumnsReturn<T> => {
  const containerWidth = ref(0)
  const isReady = ref(false)

  useResizeObserver(opts.containerRef, (entries) => {
    const entry = entries[0]
    if (!entry) return
    containerWidth.value = entry.contentRect.width
    if (!isReady.value) isReady.value = true
  })

  const colCount = computed(() => {
    const w = containerWidth.value
    const min = toValue(opts.colMinWidth) ?? 256
    const g = toValue(opts.gap) ?? 24
    if (w <= 0) return 1
    return Math.max(1, Math.floor((w + g) / (min + g)))
  })

  const columns = computed<T[][]>(() => {
    const n = colCount.value
    const list = toValue(items)
    const cols: T[][] = Array.from({ length: n }, () => [])
    // Pseudo-heights track item count per column — see file header for the
    // rationale (no real DOM measurement, no flicker, simple SSR story).
    const heights = new Array<number>(n).fill(0)
    for (const item of list) {
      let shortest = 0
      let minH = heights[0]!
      for (let i = 1; i < n; i++) {
        if (heights[i]! < minH) {
          minH = heights[i]!
          shortest = i
        }
      }
      cols[shortest]!.push(item)
      heights[shortest]!++
    }
    return cols
  })

  return { containerWidth, colCount, columns, isReady }
}
