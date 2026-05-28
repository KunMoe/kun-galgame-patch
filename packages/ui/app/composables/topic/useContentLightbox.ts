import type { Ref } from 'vue'

interface ImageItem {
  src: string
  alt?: string
}

// useContentLightbox — make every <img> inside a rendered-markdown
// container open in KunLightbox, via EVENT DELEGATION.
//
// Sibling to useSpoilerContent: both take the prose container ref and
// inject a DOM-level interaction onto the v-html output. KunContent wires
// this internally so any `<KunContent :content>` gets clickable images for
// free — but it's also usable standalone for hand-rolled
// `<div ref="x" class="kun-prose" v-html>` blocks that can't migrate to
// KunContent yet (pass your own ref, place a <KunLightbox> bound to the
// returned refs).
//
// Why delegation (not per-image listeners / not <KunLightboxGalleryItem>):
//   - v-html output is raw <img> DOM, not Vue components, so you can't
//     wrap each one in <KunLightboxGalleryItem>.
//   - Comment lists paginate / append / re-render: images that appear
//     AFTER mount must still be clickable. A one-time scan misses them.
//     The listener lives on the persistent container and we re-scan its
//     <img> set LIVE on every click, so newly inserted images Just Work
//     with zero re-init.
//
// Scope: the gallery is scoped to THIS container's images (the live
// querySelectorAll runs against containerRef). A comment card's lightbox
// only cycles through that comment's images, not the whole page's.
export const useContentLightbox = (containerRef: Ref<HTMLElement | null>) => {
  const images = ref<ImageItem[]>([])
  const isLightboxOpen = ref(false)
  const currentImageIndex = ref(0)

  const onClick = (e: MouseEvent) => {
    const target = e.target as HTMLElement | null
    if (!target || target.tagName !== 'IMG') return

    const container = containerRef.value
    if (!container) return

    // Live scan — NOT cached. This is what makes pagination / async
    // image loads work: whatever <img> are in the container at click
    // time is the gallery set.
    const imgs = Array.from(container.querySelectorAll('img'))
    const index = imgs.indexOf(target as HTMLImageElement)
    if (index < 0) return

    // Clicking an image means "open the viewer", not "activate the
    // surrounding card / link". Stop the event before it bubbles to an
    // ancestor handler (e.g. a comment-card navigation click).
    e.stopPropagation()
    e.preventDefault()

    images.value = imgs.map((img) => ({
      // getAttribute('src') keeps the author URL; fall back to the
      // resolved .src if the attribute is somehow absent.
      src: img.getAttribute('src') || img.src,
      alt: img.getAttribute('alt') || ''
    }))
    currentImageIndex.value = index
    isLightboxOpen.value = true
  }

  // Bind to the container element itself (not document) — clicks on inner
  // <img> bubble up to it. innerHTML changes (v-html re-render) don't
  // replace the container element, so the listener survives content swaps.
  onMounted(() => {
    containerRef.value?.addEventListener('click', onClick)
  })
  onBeforeUnmount(() => {
    containerRef.value?.removeEventListener('click', onClick)
  })

  return {
    images,
    isLightboxOpen,
    currentImageIndex
  }
}
