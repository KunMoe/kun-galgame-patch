<script setup lang="ts">
import { computed, ref, type ComponentPublicInstance } from 'vue'
import { useImageLoadingStatus } from '../../../composables/useImageLoadingStatus'

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
  // Renders a sibling skeleton overlay (NOT a class on the <img>)
  // while loading. Pattern is the shadcn/Radix Avatar architecture
  // adapted to Vue: 3-state machine (loading/loaded/error) drives
  // cross-fade between skeleton and image, both sitting at
  // `position: absolute; inset: 0` inside an inline-block wrapper.
  //
  // Why a wrapper + sibling skeleton (and not the old
  // animate-pulse-on-<img>) — see commit message / doc on the
  // v0.5.x bump. Two bugs the old approach had:
  //   - cache race left isLoaded=false forever, so animate-pulse
  //     opacity-cycled the PAINTED image
  //   - bg-default-200 leaked through transparent PNGs
  //
  // Default true; `:skeleton="false"` to opt out (icons / decorative
  // images where the wrapper is unwanted).
  skeleton?: boolean
  // CSS aspect-ratio applied to the wrapper, e.g. "16 / 9" or "1".
  // When set, the image is absolutely positioned and fills the
  // wrapper — useful for responsive cards / banners where you want
  // a reserved box before the image bytes arrive. When omitted, the
  // wrapper sizes to the image's intrinsic dimensions (via width/
  // height props or natural size), preserving the pre-v0.5.0 layout
  // behavior.
  aspectRatio?: string
  // object-fit applied to the rendered <img>. Default `cover` covers
  // both natural-size and aspectRatio modes. Override with `contain`
  // for logos / portraits that shouldn't be cropped.
  objectFit?: 'cover' | 'contain' | 'fill' | 'none' | 'scale-down'
  // Tailwind classes routed to the inner <NuxtImg>. Use this for
  // image-specific overrides (e.g. `grayscale`, `blur-sm`,
  // explicit `object-position`). `className` goes to the wrapper.
  imageClassName?: string
  // ----- v0.4.6 pass-throughs (moyu feedback) -----
  // Provider switch — pass "none" for pre-optimized static assets
  // (author-time AVIF/WebP) to skip the IPX → sharp round-trip. The
  // `none` provider is registered in KunUI's nuxt.config so this works
  // out of the box for all downstream apps. The `(string & {})` half
  // keeps autocomplete for the two known values while still accepting
  // any provider names a downstream fork registers (cloudinary, etc.).
  //
  // Template-side cast `(provider as never)` is intentional. @nuxt/image's
  // generated BaseImageProps types `provider` via a generic that narrows to
  // ProviderDefaults.provider; with KunUI's custom `none` default provider
  // (set in nuxt.config) that generated type resolves to `undefined` in
  // consumer apps, so the real prop type (`'ipx' | 'none' | string`) isn't
  // assignable — an earlier `as 'ipx'` cast failed typecheck for the same
  // reason. Runtime accepts any registered provider regardless; casting to
  // `never` (assignable to whatever the generated prop expects) tells TS to
  // trust us without hardcoding a literal that breaks when the provider
  // config changes. Don't remove without verifying @nuxt/image upstream
  // fixed the generated type.
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
  skeleton: true,
  aspectRatio: undefined,
  objectFit: 'cover',
  imageClassName: undefined
})

const imgEl = ref<ComponentPublicInstance | null>(null)
const srcRef = computed(() => props.src)
const { status, onLoad, onError } = useImageLoadingStatus(imgEl, srcRef)

const objectFitClass = computed(() => {
  switch (props.objectFit) {
    case 'contain':
      return 'object-contain'
    case 'fill':
      return 'object-fill'
    case 'none':
      return 'object-none'
    case 'scale-down':
      return 'object-scale-down'
    case 'cover':
    default:
      return 'object-cover'
  }
})

const wrapperStyle = computed(() =>
  props.aspectRatio ? { aspectRatio: props.aspectRatio } : undefined
)

// When skeleton is off, render bare <NuxtImg> with no wrapper — keeps
// the pre-v0.5.0 single-element DOM shape for consumers who explicitly
// opted out and may have layout assumptions that depend on it.
const wrap = computed(() => props.skeleton)
</script>

<template>
  <NuxtImg
    v-if="!wrap"
    :src="src"
    :alt="alt"
    :loading="loading"
    :placeholder="placeholder"
    :class="cn(className, imageClassName)"
    :aria-label="ariaLabel"
    :format="format"
    :quality="quality"
    :width="width"
    :height="height"
    :preload="preload"
    :provider="(provider as never)"
    :densities="densities"
    :sizes="sizes"
    :fetchpriority="fetchpriority"
    :decoding="decoding"
  />
  <div
    v-else
    :class="
      cn(
        'relative overflow-hidden',
        aspectRatio ? 'block w-full' : 'inline-block',
        className
      )
    "
    :style="wrapperStyle"
  >
    <!-- Sibling skeleton layer. Lives outside the <img> so:
         (a) cache-race or any stuck state animates the OVERLAY, not
             the painted image bytes underneath
         (b) bg-default-200 sits behind transparent PNGs / icons
             without leaking through the rendered pixels
         Cross-fades out on `loaded`; hidden on `error` (broken-image
         icon at the <img> is the better signal than a perpetual
         shimmer). aria-hidden because it's decoration. -->
    <div
      v-if="status !== 'loaded'"
      aria-hidden="true"
      class="pointer-events-none absolute inset-0 transition-opacity duration-300"
      :class="
        status === 'error'
          ? 'opacity-0'
          : 'bg-default-200 animate-pulse opacity-100'
      "
    />
    <NuxtImg
      ref="imgEl"
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
      :provider="(provider as never)"
      :densities="densities"
      :sizes="sizes"
      :fetchpriority="fetchpriority"
      :decoding="decoding"
      :class="
        cn(
          'block size-full transition-opacity duration-300',
          aspectRatio ? 'absolute inset-0' : '',
          objectFitClass,
          status === 'loaded' ? 'opacity-100' : 'opacity-0',
          imageClassName
        )
      "
      @load="onLoad"
      @error="onError"
    />
  </div>
</template>
