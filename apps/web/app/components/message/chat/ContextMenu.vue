<script setup lang="ts">
// Ported from next-web ChatMessageContextMenu.tsx. A fixed-position panel
// anchored at the right-click point: quick-reaction grid, message actions
// (reply / edit / delete), and a read-only breakdown of existing reactions.
import { onClickOutside } from '@vueuse/core'

const props = defineProps<{
  open: boolean
  anchor: { x: number; y: number }
  isOwner: boolean
  reactions: ChatMessageReactionItem[]
}>()

const emit = defineEmits<{
  reply: []
  edit: []
  delete: []
  reaction: [emoji: string]
  close: []
}>()

const commonReactions = [
  '🥰',
  '👍',
  '❤️',
  '🤨',
  '🙄',
  '😎',
  '😱',
  '😭',
  '🔥',
  '🎉'
]

const panel = ref<HTMLElement | null>(null)
onClickOutside(panel, () => {
  if (props.open) emit('close')
})

const onKey = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.open) emit('close')
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))

// Group reactions by emoji for the read-only detail section.
const grouped = computed(() => {
  const acc: Record<string, ChatMessageReactionItem[]> = {}
  for (const r of props.reactions ?? []) {
    ;(acc[r.emoji] ||= []).push(r)
  }
  return Object.entries(acc)
})

// Keep the panel on-screen: clamp so it doesn't overflow the viewport.
const style = computed(() => {
  if (!import.meta.client) return {}
  const pad = 8
  const w = 280
  const x = Math.min(props.anchor.x, window.innerWidth - w - pad)
  const y = Math.min(props.anchor.y, window.innerHeight - 320)
  return { top: `${Math.max(pad, y)}px`, left: `${Math.max(pad, x)}px` }
})

const pick = (emoji: string) => {
  emit('reaction', emoji)
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <Transition name="kun-ctx">
      <div
        v-if="open"
        ref="panel"
        class="kun-ctx-panel bg-content1 shadow-kun-lg fixed z-[200] w-70 origin-top-left rounded-xl p-2"
        :style="style"
      >
      <!-- quick reactions -->
      <div class="grid grid-cols-5 gap-1 pb-2">
        <KunButton
          v-for="e in commonReactions"
          :key="e"
          variant="light"
          color="default"
          size="sm"
          is-icon-only
          rounded="full"
          class-name="text-lg"
          @click="pick(e)"
        >
          {{ e }}
        </KunButton>
      </div>

      <div class="border-default/20 border-t pt-1">
        <p class="text-default-400 px-2 py-1 text-xs">消息操作</p>
        <KunButton
          variant="light"
          color="default"
          size="sm"
          full-width
          rounded="md"
          class-name="justify-start"
          @click="emit('reply'); emit('close')"
        >
          <KunIcon name="lucide:corner-down-right" class="size-4" />
          回复
        </KunButton>
        <KunButton
          v-if="isOwner"
          variant="light"
          color="default"
          size="sm"
          full-width
          rounded="md"
          class-name="justify-start"
          @click="emit('edit'); emit('close')"
        >
          <KunIcon name="lucide:pencil" class="size-4" />
          编辑
        </KunButton>
        <KunButton
          v-if="isOwner"
          variant="light"
          color="danger"
          size="sm"
          full-width
          rounded="md"
          class-name="justify-start"
          @click="emit('delete'); emit('close')"
        >
          <KunIcon name="lucide:trash-2" class="size-4" />
          删除
        </KunButton>
      </div>

      <div
        v-if="grouped.length"
        class="border-default/20 mt-1 border-t pt-1"
      >
        <p class="text-default-400 px-2 py-1 text-xs">回应表情</p>
        <div
          v-for="[emoji, list] in grouped"
          :key="emoji"
          class="flex items-center gap-2 px-2 py-1 text-xs"
        >
          <span>{{ emoji }}</span>
          <span class="text-default-600 truncate">
            {{ list[0]?.user?.name ?? '未知用户' }}
            <span v-if="list.length > 1" class="text-default-500">
              等 {{ list.length - 1 }} 人
            </span>
          </span>
          <span class="text-default-500 ml-auto">{{ list.length }}</span>
        </div>
      </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.kun-ctx-enter-active,
.kun-ctx-leave-active {
  transition:
    opacity 0.14s ease,
    transform 0.14s ease;
}
.kun-ctx-enter-from,
.kun-ctx-leave-to {
  opacity: 0;
  transform: scale(0.92);
}
</style>
