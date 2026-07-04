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
// <KunEditor> / <KunEditorToolbar> / <KunEditorViewSwitch> are auto-imported by the
// @kungal/editor-nuxt layer; useKunEditorAdapters + <KunMarkdownImageDialog> are moyu's.
import type { KunToolbarItem } from '@kungal/editor-vue'

const props = withDefaults(
  defineProps<{
    /** Bound markdown (v-model). */
    modelValue: string
    /** false → image-free editor (galgame 简介). Default true. */
    image?: boolean
    /** UI language for the toolbar labels / placeholders. */
    locale?: string
    /** Placeholder shown while the editor is empty (visual decoration only —
        not part of the document / markdown output). */
    placeholder?: string
  }>(),
  { image: true, locale: 'zh-cn', placeholder: '' }
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

// Built once from the image flag (constant per call site). The adapter closures
// capture the runtime apiBase in setup — safe here (composable runs in setup).
const adapters = useKunEditorAdapters({ image: props.image })

const onUpdate = (value: string) => emit('update:modelValue', value)

// moyu drops the 分栏 (split) view everywhere — it's for long-form desktop
// writing, overkill for comments / intros. Keep 预览(WYSIWYG) + Markdown only.
// Link button, selection-bubble toolbar and doc-mode placeholder stay on their
// (sensible) defaults.
const editorViews: ('wysiwyg' | 'source')[] = ['wysiwyg', 'source']

// The KunUI toolbar WITHOUT its built-in image button — moyu supplies its own
// <KunMarkdownImageDialog> beside it (paste-URL / multi-upload / drag / persisted
// history). 'image' is dropped so there's no duplicate; the dialog only renders
// when an uploadImage adapter exists (not the image-free 简介 editor). Pasting /
// dropping straight into the editor still auto-uploads via the adapter.
const toolbarItems: KunToolbarItem[] = [
  'heading',
  '|',
  'bold',
  'italic',
  'strike',
  'code',
  'link',
  '|',
  'bulletList',
  'orderedList',
  'quote',
  'codeBlock',
  'hr',
  '|',
  'spoiler',
  '|',
  'picker'
]
</script>

<template>
  <KunEditor
    :model-value="modelValue"
    :adapters="adapters"
    :locale="locale"
    :placeholder="placeholder"
    :views="editorViews"
    @update:model-value="onUpdate"
  >
    <!-- Preview/Markdown switch as a real KunUI <KunTab variant="underlined">
         (editor-nuxt's <KunEditorViewSwitch>), replacing the headless default. -->
    <template #view-switch="s">
      <KunEditorViewSwitch v-bind="s" />
    </template>
    <!-- Fixed toolbar (built-in image button removed) + moyu's own image dialog. -->
    <template #toolbar="api">
      <div class="flex flex-wrap items-center gap-0.5">
        <KunEditorToolbar v-bind="api" :items="toolbarItems" />
        <template v-if="api.adapters.uploadImage">
          <span class="bg-default-200 mx-1 h-5 w-px" aria-hidden="true" />
          <KunMarkdownImageDialog :api="api" :upload="api.adapters.uploadImage!" />
        </template>
      </div>
    </template>
  </KunEditor>
</template>
