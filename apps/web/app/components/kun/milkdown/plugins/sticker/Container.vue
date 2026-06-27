<script setup lang="ts">
import { callCommand } from '@milkdown/kit/utils'
import { insertImageCommand } from '@milkdown/kit/preset/commonmark'
import type { UseEditorReturn } from '@milkdown/vue'
import { stickerArray } from './_stickers'

const props = defineProps<{
  editorInfo: UseEditorReturn
}>()

const selectSticker = (stickerUrl: string) => {
  props.editorInfo.get()?.action(
    callCommand(insertImageCommand.key, {
      src: stickerUrl,
      title: 'Sticker',
      alt: 'Sticker'
    })
  )
}
</script>

<template>
  <!-- Every sticker in one vertically-scrolling grid (no pagination). The square
       cell comes from a wrapper <div>, NOT the <img>: Firefox doesn't honor
       aspect-ratio on a grid-item replaced element, which collapsed every row to
       ~0 height and stacked the stickers. -->
  <div class="grid max-h-72 w-64 grid-cols-5 gap-1 overflow-y-auto p-1">
    <div
      v-for="(sticker, index) in stickerArray"
      :key="index"
      @click="selectSticker(sticker)"
      class="aspect-square cursor-pointer"
    >
      <KunImage
        :src="sticker"
        loading="lazy"
        class="size-full rounded-lg object-contain"
        :alt="`Sticker ${index + 1}`"
        placeholder="/favicon.webp"
      />
    </div>
  </div>
</template>
