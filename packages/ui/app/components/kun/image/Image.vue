<script setup lang="ts">
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
withDefaults(defineProps<KunImageProps>(), {
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
  decoding: undefined
})
</script>

<template>
  <NuxtImg
    :class="cn('', className)"
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
  />
</template>
