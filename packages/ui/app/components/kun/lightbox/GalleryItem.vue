<script setup lang="ts">
import { inject, onUnmounted, reactive, watchEffect } from 'vue'
import { KunLightboxGalleryKey } from './Gallery.vue'

// KunLightboxGalleryItem: registers itself with the nearest enclosing
// <KunLightboxGallery> so clicking opens the shared lightbox at this
// item's current position.
//
// Two usage modes:
//
//  1. Auto-wrap (default, 90% of cases) — wrap a thumbnail and the
//     wrapper handles click + keyboard + a11y:
//
//        <KunLightboxGalleryItem :src="full" :alt="caption">
//          <img :src="thumb" />
//        </KunLightboxGalleryItem>
//
//  2. Render-prop (advanced) — use scoped slot `{ open }` to control
//     exactly which element triggers, useful for cards with multiple
//     interactive zones:
//
//        <KunLightboxGalleryItem :src="full" v-slot="{ open }">
//          <div class="card">
//            <img :src="thumb" @click="open" />
//            <button @click="like()">like</button>
//          </div>
//        </KunLightboxGalleryItem>
//
//     With v-slot consumed, pass :wrap="false" to skip the wrapper's
//     own click/role/tabindex (otherwise both the wrapper and your
//     own @click="open" fire).

const props = withDefaults(
  defineProps<{
    src: string
    alt?: string
    // Wrapper element. Default 'span' keeps inline-block flow so the
    // wrapper inherits the slot's box behavior. Pass e.g. 'div' for
    // block-level layouts or 'button' for stricter semantics.
    as?: string
    // Set false when using v-slot="{ open }" + your own @click handler
    // inside the slot — avoids double-firing.
    wrap?: boolean
  }>(),
  { as: 'span', wrap: true }
)

const ctx = inject(KunLightboxGalleryKey)
if (!ctx) {
  throw new Error(
    'KunLightboxGalleryItem must be a child of <KunLightboxGallery>.'
  )
}

// Per-instance identity so two items with identical src don't merge.
// Symbol is the cheapest identity primitive and works as a Map key.
const id = Symbol('KunLightboxGalleryItem')
const item = reactive({ id, src: props.src, alt: props.alt })

// Sync prop → registered item so reactive src/alt changes flow to the
// open lightbox (e.g. consumer swaps URL, alt locale switch).
watchEffect(() => {
  item.src = props.src
  item.alt = props.alt
})

const unregister = ctx.register(item)
onUnmounted(unregister)

const open = () => ctx.open(item)

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    open()
  }
}
</script>

<template>
  <component
    :is="as"
    v-if="wrap"
    role="button"
    tabindex="0"
    :aria-label="alt || '查看大图'"
    class="cursor-zoom-in"
    @click="open"
    @keydown="onKeydown"
  >
    <slot :open="open" />
  </component>

  <!-- wrap=false: render slot as-is, scoped-slot binding `open` is the
       only way to trigger from inside. No wrapper element interferes
       with parent layout, no double-click hazard. -->
  <slot v-else :open="open" />
</template>
