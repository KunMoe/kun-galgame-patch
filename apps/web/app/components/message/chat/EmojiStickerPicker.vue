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
    <KunTab
      v-model="tab"
      :items="[
        { value: 'emoji', textValue: '表情' },
        { value: 'sticker', textValue: '贴纸' }
      ]"
      variant="underlined"
      color="primary"
      size="sm"
      class="mb-2"
    />

    <div v-show="tab === 'emoji'" class="h-48 overflow-y-auto">
      <div class="grid grid-cols-8 gap-1 p-1">
        <KunButton
          v-for="(e, i) in emojiArray"
          :key="i"
          variant="light"
          color="default"
          size="sm"
          is-icon-only
          class-name="text-xl"
          @click="emit('emoji', e)"
        >
          {{ e }}
        </KunButton>
      </div>
    </div>

    <div v-show="tab === 'sticker'" class="h-48 overflow-y-auto">
      <div class="grid grid-cols-5 gap-2 p-1">
        <KunButton
          v-for="url in stickers"
          :key="url"
          variant="light"
          color="default"
          size="sm"
          is-icon-only
          class-name="p-1"
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
        </KunButton>
      </div>
    </div>
  </div>
</template>
