<script lang="ts">
import type { InjectionKey } from 'vue'

// Shape registered by each <KunLightboxGalleryItem>. `id` is a per-Item
// Symbol so two items with identical src/alt are still distinguishable
// (and removal is O(1) by identity, not value-equality).
//
// src/alt are PLAIN fields, not refs — we mutate via watchEffect in Item
// so the parent's reactive items[] sees the change without each consumer
// paying for nested reactivity.
export interface RegisteredItem {
  id: symbol
  src: string
  alt?: string
}

export interface GalleryContext {
  register: (item: RegisteredItem) => () => void
  open: (item: RegisteredItem) => void
}

// Symbol-typed InjectionKey so two nested Galleries can't collide and
// inject always returns the nearest provider's context. Exported from
// the SFC itself (not a sibling .ts file) so Nuxt's component scanner
// doesn't pick it up as a phantom "KunLightboxGalleryKey" component.
export const KunLightboxGalleryKey: InjectionKey<GalleryContext> =
  Symbol('KunLightboxGallery')
</script>

<script setup lang="ts">
import { provide, ref } from 'vue'

// KunLightboxGallery: declarative wrapper around KunLightbox.
//
// Pattern: child <KunLightboxGalleryItem>s register themselves on mount
// via provide/inject. Clicking any item opens the shared lightbox
// (mounted ONCE inside this Gallery) at that item's current index.
// Items unmount → auto-deregister; src/alt prop changes → reactive items
// list stays in sync; v-for reorderings flow through naturally because
// the items array preserves DOM mount order.
//
// Usage (the 90% case):
//
//   <KunLightboxGallery>
//     <KunLightboxGalleryItem
//       v-for="img in covers"
//       :key="img.id"
//       :src="img.full"
//       :alt="img.caption"
//     >
//       <img :src="img.thumb" />
//     </KunLightboxGalleryItem>
//   </KunLightboxGallery>
//
// Multiple <KunLightboxGallery> on the same page do NOT share state —
// each provides its own context, each gets its own underlying lightbox.
//
// For programmatic-open ("from a button click", "from a notification")
// use the lower-level <KunLightbox> directly with your own array.

const items = ref<RegisteredItem[]>([])
const isOpen = ref(false)
const initialIndex = ref(0)

// register: called by each <Item> on mount. Returns an unregister fn
// the Item should call on unmount. Index isn't fixed at registration —
// items.value is the source of truth, and order = mount order, which
// for v-for matches the source array order.
const register = (item: RegisteredItem) => {
  items.value.push(item)
  return () => {
    const i = items.value.findIndex((x) => x.id === item.id)
    if (i >= 0) items.value.splice(i, 1)
  }
}

// open: called when an Item is clicked. Finds the item's current index
// (computed at click time so v-for shuffles don't desync) and opens
// the lightbox there.
const open = (item: RegisteredItem) => {
  const i = items.value.findIndex((x) => x.id === item.id)
  if (i < 0) return
  initialIndex.value = i
  isOpen.value = true
}

provide(KunLightboxGalleryKey, { register, open })
</script>

<template>
  <!-- display:contents wrapper preserves the grid/flex layout the parent
       declares — the Gallery itself is a logical group, not a layout box.
       Children render directly into the parent's formatting context. -->
  <div class="contents">
    <slot />
  </div>

  <!-- Single shared lightbox instance. The images array is derived from
       the registered items, mapped to the {src, alt} shape KunLightbox
       expects. v-model:is-open keeps Gallery state in sync with both
       Item.open() calls and the user's own close gestures. -->
  <KunLightbox
    v-model:is-open="isOpen"
    :images="items"
    :initial-index="initialIndex"
  />
</template>
