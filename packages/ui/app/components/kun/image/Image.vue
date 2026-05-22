<script setup lang="ts">
import { computed, ref } from 'vue'

interface KunImageProps {
  src: string
  alt?: string
  loading?: 'lazy' | 'eager'
  placeholder?:
    | string
    | number
    | boolean
    | [w: number, h: number, q?: number | undefined, b?: number | undefined]
    | undefined
  className?: string
  ariaLabel?: string
  format?: string
  quality?: string | number
  width?: string | number
  height?: string | number
  preload?:
    | boolean
    | {
        fetchPriority: 'auto' | 'high' | 'low'
      }
  // ----- v0.5.0 skeleton (loading placeholder) -----
  // Toggles a `bg-default-200 animate-pulse` skeleton background on the
  // <img> element while the image is loading. Cleared on `@load` /
  // `@error`. The skeleton lives on the same <img> (not a wrapper div)
  // to avoid changing the rendered element type — consumers' layouts
  // that assume KunImage renders a single <img> keep working.
  //
  // Requires explicit `width` + `height` (or aspect-ratio via parent)
  // for the <img> to reserve space before the bytes arrive; otherwise
  // the skeleton has zero size and is invisible.
  //
  // Pairs cleanly with NuxtImg's `placeholder` prop (low-quality
  // blurred preview): skeleton shows first → @load fires → skeleton
  // off → placeholder blur fades into sharp. Default true; pass
  // `:skeleton="false"` to opt out (e.g. for tiny icons / decorations
  // where the flash isn't worth it).
  skeleton?: boolean
  // ----- v0.4.6 pass-throughs (moyu feedback) -----
  // Provider switch — pass "none" for pre-optimized static assets
  // (author-time AVIF/WebP) to skip the IPX → sharp round-trip. The
  // `none` provider is registered in KunUI's nuxt.config so this works
  // out of the box for all downstream apps. The `(string & {})` half
  // keeps autocomplete for the two known values while still accepting
  // any provider names a downstream fork registers (cloudinary, etc.).
  //
  // Template-side cast `(provider as 'ipx')` is intentional: @nuxt/
  // image's generated BaseImageProps types `provider` via a generic
  // whose default narrows to ProviderDefaults.provider — which is
  // hardcoded to `"ipx"` only, even when other providers are
  // registered. Runtime accepts any registered provider; the cast
  // tells TS to trust us. Don't remove the cast without verifying
  // @nuxt/image upstream has fixed the type.
  provider?: 'ipx' | 'none' | (string & {})
  // Responsive density hint forwarded to NuxtImg srcset, e.g. "1x 2x".
  densities?: string
  // Responsive sizes attribute, e.g. "sm:100vw md:50vw lg:400px".
  sizes?: string
  // HTML `fetchpriority` attribute. Set "high" on LCP images, "low"
  // on offscreen / below-the-fold. Different from `preload`'s priority
  // (that one controls <link rel=preload>; this controls the <img>
  // request directly).
  fetchpriority?: 'high' | 'low' | 'auto'
  // HTML `decoding` attribute. "async" lets the browser decode off
  // the main thread (good for non-LCP). "sync" forces synchronous
  // decode (use sparingly — blocks paint).
  decoding?: 'sync' | 'async' | 'auto'
}
const props = withDefaults(defineProps<KunImageProps>(), {
  alt: 'image',
  loading: undefined,
  placeholder: undefined,
  className: undefined,
  ariaLabel: undefined,
  format: undefined,
  quality: undefined,
  width: undefined,
  height: undefined,
  preload: undefined,
  provider: undefined,
  densities: undefined,
  sizes: undefined,
  fetchpriority: undefined,
  decoding: undefined,
  skeleton: true
})

// `isLoaded` is set on either successful load OR error — on error we
// still drop the pulse animation so a broken image doesn't shimmer
// forever (the broken-image icon is a better signal than perpetual
// pulse).
const isLoaded = ref(false)
const onLoad = () => {
  isLoaded.value = true
}
const onError = () => {
  isLoaded.value = true
}

const skeletonClass = computed(() =>
  props.skeleton && !isLoaded.value ? 'bg-default-200 animate-pulse' : ''
)
</script>

<template>
  <NuxtImg
    :class="cn(skeletonClass, className)"
    :src="src"
    :alt="alt"
    :loading="loading"
    :placeholder="placeholder"
    :aria-label="ariaLabel"
    :format="format"
    :quality="quality"
    :width="width"
    :height="height"
    :preload="preload"
    :provider="(provider as 'ipx')"
    :densities="densities"
    :sizes="sizes"
    :fetchpriority="fetchpriority"
    :decoding="decoding"
    @load="onLoad"
    @error="onError"
  />
</template>
