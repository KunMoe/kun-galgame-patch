<script setup lang="ts">
import { editorViewCtx } from '@milkdown/kit/core'
import { emojiArray } from './_isoEmoji'
import type { UseEditorReturn } from '@milkdown/vue'

const props = defineProps<{
  editorInfo: UseEditorReturn
}>()

const selectEmoji = (emoji: string) => {
  props.editorInfo.get()?.action((ctx) => {
    const view = ctx.get(editorViewCtx)
    const { state } = view
    const { tr } = state
    view.dispatch(tr.insertText(emoji, state.selection.from))
  })
}
</script>

<template>
  <!-- Every emoji in one vertically-scrolling grid (no pagination). Cell-filling
       buttons (w-full) so 7 columns fit w-64 exactly — no horizontal overflow. -->
  <div
    class="grid max-h-72 w-64 grid-cols-7 gap-0.5 overflow-x-hidden overflow-y-auto p-1"
  >
    <button
      v-for="(emoji, index) in emojiArray"
      :key="index"
      type="button"
      @click="selectEmoji(emoji)"
      class="hover:bg-default-100 flex aspect-square w-full cursor-pointer items-center justify-center rounded-md text-xl transition-colors"
    >
      {{ emoji }}
    </button>
  </div>
</template>
