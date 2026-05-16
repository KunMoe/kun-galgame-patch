<script setup lang="ts">
// Ported from next-web EmojiStickerPicker.tsx. Two tabs: emoji (inserted at
// caret) and stickers (sent immediately as a message).
import { emojiArray } from '~/constants/emoji'

const emit = defineEmits<{
  emoji: [emoji: string]
  sticker: [url: string]
}>()

const tab = ref<'emoji' | 'sticker'>('emoji')
const stickers = chatStickerArray()
</script>

<template>
  <div class="w-80 sm:w-96">
    <div class="border-default/20 mb-2 flex border-b">
      <button
        type="button"
        class="flex-1 py-2 text-sm transition-colors"
        :class="
          tab === 'emoji'
            ? 'text-primary border-primary border-b-2 font-medium'
            : 'text-default-500'
        "
        @click="tab = 'emoji'"
      >
        表情
      </button>
      <button
        type="button"
        class="flex-1 py-2 text-sm transition-colors"
        :class="
          tab === 'sticker'
            ? 'text-primary border-primary border-b-2 font-medium'
            : 'text-default-500'
        "
        @click="tab = 'sticker'"
      >
        贴纸
      </button>
    </div>

    <div v-show="tab === 'emoji'" class="h-48 overflow-y-auto">
      <div class="grid grid-cols-8 gap-1 p-1">
        <button
          v-for="(e, i) in emojiArray"
          :key="i"
          type="button"
          class="hover:bg-default-200 rounded-md p-1 text-xl transition-colors"
          @click="emit('emoji', e)"
        >
          {{ e }}
        </button>
      </div>
    </div>

    <div v-show="tab === 'sticker'" class="h-48 overflow-y-auto">
      <div class="grid grid-cols-5 gap-2 p-1">
        <button
          v-for="url in stickers"
          :key="url"
          type="button"
          class="hover:bg-default-200 rounded-md p-1 transition-colors"
          @click="emit('sticker', url)"
        >
          <img
            :src="url"
            alt="sticker"
            width="64"
            height="64"
            loading="lazy"
            class="size-16 object-contain"
          />
        </button>
      </div>
    </div>
  </div>
</template>
