<script setup lang="ts">
import { KUN_ICONS } from './icons'

// KunIcon renders a hardcoded inline <svg> from the generated registry
// (./icons.ts) instead of @nuxt/icon. The body is static, build-time data
// copied from @iconify-json (see scripts/generate-icon-list.mjs), so server and
// client emit identical markup with no runtime fetch — no @nuxt/icon, no
// hydration "double load". The `name="collection:icon"` API is unchanged, incl.
// dynamic `:name="var"` bindings (a string-keyed registry is the only thing
// that supports those).
const props = withDefaults(
  defineProps<{
    name?: string
    class?: string
    className?: string
  }>(),
  {
    name: '',
    class: '',
    className: ''
  }
)

const icon = computed(() => (props.name ? KUN_ICONS[props.name] : undefined))

// Dev aid: a name missing from the registry renders nothing — usually a new
// <KunIcon name="..."> added without re-running `npm run icons`, or a name
// computed at runtime (add it to MANUAL_ICONS in the generator).
if (import.meta.dev) {
  watchEffect(() => {
    if (props.name && !icon.value) {
      console.warn(
        `[KunIcon] no hardcoded icon for "${props.name}" — run \`npm run icons\``
      )
    }
  })
}
</script>

<template>
  <!-- v-html body is trusted build-time iconify data (never user input). -->
  <svg
    v-if="icon"
    xmlns="http://www.w3.org/2000/svg"
    :viewBox="icon.v"
    width="1em"
    height="1em"
    aria-hidden="true"
    focusable="false"
    :class="cn('shrink-0 text-inherit', props.class, props.className)"
    v-html="icon.b"
  />
</template>

<style scoped>
/* lucide/fa/spinner bodies paint with fill/stroke="currentColor"; color is an
   inherited property, so the svg's color (from text-* classes) flows to the
   child paths and currentColor resolves. */
svg {
  color: inherit;
}
</style>
