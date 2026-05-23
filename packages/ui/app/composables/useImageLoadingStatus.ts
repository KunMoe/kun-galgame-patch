import {
  computed,
  nextTick,
  onMounted,
  ref,
  watch,
  type ComponentPublicInstance,
  type Ref
} from 'vue'

// 3-state image loading machine.
//   loading — bytes not in, or in but not yet decoded
//   loaded  — fully decoded and paintable (intrinsic size > 0)
//   error   — load failed, no src given, or src resolved to a zero-byte image
export type ImageLoadingStatus = 'loading' | 'loaded' | 'error'

type ImgRef =
  | HTMLImageElement
  | ComponentPublicInstance<unknown, unknown, unknown>
  | { $el?: Element }
  | null
  | undefined

const resolveEl = (v: ImgRef): HTMLImageElement | null => {
  if (!v) return null
  if (v instanceof HTMLImageElement) return v
  const el = (v as { $el?: Element }).$el
  return el instanceof HTMLImageElement ? el : null
}

// Adapted from shadcn/Radix `useImageLoadingStatus` (React). Two
// invariants worth preserving across the Vue port:
//
// 1) **Cache-race fix.** When `src` is in HTTP cache the browser may
//    fire `load` synchronously, before the framework wires `@load` —
//    leaving a naive `ref(false)` stuck and the skeleton pulsing over
//    a painted image forever. Radix dodges this by calling its
//    resolver inside `useState`'s initializer (synchronous). Vue has
//    no equivalent for "synchronously inspect a DOM node that the
//    template hasn't created yet," so the same effect is produced by
//    re-reading `.complete` (a) on `onMounted` and (b) after any
//    `src` change. Both moments happen AFTER the `<img>` exists, so
//    cached state is observable. Issue: radix-ui/primitives#2044.
//
// 2) **Use the rendered `<img>`, not a hidden `new Image()`
//    preloader.** Radix's React code can get away with `new Image()`
//    because next/image and friends ship the same byte stream. With
//    `<NuxtImg>` the rendered element negotiates IPX URL / srcset /
//    format — a preloader on the raw `src` would (a) double-fetch and
//    (b) report "loaded" for a URL that doesn't match what the DOM
//    is asking for. Reading status off the actually-rendered element
//    avoids both pitfalls.
//
// `imgEl` accepts either a raw `<img>` ref or a Vue component ref
// whose `$el` is an `<img>` (so `<NuxtImg>` works out of the box).
export const useImageLoadingStatus = (
  imgEl: Ref<ImgRef>,
  src: Ref<string | undefined>
) => {
  // Start in `loading`, not `error`, so SSR-rendered HTML emits the
  // skeleton layer. On client hydration `syncFromDom` corrects to
  // `loaded` (cached) or leaves it as `loading` (in flight). A
  // momentary skeleton-fade-out on hydration is preferable to a
  // skeleton popping IN after hydration (which would happen if we
  // started in `loaded` and downgraded).
  const status = ref<ImageLoadingStatus>(src.value ? 'loading' : 'error')

  const syncFromDom = () => {
    if (!src.value) {
      status.value = 'error'
      return
    }
    const el = resolveEl(imgEl.value)
    if (!el) {
      // Ref not populated yet (pre-mount on parent, or unmounted) —
      // keep current state, the next syncFromDom call will catch up.
      return
    }
    if (!el.complete) {
      status.value = 'loading'
      return
    }
    // `.complete && naturalWidth === 0` = decode failure or broken img.
    // Same heuristic Radix uses.
    status.value = el.naturalWidth === 0 ? 'error' : 'loaded'
  }

  onMounted(syncFromDom)

  // Re-evaluate on every src swap. Optimistic reset to `loading` so
  // consumer's skeleton can re-show during the swap; the nextTick
  // check upgrades to `loaded` immediately if the new src is also
  // cached (e.g. revisiting a thumbnail in a gallery).
  watch(src, (next) => {
    status.value = next ? 'loading' : 'error'
    void nextTick(syncFromDom)
  })

  return {
    status: computed(() => status.value),
    // Handlers consumers wire to `@load` / `@error` on the underlying
    // element. They short-circuit the post-mount path — when the
    // listener does catch the load event we use it directly instead
    // of re-reading `.complete`.
    onLoad: () => {
      status.value = 'loaded'
    },
    onError: () => {
      status.value = 'error'
    }
  }
}
