<script setup lang="ts">
import { useContentBlurUp } from '@kungal/ui-vue'
// Resource note (rendered markdown). Collapses to `maxHeight` px when the
// content is taller, with a 展开 / 收起 toggle + bottom fade, so a long note
// doesn't dominate the resource list.
//
// It starts clamped (before measurement) so a long note never flashes fully
// open on hydration; after onMounted measures scrollHeight, short notes drop
// the clamp (no button) and long ones keep it.
//
// `html` is the server-rendered note_html — markdown.MustRender output from the
// Go API's goldmark pipeline (no html.WithUnsafe: raw HTML is escaped and
// javascript:/data: URLs are dropped server-side). It is already safe, so it's
// bound directly with no client-side sanitizer.
const props = withDefaults(
  defineProps<{ html: string; maxHeight?: number }>(),
  { maxHeight: 100 }
)

const contentRef = ref<HTMLElement | null>(null)
// ThumbHash blur-up for the note's body images (KunUI decodes the
// data-thumbhash the API now emits on each <img>). Reuses the same container ref.
useContentBlurUp(contentRef)
const measured = ref(false)
const collapsible = ref(false)
const collapsed = ref(true)

// Clamp before measuring (avoids the long-note open→clamp flash) and whenever a
// collapsible note is in its collapsed state.
const clampStyle = computed(() =>
  !measured.value || (collapsible.value && collapsed.value)
    ? { maxHeight: `${props.maxHeight}px` }
    : {}
)
const showFade = computed(() => collapsible.value && collapsed.value)

const measure = () => {
  const el = contentRef.value
  if (!el) return
  // scrollHeight reports full content height even while clamped/overflow-hidden.
  collapsible.value = el.scrollHeight > props.maxHeight + 4
  measured.value = true
}

onMounted(() => nextTick(measure))
watch(
  () => props.html,
  () => {
    measured.value = false
    collapsed.value = true
    nextTick(measure)
  }
)
</script>

<template>
  <div class="border-default/15 bg-default-50 rounded-xl border p-3 text-sm">
    <div class="relative">
      <div
        ref="contentRef"
        class="kun-prose overflow-hidden transition-[max-height] duration-300"
        :style="clampStyle"
        v-html="props.html"
      />
      <!-- bottom fade hint while collapsed -->
      <div
        v-if="showFade"
        class="from-default-50 pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t to-transparent"
      />
    </div>

    <button
      v-if="collapsible"
      type="button"
      class="text-primary hover:text-primary-600 mt-2 flex items-center gap-1 text-xs font-medium transition-colors"
      @click="collapsed = !collapsed"
    >
      <KunIcon
        :name="collapsed ? 'lucide:chevron-down' : 'lucide:chevron-up'"
        class="size-3.5"
      />
      {{ collapsed ? '展开' : '收起' }}
    </button>
  </div>
</template>
