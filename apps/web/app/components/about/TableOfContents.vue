<script setup lang="ts">
// Right-rail "本页索引" panel. The TOC entries themselves come pre-baked from
// the backend (apps/api/internal/about/handler.GetPost) so we don't re-parse
// the rendered HTML on the client. We do still attach an IntersectionObserver
// to highlight the heading currently in view.
interface Props {
  items: KunTOCItem[]
}

const props = defineProps<Props>()

const activeId = ref<string>('')

// Smooth-scroll instead of letting `#id` jumps cause an instant page-jump.
const scrollTo = (id: string, e: MouseEvent) => {
  e.preventDefault()
  const el = document.getElementById(id)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  // Reflect in URL so refresh / share preserves the anchor.
  history.replaceState(null, '', `#${id}`)
  activeId.value = id
}

let observer: IntersectionObserver | null = null

const setupObserver = () => {
  if (!import.meta.client || !props.items.length) return
  observer?.disconnect()

  // rootMargin: -80px from top tracks "this heading just scrolled past the
  // top bar"; -65% from the bottom prevents the very last heading staying
  // active when the document is scrolled past it.
  observer = new IntersectionObserver(
    (entries) => {
      // Pick the topmost entry currently intersecting the rootMargin window.
      const visible = entries
        .filter((e) => e.isIntersecting)
        .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (visible.length && visible[0]?.target.id) {
        activeId.value = visible[0].target.id
      }
    },
    { rootMargin: '-80px 0px -65% 0px', threshold: 0 }
  )

  for (const item of props.items) {
    const el = document.getElementById(item.id)
    if (el) observer.observe(el)
  }
}

onMounted(() => {
  setupObserver()
  // If the URL already carries a hash (deep link), use it as the initial
  // active heading.
  if (location.hash) activeId.value = decodeURIComponent(location.hash.slice(1))
})

watch(() => props.items, setupObserver)
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <!-- The parent <aside> handles sticky positioning + scroll bounds. -->
  <nav v-if="props.items.length">
    <h3 class="text-default-500 mb-3 px-2 text-xs font-semibold uppercase">
      本页索引
    </h3>
    <ul class="space-y-1 text-sm">
      <li
        v-for="item in props.items"
        :key="item.id"
        :style="{ paddingLeft: `${(item.level - 1) * 0.75}rem` }"
      >
        <a
          :href="`#${item.id}`"
          :class="[
            'hover:text-primary block rounded px-2 py-1 transition-colors',
            activeId === item.id
              ? 'text-primary font-medium'
              : 'text-default-600'
          ]"
          @click="scrollTo(item.id, $event)"
        >
          {{ item.text }}
        </a>
      </li>
    </ul>
  </nav>
</template>
