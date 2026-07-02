<script setup lang="ts">
// moyu's single Markdown editor — a thin wrapper over the shared @kungal/editor
// <KunEditor>. It owns the two things every moyu call site needs identically, so
// they aren't repeated at each usage:
//   1. host policy (upload / mention / sticker / notify) via useKunEditorAdapters
//   2. the KunUI toolbar — @kungal/editor-nuxt's <KunEditorToolbar> dropped into
//      <KunEditor>'s #toolbar scoped slot, so the chrome is native KunUI
//      (KunButton / KunIcon / KunTooltip / KunPopover) instead of the headless
//      default toolbar.
//
// Pass `:image="false"` for the galgame 简介 editor — omitting the uploadImage /
// stickerSource adapters is how the editor drops every image affordance.
//
// <KunEditor> and <KunEditorToolbar> are auto-imported by the @kungal/editor-nuxt
// layer; useKunEditorAdapters is a moyu composable (app/composables).
const props = withDefaults(
  defineProps<{
    /** Bound markdown (v-model). */
    modelValue: string
    /** false → image-free editor (galgame 简介). Default true. */
    image?: boolean
    /** UI language for the toolbar labels / placeholders. */
    locale?: string
  }>(),
  { image: true, locale: 'zh-cn' }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

// Built once from the image flag (constant per call site). The adapter closures
// capture the runtime apiBase in setup — safe here (composable runs in setup).
const adapters = useKunEditorAdapters({ image: props.image })

const onUpdate = (value: string) => emit('update:modelValue', value)
</script>

<template>
  <KunEditor
    :model-value="modelValue"
    :adapters="adapters"
    :locale="locale"
    @update:model-value="onUpdate"
  >
    <template #toolbar="api">
      <KunEditorToolbar v-bind="api" />
    </template>
  </KunEditor>
</template>
